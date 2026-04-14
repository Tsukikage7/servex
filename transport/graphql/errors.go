package graphql

import "github.com/Tsukikage7/servex/v2/errors"

var (
	// ErrNilSchema 表示传入的 schema 为 nil.
	ErrNilSchema = errors.New(60201, "transport.graphql.nil_schema", "schema 不能为空")
	// ErrInvalidRequest 表示请求格式无效.
	ErrInvalidRequest = errors.New(60202, "transport.graphql.invalid_request", "请求格式无效")
	// ErrEmptyQuery 表示查询字符串为空.
	ErrEmptyQuery = errors.New(60203, "transport.graphql.empty_query", "查询字符串为空")
	// ErrInternalError 表示 GraphQL 内部错误（如 panic 恢复后返回）.
	ErrInternalError = errors.New(60204, "transport.graphql.internal_error", "内部错误")
)
