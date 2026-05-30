package botserver

import "log"

// route 单条路由。
type route struct {
	pattern     string
	handler     HandlerFunc
	middlewares []Middleware
}

// Router 命令路由器。
type Router struct {
	routes     []route
	globalMW   []Middleware
	errHandler func(ctx Context, err error)
}

func newRouter() *Router {
	return &Router{
		errHandler: func(ctx Context, err error) {
			log.Printf("component=BotServer 处理器执行失败 chat=%s error=%v", ctx.ChatID(), err)
		},
	}
}

// NewRouter 创建命令路由器供外部包使用。
func NewRouter() *Router {
	return newRouter()
}

// SetErrorHandler 设置 handler 错误处理函数。
func (r *Router) SetErrorHandler(h func(ctx Context, err error)) {
	r.errHandler = h
}

// Handle 注册路由。
func (r *Router) Handle(pattern string, handler HandlerFunc, middlewares ...Middleware) {
	r.routes = append(r.routes, route{
		pattern:     pattern,
		handler:     handler,
		middlewares: middlewares,
	})
}

// Use 注册全局中间件。
func (r *Router) Use(middlewares ...Middleware) {
	r.globalMW = append(r.globalMW, middlewares...)
}

// Dispatch 分发消息到对应 handler。
// 匹配优先级：精确命令匹配 > 通配符 "*" > 忽略。
func (r *Router) Dispatch(ctx Context) error {
	cmd := ctx.Command()

	var matched *route
	var wildcard *route

	for i := range r.routes {
		rt := &r.routes[i]
		if rt.pattern == "*" {
			wildcard = rt
			continue
		}
		if rt.pattern == cmd {
			matched = rt
			break
		}
	}

	if matched == nil {
		matched = wildcard
	}
	if matched == nil {
		return nil
	}

	all := make([]Middleware, 0, len(r.globalMW)+len(matched.middlewares))
	all = append(all, r.globalMW...)
	all = append(all, matched.middlewares...)

	h := chain(matched.handler, all...)
	if err := h(ctx); err != nil {
		r.errHandler(ctx, err)
		return err
	}
	return nil
}

// chain 构建完整中间件链：中间件按顺序包裹 handler。
func chain(handler HandlerFunc, middlewares ...Middleware) HandlerFunc {
	if len(middlewares) == 0 {
		return handler
	}
	h := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
