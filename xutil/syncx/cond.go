package syncx

import (
	"context"
	"sync"
)

// Cond 带 context 的条件变量，支持在等待时响应 context 取消/超时.
// 零值不可用，需通过 NewCond 创建.
type Cond struct {
	l       sync.Locker
	mu      sync.Mutex
	waiters []chan struct{}
}

// NewCond 创建一个与锁 l 关联的 Cond.
func NewCond(l sync.Locker) *Cond {
	return &Cond{l: l}
}

// subscribe 注册并返回一个等待 channel.
func (c *Cond) subscribe() chan struct{} {
	ch := make(chan struct{})
	c.mu.Lock()
	c.waiters = append(c.waiters, ch)
	c.mu.Unlock()
	return ch
}

// unsubscribe 从等待列表中移除指定 channelctx 取消时调用.
func (c *Cond) unsubscribe(ch chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, w := range c.waiters {
		if w == ch {
			c.waiters = append(c.waiters[:i], c.waiters[i+1:]...)
			return
		}
	}
}

// Wait 释放锁，等待 Signal 或 Broadcast 通知，或者 ctx 完成.
// 被唤醒后重新获取锁再返回.
// 若 ctx 超时或取消，返回 ctx.Err()；否则返回 nil.
func (c *Cond) Wait(ctx context.Context) error {
	ch := c.subscribe()

	// 解锁，让其他 goroutine 能调用 Signal/Broadcast
	c.l.Unlock()

	var err error
	select {
	case <-ch:
		// 收到信号，channel 已被 Signal/Broadcast 关闭
	case <-ctx.Done():
		err = ctx.Err()
		c.unsubscribe(ch)
		// unsubscribe 和 Signal 之间可能存在竞态：Signal 可能在 unsubscribe 之前
		// 已取出 ch 并准备 close。尝试消费 ch 避免丢失唤醒信号.
		select {
		case <-ch:
			// 信号在 unsubscribe 之前已发出，优先返回成功
			err = nil
		default:
		}
	}

	// 重新加锁
	c.l.Lock()
	return err
}

// Signal 唤醒一个正在等待的 goroutine.
func (c *Cond) Signal() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.waiters) > 0 {
		ch := c.waiters[0]
		c.waiters = c.waiters[1:]
		close(ch)
	}
}

// Broadcast 唤醒所有正在等待的 goroutine.
func (c *Cond) Broadcast() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.waiters {
		close(ch)
	}
	c.waiters = nil
}
