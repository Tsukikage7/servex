package promptgorm_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/prompt"
	promptgorm "github.com/Tsukikage7/servex/v2/llm/prompt/gorm"
)

// newTestDB 创建 sqlite in-memory DB 并 AutoMigrate prompt_versions 表.
// 每个测试一个独立 DB 避免串扰.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := promptgorm.AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// TestGORMStore_SaveAndLoad 验证插入后 LoadAll 能按 Version 升序返回.
func TestGORMStore_SaveAndLoad(t *testing.T) {
	s := promptgorm.NewGORMStore(newTestDB(t))
	ctx := context.Background()

	// 乱序插入 v2、v1，验证 LoadAll 按 Version 升序排序.
	if err := s.Save(ctx, &prompt.Version{Name: "p", Version: 2, Role: llm.RoleSystem, Text: "v2"}); err != nil {
		t.Fatalf("Save v2: %v", err)
	}
	if err := s.Save(ctx, &prompt.Version{Name: "p", Version: 1, Role: llm.RoleSystem, Text: "v1", Active: true}); err != nil {
		t.Fatalf("Save v1: %v", err)
	}

	list, err := s.LoadAll(ctx, "p")
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("期望 2 条，实际=%d", len(list))
	}
	if list[0].Version != 1 || list[1].Version != 2 {
		t.Errorf("期望升序返回 v1 v2，实际=%+v", list)
	}
	if !list[0].Active {
		t.Errorf("期望 v1 Active=true")
	}
}

// TestGORMStore_SaveOverwrites 再次 Save 同 (Name, Version) 会覆盖.
func TestGORMStore_SaveOverwrites(t *testing.T) {
	s := promptgorm.NewGORMStore(newTestDB(t))
	ctx := context.Background()

	if err := s.Save(ctx, &prompt.Version{Name: "p", Version: 1, Text: "v1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(ctx, &prompt.Version{Name: "p", Version: 1, Text: "v1.1"}); err != nil {
		t.Fatalf("Save 覆盖: %v", err)
	}
	list, _ := s.LoadAll(ctx, "p")
	if len(list) != 1 || list[0].Text != "v1.1" {
		t.Errorf("期望 v1.1 覆盖，实际=%+v", list)
	}
}

// TestGORMStore_LoadAllEmpty 不存在的 name 返回 (nil, nil).
func TestGORMStore_LoadAllEmpty(t *testing.T) {
	s := promptgorm.NewGORMStore(newTestDB(t))
	list, err := s.LoadAll(context.Background(), "missing")
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if list != nil {
		t.Errorf("期望 nil，实际=%+v", list)
	}
}

// TestGORMStore_LoadAllNames 按字典序返回所有已注册的 name.
func TestGORMStore_LoadAllNames(t *testing.T) {
	s := promptgorm.NewGORMStore(newTestDB(t))
	ctx := context.Background()
	_ = s.Save(ctx, &prompt.Version{Name: "b", Version: 1, Text: "x"})
	_ = s.Save(ctx, &prompt.Version{Name: "a", Version: 1, Text: "y"})
	_ = s.Save(ctx, &prompt.Version{Name: "a", Version: 2, Text: "y2"}) // 同名多版本只计一次
	names, err := s.LoadAllNames(ctx)
	if err != nil {
		t.Fatalf("LoadAllNames: %v", err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("期望 [a, b]，实际=%v", names)
	}
}

// TestGORMStore_UpdateFlags 验证 Active 与 Weight 更新.
func TestGORMStore_UpdateFlags(t *testing.T) {
	s := promptgorm.NewGORMStore(newTestDB(t))
	ctx := context.Background()
	_ = s.Save(ctx, &prompt.Version{Name: "p", Version: 1, Text: "v1", Active: true})
	if err := s.UpdateFlags(ctx, "p", 1, false, 40); err != nil {
		t.Fatalf("UpdateFlags: %v", err)
	}
	list, _ := s.LoadAll(ctx, "p")
	if list[0].Active || list[0].Weight != 40 {
		t.Errorf("UpdateFlags 未生效，实际=%+v", list[0])
	}
}

// TestGORMStore_UpdateFlags_NotFound 不存在的版本返回 ErrNotFound.
func TestGORMStore_UpdateFlags_NotFound(t *testing.T) {
	s := promptgorm.NewGORMStore(newTestDB(t))
	err := s.UpdateFlags(context.Background(), "p", 1, false, 0)
	if !errors.Is(err, prompt.ErrNotFound) {
		t.Errorf("期望 ErrNotFound，实际=%v", err)
	}
}

// TestGORMStore_IntegrationWithRegistry GORM Store 作为 Registry 的后端，端到端跑一遍.
func TestGORMStore_IntegrationWithRegistry(t *testing.T) {
	store := promptgorm.NewGORMStore(newTestDB(t))
	r, err := prompt.NewRegistry(store)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	ctx := context.Background()
	v1, err := r.Register(ctx, "greet", prompt.MustNew(llm.RoleSystem, "v1 {{.Name}}"))
	if err != nil {
		t.Fatalf("Register v1: %v", err)
	}
	v2, err := r.Register(ctx, "greet", prompt.MustNew(llm.RoleSystem, "v2 {{.Name}}"))
	if err != nil {
		t.Fatalf("Register v2: %v", err)
	}
	if v1 != 1 || v2 != 2 {
		t.Errorf("期望 v1=1 v2=2，实际=%d %d", v1, v2)
	}

	// Get 返回最新版本.
	tmpl, err := r.Get(ctx, "greet")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	msg, _ := tmpl.Render(map[string]string{"Name": "A"})
	if msg.Content != "v2 A" {
		t.Errorf("期望 'v2 A'，实际=%q", msg.Content)
	}

	// 回滚到 v1 并验证持久化：新建 Registry 还能读到正确 Active.
	if err := r.SetActive(ctx, "greet", 1); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	// 重新 open 一个 Registry 共享同一个 store.
	r2, _ := prompt.NewRegistry(store)
	tmpl2, err := r2.Get(ctx, "greet")
	if err != nil {
		t.Fatalf("Get r2: %v", err)
	}
	msg2, _ := tmpl2.Render(map[string]string{"Name": "B"})
	if msg2.Content != "v1 B" {
		t.Errorf("期望回滚持久化 'v1 B'，实际=%q", msg2.Content)
	}
}
