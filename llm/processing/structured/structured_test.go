package structured_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/processing/structured"
)

// Person 测试用结构体.
type Person struct {
	Name  string `json:"name" description:"姓名"`
	Age   int    `json:"age" description:"年龄"`
	Email string `json:"email" description:"邮箱"`
}

// mockModel 模拟 ChatModel，按顺序返回预设响应.
type mockModel struct {
	responses []*llm.ChatResponse
	callCount int
}

func (m *mockModel) Generate(ctx context.Context, msgs []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	if m.callCount >= len(m.responses) {
		return nil, fmt.Errorf("no more responses")
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func (m *mockModel) Stream(ctx context.Context, msgs []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// TestSchemaFrom 验证从 Person struct 生成的 JSON Schema 正确性.
func TestSchemaFrom(t *testing.T) {
	schema := structured.SchemaFrom[Person]()

	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("SchemaFrom 返回了无效的 JSON: %v", err)
	}

	// 验证顶层 type 为 object.
	if m["type"] != "object" {
		t.Errorf("期望 type=object，得到 %v", m["type"])
	}

	// 验证 properties 存在.
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("期望 properties 为 map，得到 %T", m["properties"])
	}

	// 验证各字段存在.
	for _, fieldName := range []string{"name", "email"} {
		if _, exists := props[fieldName]; !exists {
			t.Errorf("期望 properties 中包含字段 %q", fieldName)
		}
	}

	// 验证 name 字段为 string 类型且有 description.
	nameProp, ok := props["name"].(map[string]any)
	if !ok {
		t.Fatalf("name 字段的 schema 类型异常: %T", props["name"])
	}
	if nameProp["type"] != "string" {
		t.Errorf("name 字段期望 type=string，得到 %v", nameProp["type"])
	}
	if nameProp["description"] != "姓名" {
		t.Errorf("name 字段期望 description=姓名，得到 %v", nameProp["description"])
	}

	// 验证 age 字段为 integer 类型.
	ageProp, ok := props["age"].(map[string]any)
	if !ok {
		t.Fatalf("age 字段的 schema 类型异常: %T", props["age"])
	}
	if ageProp["type"] != "integer" {
		t.Errorf("age 字段期望 type=integer，得到 %v", ageProp["type"])
	}
	if ageProp["description"] != "年龄" {
		t.Errorf("age 字段期望 description=年龄，得到 %v", ageProp["description"])
	}

	// 验证 required 列表包含所有字段.
	required, ok := m["required"].([]any)
	if !ok {
		t.Fatalf("期望 required 为数组，得到 %T", m["required"])
	}
	if len(required) != 3 {
		t.Errorf("期望 required 包含 3 个字段，得到 %d", len(required))
	}
}

// TestExtract 验证 Extract 成功解析模型返回的有效 JSON.
func TestExtract(t *testing.T) {
	validJSON := `{"name":"张三","age":30,"email":"zhangsan@example.com"}`

	model := &mockModel{
		responses: []*llm.ChatResponse{
			{Message: llm.AssistantMessage(validJSON)},
		},
	}

	person, err := structured.Extract[Person](t.Context(), model, "请告诉我张三的信息")
	if err != nil {
		t.Fatalf("Extract 失败: %v", err)
	}

	if person.Name != "张三" {
		t.Errorf("期望 Name=张三，得到 %q", person.Name)
	}
	if person.Age != 30 {
		t.Errorf("期望 Age=30，得到 %d", person.Age)
	}
	if person.Email != "zhangsan@example.com" {
		t.Errorf("期望 Email=zhangsan@example.com，得到 %q", person.Email)
	}

	// 验证模型只被调用了一次.
	if model.callCount != 1 {
		t.Errorf("期望模型调用 1 次，实际调用 %d 次", model.callCount)
	}
}

// TestExtract_Retry 验证 Extract 在首次返回无效 JSON 时自动重试.
func TestExtract_Retry(t *testing.T) {
	invalidJSON := `这是一段无效的 JSON 输出`
	validJSON := `{"name":"李四","age":25,"email":"lisi@example.com"}`

	model := &mockModel{
		responses: []*llm.ChatResponse{
			{Message: llm.AssistantMessage(invalidJSON)}, // 第一次返回无效 JSON.
			{Message: llm.AssistantMessage(validJSON)},   // 第二次返回有效 JSON.
		},
	}

	person, err := structured.Extract[Person](t.Context(), model, "请告诉我李四的信息",
		structured.WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("Extract 失败: %v", err)
	}

	if person.Name != "李四" {
		t.Errorf("期望 Name=李四，得到 %q", person.Name)
	}
	if person.Age != 25 {
		t.Errorf("期望 Age=25，得到 %d", person.Age)
	}

	// 验证模型被调用了两次（第一次失败，第二次成功）.
	if model.callCount != 2 {
		t.Errorf("期望模型调用 2 次，实际调用 %d 次", model.callCount)
	}
}

// TestExtractFromMessages 验证 ExtractFromMessages 正确使用自定义消息列表.
func TestExtractFromMessages(t *testing.T) {
	validJSON := `{"name":"王五","age":40,"email":"wangwu@example.com"}`

	model := &mockModel{
		responses: []*llm.ChatResponse{
			{Message: llm.AssistantMessage(validJSON)},
		},
	}

	messages := []llm.Message{
		llm.UserMessage("背景：这是一个测试场景"),
		llm.AssistantMessage("好的，我明白了"),
		llm.UserMessage("请告诉我王五的信息"),
	}

	person, err := structured.ExtractFromMessages[Person](t.Context(), model, messages)
	if err != nil {
		t.Fatalf("ExtractFromMessages 失败: %v", err)
	}

	if person.Name != "王五" {
		t.Errorf("期望 Name=王五，得到 %q", person.Name)
	}
	if person.Age != 40 {
		t.Errorf("期望 Age=40，得到 %d", person.Age)
	}
	if person.Email != "wangwu@example.com" {
		t.Errorf("期望 Email=wangwu@example.com，得到 %q", person.Email)
	}
}

// TestExtract_MarkdownCodeBlock 验证 Extract 能正确处理模型在代码块中包裹 JSON 的情况.
func TestExtract_MarkdownCodeBlock(t *testing.T) {
	wrappedJSON := "```json\n{\"name\":\"赵六\",\"age\":35,\"email\":\"zhaoliu@example.com\"}\n```"

	model := &mockModel{
		responses: []*llm.ChatResponse{
			{Message: llm.AssistantMessage(wrappedJSON)},
		},
	}

	person, err := structured.Extract[Person](t.Context(), model, "请告诉我赵六的信息")
	if err != nil {
		t.Fatalf("Extract（代码块格式）失败: %v", err)
	}

	if person.Name != "赵六" {
		t.Errorf("期望 Name=赵六，得到 %q", person.Name)
	}
}

// TestExtract_MaxRetriesExceeded 验证超出最大重试次数时返回错误.
func TestExtract_MaxRetriesExceeded(t *testing.T) {
	invalidJSON := `这不是 JSON`

	model := &mockModel{
		responses: []*llm.ChatResponse{
			{Message: llm.AssistantMessage(invalidJSON)},
			{Message: llm.AssistantMessage(invalidJSON)},
			{Message: llm.AssistantMessage(invalidJSON)},
			{Message: llm.AssistantMessage(invalidJSON)},
		},
	}

	_, err := structured.Extract[Person](t.Context(), model, "测试",
		structured.WithMaxRetries(2),
	)
	if err == nil {
		t.Fatal("期望返回错误，但得到 nil")
	}
}

// --- SchemaFrom with nested struct ---

type Address struct {
	Street string `json:"street" description:"街道"`
	City   string `json:"city" description:"城市"`
}

type PersonWithAddress struct {
	Name    string  `json:"name" description:"姓名"`
	Address Address `json:"address" description:"地址"`
}

func TestSchemaFrom_NestedStruct(t *testing.T) {
	schema := structured.SchemaFrom[PersonWithAddress]()

	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", m["properties"])
	}

	addrProp, ok := props["address"].(map[string]any)
	if !ok {
		t.Fatalf("expected address property, got %T", props["address"])
	}
	if addrProp["type"] != "object" {
		t.Errorf("expected address type=object, got %v", addrProp["type"])
	}

	// Check nested properties.
	nestedProps, ok := addrProp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested properties, got %T", addrProp["properties"])
	}
	if _, ok := nestedProps["street"]; !ok {
		t.Error("expected street field in nested address")
	}
	if _, ok := nestedProps["city"]; !ok {
		t.Error("expected city field in nested address")
	}
}

