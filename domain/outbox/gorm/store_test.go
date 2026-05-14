package outboxgorm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tsukikage7/servex/v2/domain/outbox"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// --- mock Publisher ---

type mockPublisher struct {
	mu      sync.Mutex
	sent    []*pubsub.Message
	sendErr error
	closed  bool
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{}
}

func (p *mockPublisher) Publish(_ context.Context, topic string, msgs ...*pubsub.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sendErr != nil {
		return p.sendErr
	}
	p.sent = append(p.sent, msgs...)
	return nil
}

func (p *mockPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func (p *mockPublisher) sentMessages() []*pubsub.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]*pubsub.Message, len(p.sent))
	copy(cp, p.sent)
	return cp
}

// --- helpers ---

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// SQLite :memory: 每个连接是独立数据库，必须限制为单连接，
	// 否则 relay goroutine 拿到不同连接会访问到空数据库.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	return db
}

func setupTestStore(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	store := NewStoreFromDB(db)
	require.NoError(t, store.AutoMigrate())
	return store, db
}

// --- Store 测试 ---

func TestStore_Save(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	err := store.Save(ctx,
		&outbox.OutboxMessage{Topic: "t1", Value: []byte("v1")},
		&outbox.OutboxMessage{Topic: "t2", Value: []byte("v2")},
	)
	require.NoError(t, err)

	var count int64
	db.Model(&outbox.OutboxMessage{}).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestStore_Save_Empty(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.Save(t.Context())
	assert.NoError(t, err)
}

func TestStore_WithTx(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	err := store.WithTx(ctx, func(txCtx context.Context) error {
		return store.Save(txCtx,
			&outbox.OutboxMessage{Topic: "t1", Value: []byte("v1")},
			&outbox.OutboxMessage{Topic: "t2", Value: []byte("v2")},
		)
	})
	require.NoError(t, err)

	var count int64
	db.Model(&outbox.OutboxMessage{}).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestStore_WithTx_Rollback(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	err := store.WithTx(ctx, func(txCtx context.Context) error {
		if err := store.Save(txCtx, &outbox.OutboxMessage{Topic: "t1", Value: []byte("v1")}); err != nil {
			return err
		}
		return errors.New("模拟事务回滚")
	})
	assert.Error(t, err)

	// 事务已回滚，不应有记录
	var count int64
	db.Model(&outbox.OutboxMessage{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestStore_FetchPending(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	for i := range 3 {
		db.Create(&outbox.OutboxMessage{
			Topic: "topic",
			Value: []byte{byte(i)},
		})
	}

	msgs, err := store.FetchPending(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)

	var processing int64
	db.Model(&outbox.OutboxMessage{}).Where("status = ?", outbox.StatusProcessing).Count(&processing)
	assert.Equal(t, int64(2), processing)

	var pending int64
	db.Model(&outbox.OutboxMessage{}).Where("status = ?", outbox.StatusPending).Count(&pending)
	assert.Equal(t, int64(1), pending)
}

func TestStore_FetchPending_Empty(t *testing.T) {
	store, _ := setupTestStore(t)
	msgs, err := store.FetchPending(t.Context(), 10)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestStore_MarkSent(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	db.Create(&outbox.OutboxMessage{Topic: "t", Value: []byte("v"), Status: outbox.StatusProcessing})
	db.Create(&outbox.OutboxMessage{Topic: "t", Value: []byte("v"), Status: outbox.StatusProcessing})

	err := store.MarkSent(ctx, []uint64{1, 2})
	require.NoError(t, err)

	var msgs []outbox.OutboxMessage
	db.Find(&msgs)
	for _, m := range msgs {
		assert.Equal(t, outbox.StatusSent, m.Status)
		assert.NotNil(t, m.SentAt)
	}
}

func TestStore_MarkSent_Empty(t *testing.T) {
	store, _ := setupTestStore(t)
	assert.NoError(t, store.MarkSent(t.Context(), nil))
	assert.NoError(t, store.MarkSent(t.Context(), []uint64{}))
}

func TestStore_MarkFailed(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	db.Create(&outbox.OutboxMessage{Topic: "t", Value: []byte("v"), Status: outbox.StatusProcessing})

	err := store.MarkFailed(ctx, 1, "send timeout")
	require.NoError(t, err)

	var msg outbox.OutboxMessage
	db.First(&msg, 1)
	assert.Equal(t, outbox.StatusFailed, msg.Status)
	assert.Equal(t, 1, msg.RetryCount)
	assert.Equal(t, "send timeout", msg.LastError)
}

func TestStore_ResetStale(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	db.Create(&outbox.OutboxMessage{Topic: "t", Value: []byte("v"), Status: outbox.StatusProcessing})
	db.Model(&outbox.OutboxMessage{}).Where("id = 1").
		Update("updated_at", time.Now().Add(-10*time.Minute))

	n, err := store.ResetStale(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	var msg outbox.OutboxMessage
	db.First(&msg, 1)
	assert.Equal(t, outbox.StatusPending, msg.Status)
}

func TestStore_Cleanup(t *testing.T) {
	store, db := setupTestStore(t)
	ctx := t.Context()

	past := time.Now().Add(-48 * time.Hour)
	db.Create(&outbox.OutboxMessage{Topic: "t", Value: []byte("v"), Status: outbox.StatusSent, SentAt: &past})

	n, err := store.Cleanup(ctx, time.Now().Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	var count int64
	db.Model(&outbox.OutboxMessage{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

// --- Relay 测试 ---

func TestRelay_StartStop(t *testing.T) {
	store, _ := setupTestStore(t)
	producer := newMockPublisher()

	relay, err := outbox.NewRelay(store, producer,
		outbox.WithPollInterval(50*time.Millisecond),
		outbox.WithCleanupInterval(50*time.Millisecond),
	)
	require.NoError(t, err)

	ctx := t.Context()
	require.NoError(t, relay.Start(ctx))
	t.Cleanup(func() {
		stopCtx, c := context.WithTimeout(t.Context(), 5*time.Second)
		defer c()
		relay.Stop(stopCtx)
	})

	assert.ErrorIs(t, relay.Start(ctx), outbox.ErrRelayAlreadyRunning)

	stopCtx, stopCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopCancel()
	require.NoError(t, relay.Stop(stopCtx))

	assert.ErrorIs(t, relay.Stop(stopCtx), outbox.ErrRelayNotRunning)
}

func TestRelay_PollAndDeliver(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping relay test in short mode")
	}
	store, db := setupTestStore(t)
	producer := newMockPublisher()

	relay, err := outbox.NewRelay(store, producer,
		outbox.WithPollInterval(50*time.Millisecond),
		outbox.WithCleanupInterval(time.Hour),
	)
	require.NoError(t, err)

	ctx := t.Context()

	db.Create(&outbox.OutboxMessage{
		Topic: "orders",
		Key:   []byte("key-1"),
		Value: []byte(`{"id":"1"}`),
	})

	require.NoError(t, relay.Start(ctx))
	t.Cleanup(func() {
		stopCtx, c := context.WithTimeout(t.Context(), 5*time.Second)
		defer c()
		relay.Stop(stopCtx)
	})

	require.Eventually(t, func() bool {
		var msg outbox.OutboxMessage
		db.First(&msg, 1)
		return msg.Status == outbox.StatusSent
	}, 10*time.Second, 100*time.Millisecond)

	sent := producer.sentMessages()
	require.Len(t, sent, 1)
	assert.Equal(t, "orders", sent[0].Topic)
	assert.Equal(t, []byte("key-1"), sent[0].Key)

	var msg outbox.OutboxMessage
	db.First(&msg, 1)
	assert.Equal(t, outbox.StatusSent, msg.Status)
	assert.NotNil(t, msg.SentAt)
}

func TestRelay_SendFailure(t *testing.T) {
	store, db := setupTestStore(t)
	producer := newMockPublisher()
	producer.sendErr = errors.New("connection refused")

	relay, err := outbox.NewRelay(store, producer,
		outbox.WithPollInterval(50*time.Millisecond),
		outbox.WithCleanupInterval(time.Hour),
	)
	require.NoError(t, err)

	ctx := t.Context()

	db.Create(&outbox.OutboxMessage{
		Topic: "events",
		Value: []byte("data"),
	})

	require.NoError(t, relay.Start(ctx))
	t.Cleanup(func() {
		stopCtx, c := context.WithTimeout(t.Context(), 5*time.Second)
		defer c()
		relay.Stop(stopCtx)
	})

	require.Eventually(t, func() bool {
		var msg outbox.OutboxMessage
		db.First(&msg, 1)
		return msg.Status == outbox.StatusFailed
	}, 10*time.Second, 100*time.Millisecond)

	var msg outbox.OutboxMessage
	db.First(&msg, 1)
	assert.Equal(t, outbox.StatusFailed, msg.Status)
	assert.Equal(t, "connection refused", msg.LastError)
	assert.Equal(t, 1, msg.RetryCount)
}

func TestRelay_MaxRetriesSkip(t *testing.T) {
	store, db := setupTestStore(t)
	producer := newMockPublisher()

	relay, err := outbox.NewRelay(store, producer,
		outbox.WithPollInterval(50*time.Millisecond),
		outbox.WithCleanupInterval(time.Hour),
		outbox.WithMaxRetries(2),
	)
	require.NoError(t, err)

	ctx := t.Context()

	db.Create(&outbox.OutboxMessage{
		Topic:      "events",
		Value:      []byte("data"),
		RetryCount: 2,
	})

	require.NoError(t, relay.Start(ctx))
	t.Cleanup(func() {
		stopCtx, c := context.WithTimeout(t.Context(), 5*time.Second)
		defer c()
		relay.Stop(stopCtx)
	})

	time.Sleep(200 * time.Millisecond)

	assert.Empty(t, producer.sentMessages())
}
