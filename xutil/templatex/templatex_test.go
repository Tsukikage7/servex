package templatex

import (
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderString(t *testing.T) {
	e := New()

	result, err := e.RenderString("Hello, {{.Name}}!", map[string]string{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)
}

func TestRenderStringBuiltinFuncs(t *testing.T) {
	e := New()

	tests := []struct {
		name string
		tpl  string
		data any
		want string
	}{
		{"upper", `{{upper .}}`, "hello", "HELLO"},
		{"lower", `{{lower .}}`, "HELLO", "hello"},
		{"title", `{{title .}}`, "hello world", "Hello World"},
		{"trim", `{{trim .}}`, "  hi  ", "hi"},
		{"contains", `{{contains "go" .}}`, "golang", "true"},
		{"replace", `{{replace "a" "b" .}}`, "aaa", "bbb"},
		{"join", `{{join ", " .}}`, []string{"a", "b", "c"}, "a, b, c"},
		{"split", `{{split "," .}}`, "a,b,c", "[a b c]"},
		{"default_used", `{{default "fallback" .}}`, "", "fallback"},
		{"default_not_used", `{{default "fallback" .}}`, "value", "value"},
		{"indent", `{{indent 4 .}}`, "line1\nline2", "    line1\n    line2"},
		{"plural_one", `{{plural 1 "item" "items"}}`, nil, "item"},
		{"plural_many", `{{plural 3 "item" "items"}}`, nil, "items"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.RenderString(tt.tpl, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestRenderStringDate(t *testing.T) {
	e := New()
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	result, err := e.RenderString(`{{date "2006-01-02" .}}`, tm)
	require.NoError(t, err)
	assert.Equal(t, "2024-01-15", result)
}

func TestRenderStringToJSON(t *testing.T) {
	e := New()

	result, err := e.RenderString(`{{toJSON .}}`, map[string]int{"a": 1})
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, result)
}

func TestRenderStringFromJSON(t *testing.T) {
	e := New()

	result, err := e.RenderString(`{{$m := fromJSON .}}{{$m}}`, `{"key":"val"}`)
	require.NoError(t, err)
	assert.Contains(t, result, "key")
}

func TestRenderNotFound(t *testing.T) {
	e := New()

	_, err := e.Render("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRenderHTMLNotFound(t *testing.T) {
	e := New()

	_, err := e.RenderHTML("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestParseFileAndRender(t *testing.T) {
	dir := t.TempDir()

	// 写入模板文件.
	tplPath := filepath.Join(dir, "greeting.tpl")
	err := os.WriteFile(tplPath, []byte("Hello, {{.Name}}!"), 0o644)
	require.NoError(t, err)

	e := New()
	err = e.ParseFile(tplPath)
	require.NoError(t, err)

	result, err := e.Render("greeting.tpl", map[string]string{"Name": "Go"})
	require.NoError(t, err)
	assert.Equal(t, "Hello, Go!", result)
}

func TestParseFileAndRenderHTML(t *testing.T) {
	dir := t.TempDir()

	tplPath := filepath.Join(dir, "page.html")
	err := os.WriteFile(tplPath, []byte("<p>{{.Content}}</p>"), 0o644)
	require.NoError(t, err)

	e := New()
	err = e.ParseFile(tplPath)
	require.NoError(t, err)

	result, err := e.RenderHTML("page.html", map[string]string{"Content": "<script>alert('xss')</script>"})
	require.NoError(t, err)
	assert.Contains(t, result, "&lt;script&gt;")
	assert.NotContains(t, result, "<script>")
}

func TestParseGlob(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"a.tpl", "b.tpl"} {
		err := os.WriteFile(filepath.Join(dir, name), []byte("Template "+name), 0o644)
		require.NoError(t, err)
	}

	e := New()
	err := e.ParseGlob(filepath.Join(dir, "*.tpl"))
	require.NoError(t, err)

	result, err := e.Render("a.tpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "Template a.tpl", result)
}

func TestParseGlobNoMatch(t *testing.T) {
	e := New()
	err := e.ParseGlob("/nonexistent/path/*.tpl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "matched no files")
}

func TestWithBaseDir(t *testing.T) {
	dir := t.TempDir()

	tplPath := filepath.Join(dir, "test.tpl")
	err := os.WriteFile(tplPath, []byte("base dir works"), 0o644)
	require.NoError(t, err)

	e := New(WithBaseDir(dir))
	err = e.ParseFile("test.tpl")
	require.NoError(t, err)

	result, err := e.Render("test.tpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "base dir works", result)
}

func TestWithDelims(t *testing.T) {
	e := New(WithDelims("<<", ">>"))

	result, err := e.RenderString("Hello, <<.Name>>!", map[string]string{"Name": "Custom"})
	require.NoError(t, err)
	assert.Equal(t, "Hello, Custom!", result)
}

func TestWithFuncMap(t *testing.T) {
	fm := template.FuncMap{
		"double": func(s string) string { return s + s },
	}

	e := New(WithFuncMap(fm))

	result, err := e.RenderString(`{{double .}}`, "ab")
	require.NoError(t, err)
	assert.Equal(t, "abab", result)
}

func TestWithFuncMapNil(t *testing.T) {
	// 不应 panic.
	e := New(WithFuncMap(nil))
	result, err := e.RenderString(`{{upper .}}`, "test")
	require.NoError(t, err)
	assert.Equal(t, "TEST", result)
}

func TestRenderStringInvalid(t *testing.T) {
	e := New()
	_, err := e.RenderString(`{{.invalid`, nil)
	assert.Error(t, err)
}

func TestIndentEmptyLines(t *testing.T) {
	e := New()
	result, err := e.RenderString(`{{indent 2 .}}`, "a\n\nb")
	require.NoError(t, err)
	assert.Equal(t, "  a\n\n  b", result)
}

func TestDefaultWithNil(t *testing.T) {
	e := New()
	result, err := e.RenderString(`{{default "none" .Val}}`, map[string]any{"Val": nil})
	require.NoError(t, err)
	assert.Equal(t, "none", result)
}

func TestParseGlobWithBaseDir(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "x.tpl"), []byte("X"), 0o644)
	require.NoError(t, err)

	e := New(WithBaseDir(dir))
	err = e.ParseGlob("*.tpl")
	require.NoError(t, err)

	result, err := e.Render("x.tpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "X", result)
}