// --- SchemaFrom with slice fields ---

type PersonWithTags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags" description:"标签列表"`
}

func TestSchemaFrom_SliceField(t *testing.T) {
	schema := structured.SchemaFrom[PersonWithTags]()

	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	props := m["properties"].(map[string]any)
	tagsProp := props["tags"].(map[string]any)

	if tagsProp["type"] != "array" {
		t.Errorf("expected tags type=array, got %v", tagsProp["type"])
	}

	items, ok := tagsProp["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected items schema, got %T", tagsProp["items"])
	}
	if items["type"] != "string" {
		t.Errorf("expected items type=string, got %v", items["type"])
	}
}

// --- SchemaFrom with nested slice of structs ---

type Team struct {
	Name    string   `json:"name"`
	Members []Person `json:"members" description:"团队成员"`
}

func TestSchemaFrom_SliceOfStructs(t *testing.T) {
	schema := structured.SchemaFrom[Team]()

	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	props := m["properties"].(map[string]any)
	membersProp := props["members"].(map[string]any)

	if membersProp["type"] != "array" {
		t.Errorf("expected members type=array, got %v", membersProp["type"])
	}

	items := membersProp["items"].(map[string]any)
	if items["type"] != "object" {
		t.Errorf("expected items type=object, got %v", items["type"])
	}
}

