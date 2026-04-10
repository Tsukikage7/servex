// Package memory 提供 AI 对话的持久化记忆功能.
//
// 扩展 conversation.Memory 接口，支持内存、Redis 等多种存储后端，
// 以及摘要记忆（SummaryMemory）和实体记忆（EntityMemory）等高级记忆策略.
package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/agent/conversation"
	"github.com/Tsukikage7/servex/storage/cache"
)

// 包级错误变量.
var (
	// ErrSessionNotFound 指定会话 ID 不存在时返回.
	ErrSessionNotFound = errors.New("memory: session not found")
	// ErrNilStore Store 为 nil 时返回.
	ErrNilStore = errors.New("memory: store is nil")
	// ErrNilModel ChatModel 为 nil 时返回.
	ErrNilModel = errors.New("memory: model is nil")
)

// Store 记忆持久化存储接口.
type Store interface {
	// Save 将消息列表和元数据保存到指定会话.
	Save(ctx context.Context, sessionID string, messages []llm.Message, metadata map[string]any) error
	// Load 从指定会话加载消息列表和元数据.
	Load(ctx context.Context, sessionID string) ([]llm.Message, map[string]any, error)
	// Delete 删除指定会话的所有数据.
	Delete(ctx context.Context, sessionID string) error
	// List 列出所有会话 ID.
	List(ctx context.Context) ([]string, error)
}

// =============================================================================
// MemoryStore — 内存实现
// =============================================================================

// memoryEntry 内存存储中单个会话的数据.
type memoryEntry struct {
	messages []llm.Message
	metadata map[string]any
}

// MemoryStore 基于内存的记忆存储，线程安全.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*memoryEntry
}

// NewMemoryStore 创建基于内存的记忆存储.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*memoryEntry),
	}
}

// Save 保存会话消息和元数据.
func (s *MemoryStore) Save(_ context.Context, sessionID string, messages []llm.Message, metadata map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 深拷贝消息切片，防止外部修改.
	msgsCopy := make([]llm.Message, len(messages))
	copy(msgsCopy, messages)

	// 浅拷贝元数据.
	var metaCopy map[string]any
	if metadata != nil {
		metaCopy = make(map[string]any, len(metadata))
		for k, v := range metadata {
			metaCopy[k] = v
		}
	}

	s.sessions[sessionID] = &memoryEntry{
		messages: msgsCopy,
		metadata: metaCopy,
	}
	return nil
}

// Load 加载会话消息和元数据，会话不存在时返回 ErrSessionNotFound.
func (s *MemoryStore) Load(_ context.Context, sessionID string) ([]llm.Message, map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return nil, nil, ErrSessionNotFound
	}

	msgsCopy := make([]llm.Message, len(entry.messages))
	copy(msgsCopy, entry.messages)

	var metaCopy map[string]any
	if entry.metadata != nil {
		metaCopy = make(map[string]any, len(entry.metadata))
		for k, v := range entry.metadata {
			metaCopy[k] = v
		}
	}

	return msgsCopy, metaCopy, nil
}

