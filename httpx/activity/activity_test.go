package activity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockStore 测试用的 Mock Store.
type mockStore struct {
	lastActive map[string]*Event
	online     map[string]bool
}

func newMockStore() *mockStore {
	return &mockStore{
		lastActive: make(map[string]*Event),
		online:     make(map[string]bool),
	}
}

func (s *mockStore) SetLastActive(ctx context.Context, userID string, event *Event) error {
	s.lastActive[userID] = event
	return nil
}

func (s *mockStore) GetLastActive(ctx context.Context, userID string) (*Status, error) {
	event, ok := s.lastActive[userID]
	if !ok {
		return nil, nil
	}
	return &Status{
		UserID:       userID,
		LastActiveAt: event.Timestamp,
		LastPlatform: event.Platform,
		LastIP:       event.IP,
	}, nil
}

func (s *mockStore) GetMultiLastActive(ctx context.Context, userIDs []string) (map[string]*Status, error) {
	result := make(map[string]*Status)
	for _, userID := range userIDs {
		if status, _ := s.GetLastActive(ctx, userID); status != nil {
			result[userID] = status
		}
	}
	return result, nil
}

func (s *mockStore) SetOnline(ctx context.Context, userID string, ttl time.Duration) error {
	s.online[userID] = true
	return nil
}

func (s *mockStore) IsOnline(ctx context.Context, userID string) (bool, error) {
	return s.online[userID], nil
}

func (s *mockStore) GetOnlineCount(ctx context.Context) (int64, error) {
	return int64(len(s.online)), nil
}

func TestTracker(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false), // 同步模式便于测试
		WithDedupeWindow(0),  // 禁用去重便于测试
	)

	ctx := t.Context()

	// 追踪事件
	event := &Event{
		UserID:    "user123",
		EventType: EventTypeRequest,
		Platform:  "iOS",
		IP:        "192.168.1.1",
	}

	err := tracker.Track(ctx, event)
	if err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	// 获取状态
	status, err := tracker.GetStatus(ctx, "user123")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.UserID != "user123" {
		t.Errorf("UserID = %q, want %q", status.UserID, "user123")
	}
	if status.LastPlatform != "iOS" {
		t.Errorf("LastPlatform = %q, want %q", status.LastPlatform, "iOS")
	}
	if status.LastIP != "192.168.1.1" {
		t.Errorf("LastIP = %q, want %q", status.LastIP, "192.168.1.1")
	}
	if !status.IsOnline {
		t.Error("IsOnline = false, want true")
	}
}

func TestTrackerSkipEmptyUserID(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
	)

	ctx := t.Context()

	// 空用户 ID 应该被跳过
	event := &Event{
		UserID:    "",
		EventType: EventTypeRequest,
	}

	err := tracker.Track(ctx, event)
	if err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	// store 应该是空的
	if len(store.lastActive) != 0 {
		t.Error("Store should be empty for empty user ID")
	}
}

func TestTrackerDedupe(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
		WithDedupeWindow(time.Minute),
	)

	ctx := t.Context()
	now := time.Now()

	// 第一次追踪
	event1 := &Event{
		UserID:    "user123",
		EventType: EventTypeRequest,
		Timestamp: now,
	}
	_ = tracker.Track(ctx, event1)

	// 第二次追踪在去重窗口内
	event2 := &Event{
		UserID:    "user123",
		EventType: EventTypeRequest,
		Timestamp: now.Add(10 * time.Second),
		Platform:  "Android", // 不同的平台
	}
	_ = tracker.Track(ctx, event2)

	// 由于去重，平台应该还是第一次的值空
	status, _ := tracker.GetStatus(ctx, "user123")
	if status.LastPlatform == "Android" {
		t.Error("Second event should be deduped")
	}
}

func TestIsActive(t *testing.T) {
	status := &Status{
		LastActiveAt: time.Now().Add(-2 * time.Minute),
	}

	if !status.IsActive(5 * time.Minute) {
		t.Error("Should be active within 5 minutes")
	}

	if status.IsActive(1 * time.Minute) {
		t.Error("Should not be active within 1 minute")
	}
}

