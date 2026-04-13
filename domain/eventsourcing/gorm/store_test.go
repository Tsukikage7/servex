package esgorm

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Tsukikage7/servex/v2/domain/eventsourcing"
)

// --- 测试用聚合：银行账户 ---

type bankAccount struct {
	eventsourcing.BaseAggregate
	Balance int64  `json:"balance"`
	Owner   string `json:"owner"`
}

func newBankAccount(id, owner string) *bankAccount {
	return &bankAccount{
		BaseAggregate: eventsourcing.NewBaseAggregate(id, "BankAccount"),
		Owner:         owner,
	}
}

func (a *bankAccount) ApplyEvent(event eventsourcing.Event) error {
	switch event.EventType {
	case "AccountCreated":
		var data struct {
			Owner string `json:"owner"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.Owner = data.Owner
	case "Deposited":
		var data struct {
			Amount int64 `json:"amount"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.Balance += data.Amount
	case "Withdrawn":
		var data struct {
			Amount int64 `json:"amount"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		if a.Balance < data.Amount {
			return errors.New("余额不足")
		}
		a.Balance -= data.Amount
	}
	return nil
}

func (a *bankAccount) Deposit(amount int64) error {
	return a.RaiseEvent(a.ApplyEvent, "Deposited", map[string]int64{"amount": amount})
}

func (a *bankAccount) Withdraw(amount int64) error {
	return a.RaiseEvent(a.ApplyEvent, "Withdrawn", map[string]int64{"amount": amount})
}

func (a *bankAccount) Create(owner string) error {
	return a.RaiseEvent(a.ApplyEvent, "AccountCreated", map[string]string{"owner": owner})
}

// --- helpers ---

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	return db
}

func setupEventStore(t *testing.T) (*EventStore, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	store := NewEventStore(db)
	require.NoError(t, store.AutoMigrate())
	return store, db
}

func setupSnapshotStore(t *testing.T, db *gorm.DB) *SnapshotStore {
	t.Helper()
	store := NewSnapshotStore(db)
	require.NoError(t, store.AutoMigrate())
	return store
}

// --- EventStore 测试 ---

func TestEventStore_SaveAndLoad(t *testing.T) {
	store, _ := setupEventStore(t)
	ctx := t.Context()

	account := newBankAccount("acc-1", "张三")
	require.NoError(t, account.Create("张三"))
	require.NoError(t, account.Deposit(100))
	require.NoError(t, account.Deposit(50))

	// 保存事件
	err := store.Save(ctx, account.UncommittedEvents())
	require.NoError(t, err)

	// LoadAll
	events, err := store.LoadAll(ctx, "acc-1")
	require.NoError(t, err)
	assert.Len(t, events, 3)
	assert.Equal(t, int64(1), events[0].Version)
	assert.Equal(t, int64(2), events[1].Version)
	assert.Equal(t, int64(3), events[2].Version)

	// Load（从版本 1 之后）
	events, err = store.Load(ctx, "acc-1", 1)
	require.NoError(t, err)
	assert.Len(t, events, 2)
	assert.Equal(t, int64(2), events[0].Version)

	// Load 不存在的聚合
	events, err = store.Load(ctx, "not-exists", 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestEventStore_Save_Empty(t *testing.T) {
	store, _ := setupEventStore(t)
	err := store.Save(t.Context(), nil)
	assert.ErrorIs(t, err, eventsourcing.ErrNoEvents)
}

// --- SnapshotStore 测试 ---

func TestSnapshotStore_SaveAndLoad(t *testing.T) {
	db := setupTestDB(t)
	store := setupSnapshotStore(t, db)
	ctx := t.Context()

	snapshot := eventsourcing.Snapshot{
		AggregateID:   "acc-1",
		AggregateType: "BankAccount",
		Version:       5,
		Data:          json.RawMessage(`{"balance":500,"owner":"张三"}`),
	}

	// 保存快照
	err := store.Save(ctx, snapshot)
	require.NoError(t, err)

	// 加载快照
	loaded, err := store.Load(ctx, "acc-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, int64(5), loaded.Version)
	assert.Equal(t, "BankAccount", loaded.AggregateType)

	// Upsert：更新快照
	snapshot.Version = 10
	snapshot.Data = json.RawMessage(`{"balance":1000,"owner":"张三"}`)
	err = store.Save(ctx, snapshot)
	require.NoError(t, err)

	loaded, err = store.Load(ctx, "acc-1")
	require.NoError(t, err)
	assert.Equal(t, int64(10), loaded.Version)

	// 加载不存在的快照
	loaded, err = store.Load(ctx, "not-exists")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

// --- Repository 测试（无快照） ---

func TestRepository_SaveAndLoad(t *testing.T) {
	store, _ := setupEventStore(t)
	ctx := t.Context()

	factory := func() *bankAccount {
		return &bankAccount{BaseAggregate: eventsourcing.NewBaseAggregate("", "BankAccount")}
	}

	repo, err := eventsourcing.NewRepository[*bankAccount](store, factory)
	require.NoError(t, err)

	// 创建并保存聚合
	account := newBankAccount("acc-1", "")
	require.NoError(t, account.Create("张三"))
	require.NoError(t, account.Deposit(100))
	require.NoError(t, account.Deposit(50))

	err = repo.Save(ctx, account)
	require.NoError(t, err)
	assert.Empty(t, account.UncommittedEvents())

	// 加载聚合
	loaded, err := repo.Load(ctx, "acc-1")
	require.NoError(t, err)
	assert.Equal(t, "张三", loaded.Owner)
	assert.Equal(t, int64(150), loaded.Balance)
	assert.Equal(t, int64(3), loaded.Version())

	// 加载不存在的聚合
	_, err = repo.Load(ctx, "not-exists")
	assert.ErrorIs(t, err, eventsourcing.ErrAggregateNotFound)
}

// --- Repository 测试（带快照） ---

func TestRepository_WithSnapshot(t *testing.T) {
	db := setupTestDB(t)
	eventStore := NewEventStore(db)
	require.NoError(t, eventStore.AutoMigrate())
	snapshotStore := setupSnapshotStore(t, db)
	ctx := t.Context()

	factory := func() *bankAccount {
		return &bankAccount{BaseAggregate: eventsourcing.NewBaseAggregate("", "BankAccount")}
	}

	repo, err := eventsourcing.NewRepository[*bankAccount](eventStore, factory,
		eventsourcing.WithSnapshotStore[*bankAccount](snapshotStore),
		eventsourcing.WithSnapshotEvery[*bankAccount](2),
	)
	require.NoError(t, err)

	// 创建聚合，产生 2 个事件 → 触发快照
	account := newBankAccount("acc-1", "")
	require.NoError(t, account.Create("张三"))
	require.NoError(t, account.Deposit(100))

	err = repo.Save(ctx, account)
	require.NoError(t, err)

	// 验证快照已保存
	snapshot, err := snapshotStore.Load(ctx, "acc-1")
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, int64(2), snapshot.Version)

	// 再追加一个事件（版本 3）
	require.NoError(t, account.Deposit(50))
	err = repo.Save(ctx, account)
	require.NoError(t, err)

	// 加载聚合：应从快照版本 2 开始，只需加载版本 3
	loaded, err := repo.Load(ctx, "acc-1")
	require.NoError(t, err)
	assert.Equal(t, int64(150), loaded.Balance)
	assert.Equal(t, int64(3), loaded.Version())
	assert.Equal(t, "张三", loaded.Owner)
}

// --- 并发冲突测试 ---

func TestConcurrencyConflict(t *testing.T) {
	store, _ := setupEventStore(t)
	ctx := t.Context()

	// 第一次保存
	account1 := newBankAccount("acc-1", "")
	require.NoError(t, account1.Create("张三"))
	err := store.Save(ctx, account1.UncommittedEvents())
	require.NoError(t, err)

	// 模拟并发：用相同的 aggregate_id + version 再次保存
	account2 := newBankAccount("acc-1", "")
	require.NoError(t, account2.Create("李四"))
	err = store.Save(ctx, account2.UncommittedEvents())
	assert.ErrorIs(t, err, eventsourcing.ErrConcurrencyConflict)
}