// --- SchemaFrom with primitive types ---

func TestSchemaFrom_String(t *testing.T) {
	schema := structured.SchemaFrom[string]()
	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["type"] != "string" {
		t.Errorf("expected type=string, got %v", m["type"])
	}
}

func TestSchemaFrom_Bool(t *testing.T) {
	schema := structured.SchemaFrom[bool]()
	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["type"] != "boolean" {
		t.Errorf("expected type=boolean, got %v", m["type"])
	}
}

func TestSchemaFrom_Float(t *testing.T) {
	schema := structured.SchemaFrom[float64]()
	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["type"] != "number" {
		t.Errorf("expected type=number, got %v", m["type"])
	}
}

// --- SchemaFrom with json:"-" tag ---

type WithIgnored struct {
	Visible string `json:"visible"`
	Hidden  string `json:"-"`
}

func TestSchemaFrom_IgnoredField(t *testing.T) {
	schema := structured.SchemaFrom[WithIgnored]()
	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	props := m["properties"].(map[string]any)
	if _, ok := props["Hidden"]; ok {
		t.Error("Hidden field should be excluded by json:\"-\"")
	}
	if _, ok := props["visible"]; !ok {
		t.Error("visible field should be present")
	}
}

// --- WithSchemaDescription ---

func TestExtract_WithSchemaDescription(t *testing.T) {
	validJSON := `{"name":"Test","age":1,"email":"t@t.com"}`
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{Message: llm.AssistantMessage(validJSON)},
		},
	}

	person, err := structured.Extract[Person](t.Context(), model, "test",
		structured.WithSchemaDescription("A person object"),
	)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if person.Name != "Test" {
		t.Errorf("expected Name=Test, got %q", person.Name)
	}
}
