// jobqueue/database/store_test.go
package database

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

func TestNewStore_NilDB(t *testing.T) {
	_, err := NewStore(nil)
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestStore_ImplementsInterface(t *testing.T) {
	var _ jobqueue.Store = (*Store)(nil)
}
