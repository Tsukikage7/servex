package redis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeBucketStore 纯内存 token bucket,忠实模拟 Redis Lua 原子执行.
// 语义对齐 luaScript:使用调用方传入的 nowMs(args[2])做 refill 计算.
//
// 注:fake 故意不引入自己的"当前时间";测试要控制时间请通过 RedisTokenBucket.nowFn
// 注入(setNowFnForTest),保持"client → Lua"契约真实.
type fakeBucketStore struct {
	mu      sync.Mutex
	buckets map[string]*fakeBucket
	lastTTL int64 // 记录最近一次传入的 TTL(秒),用于断言 client→Lua 契约
}

type fakeBucket struct {
	tokens float64
	lastMs int64 // 与 Lua 一致:存毫秒时间戳
}

func newFakeBucketStore() *fakeBucketStore {
	return &fakeBucketStore{
		buckets: make(map[string]*fakeBucket),
	}
}

// runner 生成一个 scriptRunner,按 ARGV 顺序 (rate, capacity, nowMs, n, ttl) 执行.
// 严格使用 args[2] 的 nowMs,与 Lua 脚本等价.
func (f *fakeBucketStore) runner() scriptRunner {
	return func(_ context.Context, keys []string, args ...any) (int64, error) {
		if len(keys) != 1 || len(args) < 5 {
			return 0, errors.New("fakeBucketStore: bad args")
		}
		rate := toFloat(args[0])
		capacity := toFloat(args[1])
		nowMs := toInt64(args[2])
		n := toFloat(args[3])
		ttl := toInt64(args[4])

		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastTTL = ttl

		b, ok := f.buckets[keys[0]]
		if !ok {
			b = &fakeBucket{tokens: capacity, lastMs: nowMs}
			f.buckets[keys[0]] = b
		}
		// refill 基于 client 传入的 nowMs - 存储的 lastMs.
		deltaSec := float64(nowMs-b.lastMs) / 1000.0
		if deltaSec < 0 {
			deltaSec = 0
		}
		b.tokens = math.Min(capacity, b.tokens+deltaSec*rate)
		b.lastMs = nowMs
		if b.tokens >= n {
			b.tokens -= n
			return 1, nil
		}
		return 0, nil
	}
}

// lastReceivedTTL 返回 fake 最近一次接收到的 TTL(秒).
func (f *fakeBucketStore) lastReceivedTTL() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastTTL
}

// tokens 返回当前某个 key 的令牌数,便于断言.
func (f *fakeBucketStore) tokens(key string) float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.buckets[key]; ok {
		return b.tokens
	}
	return -1
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	default:
		// 最后兜底,字符串化再解析(与 go-redis 对 any 的处理一致).
		if f, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
			return f
		}
		return 0
	}
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	default:
		if n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64); err == nil {
			return n
		}
		return 0
	}
}

// newTestBucket 构造 TokenBucket 并注入 fake runner.
// 默认使用真实时钟,若需冻结/前进时间,用 freezeAt/advanceTo 辅助.
func newTestBucket(t *testing.T, rate, capacity float64, store *fakeBucketStore, opts ...Option) *RedisTokenBucket {
	t.Helper()
	tb := NewTokenBucket(nil, rate, capacity, opts...) // client 可为 nil,后续替换 runner
	tb.setScriptRunnerForTest(store.runner())
	return tb
}

// freezeAt 把 bucket 的时钟冻结在给定的 ms,返回一个可写入的 *int64 游标(回调层仅读取).
func freezeAt(tb *RedisTokenBucket, ms int64) *int64 {
	cur := new(int64)
	*cur = ms
	tb.setNowFnForTest(func() int64 { return *cur })
	return cur
}

// --- 用例 ---

func TestTokenBucket_AllowFromFullBucket(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 10, 10, store)
	for i := range 10 {
		if !tb.Allow(context.Background()) {
			t.Fatalf("第 %d 次应当允许", i+1)
		}
	}
	// 满桶已耗尽,紧接下一次应拒绝(忽略微量补充).
	if tb.Allow(context.Background()) {
		t.Error("桶应当已空,但被允许")
	}
}

func TestTokenBucket_DenyWhenEmpty(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 1, 2, store)
	// 冻结客户端时钟,避免 refill 干扰.
	freezeAt(tb, 1_700_000_000_000)
	// 消耗 2 个
	tb.Allow(context.Background())
	tb.Allow(context.Background())
	if tb.Allow(context.Background()) {
		t.Error("桶空仍被允许")
	}
}

