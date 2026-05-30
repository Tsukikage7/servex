// Package openapi 提供 OpenAPI 3.1 文档的生成与注册能力.
package openapi

// Spec 表示一个 OpenAPI 3.1 文档.
type Spec struct {
	OpenAPI  string               `json:"openapi" yaml:"openapi"`
	Info     Info                 `json:"info" yaml:"info"`
	Servers  []Server             `json:"servers,omitzero" yaml:"servers,omitempty"`
	Paths    map[string]*PathItem `json:"paths" yaml:"paths"`
	Webhooks map[string]*PathItem `json:"webhooks,omitzero" yaml:"webhooks,omitempty"`
}

// Info 表示 OpenAPI 文档的基本信息.
type Info struct {
	Title       string `json:"title" yaml:"title"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Server 表示 API 的服务器地址.
type Server struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// PathItem 对应一个路径下的所有操作.
type PathItem struct {
	GET    *OperationSpec `json:"get,omitempty" yaml:"get,omitempty"`
	POST   *OperationSpec `json:"post,omitempty" yaml:"post,omitempty"`
	PUT    *OperationSpec `json:"put,omitempty" yaml:"put,omitempty"`
	DELETE *OperationSpec `json:"delete,omitempty" yaml:"delete,omitempty"`
	PATCH  *OperationSpec `json:"patch,omitempty" yaml:"patch,omitempty"`
}

// OperationSpec 对应一个 HTTP 操作的 OpenAPI 描述.
type OperationSpec struct {
	Summary     string               `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID string               `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Tags        []string             `json:"tags,omitzero" yaml:"tags,omitempty"`
	Parameters  []Parameter          `json:"parameters,omitzero" yaml:"parameters,omitempty"`
	RequestBody *RequestBody         `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]*Response `json:"responses" yaml:"responses"`
}

// Parameter 表示 API 操作的一个参数.
type Parameter struct {
	Name        string  `json:"name" yaml:"name"`
	In          string  `json:"in" yaml:"in"` // "query", "path", "header"
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool    `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// RequestBody 表示请求体定义.
type RequestBody struct {
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool                 `json:"required,omitempty" yaml:"required,omitempty"`
	Content     map[string]MediaType `json:"content" yaml:"content"`
}

// Response 表示响应定义.
type Response struct {
	Description string               `json:"description" yaml:"description"`
	Content     map[string]MediaType `json:"content,omitzero" yaml:"content,omitempty"`
}

// MediaType 表示媒体类型及其 Schema.
type MediaType struct {
	Schema *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// Schema 表示 JSON Schema.
type Schema struct {
	Type        string             `json:"type,omitempty" yaml:"type,omitempty"`
	Format      string             `json:"format,omitempty" yaml:"format,omitempty"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitzero" yaml:"properties,omitempty"`
	Required    []string           `json:"required,omitzero" yaml:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty" yaml:"items,omitempty"`
	Enum        []any              `json:"enum,omitzero" yaml:"enum,omitempty"`
	Minimum     *float64           `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum     *float64           `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinLength   *int               `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength   *int               `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
}
