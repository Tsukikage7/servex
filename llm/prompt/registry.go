package prompt

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
)

// 注册表错误.
var (
	// ErrNilStore NewRegistry 传入 nil Store.
	ErrNilStore = errors.New("prompt: nil store")
	// ErrNilTemplate Register 传入 nil Template.
	ErrNilTemplate = errors.New("prompt: nil template")
	// ErrEmptyName Register/Get 等方法传入空 name.
	ErrEmptyName = errors.New("prompt: empty name")
	// ErrNotFound 按 name 或 version 未找到.
	ErrNotFound = errors.New("prompt: not found")
	// ErrInvalidWeights SetABWeights 的权重集合不合法例如总和 != 100.
	ErrInvalidWeights = errors.New("prompt: invalid ab weights")
)

// Version 单个版本的元数据与模板快照.
//
// Store 持久化的最小单元；Registry 在内存中缓存 []Version 并按 Active/Weight 做路由决策.
// 跨版本语义：同一个 name 下 Version 号从 1 开始单调递增；最新版本默认自动 Active.
type Version struct {
	// Name 逻辑标识如 "chat.default_system".
	Name string `json:"name" gorm:"primaryKey;size:191"`
	// Version 版本号，从 1 开始，对同一 Name 唯一.
	Version int `json:"version" gorm:"primaryKey"`
	// Role 消息角色持久化需要；Template.Role() 的快照.
	Role llm.Role `json:"role" gorm:"size:32"`
	// Text 原始模板文本持久化需要；Template.Text() 的快照.
	Text string `json:"text" gorm:"type:text"`
	// Active 当前是否为 name 的默认 Active 版本.
	Active bool `json:"active"`
	// Weight AB 权重，0 表示不参与 AB 分流.
	Weight int `json:"weight"`
	// CreatedAt 创建时间UTC.
	CreatedAt time.Time `json:"created_at"`
}

// TableName 返回 GORM 表名.
func (Version) TableName() string { return "prompt_versions" }

// Store 版本持久化接口.
//
// 实现约定：
//   - 方法均应幂等且支持并发调用；Registry 层不保证内部持久化调用不并发
//   - Save：若 (Name, Version) 已存在则覆盖；Registry 严格按 LoadAll 当前最大 Version+1 调用
//   - UpdateFlags：原子更新 (Active, Weight)；若 (Name, Version) 不存在返回 ErrNotFound
//   - LoadAll：返回该 Name 下所有版本按 Version 升序；找不到返回 nil, nil
type Store interface {
	// Save 保存一条 Version 记录新增或按 (Name, Version) 覆盖.
	Save(ctx context.Context, v *Version) error
	// LoadAll 返回指定 Name 下的所有版本.
	LoadAll(ctx context.Context, name string) ([]Version, error)
	// LoadAllNames 列出所有已注册的 Name.
	LoadAllNames(ctx context.Context) ([]string, error)
	// UpdateFlags 更新指定 (Name, Version) 的 Active 与 Weight.
	UpdateFlags(ctx context.Context, name string, version int, active bool, weight int) error
}

// Registry 提示词注册表接口.
//
// 语义：
//   - Register：同名首次注册自动 Active；后续追加版本 N+1，新版本默认 Active，旧版本 Active=false
//   - Get：若设置了 AB 权重所有参与 AB 的版本 Weight>0，按权重随机分流；否则返回 Active 版本
//   - SetActive：切换 Active 版本回滚，同时清空 AB 权重AB 与单一 Active 互斥
//   - SetABWeights：weights 全部为 0 或 nil 则关闭 AB回退 Active；否则校验 sum==100 且所有 key 存在
type Registry interface {
	// Register 注册新版本；返回分配的 version 号从 1 起自增.
	Register(ctx context.Context, name string, tmpl *Template) (int, error)
	// Get 返回 name 的当前 Active 版本若启用 AB，按权重分流.
	Get(ctx context.Context, name string) (*Template, error)
	// GetVersion 返回指定版本的 Template.
	GetVersion(ctx context.Context, name string, version int) (*Template, error)
	// SetActive 切换 name 的 Active 版本回滚，同时关闭 AB 分流.
	SetActive(ctx context.Context, name string, version int) error
	// SetABWeights 设置 AB 权重：map[version]weight，weight 之和必须 == 100.
	// 传 nil 或空 map 关闭 AB，回退到单一 Active.
	SetABWeights(ctx context.Context, name string, weights map[int]int) error
	// List 列出 name 下所有版本.
	List(ctx context.Context, name string) ([]Version, error)
}

