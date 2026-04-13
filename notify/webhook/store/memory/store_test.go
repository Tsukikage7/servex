// webhook/store/memory/store_test.go
package memory

import (
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/notify/webhook"
)

func TestStore_SaveAndGet(t *testing.T) {
	s := NewStore()
	ctx := t.Context()

	sub := &webhook.Subscription{ID: "s1", URL: "http://example.com", Events: []string{"order.created"}}
	if err := s.Save(ctx, sub); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "http://example.com" {
		t.Errorf("URL = %s", got.URL)
	}
}

func TestStore_ListByEvent(t *testing.T) {
	s := NewStore()
	ctx := t.Context()
	s.Save(ctx, &webhook.Subscription{ID: "s1", Events: []string{"order.created"}})
	s.Save(ctx, &webhook.Subscription{ID: "s2", Events: []string{"user.deleted"}})
	s.Save(ctx, &webhook.Subscription{ID: "s3"}) // 订阅所有事件

	list, _ := s.ListByEvent(ctx, "order.created")
	if len(list) != 2 {
		t.Errorf("got %d, want 2 (s1 + s3)", len(list))
	}
}

func TestStore_Delete(t *testing.T) {
	s := NewStore()
	ctx := t.Context()
	s.Save(ctx, &webhook.Subscription{ID: "s1"})
	s.Delete(ctx, "s1")

	_, err := s.Get(ctx, "s1")
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestStore_ImplementsInterface(t *testing.T) {
	var _ webhook.SubscriptionStore = (*Store)(nil)
}
