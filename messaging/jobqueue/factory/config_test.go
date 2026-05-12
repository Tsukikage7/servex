// jobqueue/factory/config_test.go
package factory

import (
	"context"
	"testing"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

func TestNewStore_NilConfig(t *testing.T) {
	_, err := NewStore(nil)
	if err == nil {
		t.Fatal("期望 nil config 返回错误，实际为 nil")
	}
}

func TestNewStore_EmptyType(t *testing.T) {
	_, err := NewStore(&StoreConfig{})
	if err == nil {
		t.Fatal("期望空 type 返回错误，实际为 nil")
	}
}

func TestNewStore_UnsupportedType(t *testing.T) {
	_, err := NewStore(&StoreConfig{Type: "unsupported"})
	if err == nil {
		t.Fatal("期望不支持的 type 返回错误，实际为 nil")
	}
}

func TestRegisterStore(t *testing.T) {
	const storeType = "test-store"
	err := RegisterStore(storeType, func(cfg *StoreConfig) (jobqueue.Store, error) {
		return nopStore{}, nil
	})
	if err != nil {
		t.Fatalf("注册 store 失败: %v", err)
	}

	store, err := NewStore(&StoreConfig{Type: " TEST-STORE "})
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	if store == nil {
		t.Fatal("期望创建 store，实际为 nil")
	}
}

func TestRegisterStore_Duplicate(t *testing.T) {
	const storeType = "duplicate-store"
	creator := func(cfg *StoreConfig) (jobqueue.Store, error) {
		return nopStore{}, nil
	}
	if err := RegisterStore(storeType, creator); err != nil {
		t.Fatalf("首次注册失败: %v", err)
	}
	if err := RegisterStore(storeType, creator); err == nil {
		t.Fatal("期望重复注册返回错误，实际为 nil")
	}
}

type nopStore struct{}

func (nopStore) Enqueue(context.Context, *jobqueue.Job) error { return nil }
func (nopStore) Dequeue(context.Context, string) (*jobqueue.Job, error) {
	return nil, jobqueue.ErrJobNotFound
}
func (nopStore) MarkRunning(context.Context, string) error                 { return nil }
func (nopStore) MarkFailed(context.Context, string, error) error           { return nil }
func (nopStore) MarkDead(context.Context, string) error                    { return nil }
func (nopStore) MarkDone(context.Context, string) error                    { return nil }
func (nopStore) Requeue(context.Context, *jobqueue.Job) error              { return nil }
func (nopStore) ListDead(context.Context, string) ([]*jobqueue.Job, error) { return nil, nil }
func (nopStore) Close() error                                              { return nil }