// ──────────────────────────────────────────
// MemoryStore
// ──────────────────────────────────────────

// memoryStore 基于内存 map 的 Store 实现，用于测试与单机无持久化场景.
type memoryStore struct {
	mu sync.RWMutex
	// data name → versions按 Version 升序保持.
	data map[string][]Version
}

// NewMemoryStore 创建内存 Store.
func NewMemoryStore() Store {
	return &memoryStore{data: map[string][]Version{}}
}

// Save 保存或覆盖一条 Version 记录.
func (s *memoryStore) Save(_ context.Context, v *Version) error {
	if v == nil {
		return fmt.Errorf("prompt: memory store: nil version")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.data[v.Name]
	for i, existing := range list {
		if existing.Version == v.Version {
			list[i] = *v
			s.data[v.Name] = list
			return nil
		}
	}
	list = append(list, *v)
	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	s.data[v.Name] = list
	return nil
}

// LoadAll 深拷贝返回指定 name 的版本列表.
func (s *memoryStore) LoadAll(_ context.Context, name string) ([]Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src, ok := s.data[name]
	if !ok {
		return nil, nil
	}
	out := make([]Version, len(src))
	copy(out, src)
	return out, nil
}

// LoadAllNames 返回所有已注册的 name.
func (s *memoryStore) LoadAllNames(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// UpdateFlags 更新指定版本的 Active/Weight.
func (s *memoryStore) UpdateFlags(_ context.Context, name string, version int, active bool, weight int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, ok := s.data[name]
	if !ok {
		return ErrNotFound
	}
	for i := range list {
		if list[i].Version == version {
			list[i].Active = active
			list[i].Weight = weight
			return nil
		}
	}
	return ErrNotFound
}

// ──────────────────────────────────────────
// Registry 实现
// ──────────────────────────────────────────

// registryImpl 默认 Registry 实现：Store 为底层持久化，内存 cache 为热路径数据.
type registryImpl struct {
	store Store
	// mu 保护 cache 的读写.
	mu sync.RWMutex
	// cache name → versions始终按 Version 升序.
	cache map[string][]Version
	// rngMu 独立锁保护 rng 的并发调用math/rand/v2 *rand.Rand 非并发安全.
	rngMu sync.Mutex
	// rng 随机源可由 Option 注入以支持可重现测试.
	rng *rand.Rand
}

// RegistryOption Registry 构造选项.
type RegistryOption func(*registryImpl)

// WithRand 注入自定义随机源默认使用 math/rand/v2 的全局源的新实例.
// 用于让 AB 分流在测试中可重现.
func WithRand(r *rand.Rand) RegistryOption {
	return func(reg *registryImpl) { reg.rng = r }
}

// NewRegistry 创建 Registry.
//
// store 不得为 nil. 构造时不会自动从 Store 加载全部数据避免冷启动慢，
// 首次访问某 name 时 lazy load；Register/Set* 会同步写回 Store 与内存 cache.
func NewRegistry(store Store, opts ...RegistryOption) (Registry, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	r := &registryImpl{
		store: store,
		cache: map[string][]Version{},
		rng:   rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0xA5A5A5A5A5A5A5A5)), //nolint:gosec // 非密码学用途，AB 分流只需可分布性
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// load 返回某 name 的版本列表 *快照*优先 cache，miss 时从 Store 加载并填充.
//
// 返回值是底层 cache 切片的浅拷贝元素是 Version 值，调用方读写返回切片不会影响 cache;
// 写路径Register/SetActive/SetABWeights始终构造新切片再整体替换 cache，
// 这样读路径Get/GetVersion/List可以无锁读取自己的快照，避免遍历时触发并发 read/write race.
func (r *registryImpl) load(ctx context.Context, name string) ([]Version, error) {
	r.mu.RLock()
	if list, ok := r.cache[name]; ok {
		snap := make([]Version, len(list))
		copy(snap, list)
		r.mu.RUnlock()
		return snap, nil
	}
	r.mu.RUnlock()

	// Cache miss：加载 Store，写入 cache.
	r.mu.Lock()
	defer r.mu.Unlock()
	// double-check.
	if list, ok := r.cache[name]; ok {
		snap := make([]Version, len(list))
		copy(snap, list)
		return snap, nil
	}
	list, err := r.store.LoadAll(ctx, name)
	if err != nil {
		return nil, err
	}
	// 把 LoadAll 的结果放进 cache；返回值是独立快照.
	r.cache[name] = list
	snap := make([]Version, len(list))
	copy(snap, list)
	return snap, nil
}

// Register 注册新版本.
//
// 并发策略:整个操作在 r.mu 写锁内完成,避免版本号竞争;cache 替换为新切片,
// 不原地修改旧切片,使并发 Get 持有的快照不受影响.
func (r *registryImpl) Register(ctx context.Context, name string, tmpl *Template) (int, error) {
	if name == "" {
		return 0, ErrEmptyName
	}
	if tmpl == nil {
		return 0, ErrNilTemplate
	}

	// 若 cache miss,先从 Store 预热(load 方法会 double-check).
	if _, err := r.load(ctx, name); err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 以 cache 当前快照为准构造新切片,避免原地修改让并发读触发 race.
	cur := r.cache[name]
	newList := make([]Version, len(cur), len(cur)+1)
	for i, v := range cur {
		v.Active = false // 旧版本全部置 Active=false.
		newList[i] = v
	}

	nextVersion := 1
	if len(cur) > 0 {
		nextVersion = cur[len(cur)-1].Version + 1
	}

	newV := Version{
		Name:      name,
		Version:   nextVersion,
		Role:      tmpl.Role(),
		Text:      tmpl.Text(),
		Active:    true, // 新注册的版本默认 Active.
		Weight:    0,
		CreatedAt: time.Now().UTC(),
	}

	// 持久化:旧版本若原本 Active=true,同步写 Store 的 active=false.
	for i := range cur {
		if cur[i].Active {
			if err := r.store.UpdateFlags(ctx, name, cur[i].Version, false, cur[i].Weight); err != nil {
				return 0, fmt.Errorf("prompt: clear old active: %w", err)
			}
		}
	}
	if err := r.store.Save(ctx, &newV); err != nil {
		return 0, fmt.Errorf("prompt: save new version: %w", err)
	}

	newList = append(newList, newV)
	sort.Slice(newList, func(i, j int) bool { return newList[i].Version < newList[j].Version })
	r.cache[name] = newList
	return nextVersion, nil
}

// Get 按 Active 或 AB 权重返回 Template.
func (r *registryImpl) Get(ctx context.Context, name string) (*Template, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	list, err := r.load(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}

	// 若存在任一 Weight > 0 的版本，进入 AB 分流.
	var abCandidates []Version
	var total int
	for _, v := range list {
		if v.Weight > 0 {
			abCandidates = append(abCandidates, v)
			total += v.Weight
		}
	}
	if len(abCandidates) > 0 && total > 0 {
		// rng 用独立锁保护：math/rand/v2 的 *rand.Rand 不是并发安全的,
		// 与 r.mu 解耦可以让 cache 读并发化不受 AB 分流影响.
		r.rngMu.Lock()
		n := r.rng.IntN(total)
		r.rngMu.Unlock()
		acc := 0
		for _, v := range abCandidates {
			acc += v.Weight
			if n < acc {
				return rebuildTemplate(v)
			}
		}
		// 理论不可达；保险返回最后一个.
		return rebuildTemplate(abCandidates[len(abCandidates)-1])
	}

	// 否则返回 Active.
	for _, v := range list {
		if v.Active {
			return rebuildTemplate(v)
		}
	}
	// 若无 Active理论不可达，Register 保障至少一个 Active，返回最新版本.
	return rebuildTemplate(list[len(list)-1])
}

// GetVersion 返回指定版本.
func (r *registryImpl) GetVersion(ctx context.Context, name string, version int) (*Template, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	list, err := r.load(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if v.Version == version {
			return rebuildTemplate(v)
		}
	}
	return nil, ErrNotFound
}

// SetActive 切换 Active 版本；同时关闭 AB.
func (r *registryImpl) SetActive(ctx context.Context, name string, version int) error {
	if name == "" {
		return ErrEmptyName
	}
	// 预热 cache.
	if _, err := r.load(ctx, name); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cur := r.cache[name]
	found := false
	for _, v := range cur {
		if v.Version == version {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}

	// 构造新切片并替换 cache,避免原地修改让并发读触发 race.
	newList := make([]Version, len(cur))
	for i, v := range cur {
		v.Active = v.Version == version
		v.Weight = 0
		newList[i] = v
		if err := r.store.UpdateFlags(ctx, name, v.Version, v.Active, 0); err != nil {
			return fmt.Errorf("prompt: update flags: %w", err)
		}
	}
	r.cache[name] = newList
	return nil
}

// SetABWeights 设置 AB 权重.
func (r *registryImpl) SetABWeights(ctx context.Context, name string, weights map[int]int) error {
	if name == "" {
		return ErrEmptyName
	}
	// 预热 cache.
	if _, err := r.load(ctx, name); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.cache[name]
	if len(cur) == 0 {
		return ErrNotFound
	}

	// nil 或空 map 关闭 AB.
	if len(weights) == 0 {
		newList := make([]Version, len(cur))
		for i, v := range cur {
			if v.Weight != 0 {
				v.Weight = 0
				if err := r.store.UpdateFlags(ctx, name, v.Version, v.Active, 0); err != nil {
					return fmt.Errorf("prompt: clear ab weights: %w", err)
				}
			}
			newList[i] = v
		}
		r.cache[name] = newList
		return nil
	}

	// 校验权重:sum==100,所有 key 必须存在,weight >= 0.
	sum := 0
	exists := map[int]bool{}
	for _, v := range cur {
		exists[v.Version] = true
	}
	for ver, w := range weights {
		if !exists[ver] {
			return fmt.Errorf("%w: version %d not found", ErrInvalidWeights, ver)
		}
		if w < 0 {
			return fmt.Errorf("%w: negative weight for version %d", ErrInvalidWeights, ver)
		}
		sum += w
	}
	if sum != 100 {
		return fmt.Errorf("%w: weights sum=%d (expected 100)", ErrInvalidWeights, sum)
	}

	// 写回:未在 weights 中的版本 Weight=0;构造新切片避免原地修改.
	newList := make([]Version, len(cur))
	for i, v := range cur {
		v.Weight = weights[v.Version]
		newList[i] = v
		if err := r.store.UpdateFlags(ctx, name, v.Version, v.Active, v.Weight); err != nil {
			return fmt.Errorf("prompt: update flags: %w", err)
		}
	}
	r.cache[name] = newList
	return nil
}

// List 列出所有版本深拷贝.
func (r *registryImpl) List(ctx context.Context, name string) ([]Version, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	list, err := r.load(ctx, name)
	if err != nil {
		return nil, err
	}
	out := make([]Version, len(list))
	copy(out, list)
	return out, nil
}

// rebuildTemplate 把 Version 还原为可渲染的 Template.
// Version 的 Text 已在 Register 时经过 New 解析校验；此处再解析一次仍可能失败极端情况数据损坏.
func rebuildTemplate(v Version) (*Template, error) {
	tmpl, err := New(v.Role, v.Text)
	if err != nil {
		return nil, fmt.Errorf("prompt: rebuild template: %w", err)
	}
	return tmpl, nil
}
