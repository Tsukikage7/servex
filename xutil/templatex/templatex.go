// Package templatex 提供增强的模板引擎，支持多模板格式和常用函数.
package templatex

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
)

// Option 引擎配置选项.
type Option func(*options)

type options struct {
	funcMap    texttemplate.FuncMap
	leftDelim  string
	rightDelim string
	baseDir    string
}

// WithFuncMap 添加自定义模板函数.
func WithFuncMap(fm texttemplate.FuncMap) Option {
	return func(o *options) {
		if fm != nil {
			o.funcMap = fm
		}
	}
}

// WithDelims 设置模板定界符.
func WithDelims(left, right string) Option {
	return func(o *options) {
		o.leftDelim = left
		o.rightDelim = right
	}
}

// WithBaseDir 设置模板文件基目录.
func WithBaseDir(dir string) Option {
	return func(o *options) {
		o.baseDir = dir
	}
}

// Engine 增强模板引擎，封装 text/template 和 html/template.
type Engine struct {
	opts    *options
	textTpl *texttemplate.Template
	htmlTpl *htmltemplate.Template
	funcMap texttemplate.FuncMap
}

// New 创建模板引擎实例.
func New(opts ...Option) *Engine {
	o := &options{}
	for _, fn := range opts {
		fn(o)
	}

	// 合并内置函数和用户自定义函数.
	fm := builtinFuncMap()
	for k, v := range o.funcMap {
		fm[k] = v
	}

	textTpl := texttemplate.New("").Funcs(fm)
	htmlTpl := htmltemplate.New("").Funcs(htmltemplate.FuncMap(fm))

	if o.leftDelim != "" && o.rightDelim != "" {
		textTpl = textTpl.Delims(o.leftDelim, o.rightDelim)
		htmlTpl = htmlTpl.Delims(o.leftDelim, o.rightDelim)
	}

	return &Engine{
		opts:    o,
		textTpl: textTpl,
		htmlTpl: htmlTpl,
		funcMap: fm,
	}
}

// Render 渲染已解析的命名模板.
func (e *Engine) Render(name string, data any) (string, error) {
	tpl := e.textTpl.Lookup(name)
	if tpl == nil {
		return "", fmt.Errorf("templatex: template %q not found", name)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("templatex: render %q: %w", name, err)
	}
	return buf.String(), nil
}

// RenderString 渲染内联模板字符串.
//
// 注意: 本方法使用 text/template，不会对输出进行 HTML 转义.
// 如果模板内容或数据来源于不可信输入，存在注入风险，
// 请改用 [Engine.RenderHTML] 或 html/template.
func (e *Engine) RenderString(tpl string, data any) (string, error) {
	t := texttemplate.New("inline").Funcs(e.funcMap)

	if e.opts.leftDelim != "" && e.opts.rightDelim != "" {
		t = t.Delims(e.opts.leftDelim, e.opts.rightDelim)
	}

	t, err := t.Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("templatex: parse inline: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("templatex: render inline: %w", err)
	}
	return buf.String(), nil
}

// RenderHTML 使用 HTML 转义渲染已解析的命名模板.
func (e *Engine) RenderHTML(name string, data any) (string, error) {
	tpl := e.htmlTpl.Lookup(name)
	if tpl == nil {
		return "", fmt.Errorf("templatex: html template %q not found", name)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("templatex: render html %q: %w", name, err)
	}
	return buf.String(), nil
}

// ParseFile 解析模板文件.
func (e *Engine) ParseFile(paths ...string) error {
	resolved := make([]string, 0, len(paths))
	for _, p := range paths {
		resolved = append(resolved, e.resolvePath(p))
	}

	// 解析到 text/template.
	if _, err := e.textTpl.ParseFiles(resolved...); err != nil {
		return fmt.Errorf("templatex: parse files (text): %w", err)
	}

	// 解析到 html/template.
	if _, err := e.htmlTpl.ParseFiles(resolved...); err != nil {
		return fmt.Errorf("templatex: parse files (html): %w", err)
	}

	return nil
}

// ParseGlob 使用 glob 模式解析模板文件.
func (e *Engine) ParseGlob(pattern string) error {
	pattern = e.resolvePath(pattern)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("templatex: glob %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("templatex: glob %q matched no files", pattern)
	}

	// 手动解析每个文件以保持名称一致.
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("templatex: read %q: %w", path, err)
		}

		name := filepath.Base(path)
		src := string(content)

		if _, err := e.textTpl.New(name).Parse(src); err != nil {
			return fmt.Errorf("templatex: parse %q (text): %w", name, err)
		}
		if _, err := e.htmlTpl.New(name).Parse(src); err != nil {
			return fmt.Errorf("templatex: parse %q (html): %w", name, err)
		}
	}

	return nil
}

// resolvePath 解析模板文件路径.
func (e *Engine) resolvePath(path string) string {
	if e.opts.baseDir != "" && !strings.HasPrefix(path, "/") {
		return filepath.Join(e.opts.baseDir, path)
	}
	return path
}