func TestTokenBucket_RefillAfterTime(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 10, 10, store) // 10 tokens/sec
	cur := freezeAt(tb, 1_700_000_000_000)

	// 耗光
	for range 10 {
		if !tb.Allow(context.Background()) {
			t.Fatal("初始消耗应允许")
		}
	}
	if tb.Allow(context.Background()) {
		t.Fatal("耗光后不应允许")
	}
	// 推进 1 秒,应补满.
	*cur += 1000
	if !tb.Allow(context.Background()) {
		t.Error("1 秒后应当补足令牌并允许")
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 10, 10, store)
	freezeAt(tb, 1_700_000_000_000) // 冻结时间避免补充

	var allowed, denied atomic.Int32
	var wg sync.WaitGroup
	const N = 100
	for range N {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tb.Allow(context.Background()) {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 10 {
		t.Errorf("期望恰好 10 次允许,得到 %d", allowed.Load())
	}
	if denied.Load() != 90 {
		t.Errorf("期望 90 次拒绝,得到 %d", denied.Load())
	}
}

func TestTokenBucket_DifferentKeysIsolated(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 5, 2, store, WithKeyFunc(func(ctx context.Context) string {
		if v, ok := ctx.Value(testKey{}).(string); ok {
			return v
		}
		return "default"
	}))
	freezeAt(tb, 1_700_000_000_000)

	ctxAlice := context.WithValue(context.Background(), testKey{}, "alice")
	ctxBob := context.WithValue(context.Background(), testKey{}, "bob")

	// alice 用光
	if !tb.Allow(ctxAlice) || !tb.Allow(ctxAlice) {
		t.Fatal("alice 两次应允许")
	}
	if tb.Allow(ctxAlice) {
		t.Fatal("alice 应当耗尽")
	}
	// bob 不受影响
	if !tb.Allow(ctxBob) {
		t.Error("bob 不应受 alice 影响")
	}
}

type testKey struct{}

func TestTokenBucket_AllowN_OverCapacityRejected(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 1, 5, store)
	if tb.AllowN(context.Background(), 10) {
		t.Error("超过桶容量的 n 应被拒绝")
	}
}

func TestTokenBucket_AllowN_NonPositive(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 1, 5, store)
	if tb.AllowN(context.Background(), 0) {
		t.Error("n=0 应拒绝")
	}
	if tb.AllowN(context.Background(), -1) {
		t.Error("n<0 应拒绝")
	}
}

func TestTokenBucket_NilClient(t *testing.T) {
	tb := NewTokenBucket(nil, 10, 10)
	if tb.Allow(context.Background()) {
		t.Error("nil client 必须拒绝(fail-closed)")
	}
	err := tb.Wait(context.Background())
	if err == nil {
		t.Error("nil client 的 Wait 应返回错误")
	}
}

func TestTokenBucket_WaitEventuallyAllowed(t *testing.T) {
	store := newFakeBucketStore()
	// 让时间真实流动(不覆盖 now),验证 Wait 能通过 rate 补充成功.
	tb := newTestBucket(t, 100, 1, store) // 100 tok/s,桶很快会充满

	// 先耗光.
	tb.Allow(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := tb.Wait(ctx); err != nil {
		t.Errorf("Wait 应在 200ms 内成功,得到 %v", err)
	}
}

func TestTokenBucket_WaitContextCanceled(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 1, 1, store)
	// 冻结时间,永远无法补充.
	freezeAt(tb, 1_700_000_000_000)
	tb.Allow(context.Background()) // 耗光

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := tb.Wait(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("期望 DeadlineExceeded,得到 %v", err)
	}
}

func TestDefaultTTLMinimum(t *testing.T) {
	// rate=100, capacity=1 -> 0.02s < 1s -> 期望 1s
	got := defaultTTL(100, 1)
	if got != time.Second {
		t.Errorf("期望 1s,得到 %v", got)
	}
}

// 验证 client → Lua 契约:client 传入的 TTL 能被 Lua(fake runner)正确接收.
func TestTokenBucket_TTLPropagatedToScript(t *testing.T) {
	store := newFakeBucketStore()
	// 用 WithTTL 覆盖默认值.
	tb := newTestBucket(t, 10, 10, store, WithTTL(42*time.Second))
	tb.Allow(context.Background())

	if got := store.lastReceivedTTL(); got != 42 {
		t.Errorf("期望 fake runner 收到 TTL=42(秒),得到 %d", got)
	}
}

// 默认 TTL(2*capacity/rate)也应被 fake runner 观察到;rate=5,capacity=10 → 4s.
func TestTokenBucket_DefaultTTLPropagatedToScript(t *testing.T) {
	store := newFakeBucketStore()
	tb := newTestBucket(t, 5, 10, store) // 期望 TTL=4s
	tb.Allow(context.Background())

	if got := store.lastReceivedTTL(); got != 4 {
		t.Errorf("期望 fake runner 收到 TTL=4(秒),得到 %d", got)
	}
}
