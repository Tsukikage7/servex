package prompt_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/prompt"
)

// ──────────────────────────────────────────
// 构造校验
// ──────────────────────────────────────────

// TestNewRegistry_NilStore 验证 nil Store 返回 ErrNilStore.
func TestNewRegistry_NilStore(t *testing.T) {
	_, err := prompt.NewRegistry(nil)
	if !errors.Is(err, prompt.ErrNilStore) {
		t.Errorf("期望 ErrNilStore，实际=%v", err)
	}
}

// mustRegistry 构造失败立即 fail.
func mustRegistry(t *testing.T, opts ...prompt.RegistryOption) prompt.Registry {
	t.Helper()
	r, err := prompt.NewRegistry(prompt.NewMemoryStore(), opts...)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return r
}

// ──────────────────────────────────────────
// Register
// ──────────────────────────────────────────

// TestRegister_BasicAssignsVersionFromOne 验证首次注册 Version=1、Active=true.
func TestRegister_BasicAssignsVersionFromOne(t *testing.T) {
	r := mustRegistry(t)
	tmpl := prompt.MustNew(llm.RoleSystem, "你好 {{.Name}}")
	v, err := r.Register(context.Background(), "greet", tmpl)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if v != 1 {
		t.Errorf("期望 version=1，实际=%d", v)
	}

	vs, err := r.List(context.Background(), "greet")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("期望 1 个版本，实际=%d", len(vs))
	}
	if !vs[0].Active {
		t.Errorf("首个版本应 Active=true，实际=%v", vs[0])
	}
}

// TestRegister_MultipleVersionsLatestActive 验证多次注册版本递增，最新自动 Active.
func TestRegister_MultipleVersionsLatestActive(t *testing.T) {
	r := mustRegistry(t)
	for i := 0; i < 3; i++ {
		tmpl := prompt.MustNew(llm.RoleSystem, "v"+string(rune('A'+i)))
		v, err := r.Register(context.Background(), "p", tmpl)
		if err != nil {
			t.Fatalf("Register 第 %d 次: %v", i, err)
		}
		if v != i+1 {
			t.Errorf("第 %d 次期望 version=%d，实际=%d", i, i+1, v)
		}
	}
	vs, _ := r.List(context.Background(), "p")
	if len(vs) != 3 {
		t.Fatalf("期望 3 个版本，实际=%d", len(vs))
	}
	// 只有最新一个 Active.
	for _, v := range vs {
		if v.Active != (v.Version == 3) {
			t.Errorf("版本 %d Active 不正确：%v", v.Version, v.Active)
		}
	}
	// Get 应返回最新版本（v.Text=vC）.
	got, err := r.Get(context.Background(), "p")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	msg, _ := got.Render(nil)
	if msg.Content != "vC" {
		t.Errorf("期望 Get 返回最新模板 'vC'，实际=%q", msg.Content)
	}
}

// TestRegister_EmptyName 验证空 name 返回 ErrEmptyName.
func TestRegister_EmptyName(t *testing.T) {
	r := mustRegistry(t)
	_, err := r.Register(context.Background(), "", prompt.MustNew(llm.RoleUser, "x"))
	if !errors.Is(err, prompt.ErrEmptyName) {
		t.Errorf("期望 ErrEmptyName，实际=%v", err)
	}
}

// TestRegister_NilTemplate 验证 nil template 返回 ErrNilTemplate.
func TestRegister_NilTemplate(t *testing.T) {
	r := mustRegistry(t)
	_, err := r.Register(context.Background(), "n", nil)
	if !errors.Is(err, prompt.ErrNilTemplate) {
		t.Errorf("期望 ErrNilTemplate，实际=%v", err)
	}
}

// ──────────────────────────────────────────
// Get / GetVersion
// ──────────────────────────────────────────

// TestGet_NotFound 验证未注册的 name 返回 ErrNotFound.
func TestGet_NotFound(t *testing.T) {
	r := mustRegistry(t)
	_, err := r.Get(context.Background(), "missing")
	if !errors.Is(err, prompt.ErrNotFound) {
		t.Errorf("期望 ErrNotFound，实际=%v", err)
	}
}

// TestGet_EmptyName 验证空 name.
func TestGet_EmptyName(t *testing.T) {
	r := mustRegistry(t)
	_, err := r.Get(context.Background(), "")
	if !errors.Is(err, prompt.ErrEmptyName) {
		t.Errorf("期望 ErrEmptyName，实际=%v", err)
	}
}

// TestGetVersion 验证按版本号取.
func TestGetVersion(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v2"))

	got, err := r.GetVersion(context.Background(), "p", 1)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	msg, _ := got.Render(nil)
	if msg.Content != "v1" {
		t.Errorf("期望 v1，实际=%q", msg.Content)
	}
}

