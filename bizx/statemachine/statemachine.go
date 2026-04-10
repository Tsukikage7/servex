// Package statemachine 提供有限状态机实现.
// 支持状态转换定义、守卫条件、转换动作、状态进入/离开回调等功能，
// 适用于订单流程、工作流等业务场景.
// 基本用法:
//
//	sm := statemachine.New("pending", []statemachine.Transition{
//	    {From: "pending", Event: "pay",     To: "paid"},
//	    {From: "paid",    Event: "ship",    To: "shipped"},
//	    {From: "shipped", Event: "deliver", To: "delivered"},
//	    {From: "pending", Event: "cancel",  To: "cancelled"},
//	})
//	sm.Fire(ctx, "pay", nil)   // pending → paid
//	sm.Current()               // "paid"
//	sm.Can("ship")             // true
//	sm.Can("cancel")           // false
package statemachine

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// State 状态类型.
type State string

// Event 事件类型.
type Event string

var (
	// ErrInvalidTransition 无效的状态转换.
	ErrInvalidTransition = errors.New("statemachine: invalid transition")
	// ErrGuardRejected 守卫条件拒绝.
	ErrGuardRejected = errors.New("statemachine: guard rejected")
	// ErrActionFailed 转换动作执行失败.
	ErrActionFailed = errors.New("statemachine: action failed")
)

// Transition 状态转换定义.
type Transition struct {
	From   State
	Event  Event
	To     State
	Guard  func(ctx context.Context, data any) bool  // 可选，守卫条件
	Action func(ctx context.Context, data any) error // 可选，转换时执行
}

// transitionCallback 转换回调.
type transitionCallback func(from, to State, event Event)

// stateCallback 状态回调.
type stateCallback func(ctx context.Context, data any)

// Machine 状态机.
type Machine struct {
	mu          sync.RWMutex
	current     State
	transitions []Transition

	onTransition []transitionCallback
	onEnter      map[State][]stateCallback
	onLeave      map[State][]stateCallback
}

// New 创建状态机.
func New(initial State, transitions []Transition) *Machine {
	return &Machine{
		current:     initial,
		transitions: transitions,
		onEnter:     make(map[State][]stateCallback),
		onLeave:     make(map[State][]stateCallback),
	}
}

// Fire 触发事件.
func (m *Machine) Fire(ctx context.Context, event Event, data any) error {
	m.mu.Lock()

	// 查找匹配的转换
	t, ok := m.findTransition(event)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("%w: no transition from %q on event %q", ErrInvalidTransition, m.current, event)
	}

	// 检查守卫条件
	if t.Guard != nil && !t.Guard(ctx, data) {
		m.mu.Unlock()
		return ErrGuardRejected
	}

	// 执行转换动作
	if t.Action != nil {
		if err := t.Action(ctx, data); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("%w: %v", ErrActionFailed, err)
		}
	}

	from := m.current
	to := t.To

	// 复制回调列表，以便在锁外执行
	leaveCbs := make([]stateCallback, len(m.onLeave[from]))
	copy(leaveCbs, m.onLeave[from])
	enterCbs := make([]stateCallback, len(m.onEnter[to]))
	copy(enterCbs, m.onEnter[to])
	transitionCbs := make([]transitionCallback, len(m.onTransition))
	copy(transitionCbs, m.onTransition)

	// 更新状态
	m.current = to
	m.mu.Unlock()

	// 在锁外执行回调，避免持锁时回调阻塞或死锁
	for _, cb := range leaveCbs {
		cb(ctx, data)
	}
	for _, cb := range enterCbs {
		cb(ctx, data)
	}
	for _, cb := range transitionCbs {
		cb(from, to, event)
	}

	return nil
}

// Can 检查当前状态是否可以触发指定事件.
func (m *Machine) Can(event Event) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.findTransition(event)
	return ok
}

// Current 获取当前状态.
func (m *Machine) Current() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// AvailableEvents 获取当前状态可触发的事件列表.
func (m *Machine) AvailableEvents() []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[Event]bool)
	var events []Event
	for _, t := range m.transitions {
		if t.From == m.current && !seen[t.Event] {
			seen[t.Event] = true
			events = append(events, t.Event)
		}
	}
	return events
}

// OnTransition 注册转换回调（所有转换都会触发）.
func (m *Machine) OnTransition(fn func(from, to State, event Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTransition = append(m.onTransition, fn)
}

// OnEnter 注册进入某状态时的回调.
func (m *Machine) OnEnter(state State, fn func(ctx context.Context, data any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEnter[state] = append(m.onEnter[state], fn)
}

// OnLeave 注册离开某状态时的回调.
func (m *Machine) OnLeave(state State, fn func(ctx context.Context, data any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onLeave[state] = append(m.onLeave[state], fn)
}

// findTransition 查找当前状态下匹配事件的转换（调用者需持有锁）.
func (m *Machine) findTransition(event Event) (*Transition, bool) {
	for i := range m.transitions {
		if m.transitions[i].From == m.current && m.transitions[i].Event == event {
			return &m.transitions[i], true
		}
	}
	return nil, false
}
