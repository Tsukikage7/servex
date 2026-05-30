package openapi

// Operation 描述一个 API 端点用户注册时传入的数据.
type Operation struct {
	Method       string
	Path         string
	Summary      string
	Description  string
	Tags         []string
	OperationID  string
	RequestType  any
	ResponseType any
	ErrorTypes   []any
}

// Builder 链式构建 Operation.
type Builder struct {
	op Operation
}

func newBuilder(method, path string) *Builder {
	return &Builder{op: Operation{Method: method, Path: path}}
}

// GET 创建一个 GET 操作的 Builder.
func GET(path string) *Builder { return newBuilder("GET", path) }

// POST 创建一个 POST 操作的 Builder.
func POST(path string) *Builder { return newBuilder("POST", path) }

// PUT 创建一个 PUT 操作的 Builder.
func PUT(path string) *Builder { return newBuilder("PUT", path) }

// DELETE 创建一个 DELETE 操作的 Builder.
func DELETE(path string) *Builder { return newBuilder("DELETE", path) }

// PATCH 创建一个 PATCH 操作的 Builder.
func PATCH(path string) *Builder { return newBuilder("PATCH", path) }

// Summary 设置操作摘要.
func (b *Builder) Summary(s string) *Builder { b.op.Summary = s; return b }

// Description 设置操作描述.
func (b *Builder) Description(s string) *Builder { b.op.Description = s; return b }

// Tags 添加操作标签.
func (b *Builder) Tags(tags ...string) *Builder { b.op.Tags = append(b.op.Tags, tags...); return b }

// OperationID 设置操作唯一标识.
func (b *Builder) OperationID(id string) *Builder { b.op.OperationID = id; return b }

// Request 设置请求体类型.
func (b *Builder) Request(v any) *Builder { b.op.RequestType = v; return b }

// Response 设置响应体类型.
func (b *Builder) Response(v any) *Builder { b.op.ResponseType = v; return b }

// Errors 设置错误响应类型.
func (b *Builder) Errors(types ...any) *Builder { b.op.ErrorTypes = types; return b }

// Build 构建并返回 Operation.
func (b *Builder) Build() *Operation { op := b.op; return &op }
