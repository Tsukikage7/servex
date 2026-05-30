package httpserver

import (
	"net/http"
	"slices"
	"strings"
)

// Middleware 是标准 http.Handler 中间件类型.
type Middleware = func(http.Handler) http.Handler

// Router HTTP 路由器，支持路由分组与多级中间件.
//
// 与 httpserver.New 搭配：Router 实现 http.Handler，可直接作为 handler 传入.
//
// 示例：
//
//	router := httpserver.NewRouter()
//
//	// 公开路由
//	router.POST("/login", httpserver.Handle(loginHandler))
//
//	// 带认证的 API 分组继承 router 的所有中间件
//	api := router.Group("/api/v1", jwtMiddleware)
//	api.GET("/users/{id}", httpserver.HandleWith(decodeID, getUser))
//	api.POST("/users", httpserver.Handle(createUser))
//
//	// 嵌套分组 + 额外中间件
//	admin := api.Group("/admin", adminOnlyMiddleware)
//	admin.DELETE("/users/{id}", httpserver.HandleWith(decodeID, deleteUser))
//
//	srv := httpserver.New(router,
//	    httpserver.WithLogger(log),
//	)
type Router struct {
	mux    *http.ServeMux
	prefix string
	mws    []Middleware
}

// NewRouter 创建根路由器，可选传入全局中间件.
func NewRouter(mws ...Middleware) *Router {
	return &Router{
		mux: http.NewServeMux(),
		mws: mws,
	}
}

// Use 向当前路由器追加中间件.
//
// 追加后注册的所有路由都会应用这些中间件.
func (r *Router) Use(mws ...Middleware) {
	r.mws = append(r.mws, mws...)
}

// Group 创建子路由分组.
//
// 子分组继承父路由器的完整中间件链，再叠加 mws 指定的分组级中间件.
// 中间件执行顺序：外层分组 → 内层分组 → 路由级.
func (r *Router) Group(prefix string, mws ...Middleware) *Router {
	return &Router{
		mux:    r.mux,
		prefix: r.prefix + prefix,
		mws:    append(slices.Clone(r.mws), mws...),
	}
}

// Handle 注册任意方法路由.
//
// pattern 格式同 http.ServeMux："/path" 或 "METHOD /path"例如 "GET /users/{id}".
// routeMws 仅对当前路由生效，执行顺序在分组中间件之后.
func (r *Router) Handle(pattern string, h http.Handler, routeMws ...Middleware) {
	r.register(pattern, h, routeMws)
}

// GET 注册 GET 路由.
func (r *Router) GET(path string, h http.Handler, routeMws ...Middleware) {
	r.register("GET "+path, h, routeMws)
}

// POST 注册 POST 路由.
func (r *Router) POST(path string, h http.Handler, routeMws ...Middleware) {
	r.register("POST "+path, h, routeMws)
}

// PUT 注册 PUT 路由.
func (r *Router) PUT(path string, h http.Handler, routeMws ...Middleware) {
	r.register("PUT "+path, h, routeMws)
}

// PATCH 注册 PATCH 路由.
func (r *Router) PATCH(path string, h http.Handler, routeMws ...Middleware) {
	r.register("PATCH "+path, h, routeMws)
}

// DELETE 注册 DELETE 路由.
func (r *Router) DELETE(path string, h http.Handler, routeMws ...Middleware) {
	r.register("DELETE "+path, h, routeMws)
}

// ServeHTTP 实现 http.Handler，使 Router 可直接传入 httpserver.New.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// register 合并中间件并向底层 mux 注册路由.
func (r *Router) register(pattern string, h http.Handler, routeMws []Middleware) {
	all := append(slices.Clone(r.mws), routeMws...)
	// 逆序应用：声明顺序即执行顺序先声明的先触达请求
	for _, mw := range slices.Backward(all) {
		h = mw(h)
	}
	r.mux.Handle(r.fullPattern(pattern), h)
}

// fullPattern 将分组前缀注入到 pattern 中.
//
// 支持 "GET /path" 和 "/path" 两种格式.
func (r *Router) fullPattern(pattern string) string {
	if r.prefix == "" {
		return pattern
	}
	// 带方法前缀："GET /path" → "GET /prefix/path"
	if method, path, ok := strings.Cut(pattern, " "); ok {
		return method + " " + r.prefix + path
	}
	return r.prefix + pattern
}
