// jobqueue/redis/store_test.go
package redis

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

func TestNewStore_NilClient(t *testing.T) {
	_, err := NewStore(nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestStore_ImplementsInterface(t *testing.T) {
	var _ jobqueue.Store = (*Store)(nil)
}

func TestStoreOptions(t *testing.T) {
	var o options
	WithPrefix("myapp")(&o)
	if o.prefix != "myapp" {
		t.Errorf("got %s, want myapp", o.prefix)
	}
}
