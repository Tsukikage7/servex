package statemachine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 订单状态定义.
const (
	StatePending   State = "pending"
	StatePaid      State = "paid"
	StateShipped   State = "shipped"
	StateDelivered State = "delivered"
	StateCancelled State = "cancelled"
)

// 订单事件定义.
const (
	EventPay     Event = "pay"
	EventShip    Event = "ship"
	EventDeliver Event = "deliver"
	EventCancel  Event = "cancel"
)

func newOrderMachine() *Machine {
	return New(StatePending, []Transition{
		{From: StatePending, Event: EventPay, To: StatePaid},
		{From: StatePaid, Event: EventShip, To: StateShipped},
		{From: StateShipped, Event: EventDeliver, To: StateDelivered},
		{From: StatePending, Event: EventCancel, To: StateCancelled},
	})
}

func TestFire(t *testing.T) {
	ctx := t.Context()
	sm := newOrderMachine()

	assert.Equal(t, StatePending, sm.Current())

	err := sm.Fire(ctx, EventPay, nil)
	require.NoError(t, err)
	assert.Equal(t, StatePaid, sm.Current())

	err = sm.Fire(ctx, EventShip, nil)
	require.NoError(t, err)
	assert.Equal(t, StateShipped, sm.Current())

	err = sm.Fire(ctx, EventDeliver, nil)
	require.NoError(t, err)
	assert.Equal(t, StateDelivered, sm.Current())
}

func TestCan(t *testing.T) {
	sm := newOrderMachine()

	assert.True(t, sm.Can(EventPay))
	assert.True(t, sm.Can(EventCancel))
	assert.False(t, sm.Can(EventShip))
	assert.False(t, sm.Can(EventDeliver))
}

func TestGuard(t *testing.T) {
	ctx := t.Context()

	sm := New(StatePending, []Transition{
		{
			From:  StatePending,
			Event: EventPay,
			To:    StatePaid,
			Guard: func(_ context.Context, data any) bool {
				amount, ok := data.(float64)
				return ok && amount > 0
			},
		},
	})

	// 守卫拒绝金额为 0
	err := sm.Fire(ctx, EventPay, float64(0))
	assert.ErrorIs(t, err, ErrGuardRejected)
	assert.Equal(t, StatePending, sm.Current())

	// 守卫通过
	err = sm.Fire(ctx, EventPay, float64(100))
	require.NoError(t, err)
	assert.Equal(t, StatePaid, sm.Current())
}

func TestAction(t *testing.T) {
	ctx := t.Context()
	actionErr := errors.New("payment failed")

	sm := New(StatePending, []Transition{
		{
			From:  StatePending,
			Event: EventPay,
			To:    StatePaid,
			Action: func(_ context.Context, _ any) error {
				return actionErr
			},
		},
	})

	err := sm.Fire(ctx, EventPay, nil)
	assert.ErrorIs(t, err, ErrActionFailed)
	assert.Equal(t, StatePending, sm.Current()) // 状态不应改变
}

func TestOnEnter(t *testing.T) {
	ctx := t.Context()
	sm := newOrderMachine()

	entered := false
	sm.OnEnter(StatePaid, func(_ context.Context, _ any) {
		entered = true
	})

	_ = sm.Fire(ctx, EventPay, nil)
	assert.True(t, entered)
}

func TestOnLeave(t *testing.T) {
	ctx := t.Context()
	sm := newOrderMachine()

	left := false
	sm.OnLeave(StatePending, func(_ context.Context, _ any) {
		left = true
	})

	_ = sm.Fire(ctx, EventPay, nil)
	assert.True(t, left)
}

func TestInvalidTransition(t *testing.T) {
	ctx := t.Context()
	sm := newOrderMachine()

	// 从 pending 状态触发 ship 事件无效
	err := sm.Fire(ctx, EventShip, nil)
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.Equal(t, StatePending, sm.Current())
}

func TestAvailableEvents(t *testing.T) {
	sm := newOrderMachine()

	events := sm.AvailableEvents()
	assert.Len(t, events, 2)
	assert.Contains(t, events, EventPay)
	assert.Contains(t, events, EventCancel)

	_ = sm.Fire(t.Context(), EventPay, nil)
	events = sm.AvailableEvents()
	assert.Len(t, events, 1)
	assert.Contains(t, events, EventShip)
}