func TestEventMarshal(t *testing.T) {
	event := &Event{
		UserID:    "user123",
		Timestamp: time.Now(),
		EventType: EventTypeLogin,
		Platform:  "iOS",
		IP:        "192.168.1.1",
		Extra: map[string]string{
			"version": "1.0.0",
		},
	}

	data, err := MarshalEvent(event)
	if err != nil {
		t.Fatalf("MarshalEvent() error = %v", err)
	}

	parsed, err := UnmarshalEvent(data)
	if err != nil {
		t.Fatalf("UnmarshalEvent() error = %v", err)
	}

	if parsed.UserID != event.UserID {
		t.Errorf("UserID = %q, want %q", parsed.UserID, event.UserID)
	}
	if parsed.EventType != event.EventType {
		t.Errorf("EventType = %q, want %q", parsed.EventType, event.EventType)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
		WithDedupeWindow(0),
		WithUserIDExtractor(func(ctx context.Context) string {
			return "test-user"
		}),
	)

	handler := HTTPMiddleware(tracker)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	// 检查是否记录了活跃
	status, _ := tracker.GetStatus(t.Context(), "test-user")
	if status == nil {
		t.Fatal("Activity should be tracked")
	}
	if status.LastPlatform != "iOS" {
		t.Errorf("Platform = %q, want %q", status.LastPlatform, "iOS")
	}
}

func TestHTTPMiddlewareSkipPaths(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
		WithUserIDExtractor(func(ctx context.Context) string {
			return "test-user"
		}),
	)

	handler := HTTPMiddleware(tracker)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 请求健康检查路径
	req := httptest.NewRequest("GET", "/health", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// 不应该记录活跃
	if len(store.lastActive) != 0 {
		t.Error("Health check path should be skipped")
	}
}

func TestConsumer(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
	)
	consumer := NewConsumer(tracker)

	event := &Event{
		UserID:    "user456",
		Timestamp: time.Now(),
		EventType: EventTypeRequest,
		Platform:  "Android",
	}
	data, _ := MarshalEvent(event)

	err := consumer.HandleMessage(t.Context(), data)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	status, _ := tracker.GetStatus(t.Context(), "user456")
	if status == nil {
		t.Fatal("Event should be consumed")
	}
	if status.LastPlatform != "Android" {
		t.Errorf("Platform = %q, want %q", status.LastPlatform, "Android")
	}
}

func TestBatchHandler(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
	)
	handler := NewBatchHandler(tracker, 100)

	now := time.Now()
	messages := [][]byte{}

	// 同一用户的多条消息
	for i := 0; i < 3; i++ {
		event := &Event{
			UserID:    "user789",
			Timestamp: now.Add(time.Duration(i) * time.Second),
			EventType: EventTypeRequest,
			Platform:  "iOS",
		}
		data, _ := MarshalEvent(event)
		messages = append(messages, data)
	}

	err := handler.HandleBatch(t.Context(), messages)
	if err != nil {
		t.Fatalf("HandleBatch() error = %v", err)
	}

	// 应该只有最新的一条
	status, _ := tracker.GetStatus(t.Context(), "user789")
	if status == nil {
		t.Fatal("Batch should be processed")
	}
}

func TestGetOnlineCount(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
		WithDedupeWindow(0),
	)

	ctx := t.Context()

	// 追踪多个用户
	for _, userID := range []string{"user1", "user2", "user3"} {
		event := &Event{
			UserID:    userID,
			EventType: EventTypeRequest,
		}
		_ = tracker.Track(ctx, event)
	}

	count, err := tracker.GetOnlineCount(ctx)
	if err != nil {
		t.Fatalf("GetOnlineCount() error = %v", err)
	}
	if count != 3 {
		t.Errorf("OnlineCount = %d, want 3", count)
	}
}

func TestGetMultiStatus(t *testing.T) {
	store := newMockStore()
	tracker := NewTracker(
		WithStore(store),
		WithAsyncMode(false),
		WithDedupeWindow(0),
	)

	ctx := t.Context()

	// 追踪多个用户
	users := []string{"user1", "user2", "user3"}
	for _, userID := range users {
		event := &Event{
			UserID:    userID,
			EventType: EventTypeRequest,
			Platform:  userID + "-platform",
		}
		_ = tracker.Track(ctx, event)
	}

	// 批量获取
	statuses, err := tracker.GetMultiStatus(ctx, users)
	if err != nil {
		t.Fatalf("GetMultiStatus() error = %v", err)
	}

	if len(statuses) != 3 {
		t.Errorf("len(statuses) = %d, want 3", len(statuses))
	}

	for _, userID := range users {
		status, ok := statuses[userID]
		if !ok {
			t.Errorf("Missing status for %s", userID)
			continue
		}
		expectedPlatform := userID + "-platform"
		if status.LastPlatform != expectedPlatform {
			t.Errorf("Platform for %s = %q, want %q", userID, status.LastPlatform, expectedPlatform)
		}
	}
}
