// Package document 提供文档加载器，将各种文件格式转换为 rag.Document，供 RAG 管线使用.
package document

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// 预定义错误.
var (
	// ErrEmptyContent 文档内容为空.
	ErrEmptyContent = errors.New("document: empty content")
	// ErrInvalidFormat 文档格式无效.
	ErrInvalidFormat = errors.New("document: invalid format")
)

// Loader 文档加载器接口.
type Loader interface {
	Load(ctx context.Context) ([]rag.Document, error)
}

// Option 加载器配置项.
type Option func(*options)

type options struct {
	// metadata 用户自定义元数据，会与加载器生成的元数据合并.
	metadata map[string]any
	// idPrefix 文档 ID 前缀.
	idPrefix string
	// csvContentCol CSV 内容列名，默认 "content".
	csvContentCol string
	// csvMetadataCols CSV 元数据列名列表.
	csvMetadataCols []string
	// jsonContentField JSON 内容字段名，默认 "content".
	jsonContentField string
	// jsonMetadataFields JSON 元数据字段名列表.
	jsonMetadataFields []string
}

// defaultOptions 返回默认配置.
func defaultOptions() options {
	return options{
		csvContentCol:    "content",
		jsonContentField: "content",
	}
}

// applyOptions 将 Option 列表应用到默认配置.
func applyOptions(opts []Option) options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithMetadata 设置用户自定义元数据，会与加载器生成的元数据合并.
func WithMetadata(meta map[string]any) Option {
	return func(o *options) { o.metadata = meta }
}

// WithIDPrefix 设置文档 ID 前缀.
func WithIDPrefix(prefix string) Option {
	return func(o *options) { o.idPrefix = prefix }
}

// WithCSVContentColumn 设置 CSV 中作为文档内容的列名（默认 "content"）.
func WithCSVContentColumn(col string) Option {
	return func(o *options) { o.csvContentCol = col }
}

// WithCSVMetadataColumns 设置 CSV 中作为文档元数据的列名列表.
func WithCSVMetadataColumns(cols ...string) Option {
	return func(o *options) { o.csvMetadataCols = cols }
}

// WithJSONContentField 设置 JSON 中作为文档内容的字段名（默认 "content"）.
func WithJSONContentField(field string) Option {
	return func(o *options) { o.jsonContentField = field }
}

// WithJSONMetadataFields 设置 JSON 中作为文档元数据的字段名列表.
func WithJSONMetadataFields(fields ...string) Option {
	return func(o *options) { o.jsonMetadataFields = fields }
}

// mergeMeta 将加载器生成的元数据与用户自定义元数据合并，用户元数据优先.
func mergeMeta(loaderMeta map[string]any, userMeta map[string]any) map[string]any {
	if len(loaderMeta) == 0 && len(userMeta) == 0 {
		return nil
	}
	merged := make(map[string]any, len(loaderMeta)+len(userMeta))
	for k, v := range loaderMeta {
		merged[k] = v
	}
	for k, v := range userMeta {
		merged[k] = v
	}
	return merged
}

// ──────────────────────────────────────────
// TextLoader
// ──────────────────────────────────────────

type textLoader struct {
	reader io.Reader
	opts   options
}

// NewTextLoader 从 io.Reader 读取全部内容，返回单个 Document 的加载器.
func NewTextLoader(reader io.Reader, opts ...Option) Loader {
	return &textLoader{reader: reader, opts: applyOptions(opts)}
}

// NewTextFileLoader 从文件路径读取全部内容，返回单个 Document 的加载器.
func NewTextFileLoader(path string, opts ...Option) Loader {
	return &fileLoader{path: path, opts: applyOptions(opts)}
}

