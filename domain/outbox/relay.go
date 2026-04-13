package outbox

import (
	"context"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Relay 事务发件箱中继器.
// 异步轮询数据库中的待发送消息并投递到消息队列.
type Relay struct {
	store     Store
	publisher pubsub.Publisher
	opts      *options

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
}

// NewRelay 创建中继器.
func NewRelay(store Store, publisher pubsub.Publisher, opts ...Option) (*Relay, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	if publisher == nil {
		return nil, ErrNilProducer
	}
	return &Relay{
		store:     store,
		publisher: publisher,
		opts:      applyOptions(opts),
	}, nil
}

// Start 启动中继器.
// 启动轮询和清理两个后台 goroutine.
func (r *Relay) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return ErrRelayAlreadyRunning
	}

	relayCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.running = true

	r.wg.Go(func() { r.pollLoop(relayCtx) })
	r.wg.Go(func() { r.cleanupLoop(relayCtx) })
	r.wg.Go(func() { r.resetStaleLoop(relayCtx) })

	r.logDebug("中继器已启动")
	return nil
}

// Stop 优雅关闭中继器.
func (r *Relay) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return ErrRelayNotRunning
	}
	r.cancel()
	r.running = false
	r.mu.Unlock()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logDebug("中继器已停止")
		return nil
	case <-ctx.Done():
		r.logWarn("中继器关闭超时")
		return ctx.Err()
	}
}

// pollLoop 轮询投递循环.
func (r *Relay) pollLoop(ctx context.Context) {
	// 立即执行首次轮询，避免等待第一个 tick.
	r.poll(ctx)

	ticker := time.NewTicker(r.opts.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.poll(ctx)
		}
	}
}

// poll 单次轮询：拉取 → 投递 → 标记.
// DB 操作使用独立的 context.Background()，确保一旦开始的轮询能原子完成，
// 不受 relay 生命周期 context 取消的影响（生命周期 context 仅控制循环退出）.
func (r *Relay) poll(ctx context.Context) {
	dbCtx := context.Background()

	msgs, err := r.store.FetchPending(dbCtx, r.opts.batchSize)
	if err != nil {
		r.logErrorf("拉取待发送消息失败: %v", err)
		return
	}
	if len(msgs) == 0 {
		return
	}

	var sentIDs []uint64
	for _, msg := range msgs {
		if err := r.send(ctx, msg); err != nil {
			r.logErrorf("发送消息失败 [id:%d topic:%s]: %v", msg.ID, msg.Topic, err)
			if markErr := r.store.MarkFailed(dbCtx, msg.ID, err.Error()); markErr != nil {
				r.logErrorf("标记消息失败状态失败 [id:%d]: %v", msg.ID, markErr)
			}
			continue
		}
		sentIDs = append(sentIDs, msg.ID)
	}

	if len(sentIDs) > 0 {
		if err := r.store.MarkSent(dbCtx, sentIDs); err != nil {
			r.logErrorf("批量标记已发送失败: %v", err)
		}
	}
}

// send 发送单条消息到消息队列.
func (r *Relay) send(ctx context.Context, msg *OutboxMessage) error {
	if msg.RetryCount >= r.opts.maxRetries {
		r.logWarnf("消息已达最大重试次数，标记为死信 [id:%d retries:%d]", msg.ID, msg.RetryCount)
		return ErrMaxRetriesExceeded
	}
	return r.publisher.Publish(ctx, msg.Topic, msg.ToMessage())
}

// resetStaleLoop 定期重置超时的 Processing/Failed 消息.
// 仅重置超过 staleTimeout（默认 5 分钟）的消息，避免干扰正在处理的消息.
func (r *Relay) resetStaleLoop(ctx context.Context) {
	// 重置间隔为 staleTimeout，确保不会过于频繁
	ticker := time.NewTicker(r.opts.staleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := r.store.ResetStale(context.Background(), r.opts.staleTimeout); err != nil {
				r.logErrorf("重置过期消息失败: %v", err)
			} else if n > 0 {
				r.logDebugf("已重置 %d 条过期消息", n)
			}
		}
	}
}

// cleanupLoop 定期清理已发送消息.
func (r *Relay) cleanupLoop(ctx context.Context) {
	cleanup := func() {
		// 使用独立 context，确保清理操作不受 relay 生命周期 context 取消的影响
		before := time.Now().Add(-r.opts.cleanupAge)
		if n, err := r.store.Cleanup(context.Background(), before); err != nil {
			r.logErrorf("清理已发送消息失败: %v", err)
		} else if n > 0 {
			r.logDebugf("已清理 %d 条过期消息", n)
		}
	}

	cleanup()

	ticker := time.NewTicker(r.opts.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

// 日志辅助方法.

func (r *Relay) logger() logger.Logger {
	return r.opts.logger
}

func (r *Relay) logDebug(msg string) {
	if log := r.logger(); log != nil {
		log.Debug("[Outbox] " + msg)
	}
}

func (r *Relay) logDebugf(format string, args ...any) {
	if log := r.logger(); log != nil {
		log.Debugf("[Outbox] "+format, args...)
	}
}

func (r *Relay) logWarn(msg string) {
	if log := r.logger(); log != nil {
		log.Warn("[Outbox] " + msg)
	}
}

func (r *Relay) logWarnf(format string, args ...any) {
	if log := r.logger(); log != nil {
		log.Warnf("[Outbox] "+format, args...)
	}
}

func (r *Relay) logErrorf(format string, args ...any) {
	if log := r.logger(); log != nil {
		log.Errorf("[Outbox] "+format, args...)
	}
}
