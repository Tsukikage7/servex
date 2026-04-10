package eventsourcing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- 测试用聚合：银行账户 ---

// BankAccount 银行账户聚合.
type BankAccount struct {
	BaseAggregate
	Balance int64  `json:"balance"`
	Owner   string `json:"owner"`
}

// NewBankAccount 创建银行账户.
func NewBankAccount(id, owner string) *BankAccount {
	return &BankAccount{
		BaseAggregate: NewBaseAggregate(id, "BankAccount"),
		Owner:         owner,
	}
}

// ApplyEvent 应用事件.
func (a *BankAccount) ApplyEvent(event Event) error {
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

// Deposit 存款.
func (a *BankAccount) Deposit(amount int64) error {
	return a.RaiseEvent(a.ApplyEvent, "Deposited", map[string]int64{"amount": amount})
}

// Withdraw 取款.
func (a *BankAccount) Withdraw(amount int64) error {
	return a.RaiseEvent(a.ApplyEvent, "Withdrawn", map[string]int64{"amount": amount})
}

// Create 创建账户事件.
func (a *BankAccount) Create(owner string) error {
	return a.RaiseEvent(a.ApplyEvent, "AccountCreated", map[string]string{"owner": owner})
}

// --- BaseAggregate 测试 ---

func TestBaseAggregate_RaiseEvent(t *testing.T) {
	account := NewBankAccount("acc-1", "张三")

	err := account.Deposit(100)
	require.NoError(t, err)

	assert.Equal(t, int64(100), account.Balance)
	assert.Equal(t, int64(1), account.Version())
	assert.Len(t, account.UncommittedEvents(), 1)

	event := account.UncommittedEvents()[0]
	assert.Equal(t, "acc-1", event.AggregateID)
	assert.Equal(t, "BankAccount", event.AggregateType)
	assert.Equal(t, int64(1), event.Version)
	assert.Equal(t, "Deposited", event.EventType)
	assert.NotEmpty(t, event.ID)

	// 多次操作
	err = account.Deposit(50)
	require.NoError(t, err)
	err = account.Withdraw(30)
	require.NoError(t, err)

	assert.Equal(t, int64(120), account.Balance)
	assert.Equal(t, int64(3), account.Version())
	assert.Len(t, account.UncommittedEvents(), 3)

	// 清除事件
	account.ClearUncommittedEvents()
	assert.Empty(t, account.UncommittedEvents())
}

func TestBaseAggregate_RaiseEvent_ApplyError(t *testing.T) {
	account := NewBankAccount("acc-1", "张三")

	// 余额不足时 ApplyEvent 返回错误
	err := account.Withdraw(100)
	assert.Error(t, err)

	// 版本不变，无未提交事件
	assert.Equal(t, int64(0), account.Version())
	assert.Empty(t, account.UncommittedEvents())
}

// --- NewRepository 校验测试 ---

func TestNewRepository_NilEventStore(t *testing.T) {
	factory := func() *BankAccount { return &BankAccount{} }
	_, err := NewRepository[*BankAccount](nil, factory)
	assert.ErrorIs(t, err, ErrNilEventStore)
}

func TestNewRepository_NilFactory(t *testing.T) {
	// 使用 mock EventStore 来避免依赖 GORM
	_, err := NewRepository[*BankAccount](&mockEventStore{}, nil)
	assert.ErrorIs(t, err, ErrNilFactory)
}

// --- Save 无事件测试 ---

func TestRepository_Save_NoEvents(t *testing.T) {
	factory := func() *BankAccount {
		return &BankAccount{BaseAggregate: NewBaseAggregate("", "BankAccount")}
	}
	repo, err := NewRepository[*BankAccount](&mockEventStore{}, factory)
	require.NoError(t, err)

	account := NewBankAccount("acc-1", "张三")
	err = repo.Save(t.Context(), account)
	assert.ErrorIs(t, err, ErrNoEvents)
}

// --- mock EventStore (仅用于参数校验测试) ---

type mockEventStore struct{}

func (m *mockEventStore) Save(_ context.Context, events []Event) error {
	if len(events) == 0 {
		return ErrNoEvents
	}
	return nil
}
func (m *mockEventStore) Load(_ context.Context, _ string, _ int64) ([]Event, error) {
	return nil, nil
}
func (m *mockEventStore) LoadAll(_ context.Context, _ string) ([]Event, error) {
	return nil, nil
}
