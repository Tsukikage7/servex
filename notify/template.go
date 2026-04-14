package notify

import (
	"bytes"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
)

type templateEngine struct {
	templates map[string]*template.Template
}

// TemplateOption 模板引擎配置选项.
type TemplateOption func(*templateEngine)

// WithTemplateDir 从指定目录加载模板文件.
func WithTemplateDir(dir string) TemplateOption {
	return func(e *templateEngine) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			tmpl, err := template.New(name).Parse(string(data))
			if err != nil {
				e.templates[name] = nil
				continue
			}
			e.templates[name] = tmpl
		}
	}
}

// WithTemplateFS 从 fs.FS 文件系统加载模板文件.
func WithTemplateFS(fsys fs.FS, root string) TemplateOption {
	return func(e *templateEngine) {
		sub, err := fs.Sub(fsys, root)
		if err != nil {
			return
		}
		fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(sub, path)
			if err != nil {
				return nil
			}
			tmpl, err := template.New(path).Parse(string(data))
			if err != nil {
				e.templates[path] = nil
				return nil
			}
			e.templates[path] = tmpl
			return nil
		})
	}
}

// NewTemplateEngine 创建模板引擎实例.
func NewTemplateEngine(opts ...TemplateOption) *templateEngine {
	e := &templateEngine{templates: make(map[string]*template.Template)}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *templateEngine) Render(templateID string, data map[string]any) (string, error) {
	tmpl, ok := e.templates[templateID]
	if !ok {
		return "", ErrTemplateNotFound.WithMessage("模板未找到: " + templateID)
	}
	if tmpl == nil {
		return "", ErrTemplateRender.WithMessage("模板渲染失败: " + templateID + " (解析失败)")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", ErrTemplateRender.WithCause(err)
	}
	return buf.String(), nil
}