func (l *textLoader) Load(_ context.Context) ([]rag.Document, error) {
	data, err := io.ReadAll(l.reader)
	if err != nil {
		return nil, fmt.Errorf("document: 读取内容失败: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, ErrEmptyContent
	}
	doc := rag.Document{
		ID:       l.opts.idPrefix + "0",
		Content:  content,
		Metadata: mergeMeta(map[string]any{"source": "reader"}, l.opts.metadata),
	}
	return []rag.Document{doc}, nil
}

// fileLoader 从文件路径加载文本内容.
type fileLoader struct {
	path string
	opts options
}

func (l *fileLoader) Load(_ context.Context) ([]rag.Document, error) {
	f, err := os.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("document: 打开文件失败: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("document: 读取文件失败: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, ErrEmptyContent
	}
	filename := filepath.Base(l.path)
	doc := rag.Document{
		ID:      l.opts.idPrefix + filename,
		Content: content,
		Metadata: mergeMeta(map[string]any{
			"source":   l.path,
			"filename": filename,
		}, l.opts.metadata),
	}
	return []rag.Document{doc}, nil
}

// ──────────────────────────────────────────
// CSVLoader
// ──────────────────────────────────────────

type csvLoader struct {
	reader io.Reader
	opts   options
}

// NewCSVLoader 读取 CSV 文件，每行生成一个 Document.
// 内容来自指定列（默认 "content"），元数据来自其他指定列.
func NewCSVLoader(reader io.Reader, opts ...Option) Loader {
	return &csvLoader{reader: reader, opts: applyOptions(opts)}
}

func (l *csvLoader) Load(_ context.Context) ([]rag.Document, error) {
	r := csv.NewReader(l.reader)

	// 读取表头.
	headers, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEmptyContent
		}
		return nil, fmt.Errorf("%w: %s", ErrInvalidFormat, err.Error())
	}

	// 建立列名到索引的映射.
	colIndex := make(map[string]int, len(headers))
	for i, h := range headers {
		colIndex[h] = i
	}

	// 检查内容列是否存在.
	contentIdx, ok := colIndex[l.opts.csvContentCol]
	if !ok {
		return nil, fmt.Errorf("%w: 内容列 %q 不存在", ErrInvalidFormat, l.opts.csvContentCol)
	}

	// 确定元数据列的索引.
	metaCols := l.opts.csvMetadataCols
	if len(metaCols) == 0 {
		// 默认将非内容列全部作为元数据.
		for _, h := range headers {
			if h != l.opts.csvContentCol {
				metaCols = append(metaCols, h)
			}
		}
	}

	var docs []rag.Document
	rowNum := 0
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: 第 %d 行解析失败: %s", ErrInvalidFormat, rowNum+1, err.Error())
		}

		if contentIdx >= len(record) {
			rowNum++
			continue
		}
		content := strings.TrimSpace(record[contentIdx])
		if content == "" {
			rowNum++
			continue
		}

		// 收集元数据.
		loaderMeta := map[string]any{
			"source":     "csv",
			"row_number": rowNum,
		}
		for _, col := range metaCols {
			if idx, ok := colIndex[col]; ok && idx < len(record) {
				loaderMeta[col] = record[idx]
			}
		}

		docs = append(docs, rag.Document{
			ID:       fmt.Sprintf("%s%d", l.opts.idPrefix, rowNum),
			Content:  content,
			Metadata: mergeMeta(loaderMeta, l.opts.metadata),
		})
		rowNum++
	}

	if len(docs) == 0 {
		return nil, ErrEmptyContent
	}
	return docs, nil
}

// ──────────────────────────────────────────
// JSONLoader
// ──────────────────────────────────────────

type jsonLoader struct {
	reader io.Reader
	opts   options
}

// NewJSONLoader 读取 JSON 数组或 JSONL（每行一个 JSON 对象），每个对象生成一个 Document.
// 内容来自指定字段（默认 "content"），元数据来自其他指定字段.
func NewJSONLoader(reader io.Reader, opts ...Option) Loader {
	return &jsonLoader{reader: reader, opts: applyOptions(opts)}
}

func (l *jsonLoader) Load(_ context.Context) ([]rag.Document, error) {
	data, err := io.ReadAll(l.reader)
	if err != nil {
		return nil, fmt.Errorf("document: 读取内容失败: %w", err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, ErrEmptyContent
	}

	var objects []map[string]any

	// 尝试解析为 JSON 数组.
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &objects); err != nil {
			return nil, fmt.Errorf("%w: JSON 数组解析失败: %s", ErrInvalidFormat, err.Error())
		}
	} else {
		// 尝试按行解析 JSONL.
		scanner := bufio.NewScanner(strings.NewReader(trimmed))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				return nil, fmt.Errorf("%w: JSONL 行解析失败: %s", ErrInvalidFormat, err.Error())
			}
			objects = append(objects, obj)
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("document: 读取行失败: %w", err)
		}
	}

	if len(objects) == 0 {
		return nil, ErrEmptyContent
	}

	var docs []rag.Document
	for i, obj := range objects {
		contentVal, ok := obj[l.opts.jsonContentField]
		if !ok {
			continue
		}
		content := strings.TrimSpace(fmt.Sprintf("%v", contentVal))
		if content == "" {
			continue
		}

		// 收集元数据.
		loaderMeta := map[string]any{
			"source": "json",
			"index":  i,
		}
		for _, field := range l.opts.jsonMetadataFields {
			if val, ok := obj[field]; ok {
				loaderMeta[field] = val
			}
		}

		docs = append(docs, rag.Document{
			ID:       fmt.Sprintf("%s%d", l.opts.idPrefix, i),
			Content:  content,
			Metadata: mergeMeta(loaderMeta, l.opts.metadata),
		})
	}

	if len(docs) == 0 {
		return nil, ErrEmptyContent
	}
	return docs, nil
}