// Delete 删除指定会话，会话不存在时静默返回.
func (s *MemoryStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

// List 列出所有会话 ID.
func (s *MemoryStore) List(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

// 编译期接口断言.
var _ Store = (*MemoryStore)(nil)

// =============================================================================
// RedisStore — 基于 cache.Cache 的实现
// =============================================================================

const (
	// defaultKeyPrefix Redis Key 默认前缀.
	defaultKeyPrefix = "servex:memory:"
	// defaultTTL 默认过期时间 24 小时.
	defaultTTL = 24 * time.Hour
)

// sessionData 单个会话在缓存中的序列化结构.
type sessionData struct {
	Messages []llm.Message      `json:"messages"`
	Metadata map[string]any     `json:"metadata,omitempty"`
}

// redisStoreOptions RedisStore 可选配置.
type redisStoreOptions struct {
	keyPrefix string
	ttl       time.Duration
}

// StoreOption RedisStore 配置函数.
type StoreOption func(*redisStoreOptions)

// WithKeyPrefix 设置 Redis Key 前缀，默认为 "servex:memory:".
func WithKeyPrefix(prefix string) StoreOption {
	return func(o *redisStoreOptions) { o.keyPrefix = prefix }
}

// WithTTL 设置会话数据在 Redis 中的存活时长，默认 24h.
func WithTTL(ttl time.Duration) StoreOption {
	return func(o *redisStoreOptions) { o.ttl = ttl }
}

// RedisStore 基于 cache.Cache 的记忆存储实现.
// 将消息和元数据序列化为 JSON 存入缓存.
type RedisStore struct {
	cache cache.Cache
	opts  redisStoreOptions
}

// NewRedisStore 创建 RedisStore，c 需实现 cache.Cache 接口.
func NewRedisStore(c cache.Cache, opts ...StoreOption) *RedisStore {
	o := redisStoreOptions{
		keyPrefix: defaultKeyPrefix,
		ttl:       defaultTTL,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &RedisStore{cache: c, opts: o}
}

// sessionKey 生成指定会话的缓存 Key.
func (s *RedisStore) sessionKey(sessionID string) string {
	return s.opts.keyPrefix + sessionID
}

// Save 将消息和元数据序列化后存入缓存，并设置过期时间.
func (s *RedisStore) Save(ctx context.Context, sessionID string, messages []llm.Message, metadata map[string]any) error {
	data := sessionData{
		Messages: messages,
		Metadata: metadata,
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("memory: marshal session data: %w", err)
	}

	key := s.sessionKey(sessionID)
	if err := s.cache.Set(ctx, key, string(jsonBytes), s.opts.ttl); err != nil {
		return fmt.Errorf("memory: cache Set: %w", err)
	}
	return nil
}

// Load 从缓存中读取并反序列化消息和元数据.
func (s *RedisStore) Load(ctx context.Context, sessionID string) ([]llm.Message, map[string]any, error) {
	key := s.sessionKey(sessionID)

	val, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, nil, fmt.Errorf("memory: cache Get: %w", err)
	}
	if val == "" {
		return nil, nil, ErrSessionNotFound
	}

	var data sessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, nil, fmt.Errorf("memory: unmarshal session data: %w", err)
	}

	return data.Messages, data.Metadata, nil
}

// Delete 删除指定会话的缓存 Key.
func (s *RedisStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.cache.Del(ctx, s.sessionKey(sessionID)); err != nil {
		return fmt.Errorf("memory: cache Del: %w", err)
	}
	return nil
}

// List 通过缓存接口列出会话 ID.
// cache.Cache 接口不提供 Scan 能力，因此该方法返回空列表.
// 如需 List 功能，请使用 MemoryStore 或自行维护会话 ID 索引.
func (s *RedisStore) List(_ context.Context) ([]string, error) {
	return nil, nil
}

// 编译期接口断言.
var _ Store = (*RedisStore)(nil)

// =============================================================================
// PersistentMemory — 持久化包装器
// =============================================================================

// PersistentMemory 将任意 conversation.Memory 包装为支持持久化的记忆.
// 内部委托给 inner 进行消息管理，额外提供 Save/Load 方法与存储后端交互.
type PersistentMemory struct {
	inner     conversation.Memory
	store     Store
	sessionID string
}

// NewPersistentMemory 创建持久化记忆包装器.
// inner 不能为 nil，store 不能为 nil，否则 Save/Load 会返回错误.
func NewPersistentMemory(inner conversation.Memory, store Store, sessionID string) *PersistentMemory {
	return &PersistentMemory{
		inner:     inner,
		store:     store,
		sessionID: sessionID,
	}
}

// Add 向内部记忆添加消息.
func (m *PersistentMemory) Add(msg llm.Message) {
	m.inner.Add(msg)
}

// Messages 返回内部记忆中的所有消息.
func (m *PersistentMemory) Messages() []llm.Message {
	return m.inner.Messages()
}

// Clear 清空内部记忆.
func (m *PersistentMemory) Clear() {
	m.inner.Clear()
}

// Save 将当前内存中的消息持久化到存储后端.
func (m *PersistentMemory) Save(ctx context.Context) error {
	if m.store == nil {
		return ErrNilStore
	}
	return m.store.Save(ctx, m.sessionID, m.inner.Messages(), nil)
}

// Load 从存储后端加载消息，并依次添加到内部记忆中.
// 加载前会清空内部记忆，以保证状态一致.
func (m *PersistentMemory) Load(ctx context.Context) error {
	if m.store == nil {
		return ErrNilStore
	}
	messages, _, err := m.store.Load(ctx, m.sessionID)
	if err != nil {
		return err
	}
	m.inner.Clear()
	for _, msg := range messages {
		m.inner.Add(msg)
	}
	return nil
}

// 编译期接口断言.
var _ conversation.Memory = (*PersistentMemory)(nil)

// =============================================================================
// SummaryMemory — 摘要记忆
// =============================================================================

const (
	// defaultMaxMessages 触发摘要的默认最大消息数.
	defaultMaxMessages = 20
	// defaultSummaryPrompt 默认摘要提示词.
	defaultSummaryPrompt = "请对以下对话历史做出简洁摘要，保留关键信息，用于后续对话上下文："
)

// SummaryOption SummaryMemory 配置函数.
type SummaryOption func(*SummaryMemory)

// WithMaxMessages 设置触发摘要的最大消息数，默认 20.
func WithMaxMessages(n int) SummaryOption {
	return func(m *SummaryMemory) {
		if n > 0 {
			m.maxMessages = n
		}
	}
}

// WithSummaryPrompt 设置摘要提示词.
func WithSummaryPrompt(p string) SummaryOption {
	return func(m *SummaryMemory) {
		if p != "" {
			m.summaryPrompt = p
		}
	}
}

// SummaryMemory 摘要记忆：当消息数超过阈值时，自动调用 LLM 对旧消息生成摘要.
// 对话上下文由摘要消息（system）+ 最近消息组成，保持上下文精简.
type SummaryMemory struct {
	mu            sync.Mutex
	model         llm.ChatModel
	messages      []llm.Message
	summary       string // 当前已有的摘要文本，空字符串表示尚无摘要.
	maxMessages   int
	summaryPrompt string
}

// NewSummaryMemory 创建摘要记忆，需传入用于生成摘要的 ChatModel.
func NewSummaryMemory(model llm.ChatModel, opts ...SummaryOption) *SummaryMemory {
	m := &SummaryMemory{
		model:         model,
		maxMessages:   defaultMaxMessages,
		summaryPrompt: defaultSummaryPrompt,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Add 添加消息，若消息总数超过 maxMessages 则触发摘要压缩.
func (m *SummaryMemory) Add(msg llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, msg)
	if len(m.messages) > m.maxMessages {
		// 触发摘要：压缩前半部分消息.
		m.summarizeLocked()
	}
}

// Messages 返回 [摘要系统消息（若存在）] + 当前消息列表.
func (m *SummaryMemory) Messages() []llm.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []llm.Message
	if m.summary != "" {
		result = append(result, llm.SystemMessage("对话摘要："+m.summary))
	}
	result = append(result, m.messages...)
	return result
}

// Clear 清空消息和摘要.
func (m *SummaryMemory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
	m.summary = ""
}

// summarizeLocked 对当前消息列表的前半部分生成摘要，并保留后半部分作为近期消息.
// 调用时必须已持有 m.mu 锁.
func (m *SummaryMemory) summarizeLocked() {
	// 保留最近 maxMessages/2 条消息作为近期上下文.
	keepCount := m.maxMessages / 2
	if keepCount < 1 {
		keepCount = 1
	}
	splitIdx := len(m.messages) - keepCount
	if splitIdx <= 0 {
		return
	}
	toSummarize := m.messages[:splitIdx]
	recent := m.messages[splitIdx:]

	// 构建摘要请求消息列表.
	reqMsgs := make([]llm.Message, 0, len(toSummarize)+1)
	reqMsgs = append(reqMsgs, llm.SystemMessage(m.summaryPrompt))
	reqMsgs = append(reqMsgs, toSummarize...)

	// 调用模型生成摘要（使用 Background，避免在请求 ctx 超时时丢失摘要）.
	resp, err := m.model.Generate(context.Background(), reqMsgs)
	if err != nil {
		// 摘要失败时保持原消息不变，避免数据丢失.
		return
	}

	newSummary := strings.TrimSpace(resp.Message.Content)
	if m.summary != "" {
		// 合并已有摘要.
		newSummary = m.summary + "\n" + newSummary
	}
	m.summary = newSummary

	// 仅保留近期消息.
	kept := make([]llm.Message, len(recent))
	copy(kept, recent)
	m.messages = kept
}

// 编译期接口断言.
var _ conversation.Memory = (*SummaryMemory)(nil)

// =============================================================================
// EntityMemory — 实体记忆
// =============================================================================

// defaultEntityPrompt 实体抽取的默认提示词模板.
const defaultEntityPrompt = `从以下消息中抽取命名实体（人名、地名、组织、概念等），以 JSON 对象返回，格式为 {"实体名": "简短描述"}。若无实体则返回 {}。
消息：`

// EntityOption EntityMemory 配置函数.
type EntityOption func(*EntityMemory)

// WithEntityPrompt 设置实体抽取提示词前缀.
func WithEntityPrompt(p string) EntityOption {
	return func(m *EntityMemory) {
		if p != "" {
			m.entityPrompt = p
		}
	}
}

// EntityMemory 实体记忆：在每次 Add 时自动从消息中抽取命名实体，
// 并在 Messages() 返回时将已知实体作为系统消息注入上下文.
type EntityMemory struct {
	mu           sync.Mutex
	model        llm.ChatModel
	messages     []llm.Message
	entities     map[string]string // 实体名 → 描述
	entityPrompt string
}

// NewEntityMemory 创建实体记忆，需传入用于抽取实体的 ChatModel.
func NewEntityMemory(model llm.ChatModel, opts ...EntityOption) *EntityMemory {
	m := &EntityMemory{
		model:        model,
		entities:     make(map[string]string),
		entityPrompt: defaultEntityPrompt,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Add 添加消息，并异步尝试从该消息中抽取实体.
// 抽取失败时静默忽略，不影响正常对话流程.
func (m *EntityMemory) Add(msg llm.Message) {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()

	// 仅对用户消息和助手消息做实体抽取.
	if msg.Role == llm.RoleUser || msg.Role == llm.RoleAssistant {
		m.extractEntities(msg)
	}
}

// Messages 返回实体上下文系统消息（若存在）+ 所有历史消息.
func (m *EntityMemory) Messages() []llm.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []llm.Message
	if len(m.entities) > 0 {
		result = append(result, llm.SystemMessage("已知实体："+m.buildEntityContext()))
	}
	result = append(result, m.messages...)
	return result
}

// Clear 清空消息和实体库.
func (m *EntityMemory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
	m.entities = make(map[string]string)
}

// Entities 返回当前实体库的快照（仅供调试/测试使用）.
func (m *EntityMemory) Entities() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := make(map[string]string, len(m.entities))
	for k, v := range m.entities {
		snapshot[k] = v
	}
	return snapshot
}

// extractEntities 调用 LLM 从指定消息中抽取实体并更新实体库.
func (m *EntityMemory) extractEntities(msg llm.Message) {
	content := msg.Content
	if content == "" {
		for _, p := range msg.Parts {
			if p.Type == llm.ContentTypeText {
				content += p.Text
			}
		}
	}
	if content == "" {
		return
	}

	reqMsgs := []llm.Message{
		llm.UserMessage(m.entityPrompt + content),
	}

	resp, err := m.model.Generate(context.Background(), reqMsgs)
	if err != nil {
		return
	}

	// 解析模型返回的 JSON.
	var extracted map[string]string
	raw := strings.TrimSpace(resp.Message.Content)
	if err := json.Unmarshal([]byte(raw), &extracted); err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range extracted {
		m.entities[k] = v
	}
}

// buildEntityContext 将实体库序列化为可读字符串，调用时必须已持有 m.mu 锁.
func (m *EntityMemory) buildEntityContext() string {
	parts := make([]string, 0, len(m.entities))
	for name, desc := range m.entities {
		parts = append(parts, name+": "+desc)
	}
	return strings.Join(parts, "; ")
}

// 编译期接口断言.
var _ conversation.Memory = (*EntityMemory)(nil)
