package i18n

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"golang.org/x/text/language"
)

type I18nTestSuite struct {
	suite.Suite
	tmpDir string
}

func TestI18nSuite(t *testing.T) {
	suite.Run(t, new(I18nTestSuite))
}

func (s *I18nTestSuite) SetupTest() {
	dir, err := os.MkdirTemp("", "i18n-test-*")
	s.Require().NoError(err)
	s.tmpDir = dir
}

func (s *I18nTestSuite) TearDownTest() {
	os.RemoveAll(s.tmpDir)
}

func (s *I18nTestSuite) writeJSON(name, content string) string {
	path := filepath.Join(s.tmpDir, name)
	err := os.WriteFile(path, []byte(content), 0o644)
	s.Require().NoError(err)
	return path
}

func (s *I18nTestSuite) TestNewBundle() {
	b := NewBundle(language.English)
	s.NotNil(b)
	s.Equal(language.English, b.defaultTag)
}

func (s *I18nTestSuite) TestLoadMessageFile() {
	enPath := s.writeJSON("en.json", `{"hello": "Hello", "bye": "Goodbye"}`)
	zhPath := s.writeJSON("zh.json", `{"hello": "你好", "bye": "再见"}`)

	b := NewBundle(language.English)
	s.NoError(b.LoadMessageFile(language.English, enPath))
	s.NoError(b.LoadMessageFile(language.Chinese, zhPath))

	s.Len(b.messages, 2)
}

func (s *I18nTestSuite) TestLoadMessageFile_InvalidJSON() {
	path := s.writeJSON("bad.json", `{invalid}`)

	b := NewBundle(language.English)
	err := b.LoadMessageFile(language.English, path)
	s.Error(err)
}

func (s *I18nTestSuite) TestLoadMessageFile_NotFound() {
	b := NewBundle(language.English)
	err := b.LoadMessageFile(language.English, "/nonexistent/path.json")
	s.Error(err)
}

func (s *I18nTestSuite) TestLoadMessages() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{
		"hello": "Hello",
	})
	b.LoadMessages(language.Chinese, map[string]string{
		"hello": "你好",
	})

	s.Len(b.messages, 2)
}

func (s *I18nTestSuite) TestLoadMessages_CopiesInputMap() {
	messages := map[string]string{"hello": "Hello"}
	b := NewBundle(language.English)
	b.LoadMessages(language.English, messages)

	messages["hello"] = "Mutated"

	loc := b.NewLocalizer("en")
	s.Equal("Hello", loc.Translate("hello"))
}

func (s *I18nTestSuite) TestTranslate_MatchedLanguage() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{"hello": "Hello"})
	b.LoadMessages(language.Chinese, map[string]string{"hello": "你好"})

	loc := b.NewLocalizer("zh")
	s.Equal("你好", loc.Translate("hello"))
}

func (s *I18nTestSuite) TestTranslate_FallbackToDefault() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{"hello": "Hello", "only_en": "English only"})
	b.LoadMessages(language.Chinese, map[string]string{"hello": "你好"})

	loc := b.NewLocalizer("zh")
	// "only_en" 在中文消息中不存在，应回退到英文
	s.Equal("English only", loc.Translate("only_en"))
}

func (s *I18nTestSuite) TestTranslate_MissingKey_ReturnsMessageID() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{"hello": "Hello"})

	loc := b.NewLocalizer("en")
	s.Equal("nonexistent.key", loc.Translate("nonexistent.key"))
}

func (s *I18nTestSuite) TestMustTranslate_MissingKey_ReturnsDefault() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{"hello": "Hello"})

	loc := b.NewLocalizer("en")
	s.Equal("默认消息", loc.MustTranslate("missing.key", "默认消息"))
}

func (s *I18nTestSuite) TestMustTranslate_FoundKey() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{"hello": "Hello"})

	loc := b.NewLocalizer("en")
	s.Equal("Hello", loc.MustTranslate("hello", "should not use this"))
}

func (s *I18nTestSuite) TestTranslate_WithTemplateData() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{
		"greeting": "Hello, {{.Name}}! You have {{.Count}} messages.",
	})

	loc := b.NewLocalizer("en")
	result := loc.Translate("greeting", map[string]any{
		"Name":  "Alice",
		"Count": 3,
	})
	s.Equal("Hello, Alice! You have 3 messages.", result)
}

func (s *I18nTestSuite) TestTranslate_UnknownLanguage_FallsBackToDefault() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{"hello": "Hello"})

	loc := b.NewLocalizer("ja") // 日文未注册
	s.Equal("Hello", loc.Translate("hello"))
}

func (s *I18nTestSuite) TestNewLocalizer_InvalidLanguageTag() {
	b := NewBundle(language.English)
	b.LoadMessages(language.English, map[string]string{"hello": "Hello"})

	// 无效的语言标签应忽略并回退到默认语言
	loc := b.NewLocalizer("not-a-valid-tag-!!!")
	s.Equal("Hello", loc.Translate("hello"))
}

func (s *I18nTestSuite) TestLoadMessageFile_Integration() {
	enPath := s.writeJSON("en.json", `{
		"welcome": "Welcome, {{.User}}!",
		"items": "You have {{.Count}} items."
	}`)
	zhPath := s.writeJSON("zh.json", `{
		"welcome": "欢迎，{{.User}}！",
		"items": "你有 {{.Count}} 个项目。"
	}`)

	b := NewBundle(language.English)
	s.NoError(b.LoadMessageFile(language.English, enPath))
	s.NoError(b.LoadMessageFile(language.Chinese, zhPath))

	enLoc := b.NewLocalizer("en")
	zhLoc := b.NewLocalizer("zh-CN") // zh-CN 应匹配 zh

	data := map[string]any{"User": "Bob", "Count": 5}

	s.Equal("Welcome, Bob!", enLoc.Translate("welcome", data))
	s.Equal("你有 5 个项目。", zhLoc.Translate("items", data))
}
