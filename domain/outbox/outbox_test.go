package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/messaging/pubsub"
)

// --- OutboxMessage 测试 ---

func TestNewOutboxMessage(t *testing.T) {
	msg := &pubsub.Message{
		Topic:   "orders",
		Key:     []byte("order-123"),
		Body:    []byte(`{"id":"123"}`),
		Headers: map[string]string{"trace-id": "abc"},
	}
	om := NewOutboxMessage(msg)

	assert.Equal(t, "orders", om.Topic)
	assert.Equal(t, []byte("order-123"), om.Key)
	assert.Equal(t, []byte(`{"id":"123"}`), om.Value)
	assert.Equal(t, MessageStatus(0), om.Status)

	var h map[string]string
	require.NoError(t, json.Unmarshal([]byte(om.Headers), &h))
	assert.Equal(t, "abc", h["trace-id"])
}

func TestOutboxMessage_ToMessage(t *testing.T) {
	om := &OutboxMessage{
		Topic:   "events",
		Key:     []byte("key-1"),
		Value:   []byte("data"),
		Headers: `{"x":"y"}`,
	}
	msg := om.ToMessage()

	assert.Equal(t, "events", msg.Topic)
	assert.Equal(t, []byte("key-1"), msg.Key)
	assert.Equal(t, []byte("data"), msg.Body)
	assert.Equal(t, "y", msg.Headers["x"])
}

func TestHeadersToJSON_Empty(t *testing.T) {
	assert.Equal(t, "", HeadersToJSON(nil))
	assert.Equal(t, "", HeadersToJSON(map[string]string{}))
}

func TestMessageStatus_String(t *testing.T) {
	assert.Equal(t, "Pending", StatusPending.String())
	assert.Equal(t, "Processing", StatusProcessing.String())
	assert.Equal(t, "Sent", StatusSent.String())
	assert.Equal(t, "Failed", StatusFailed.String())
	assert.Equal(t, "Unknown", MessageStatus(99).String())
}

// --- InjectTx / ExtractTx 测试 ---

func TestInjectTx_ExtractTx(t *testing.T) {
	db := &gorm.DB{}

	ctx := t.Context()
	txCtx := InjectTx(ctx, db)

	tx, ok := ExtractTx(txCtx)
	assert.True(t, ok)
	assert.Equal(t, db, tx)
}

func TestExtractTx_NotInjected(t *testing.T) {
	ctx := t.Context()
	tx, ok := ExtractTx(ctx)
	assert.False(t, ok)
	assert.Nil(t, tx)
}

// --- Options 测试 ---

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()
	assert.Equal(t, time.Second, opts.pollInterval)
	assert.Equal(t, 100, opts.batchSize)
	assert.Equal(t, 3, opts.maxRetries)
	assert.Equal(t, 7*24*time.Hour, opts.cleanupAge)
	assert.Equal(t, time.Hour, opts.cleanupInterval)
	assert.Equal(t, 5*time.Minute, opts.staleTimeout)
	assert.Nil(t, opts.logger)
}

func TestApplyOptions(t *testing.T) {
	opts := applyOptions([]Option{
		WithPollInterval(2 * time.Second),
		WithBatchSize(50),
		WithMaxRetries(5),
		WithCleanupAge(24 * time.Hour),
		WithCleanupInterval(30 * time.Minute),
		WithStaleTimeout(10 * time.Minute),
	})

	assert.Equal(t, 2*time.Second, opts.pollInterval)
	assert.Equal(t, 50, opts.batchSize)
	assert.Equal(t, 5, opts.maxRetries)
	assert.Equal(t, 24*time.Hour, opts.cleanupAge)
	assert.Equal(t, 30*time.Minute, opts.cleanupInterval)
	assert.Equal(t, 10*time.Minute, opts.staleTimeout)
}

// --- Errors 测试 ---

func TestErrors(t *testing.T) {
	assert.True(t, errors.Is(ErrNilStore, ErrNilStore))
	assert.True(t, errors.Is(ErrNilProducer, ErrNilProducer))
	assert.True(t, errors.Is(ErrRelayAlreadyRunning, ErrRelayAlreadyRunning))
	assert.True(t, errors.Is(ErrRelayNotRunning, ErrRelayNotRunning))
	assert.True(t, errors.Is(ErrEmptyTopic, ErrEmptyTopic))
	assert.True(t, errors.Is(ErrEmptyValue, ErrEmptyValue))
	assert.True(t, errors.Is(ErrNilDB, ErrNilDB))
}

// --- Relay 参数校验测试 ---

type mockStore struct{}

func (m *mockStore) Save(ctx context.Context, msgs ...*OutboxMessage) error  { return nil }
func (m *mockStore) WithTx(ctx context.Context, fn TxFunc) error             { return nil }
func (m *mockStore) FetchPending(ctx context.Context, limit int) ([]*OutboxMessage, error) {
	return nil, nil
}
func (m *mockStore) MarkSent(ctx context.Context, ids []uint64) error                      { return nil }
func (m *mockStore) MarkFailed(ctx context.Context, id uint64, errMsg string) error        { return nil }
func (m *mockStore) ResetStale(ctx context.Context, d time.Duration) (int64, error)        { return 0, nil }
func (m *mockStore) Cleanup(ctx context.Context, before time.Time) (int64, error)          { return 0, nil }
func (m *mockStore) AutoMigrate() error                                                    { return nil }

type mockPublisher struct{}

func (p *mockPublisher) Publish(_ context.Context, topic string, msgs ...*pubsub.Message) error {
	return nil
}
func (p *mockPublisher) Close() error { return nil }

func TestNewRelay_NilStore(t *testing.T) {
	_, err := NewRelay(nil, &mockPublisher{})
	assert.ErrorIs(t, err, ErrNilStore)
}

func TestNewRelay_NilProducer(t *testing.T) {
	_, err := NewRelay(&mockStore{}, nil)
	assert.ErrorIs(t, err, ErrNilProducer)
}