// ──────────────────────────────────────────
// MarkdownLoader
// ──────────────────────────────────────────

type markdownLoader struct {
	reader io.Reader
	opts   options
}

// NewMarkdownLoader 按标题（## 和 ###）分割 Markdown 内容，每节生成一个 Document.
// 标题文本作为元数据存储.
func NewMarkdownLoader(reader io.Reader, opts ...Option) Loader {
	return &markdownLoader{reader: reader, opts: applyOptions(opts)}
}

// mdSection 表示 Markdown 中的一个章节.
type mdSection struct {
	heading string
	level   int
	lines   []string
}

func (l *markdownLoader) Load(_ context.Context) ([]rag.Document, error) {
	data, err := io.ReadAll(l.reader)
	if err != nil {
		return nil, fmt.Errorf("document: 读取内容失败: %w", err)
	}

	text := string(data)
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyContent
	}

	// 按行扫描，按 ## 或 ### 标题分节.
	var sections []mdSection
	var current *mdSection

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		level, heading, isHeading := parseMarkdownHeading(line)
		if isHeading {
			// 保存上一节（如果有内容）.
			if current != nil {
				sections = append(sections, *current)
			}
			current = &mdSection{heading: heading, level: level}
		} else if current != nil {
			current.lines = append(current.lines, line)
		} else {
			// 标题前的内容归入默认节.
			if current == nil {
				current = &mdSection{heading: "", level: 0}
			}
			current.lines = append(current.lines, line)
		}
	}
	// 保存最后一节.
	if current != nil {
		sections = append(sections, *current)
	}

	var docs []rag.Document
	docIdx := 0
	for _, sec := range sections {
		content := strings.TrimSpace(strings.Join(sec.lines, "\n"))
		if content == "" {
			continue
		}

		loaderMeta := map[string]any{
			"source": "markdown",
			"index":  docIdx,
		}
		if sec.heading != "" {
			loaderMeta["heading"] = sec.heading
			loaderMeta["heading_level"] = sec.level
		}

		docs = append(docs, rag.Document{
			ID:       fmt.Sprintf("%s%d", l.opts.idPrefix, docIdx),
			Content:  content,
			Metadata: mergeMeta(loaderMeta, l.opts.metadata),
		})
		docIdx++
	}

	if len(docs) == 0 {
		return nil, ErrEmptyContent
	}
	return docs, nil
}

// parseMarkdownHeading 解析 Markdown 标题行，返回标题级别、标题文本和是否为标题.
// 仅识别 ## 和 ### 级别的标题.
func parseMarkdownHeading(line string) (level int, heading string, ok bool) {
	if strings.HasPrefix(line, "### ") {
		return 3, strings.TrimPrefix(line, "### "), true
	}
	if strings.HasPrefix(line, "## ") {
		return 2, strings.TrimPrefix(line, "## "), true
	}
	return 0, "", false
}

// ──────────────────────────────────────────
// DirectoryLoader
// ──────────────────────────────────────────

type directoryLoader struct {
	dir  string
	glob string
	opts options
}

// NewDirectoryLoader 遍历目录，按 glob 模式匹配文件，将每个文件作为文本 Document 加载.
func NewDirectoryLoader(dir string, glob string, opts ...Option) Loader {
	return &directoryLoader{dir: dir, glob: glob, opts: applyOptions(opts)}
}

func (l *directoryLoader) Load(ctx context.Context) ([]rag.Document, error) {
	var docs []rag.Document

	err := filepath.WalkDir(l.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// 按 glob 模式匹配文件名.
		filename := d.Name()
		if l.glob != "" {
			matched, err := filepath.Match(l.glob, filename)
			if err != nil {
				return fmt.Errorf("document: glob 模式匹配失败: %w", err)
			}
			if !matched {
				return nil
			}
		}

		// 读取文件内容.
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("document: 打开文件失败 %s: %w", path, err)
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("document: 读取文件失败 %s: %w", path, err)
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			return nil
		}

		loaderMeta := map[string]any{
			"source":   path,
			"filename": filename,
		}

		docs = append(docs, rag.Document{
			ID:       l.opts.idPrefix + filename,
			Content:  content,
			Metadata: mergeMeta(loaderMeta, l.opts.metadata),
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("document: 遍历目录失败: %w", err)
	}

	if len(docs) == 0 {
		return nil, ErrEmptyContent
	}
	return docs, nil
}
