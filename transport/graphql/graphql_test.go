package graphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// newTestSchema 创建用于测试的简单 schema.
func newTestSchema(t *testing.T) gql.Schema {
	t.Helper()
	queryType := gql.NewObject(gql.ObjectConfig{
		Name: "Query",
		Fields: gql.Fields{
			"hello": &gql.Field{
				Type: gql.String,
				Resolve: func(p gql.ResolveParams) (any, error) {
					return "world", nil
				},
			},
		},
	})
	schema, err := gql.NewSchema(gql.SchemaConfig{Query: queryType})
	require.NoError(t, err)
	return schema
}

func TestHandler_Query(t *testing.T) {
	schema := newTestSchema(t)
	srv := New(schema)

	body := `{"query": "{ hello }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var result struct {
		Data struct {
			Hello string `json:"hello"`
		} `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "world", result.Data.Hello)
}

func TestHandler_GET(t *testing.T) {
	schema := newTestSchema(t)
	srv := New(schema)

	req := httptest.NewRequest(http.MethodGet, "/graphql?query={hello}", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result struct {
		Data struct {
			Hello string `json:"hello"`
		} `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "world", result.Data.Hello)
}

func TestHandler_EmptyQuery(t *testing.T) {
	schema := newTestSchema(t)
	srv := New(schema)

	body := `{"query": ""}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_InvalidJSON(t *testing.T) {
	schema := newTestSchema(t)
	srv := New(schema)

	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	schema := newTestSchema(t)
	srv := New(schema)

	req := httptest.NewRequest(http.MethodPut, "/graphql", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestPlayground(t *testing.T) {
	schema := newTestSchema(t)
	srv := New(schema)

	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	rec := httptest.NewRecorder()

	srv.PlaygroundHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "GraphiQL")
}

func TestWrapResolve(t *testing.T) {
	// 测试 RecoveryMiddleware 捕获 panic
	panicResolve := func(p gql.ResolveParams) (any, error) {
		panic("test panic")
	}

	log := &noopLogger{}
	wrapped := WrapResolve(panicResolve, RecoveryMiddleware(log))

	// 使用包含 panic resolve 的 schema 执行测试
	queryType := gql.NewObject(gql.ObjectConfig{
		Name: "Query",
		Fields: gql.Fields{
			"boom": &gql.Field{
				Type:    gql.String,
				Resolve: wrapped,
			},
		},
	})
	schema, err := gql.NewSchema(gql.SchemaConfig{Query: queryType})
	require.NoError(t, err)

	srv := New(schema)

	body := `{"query": "{ boom }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result struct {
		Data   map[string]any   `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	// panic 被恢复后应该返回错误
	assert.NotEmpty(t, result.Errors)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.False(t, cfg.Pretty)
	assert.True(t, cfg.Playground)
	assert.Equal(t, "/graphql", cfg.Path)
}

func TestChainMiddleware(t *testing.T) {
	var order []string

	mw1 := func(next ResolveFunc) ResolveFunc {
		return func(p gql.ResolveParams) (any, error) {
			order = append(order, "mw1-before")
			result, err := next(p)
			order = append(order, "mw1-after")
			return result, err
		}
	}
	mw2 := func(next ResolveFunc) ResolveFunc {
		return func(p gql.ResolveParams) (any, error) {
			order = append(order, "mw2-before")
			result, err := next(p)
			order = append(order, "mw2-after")
			return result, err
		}
	}

	chained := ChainMiddleware(mw1, mw2)
	resolve := chained(func(p gql.ResolveParams) (any, error) {
		order = append(order, "resolve")
		return "ok", nil
	})

	queryType := gql.NewObject(gql.ObjectConfig{
		Name: "Query",
		Fields: gql.Fields{
			"test": &gql.Field{
				Type:    gql.String,
				Resolve: resolve,
			},
		},
	})
	schema, err := gql.NewSchema(gql.SchemaConfig{Query: queryType})
	require.NoError(t, err)

	result := gql.Do(gql.Params{
		Schema:        schema,
		RequestString: "{ test }",
	})
	require.Empty(t, result.Errors)
	assert.Equal(t, []string{"mw1-before", "mw2-before", "resolve", "mw2-after", "mw1-after"}, order)
}

func TestWithOptions(t *testing.T) {
	schema := newTestSchema(t)
	cfg := &Config{Pretty: true, Playground: false, Path: "/api/graphql"}
	log := &noopLogger{}

	srv := New(schema,
		WithConfig(cfg),
		WithLogger(log),
		WithMiddleware(RecoveryMiddleware(log)),
		WithErrorHandler(func(_ context.Context, errs []gqlerrors.FormattedError) []gqlerrors.FormattedError {
			return errs
		}),
	)

	assert.True(t, srv.config.Pretty)
	assert.False(t, srv.config.Playground)
	assert.Equal(t, "/api/graphql", srv.config.Path)
	assert.NotNil(t, srv.log)
	assert.Len(t, srv.middlewares, 1)
	assert.NotNil(t, srv.errorHandler)
}

// noopLogger 用于测试的空日志记录器，实现 logger.Logger 接口.
type noopLogger struct{}

func (n *noopLogger) Debug(_ ...any)                              {}
func (n *noopLogger) Debugf(_ string, _ ...any)                   {}
func (n *noopLogger) Info(_ ...any)                               {}
func (n *noopLogger) Infof(_ string, _ ...any)                    {}
func (n *noopLogger) Warn(_ ...any)                               {}
func (n *noopLogger) Warnf(_ string, _ ...any)                    {}
func (n *noopLogger) Error(_ ...any)                              {}
func (n *noopLogger) Errorf(_ string, _ ...any)                   {}
func (n *noopLogger) Fatal(_ ...any)                              {}
func (n *noopLogger) Fatalf(_ string, _ ...any)                   {}
func (n *noopLogger) Panic(_ ...any)                              {}
func (n *noopLogger) Panicf(_ string, _ ...any)                   {}
func (n *noopLogger) With(_ ...logger.Field) logger.Logger        { return n }
func (n *noopLogger) WithContext(_ context.Context) logger.Logger { return n }
func (n *noopLogger) Sync() error                                 { return nil }
func (n *noopLogger) Close() error                                { return nil }
