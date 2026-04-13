// webhook/store/gorm/store_test.go
package gorm

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/notify/webhook"
)

func TestNewStore_NilDB(t *testing.T) {
	_, err := NewStore(nil)
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestStore_ImplementsInterface(t *testing.T) {
	var _ webhook.SubscriptionStore = (*Store)(nil)
}
