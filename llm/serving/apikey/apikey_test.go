package apikey

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestManager(t *testing.T) (Manager, *MemoryStore) {
	t.Helper()
	store := NewMemoryStore()
	mgr, err := NewManager(store)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}
	return mgr, store
}

func TestManager_Create(t *testing.T) {
	mgr, _ := newTestManager(t)

	rawKey, key, err := mgr.Create(t.Context(),
		WithName("test-key"),
		WithOwnerID("owner1"),
	)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// 原始密钥应以 "sk-" 开头
	if rawKey[:3] != "sk-" {
		t.Errorf("原始密钥前缀错误: got %q, want prefix %q", rawKey[:3], "sk-")
	}

	// 原始密钥长度: "sk-" + 64 hex chars = 67
	if len(rawKey) != 67 {
		t.Errorf("原始密钥长度错误: got %d, want 67", len(rawKey))
	}

	// Key 对象应存在
	if key == nil {
		t.Fatal("Key 对象为 nil")
	}
	if key.ID == "" {
		t.Error("Key ID 为空")
	}
	if key.Name != "test-key" {
		t.Errorf("Key Name 错误: got %q, want %q", key.Name, "test-key")
	}
	if key.HashedKey == "" {
		t.Error("HashedKey 为空")
	}
	if key.HashedKey == rawKey {
		t.Error("HashedKey 不应等于原始密钥")
	}
	if !key.Enabled {
		t.Error("新创建的 Key 应为 Enabled")
	}
}

func TestManager_Validate(t *testing.T) {
	mgr, _ := newTestManager(t)

	rawKey, created, err := mgr.Create(t.Context(),
		WithName("validate-key"),
		WithOwnerID("owner1"),
	)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	key, err := mgr.Validate(t.Context(), rawKey)
	if err != nil {
		t.Fatalf("Validate 失败: %v", err)
	}
	if key.ID != created.ID {
		t.Errorf("验证返回的 Key ID 不匹配: got %q, want %q", key.ID, created.ID)
	}
	if key.LastUsedAt == nil {
		t.Error("验证后 LastUsedAt 应已更新")
	}
}

func TestManager_Validate_Disabled(t *testing.T) {
	mgr, _ := newTestManager(t)

	rawKey, _, err := mgr.Create(t.Context(), WithOwnerID("owner1"))
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// 先验证正常
	key, err := mgr.Validate(t.Context(), rawKey)
	if err != nil {
		t.Fatalf("首次 Validate 失败: %v", err)
	}

	// 撤销后验证
	if err := mgr.Revoke(t.Context(), key.ID); err != nil {
		t.Fatalf("Revoke 失败: %v", err)
	}

	_, err = mgr.Validate(t.Context(), rawKey)
	if err != ErrKeyDisabled {
		t.Errorf("验证已撤销的 Key 应返回 ErrKeyDisabled, got: %v", err)
	}
}

func TestManager_Validate_Expired(t *testing.T) {
	mgr, _ := newTestManager(t)

	past := time.Now().Add(-time.Hour)
	rawKey, _, err := mgr.Create(t.Context(),
		WithOwnerID("owner1"),
		WithExpiresAt(past),
	)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	_, err = mgr.Validate(t.Context(), rawKey)
	if err != ErrKeyExpired {
		t.Errorf("验证已过期的 Key 应返回 ErrKeyExpired, got: %v", err)
	}
}

func TestManager_Validate_QuotaExceeded(t *testing.T) {
	mgr, _ := newTestManager(t)

	rawKey, created, err := mgr.Create(t.Context(),
		WithOwnerID("owner1"),
		WithQuotaLimit(100),
	)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// 先用完配额
	if err := mgr.UpdateQuota(t.Context(), created.ID, 100); err != nil {
		t.Fatalf("UpdateQuota 失败: %v", err)
	}

	_, err = mgr.Validate(t.Context(), rawKey)
	if err != ErrQuotaExceeded {
		t.Errorf("验证超额 Key 应返回 ErrQuotaExceeded, got: %v", err)
	}
}

func TestManager_Revoke(t *testing.T) {
	mgr, store := newTestManager(t)

	_, created, err := mgr.Create(t.Context(), WithOwnerID("owner1"))
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	if err := mgr.Revoke(t.Context(), created.ID); err != nil {
		t.Fatalf("Revoke 失败: %v", err)
	}

	// 从 Store 检查 Enabled 状态
	key, err := store.GetByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("GetByID 失败: %v", err)
	}
	if key.Enabled {
		t.Error("撤销后 Key 应为 Disabled")
	}
}

func TestManager_List(t *testing.T) {
	mgr, _ := newTestManager(t)

	// 为 owner1 创建 2 个 Key
	for range 2 {
		if _, _, err := mgr.Create(t.Context(), WithOwnerID("owner1")); err != nil {
			t.Fatalf("Create 失败: %v", err)
		}
	}
	// 为 owner2 创建 1 个 Key
	if _, _, err := mgr.Create(t.Context(), WithOwnerID("owner2")); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	keys1, err := mgr.List(t.Context(), "owner1")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(keys1) != 2 {
		t.Errorf("owner1 应有 2 个 Key, got %d", len(keys1))
	}

	keys2, err := mgr.List(t.Context(), "owner2")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(keys2) != 1 {
		t.Errorf("owner2 应有 1 个 Key, got %d", len(keys2))
	}
}

func TestManager_UpdateQuota(t *testing.T) {
	mgr, store := newTestManager(t)

	_, created, err := mgr.Create(t.Context(),
		WithOwnerID("owner1"),
		WithQuotaLimit(1000),
	)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	if err := mgr.UpdateQuota(t.Context(), created.ID, 250); err != nil {
		t.Fatalf("UpdateQuota 失败: %v", err)
	}
	if err := mgr.UpdateQuota(t.Context(), created.ID, 150); err != nil {
		t.Fatalf("UpdateQuota 第二次失败: %v", err)
	}

	key, err := store.GetByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("GetByID 失败: %v", err)
	}
	if key.QuotaUsed != 400 {
		t.Errorf("QuotaUsed 应为 400, got %d", key.QuotaUsed)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	mgr, _ := newTestManager(t)

	rawKey, _, err := mgr.Create(t.Context(), WithOwnerID("owner1"))
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// 受保护的 handler
	handler := HTTPMiddleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := FromContext(r.Context())
		if !ok {
			t.Error("context 中应存在 Key")
		}
		if key == nil {
			t.Error("Key 不应为 nil")
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("valid_bearer_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("有效 Bearer Token 应返回 200, got %d", rec.Code)
		}
	})

	t.Run("valid_x_api_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("X-API-Key", rawKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("有效 X-API-Key 应返回 200, got %d", rec.Code)
		}
	})

	t.Run("invalid_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Authorization", "Bearer sk-invalid")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("无效密钥应返回 401, got %d", rec.Code)
		}
	})

	t.Run("missing_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("缺少密钥应返回 401, got %d", rec.Code)
		}
	})
}
