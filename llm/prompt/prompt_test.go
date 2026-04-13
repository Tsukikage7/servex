package prompt_test

import (
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/prompt"
)

func TestTemplate_Render(t *testing.T) {
	tmpl, err := prompt.New(llm.RoleUser, "你好，{{.Name}}！今天是 {{.Day}}。")
	if err != nil {
		t.Fatalf("创建模板失败: %v", err)
	}

	data := struct {
		Name string
		Day  string
	}{"Alice", "星期一"}

	msg, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	if msg.Role != llm.RoleUser {
		t.Errorf("期望 Role=user，得到 %s", msg.Role)
	}
	if !strings.Contains(msg.Content, "Alice") {
		t.Errorf("期望内容包含 'Alice'，得到 %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "星期一") {
		t.Errorf("期望内容包含 '星期一'，得到 %q", msg.Content)
	}
}

func TestTemplate_InvalidSyntax(t *testing.T) {
	_, err := prompt.New(llm.RoleSystem, "{{.Unclosed")
	if err == nil {
		t.Fatal("期望无效模板语法报错，得到 nil")
	}
}

func TestMustNew_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("期望 panic，但没有发生")
		}
	}()
	prompt.MustNew(llm.RoleSystem, "{{invalid")
}

func TestTemplate_WithMap(t *testing.T) {
	tmpl := prompt.MustNew(llm.RoleSystem, "语言: {{.lang}}")
	msg, err := tmpl.Render(map[string]string{"lang": "Go"})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if msg.Content != "语言: Go" {
		t.Errorf("期望 '语言: Go'，得到 %q", msg.Content)
	}
}

func TestMustRender_Panics(t *testing.T) {
	tmpl := prompt.MustNew(llm.RoleUser, "{{.Name}}")
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustRender to panic with incompatible data")
		}
	}()
	// Passing a non-struct/non-map that doesn't have .Name should cause execute error.
	tmpl.MustRender(42)
}

func TestMustRender_Success(t *testing.T) {
	tmpl := prompt.MustNew(llm.RoleUser, "Hello {{.Name}}")
	msg := tmpl.MustRender(map[string]string{"Name": "World"})
	if msg.Content != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", msg.Content)
	}
	if msg.Role != llm.RoleUser {
		t.Errorf("expected role user, got %s", msg.Role)
	}
}

func TestTemplate_ComplexConditional(t *testing.T) {
	text := `{{if .Admin}}Admin: {{.Name}}{{else}}User: {{.Name}}{{end}}`
	tmpl := prompt.MustNew(llm.RoleSystem, text)

	// Test admin case.
	msg, err := tmpl.Render(map[string]any{"Admin": true, "Name": "Alice"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if msg.Content != "Admin: Alice" {
		t.Errorf("expected 'Admin: Alice', got %q", msg.Content)
	}

	// Test non-admin case.
	msg, err = tmpl.Render(map[string]any{"Admin": false, "Name": "Bob"})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if msg.Content != "User: Bob" {
		t.Errorf("expected 'User: Bob', got %q", msg.Content)
	}
}

func TestTemplate_RangeLoop(t *testing.T) {
	text := `Items:{{range .Items}} {{.}}{{end}}`
	tmpl := prompt.MustNew(llm.RoleUser, text)

	msg, err := tmpl.Render(map[string]any{"Items": []string{"a", "b", "c"}})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if msg.Content != "Items: a b c" {
		t.Errorf("expected 'Items: a b c', got %q", msg.Content)
	}
}

func TestTemplate_RenderError(t *testing.T) {
	// Template that calls a nonexistent function should fail.
	tmpl := prompt.MustNew(llm.RoleUser, "{{call .Func}}")
	_, err := tmpl.Render(map[string]any{"Func": "not-a-func"})
	if err == nil {
		t.Error("expected render error for invalid call")
	}
}

func TestNew_AllRoles(t *testing.T) {
	roles := []llm.Role{llm.RoleSystem, llm.RoleUser, llm.RoleAssistant}
	for _, role := range roles {
		tmpl, err := prompt.New(role, "test")
		if err != nil {
			t.Fatalf("New(%s) error: %v", role, err)
		}
		msg, err := tmpl.Render(nil)
		if err != nil {
			t.Fatalf("Render error: %v", err)
		}
		if msg.Role != role {
			t.Errorf("expected role %s, got %s", role, msg.Role)
		}
	}
}