// TestGetVersion_NotFound 验证不存在的 version.
func TestGetVersion_NotFound(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, err := r.GetVersion(context.Background(), "p", 99)
	if !errors.Is(err, prompt.ErrNotFound) {
		t.Errorf("期望 ErrNotFound，实际=%v", err)
	}
}

// TestGet_PreservesRole 验证返回 Template 保留了原 Role.
func TestGet_PreservesRole(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "u", prompt.MustNew(llm.RoleUser, "u-text"))
	got, _ := r.Get(context.Background(), "u")
	msg, _ := got.Render(nil)
	if msg.Role != llm.RoleUser {
		t.Errorf("期望 Role=user，实际=%s", msg.Role)
	}
}

// ──────────────────────────────────────────
// SetActive（回滚）
// ──────────────────────────────────────────

// TestSetActive_Rollback 验证切换 Active 后 Get 返回旧版本.
func TestSetActive_Rollback(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v2"))

	// 回滚到 v1.
	if err := r.SetActive(context.Background(), "p", 1); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	got, _ := r.Get(context.Background(), "p")
	msg, _ := got.Render(nil)
	if msg.Content != "v1" {
		t.Errorf("期望回滚到 v1，实际=%q", msg.Content)
	}

	// 只有 v1 Active.
	vs, _ := r.List(context.Background(), "p")
	for _, v := range vs {
		if v.Active != (v.Version == 1) {
			t.Errorf("版本 %d Active 不正确：%v", v.Version, v.Active)
		}
	}
}

// TestSetActive_NotFound 验证不存在的 version 返回 ErrNotFound.
func TestSetActive_NotFound(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	err := r.SetActive(context.Background(), "p", 99)
	if !errors.Is(err, prompt.ErrNotFound) {
		t.Errorf("期望 ErrNotFound，实际=%v", err)
	}
}

// TestSetActive_ClearsAB 验证 SetActive 清空 AB 权重.
func TestSetActive_ClearsAB(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v2"))
	if err := r.SetABWeights(context.Background(), "p", map[int]int{1: 50, 2: 50}); err != nil {
		t.Fatalf("SetABWeights: %v", err)
	}
	if err := r.SetActive(context.Background(), "p", 1); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	vs, _ := r.List(context.Background(), "p")
	for _, v := range vs {
		if v.Weight != 0 {
			t.Errorf("SetActive 后 AB 权重应清空，版本 %d Weight=%d", v.Version, v.Weight)
		}
	}
}

// ──────────────────────────────────────────
// SetABWeights
// ──────────────────────────────────────────

// TestSetABWeights_InvalidSum 验证权重和不等于 100 返回错误.
func TestSetABWeights_InvalidSum(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v2"))

	cases := []map[int]int{
		{1: 30, 2: 30}, // 60
		{1: 50, 2: 60}, // 110
		{1: 100},       // 100 但只指定一个其实是允许的（见下方测试）——此处测 0
	}
	// 把最后一个改为真正非法：sum=0.
	cases[2] = map[int]int{1: 0, 2: 0}

	for i, w := range cases {
		err := r.SetABWeights(context.Background(), "p", w)
		if !errors.Is(err, prompt.ErrInvalidWeights) {
			t.Errorf("case %d: 期望 ErrInvalidWeights，实际=%v", i, err)
		}
	}
}

// TestSetABWeights_NegativeWeight 验证负权重报错.
func TestSetABWeights_NegativeWeight(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v2"))
	err := r.SetABWeights(context.Background(), "p", map[int]int{1: -10, 2: 110})
	if !errors.Is(err, prompt.ErrInvalidWeights) {
		t.Errorf("期望 ErrInvalidWeights，实际=%v", err)
	}
}

// TestSetABWeights_VersionNotFound 验证引用不存在的版本报错.
func TestSetABWeights_VersionNotFound(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	err := r.SetABWeights(context.Background(), "p", map[int]int{1: 50, 99: 50})
	if !errors.Is(err, prompt.ErrInvalidWeights) {
		t.Errorf("期望 ErrInvalidWeights，实际=%v", err)
	}
}

// TestSetABWeights_NilClearsAB 验证 nil/空 map 关闭 AB.
func TestSetABWeights_NilClearsAB(t *testing.T) {
	r := mustRegistry(t)
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v2"))
	if err := r.SetABWeights(context.Background(), "p", map[int]int{1: 50, 2: 50}); err != nil {
		t.Fatalf("SetABWeights: %v", err)
	}
	// 关闭.
	if err := r.SetABWeights(context.Background(), "p", nil); err != nil {
		t.Fatalf("SetABWeights nil: %v", err)
	}
	vs, _ := r.List(context.Background(), "p")
	for _, v := range vs {
		if v.Weight != 0 {
			t.Errorf("关闭 AB 后 Weight 应为 0，实际 v%d=%d", v.Version, v.Weight)
		}
	}
}

