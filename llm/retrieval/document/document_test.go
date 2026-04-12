package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTextLoader 测试从字符串加载单个文档.
func TestTextLoader(t *testing.T) {
	content := "这是一段测试文本内容"
	loader := NewTextLoader(strings.NewReader(content))

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, content, docs[0].Content)
	assert.Equal(t, "0", docs[0].ID)
}

// TestCSVLoader 测试读取含 content 和 title 列的 3 行 CSV.
func TestCSVLoader(t *testing.T) {
	csvContent := `content,title,author
第一篇文章内容,标题一,作者甲
第二篇文章内容,标题二,作者乙
第三篇文章内容,标题三,作者丙`

	loader := NewCSVLoader(
		strings.NewReader(csvContent),
		WithCSVMetadataColumns("title", "author"),
	)

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 3)

	assert.Equal(t, "第一篇文章内容", docs[0].Content)
	assert.Equal(t, "标题一", docs[0].Metadata["title"])
	assert.Equal(t, "作者甲", docs[0].Metadata["author"])

	assert.Equal(t, "第二篇文章内容", docs[1].Content)
	assert.Equal(t, "标题二", docs[1].Metadata["title"])

	assert.Equal(t, "第三篇文章内容", docs[2].Content)
	assert.Equal(t, "标题三", docs[2].Metadata["title"])
}

// TestJSONLoader 测试读取包含 2 个对象的 JSON 数组.
func TestJSONLoader(t *testing.T) {
	jsonContent := `[
		{"content": "第一篇文章", "title": "标题一", "category": "技术"},
		{"content": "第二篇文章", "title": "标题二", "category": "生活"}
	]`

	loader := NewJSONLoader(
		strings.NewReader(jsonContent),
		WithJSONMetadataFields("title", "category"),
	)

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 2)

	assert.Equal(t, "第一篇文章", docs[0].Content)
	assert.Equal(t, "标题一", docs[0].Metadata["title"])
	assert.Equal(t, "技术", docs[0].Metadata["category"])

	assert.Equal(t, "第二篇文章", docs[1].Content)
	assert.Equal(t, "标题二", docs[1].Metadata["title"])
}

// TestJSONLoader_JSONL 测试读取 JSONL 格式（每行一个 JSON 对象）.
func TestJSONLoader_JSONL(t *testing.T) {
	jsonlContent := `{"content": "JSONL 第一条记录", "source": "news"}
{"content": "JSONL 第二条记录", "source": "blog"}
{"content": "JSONL 第三条记录", "source": "forum"}`

	loader := NewJSONLoader(
		strings.NewReader(jsonlContent),
		WithJSONMetadataFields("source"),
	)

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 3)

	assert.Equal(t, "JSONL 第一条记录", docs[0].Content)
	assert.Equal(t, "news", docs[0].Metadata["source"])

	assert.Equal(t, "JSONL 第二条记录", docs[1].Content)
	assert.Equal(t, "blog", docs[1].Metadata["source"])

	assert.Equal(t, "JSONL 第三条记录", docs[2].Content)
}

// TestMarkdownLoader 测试按标题分割含 3 个章节的 Markdown 文档.
func TestMarkdownLoader(t *testing.T) {
	mdContent := `## 第一章

第一章的内容描述。

## 第二章

第二章的内容描述，包含更多细节。

### 第二章第一节

第二章第一节的具体内容。`

	loader := NewMarkdownLoader(strings.NewReader(mdContent))

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 3)

	assert.Equal(t, "第一章", docs[0].Metadata["heading"])
	assert.Contains(t, docs[0].Content, "第一章的内容描述")

	assert.Equal(t, "第二章", docs[1].Metadata["heading"])
	assert.Contains(t, docs[1].Content, "第二章的内容描述")

	assert.Equal(t, "第二章第一节", docs[2].Metadata["heading"])
	assert.Contains(t, docs[2].Content, "第二章第一节的具体内容")
}

// TestDirectoryLoader 测试遍历包含 2 个 .txt 文件的临时目录.
func TestDirectoryLoader(t *testing.T) {
	// 创建临时目录.
	tmpDir := t.TempDir()

	// 写入 2 个测试文件.
	file1 := filepath.Join(tmpDir, "doc1.txt")
	file2 := filepath.Join(tmpDir, "doc2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("文件一的内容"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("文件二的内容"), 0o644))

	// 写入一个不匹配的文件，确认不被加载.
	file3 := filepath.Join(tmpDir, "doc3.md")
	require.NoError(t, os.WriteFile(file3, []byte("Markdown 文件内容"), 0o644))

	loader := NewDirectoryLoader(tmpDir, "*.txt")

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 2)

	// 验证文档内容.
	contentSet := make(map[string]bool)
	for _, doc := range docs {
		contentSet[doc.Content] = true
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Metadata["source"])
		assert.NotEmpty(t, doc.Metadata["filename"])
	}
	assert.True(t, contentSet["文件一的内容"])
	assert.True(t, contentSet["文件二的内容"])
}

// TestWithMetadata 测试用户自定义元数据与加载器元数据正确合并.
func TestWithMetadata(t *testing.T) {
	userMeta := map[string]any{
		"project":  "servex",
		"language": "zh",
	}
	loader := NewTextLoader(
		strings.NewReader("元数据合并测试内容"),
		WithMetadata(userMeta),
	)

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 1)

	// 验证用户元数据存在.
	assert.Equal(t, "servex", docs[0].Metadata["project"])
	assert.Equal(t, "zh", docs[0].Metadata["language"])
	// 验证加载器生成的元数据也存在.
	assert.NotNil(t, docs[0].Metadata["source"])
}

// TestWithIDPrefix 测试文档 ID 前缀正确应用.
func TestWithIDPrefix(t *testing.T) {
	csvContent := `content,title
内容甲,标题甲
内容乙,标题乙`

	loader := NewCSVLoader(
		strings.NewReader(csvContent),
		WithIDPrefix("doc_"),
	)

	docs, err := loader.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, docs, 2)

	assert.Equal(t, "doc_0", docs[0].ID)
	assert.Equal(t, "doc_1", docs[1].ID)
}
