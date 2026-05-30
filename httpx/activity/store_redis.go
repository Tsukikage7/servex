package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis 存储实现.
type RedisStore struct {
	client    redis.UniversalClient
	keyPrefix string
	onlineTTL time.Duration
}

// RedisStoreOption Redis 存储配置选项.
type RedisStoreOption func(*RedisStore)

// WithKeyPrefix 设置 Redis key 前缀.
func WithKeyPrefix(prefix string) RedisStoreOption {
	return func(s *RedisStore) {
		s.keyPrefix = prefix
	}
}

// WithOnlineStoreTTL 设置在线状态 TTL.
func WithOnlineStoreTTL(ttl time.Duration) RedisStoreOption {
	return func(s *RedisStore) {
		s.onlineTTL = ttl
	}
}

// NewRedisStore 创建 Redis 存储.
func NewRedisStore(client redis.UniversalClient, opts ...RedisStoreOption) *RedisStore {
	s := &RedisStore{
		client:    client,
		keyPrefix: "activity",
		onlineTTL: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Key 名称规范:
// - activity:last:{user_id}    -> 最后活跃信息 (Hash)
// - activity:online:{user_id}  -> 在线状态 (String, 带 TTL)
// - activity:online:count      -> 在线计数 (HyperLogLog 或 Set)

func (s *RedisStore) lastActiveKey(userID string) string {
	return fmt.Sprintf("%s:last:%s", s.keyPrefix, userID)
}

func (s *RedisStore) onlineKey(userID string) string {
	return fmt.Sprintf("%s:online:%s", s.keyPrefix, userID)
}

func (s *RedisStore) onlineSetKey() string {
	return fmt.Sprintf("%s:online:set", s.keyPrefix)
}

// SetLastActive 设置用户最后活跃时间.
func (s *RedisStore) SetLastActive(ctx context.Context, userID string, event *Event) error {
	key := s.lastActiveKey(userID)

	data := map[string]any{
		"user_id":        userID,
		"last_active_at": event.Timestamp.Unix(),
		"platform":       event.Platform,
		"ip":             event.IP,
		"device_id":      event.DeviceID,
		"event_type":     string(event.EventType),
	}

	pipe := s.client.Pipeline()
	pipe.HSet(ctx, key, data)
	pipe.Expire(ctx, key, 30*24*time.Hour) // 30 天过期
	_, err := pipe.Exec(ctx)
	return err
}

// GetLastActive 获取用户最后活跃时间.
func (s *RedisStore) GetLastActive(ctx context.Context, userID string) (*Status, error) {
	key := s.lastActiveKey(userID)

	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	status := &Status{
		UserID:       userID,
		LastPlatform: result["platform"],
		LastIP:       result["ip"],
	}

	if ts, ok := result["last_active_at"]; ok {
		var timestamp int64
		fmt.Sscanf(ts, "%d", &timestamp)
		status.LastActiveAt = time.Unix(timestamp, 0)
	}

	return status, nil
}

// GetMultiLastActive 批量获取用户最后活跃时间.
func (s *RedisStore) GetMultiLastActive(ctx context.Context, userIDs []string) (map[string]*Status, error) {
	if len(userIDs) == 0 {
		return make(map[string]*Status), nil
	}

	pipe := s.client.Pipeline()
	cmds := make(map[string]*redis.MapStringStringCmd)

	for _, userID := range userIDs {
		key := s.lastActiveKey(userID)
		cmds[userID] = pipe.HGetAll(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make(map[string]*Status)
	for userID, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil || len(data) == 0 {
			continue
		}

		status := &Status{
			UserID:       userID,
			LastPlatform: data["platform"],
			LastIP:       data["ip"],
		}

		if ts, ok := data["last_active_at"]; ok {
			var timestamp int64
			fmt.Sscanf(ts, "%d", &timestamp)
			status.LastActiveAt = time.Unix(timestamp, 0)
		}

		result[userID] = status
	}

	return result, nil
}

// SetOnline 设置用户在线状态.
func (s *RedisStore) SetOnline(ctx context.Context, userID string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.onlineTTL
	}

	pipe := s.client.Pipeline()

	// 设置单用户在线标记
	onlineKey := s.onlineKey(userID)
	pipe.Set(ctx, onlineKey, time.Now().Unix(), ttl)

	// 添加到在线集合用于统计在线人数
	setKey := s.onlineSetKey()
	pipe.SAdd(ctx, setKey, userID)
	pipe.Expire(ctx, setKey, ttl) // 整个集合的过期时间

	_, err := pipe.Exec(ctx)
	return err
}

// IsOnline 检查用户是否在线.
func (s *RedisStore) IsOnline(ctx context.Context, userID string) (bool, error) {
	key := s.onlineKey(userID)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// GetOnlineCount 获取在线用户数.
func (s *RedisStore) GetOnlineCount(ctx context.Context) (int64, error) {
	key := s.onlineSetKey()
	return s.client.SCard(ctx, key).Result()
}

// GetOnlineUsers 获取在线用户列表分页.
func (s *RedisStore) GetOnlineUsers(ctx context.Context, cursor uint64, count int64) ([]string, uint64, error) {
	key := s.onlineSetKey()
	return s.client.SScan(ctx, key, cursor, "*", count).Result()
}

// 确保 RedisStore 实现了 Store 接口.
var _ Store = (*RedisStore)(nil)

// StatusJSON 用于 JSON 序列化的状态结构.
type StatusJSON struct {
	UserID         string `json:"user_id"`
	IsOnline       bool   `json:"is_online"`
	LastActiveAt   int64  `json:"last_active_at"`
	LastPlatform   string `json:"last_platform,omitempty"`
	LastIP         string `json:"last_ip,omitempty"`
	OnlineDuration int64  `json:"online_duration,omitempty"`
}

// MarshalJSON 实现 json.Marshaler.
func (s *Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(&StatusJSON{
		UserID:         s.UserID,
		IsOnline:       s.IsOnline,
		LastActiveAt:   s.LastActiveAt.Unix(),
		LastPlatform:   s.LastPlatform,
		LastIP:         s.LastIP,
		OnlineDuration: s.OnlineDuration,
	})
}
