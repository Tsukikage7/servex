package tenantgorm

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/tenant"
)

// testTenant 测试用租户实现.
type testTenant struct {
	id      string
	enabled bool
}

func (t *testTenant) TenantID() string    { return t.id }
func (t *testTenant) TenantEnabled() bool { return t.enabled }

var _ tenant.Tenant = (*testTenant)(nil)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("打开测试 DB 失败: %v", err)
	}
	return db
}

type TestModel struct {
	ID       uint   `gorm:"primaryKey"`
	TenantID string `gorm:"column:tenant_id"`
	Name     string
}

func TestScope(t *testing.T) {
	db := newTestDB(t)
	ctx := tenant.WithTenant(t.Context(), &testTenant{id: "t1", enabled: true})

	stmt := db.Scopes(Scope(ctx)).Find(&TestModel{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "tenant_id") {
		t.Fatalf("SQL 应包含 tenant_id 过滤，got: %s", sql)
	}
}

func TestScope_NoTenant(t *testing.T) {
	db := newTestDB(t)

	stmt := db.Scopes(Scope(t.Context())).Find(&TestModel{}).Statement
	sql := stmt.SQL.String()
	if strings.Contains(sql, "tenant_id") {
		t.Fatalf("无租户时 SQL 不应包含 tenant_id 过滤，got: %s", sql)
	}
}

func TestScope_CustomColumn(t *testing.T) {
	db := newTestDB(t)
	ctx := tenant.WithTenant(t.Context(), &testTenant{id: "t1", enabled: true})

	stmt := db.Scopes(Scope(ctx, "org_id")).Find(&TestModel{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "org_id") {
		t.Fatalf("SQL 应包含自定义列名 org_id，got: %s", sql)
	}
}

func TestAutoInject(t *testing.T) {
	// 使用非 DryRun 模式测试回调注册
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试 DB 失败: %v", err)
	}
	db.AutoMigrate(&TestModel{})

	if err := AutoInject(db); err != nil {
		t.Fatalf("AutoInject 失败: %v", err)
	}

	ctx := tenant.WithTenant(t.Context(), &testTenant{id: "auto-t", enabled: true})
	model := &TestModel{Name: "test"}
	db.WithContext(ctx).Create(model)

	// 验证 tenant_id 被注入
	var result TestModel
	db.First(&result, model.ID)
	if result.TenantID != "auto-t" {
		t.Fatalf("TenantID = %q, want %q", result.TenantID, "auto-t")
	}
}
