// Package graphql 提供基于 graphql-go 的 GraphQL 服务器适配器（code-first）.
package graphql

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"

	"github.com/Tsukikage7/servex/observability/logger"
)

// Config GraphQL 服务器配置.
type Config struct {
	// Pretty 是否格式化 JSON 输出.
	Pretty bool `json:"pretty" yaml:"pretty" mapstructure:"pretty"`
	// Playground 是否启用 GraphiQL playground.
	Playground bool `json:"playground" yaml:"playground" mapstructure:"playground"`
	// Path GraphQL 端点路径.
	Path string `json:"path" yaml:"path" mapstructure:"path"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Pretty:     false,
		Playground: true,
		Path:       "/graphql",
	}
}

// ErrorHandlerFunc 自定义错误处理函数.
type ErrorHandlerFunc func(ctx context.Context, errs []gqlerrors.FormattedError) []gqlerrors.FormattedError

// Option 选项函数.
type Option func(*Server)

// WithConfig 设置配置.
func WithConfig(cfg *Config) Option {
	return func(s *Server) {
		if cfg != nil {
			s.config = cfg
		}
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(s *Server) {
		s.log = log
	}
}

// WithMiddleware 添加 resolve 层中间件.
func WithMiddleware(mw ...Middleware) Option {
	return func(s *Server) {
		s.middlewares = append(s.middlewares, mw...)
	}
}

// WithErrorHandler 设置自定义错误处理函数.
func WithErrorHandler(fn ErrorHandlerFunc) Option {
	return func(s *Server) {
		s.errorHandler = fn
	}
}

// Server GraphQL 服务器.
type Server struct {
	schema       gql.Schema
	config       *Config
	log          logger.Logger
	middlewares  []Middleware
	errorHandler ErrorHandlerFunc
}

// New 创建 GraphQL 服务器实例.
func New(schema gql.Schema, opts ...Option) *Server {
	s := &Server{
		schema: schema,
		config: DefaultConfig(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// graphqlRequest 表示 GraphQL 请求体.
type graphqlRequest struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
	OperationName string         `json:"operationName"`
}

// Handler 返回处理 GraphQL query/mutation 的 http.Handler.
// 支持 POST（JSON body）和 GET（query params）两种方式.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest

		switch r.Method {
		case http.MethodPost:
			// 限制请求体大小（10MB），防止 OOM
			const maxBodySize = 10 << 20
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.writeError(w, ErrInvalidRequest, http.StatusBadRequest)
				return
			}
		case http.MethodGet:
			req.Query = r.URL.Query().Get("query")
			req.OperationName = r.URL.Query().Get("operationName")
			// GET 请求中 variables 以 JSON 字符串传递
			if v := r.URL.Query().Get("variables"); v != "" {
				_ = json.Unmarshal([]byte(v), &req.Variables)
			}
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if req.Query == "" {
			s.writeError(w, ErrEmptyQuery, http.StatusBadRequest)
			return
		}

		result := gql.Do(gql.Params{
			Schema:         s.schema,
			RequestString:  req.Query,
			VariableValues: req.Variables,
			OperationName:  req.OperationName,
			Context:        r.Context(),
		})

		// 应用自定义错误处理
		if s.errorHandler != nil && len(result.Errors) > 0 {
			result.Errors = s.errorHandler(r.Context(), result.Errors)
		}

		w.Header().Set("Content-Type", "application/json")

		encoder := json.NewEncoder(w)
		if s.config.Pretty {
			encoder.SetIndent("", "  ")
		}

		if err := encoder.Encode(result); err != nil && s.log != nil {
			s.log.With(logger.Field{Key: "error", Value: err.Error()}).
				Error("graphql: 编码响应失败")
		}
	})
}

// PlaygroundHandler 返回 GraphiQL playground 页面的 http.Handler.
func (s *Server) PlaygroundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(playgroundHTML(s.config.Path)))
	})
}

// writeError 向客户端写入错误响应.
func (s *Server) writeError(w http.ResponseWriter, err error, status int) {
	if s.log != nil {
		s.log.Error("GraphQL 请求处理失败", logger.Err(err))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := map[string]any{
		"errors": []map[string]any{
			{"message": "内部服务器错误"},
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// playgroundTmpl GraphiQL playground HTML 模板，endpoint 已预先 JSON 编码.
var playgroundTmpl = template.Must(template.New("graphiql").Parse(`<!DOCTYPE html>
<html>
<head>
  <title>GraphiQL</title>
  <style>
    body { height: 100%; margin: 0; width: 100%; overflow: hidden; }
    #graphiql { height: 100vh; }
  </style>
  <link rel="stylesheet" href="https://unpkg.com/graphiql/graphiql.min.css" />
</head>
<body>
  <div id="graphiql">Loading...</div>
  <script crossorigin src="https://unpkg.com/react/umd/react.production.min.js"></script>
  <script crossorigin src="https://unpkg.com/react-dom/umd/react-dom.production.min.js"></script>
  <script crossorigin src="https://unpkg.com/graphiql/graphiql.min.js"></script>
  <script>
    const graphQLFetcher = graphQLParams =>
      fetch({{.Endpoint}}, {
        method: 'post',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(graphQLParams),
      }).then(response => response.json());
    ReactDOM.render(
      React.createElement(GraphiQL, { fetcher: graphQLFetcher }),
      document.getElementById('graphiql'),
    );
  </script>
</body>
</html>`))

// playgroundHTML 生成 GraphiQL playground 的 HTML 页面，endpoint 经过 JSON 编码防止 XSS.
func playgroundHTML(endpoint string) string {
	// 使用 JSON 编码确保在 JavaScript 上下文中安全转义
	endpointJSON, _ := json.Marshal(endpoint)
	var b strings.Builder
	_ = playgroundTmpl.Execute(&b, struct{ Endpoint string }{Endpoint: string(endpointJSON)})
	return b.String()
}