// TestSetABWeights_DistributionApproximate 验证 AB 分流频率接近权重（大样本）.
func TestSetABWeights_DistributionApproximate(t *testing.T) {
	// 使用固定 seed 的随机源让测试可重现.
	rng := rand.New(rand.NewPCG(42, 1024))
	r, err := prompt.NewRegistry(prompt.NewMemoryStore(), prompt.WithRand(rng))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "A"))
	_, _ = r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "B"))

	if err := r.SetABWeights(context.Background(), "p", map[int]int{1: 70, 2: 30}); err != nil {
		t.Fatalf("SetABWeights: %v", err)
	}

	const N = 2000
	counts := map[string]int{"A": 0, "B": 0}
	for i := 0; i < N; i++ {
		got, err := r.Get(context.Background(), "p")
		if err != nil {
			t.Fatalf("Get 第 %d 次: %v", i, err)
		}
		msg, _ := got.Render(nil)
		counts[msg.Content]++
	}
	pctA := float64(counts["A"]) * 100 / float64(N)
	pctB := float64(counts["B"]) * 100 / float64(N)
	// 误差允许 5%.
	if pctA < 65 || pctA > 75 {
		t.Errorf("A 权重 70，实际分布=%.1f%%（允许 65-75）", pctA)
	}
	if pctB < 25 || pctB > 35 {
		t.Errorf("B 权重 30，实际分布=%.1f%%（允许 25-35）", pctB)
	}
}

// TestSetABWeights_NotFound 对未注册 name 设置权重应返回 ErrNotFound.
func TestSetABWeights_NotFound(t *testing.T) {
	r := mustRegistry(t)
	err := r.SetABWeights(context.Background(), "no-such", map[int]int{1: 100})
	if !errors.Is(err, prompt.ErrNotFound) {
		t.Errorf("期望 ErrNotFound，实际=%v", err)
	}
}

// ──────────────────────────────────────────
// List
// ──────────────────────────────────────────

// TestList_EmptyForNotExists 未注册的 name 返回 nil, nil.
func TestList_EmptyForNotExists(t *testing.T) {
	r := mustRegistry(t)
	vs, err := r.List(context.Background(), "missing")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("期望空列表，实际=%d 条", len(vs))
	}
}

// ──────────────────────────────────────────
// MemoryStore
// ──────────────────────────────────────────

// TestMemoryStore_SaveAndLoad 验证 Save/LoadAll 语义.
func TestMemoryStore_SaveAndLoad(t *testing.T) {
	s := prompt.NewMemoryStore()
	v1 := &prompt.Version{Name: "p", Version: 1, Role: llm.RoleSystem, Text: "v1", Active: true}
	v2 := &prompt.Version{Name: "p", Version: 2, Role: llm.RoleSystem, Text: "v2"}
	if err := s.Save(context.Background(), v1); err != nil {
		t.Fatalf("Save v1: %v", err)
	}
	if err := s.Save(context.Background(), v2); err != nil {
		t.Fatalf("Save v2: %v", err)
	}
	list, err := s.LoadAll(context.Background(), "p")
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(list) != 2 || list[0].Version != 1 || list[1].Version != 2 {
		t.Errorf("LoadAll 结果不正确：%+v", list)
	}
}

// TestMemoryStore_UpdateFlags 验证 UpdateFlags.
func TestMemoryStore_UpdateFlags(t *testing.T) {
	s := prompt.NewMemoryStore()
	_ = s.Save(context.Background(), &prompt.Version{Name: "p", Version: 1, Text: "v1", Active: true})
	if err := s.UpdateFlags(context.Background(), "p", 1, false, 50); err != nil {
		t.Fatalf("UpdateFlags: %v", err)
	}
	list, _ := s.LoadAll(context.Background(), "p")
	if list[0].Active || list[0].Weight != 50 {
		t.Errorf("UpdateFlags 未生效：%+v", list[0])
	}
}

// TestMemoryStore_UpdateFlags_NotFound 验证不存在版本返回 ErrNotFound.
func TestMemoryStore_UpdateFlags_NotFound(t *testing.T) {
	s := prompt.NewMemoryStore()
	err := s.UpdateFlags(context.Background(), "p", 1, false, 0)
	if !errors.Is(err, prompt.ErrNotFound) {
		t.Errorf("期望 ErrNotFound，实际=%v", err)
	}
}

