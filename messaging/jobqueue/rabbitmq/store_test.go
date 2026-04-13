// jobqueue/rabbitmq/store_test.go
package rabbitmq

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

func TestNewStore_NilConn(t *testing.T) {
	_, err := NewStore(nil)
	if err == nil {
		t.Fatal("expected error for nil connection")
	}
}

func TestStore_ImplementsInterface(t *testing.T) {
	var _ jobqueue.Store = (*Store)(nil)
}