// TestMemoryStore_LoadAllNames 验证 LoadAllNames.
func TestMemoryStore_LoadAllNames(t *testing.T) {
	s := prompt.NewMemoryStore()
	_ = s.Save(context.Background(), &prompt.Version{Name: "a", Version: 1, Text: "x"})
	_ = s.Save(context.Background(), &prompt.Version{Name: "b", Version: 1, Text: "y"})
	names, err := s.LoadAllNames(context.Background())
	if err != nil {
		t.Fatalf("LoadAllNames: %v", err)
	}
	if strings.Join(names, ",") != "a,b" {
		t.Errorf("期望 [a, b]，实际=%v", names)
	}
}

// TestMemoryStore_SaveOverwrites 重复 Save 同版本会覆盖.
func TestMemoryStore_SaveOverwrites(t *testing.T) {
	s := prompt.NewMemoryStore()
	_ = s.Save(context.Background(), &prompt.Version{Name: "p", Version: 1, Text: "v1"})
	_ = s.Save(context.Background(), &prompt.Version{Name: "p", Version: 1, Text: "v1.1"})
	list, _ := s.LoadAll(context.Background(), "p")
	if len(list) != 1 || list[0].Text != "v1.1" {
		t.Errorf("期望覆盖结果，实际=%+v", list)
	}
}

// ──────────────────────────────────────────
// Registry 与 Store 的一致性
// ──────────────────────────────────────────

// TestRegistry_LoadsFromExistingStore 验证 Registry 懒加载 Store 已有数据.
func TestRegistry_LoadsFromExistingStore(t *testing.T) {
	store := prompt.NewMemoryStore()
	_ = store.Save(context.Background(), &prompt.Version{
		Name: "p", Version: 1, Role: llm.RoleSystem, Text: "stored", Active: true,
	})
	r, err := prompt.NewRegistry(store)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	got, err := r.Get(context.Background(), "p")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	msg, _ := got.Render(nil)
	if msg.Content != "stored" {
		t.Errorf("期望从 Store 加载 'stored'，实际=%q", msg.Content)
	}
}

// TestRegistry_ConcurrentGetWithAB 并发调用 Get（AB 分流）应不触发 race.
// 配合 `go test -race` 使用.
func TestRegistry_ConcurrentGetWithAB(t *testing.T) {
	r := mustRegistry(t)
	ctx := context.Background()
	_, _ = r.Register(ctx, "p", prompt.MustNew(llm.RoleSystem, "v1"))
	_, _ = r.Register(ctx, "p", prompt.MustNew(llm.RoleSystem, "v2"))
	_, _ = r.Register(ctx, "p", prompt.MustNew(llm.RoleSystem, "v3"))
	if err := r.SetABWeights(ctx, "p", map[int]int{1: 30, 2: 30, 3: 40}); err != nil {
		t.Fatalf("SetABWeights: %v", err)
	}

	const (
		goroutines = 16
		iterations = 200
	)
	errCh := make(chan error, goroutines*iterations)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				tmpl, err := r.Get(ctx, "p")
				if err != nil {
					errCh <- err
					return
				}
				if _, err := tmpl.Render(nil); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("并发 Get 报错: %v", err)
	}
}

// TestRegistry_ConcurrentRegisterAndGet 并发 Register 与 Get 不触发 race.
func TestRegistry_ConcurrentRegisterAndGet(t *testing.T) {
	r := mustRegistry(t)
	ctx := context.Background()
	_, _ = r.Register(ctx, "p", prompt.MustNew(llm.RoleSystem, "v1"))

	const writers, readers = 4, 16
	var wg sync.WaitGroup
	wg.Add(writers + readers)
	// 写端：不断注册新版本.
	for i := 0; i < writers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				tmpl := prompt.MustNew(llm.RoleSystem, "w"+strconv.Itoa(id)+"-"+strconv.Itoa(j))
				if _, err := r.Register(ctx, "p", tmpl); err != nil {
					t.Errorf("Register: %v", err)
					return
				}
			}
		}(i)
	}
	// 读端：不断 Get.
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, err := r.Get(ctx, "p"); err != nil {
					t.Errorf("Get: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestRegister_AppendsToExistingStore Register 追加版本应基于 Store 中已存在的最高版本号.
func TestRegister_AppendsToExistingStore(t *testing.T) {
	store := prompt.NewMemoryStore()
	_ = store.Save(context.Background(), &prompt.Version{
		Name: "p", Version: 5, Role: llm.RoleSystem, Text: "v5", Active: true,
	})
	r, _ := prompt.NewRegistry(store)
	v, err := r.Register(context.Background(), "p", prompt.MustNew(llm.RoleSystem, "v6"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if v != 6 {
		t.Errorf("期望 version=6，实际=%d", v)
	}
}
