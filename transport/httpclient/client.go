// Package httpclient 提供 HTTP 客户端工具.
package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/discovery"
	"github.com/Tsukikage7/servex/v2/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/v2/middleware/retry"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/observability/metrics"
)

const (
	// defaultDiscoveryCacheTTL 服务发现结果缓存时间.
	defaultDiscoveryCacheTTL = 10 * time.Second
)

// Client HTTP 客户端封装.
type Client struct {
	httpClient *http.Client
	opts       *options
	balancer   Balancer

	// 地址缓存
	mu           sync.RWMutex
	cachedAddrs  []string
	lastDiscover time.Time
}

// New 创建 HTTP 客户端，必需设置 serviceName 和 discovery.
func New(opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// 验证必需参数
	if o.serviceName == "" {
		return nil, ErrMissingServiceName
	}
	if o.discovery == nil {
		return nil, ErrMissingDiscovery
	}
	if o.logger == nil {
		o.logger = logger.Nop()
	}
	o.logger = logger.WithComponent(o.logger, "HTTP")

	// 初始服务发现
	addrs, err := o.discovery.Discover(context.Background(), o.serviceName)
	if err != nil {
		return nil, ErrDiscoveryFailed.WithCause(err)
	}
	if len(addrs) == 0 {
		return nil, ErrServiceNotFound.WithMessage(o.serviceName)
	}

	// 构建负载均衡器
	balancer := o.balancer
	if balancer == nil {
		balancer = &RoundRobinBalancer{}
	}

	// 构建 RoundTripper 链（中间件）
	var rt http.RoundTripper = o.transport
	if rt == nil {
		rt = http.DefaultTransport
	}

	// Apply TLS configuration
	rt = applyTLSConfig(rt, o.tlsConfig)

	for i := len(o.middlewares) - 1; i >= 0; i-- {
		rt = o.middlewares[i](rt)
	}

	// 内置中间件（最先注册 = 最外层）
	if o.logger != nil {
		rt = LoggingMiddleware(o.logger)(rt)
	}
	if o.retryCfg != nil {
		rt = RetryMiddleware(o.retryCfg)(rt)
	}
	if o.circuitBreaker != nil {
		rt = CircuitBreakerMiddleware(o.circuitBreaker)(rt)
	}
	if o.tracerName != "" {
		rt = TracingMiddleware(o.tracerName)(rt)
	}
	if o.metricsCollector != nil {
		rt = MetricsMiddleware(o.metricsCollector)(rt)
	}

	httpClient := &http.Client{
		Timeout:   o.timeout,
		Transport: rt,
	}

	// 使用第一个地址记录日志
	baseURL := fmt.Sprintf("%s://%s", o.scheme, addrs[0])
	o.logger.With(
		logger.String("name", o.name),
		logger.String("service", o.serviceName),
		logger.String("baseURL", baseURL),
	).Info("客户端初始化成功")

	return &Client{
		httpClient:   httpClient,
		opts:         o,
		balancer:     balancer,
		cachedAddrs:  addrs,
		lastDiscover: time.Now(),
	}, nil
}

// HTTPClient 返回底层 http.Client.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// BaseURL 返回当前缓存中首个地址的 base URL（仅供参考/日志用途）.
func (c *Client) BaseURL() string {
	c.mu.RLock()
	addrs := c.cachedAddrs
	c.mu.RUnlock()
	if len(addrs) == 0 {
		return ""
	}
	return fmt.Sprintf("%s://%s", c.opts.scheme, addrs[0])
}

// Get 发送 GET 请求.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, path, nil)
}

// Post 发送 POST 请求.
func (c *Client) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, path, body)
}

// Put 发送 PUT 请求.
func (c *Client) Put(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return c.Do(ctx, http.MethodPut, path, body)
}

// Delete 发送 DELETE 请求.
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, path, nil)
}

// Do 执行 HTTP 请求.
//
// 每次调用时检查缓存有效期，过期则重新发现服务地址，通过负载均衡器选择目标节点.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	addr, err := c.pick(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s://%s%s", c.opts.scheme, addr, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, ErrRequestFailed.WithCause(err)
	}

	for key, value := range c.opts.headers {
		req.Header.Set(key, value)
	}

	return c.httpClient.Do(req)
}

// pick 从缓存或重新发现的地址中通过负载均衡器选择一个目标地址.
func (c *Client) pick(ctx context.Context) (string, error) {
	c.mu.RLock()
	addrs := c.cachedAddrs
	lastDiscover := c.lastDiscover
	c.mu.RUnlock()

	// 缓存过期时重新发现
	if time.Since(lastDiscover) > c.opts.cacheTTL {
		newAddrs, err := c.opts.discovery.Discover(ctx, c.opts.serviceName)
		if err == nil && len(newAddrs) > 0 {
			c.mu.Lock()
			c.cachedAddrs = newAddrs
			c.lastDiscover = time.Now()
			addrs = newAddrs
			c.mu.Unlock()
		}
	}

	if len(addrs) == 0 {
		return "", ErrServiceNotFound.WithMessage(c.opts.serviceName)
	}

	return c.balancer.Pick(addrs), nil
}

// Option 配置选项函数.
type Option func(*options)

// options 客户端配置.
type options struct {
	name             string
	serviceName      string
	scheme           string
	discovery        discovery.Discovery
	logger           logger.Logger
	timeout          time.Duration
	headers          map[string]string
	transport        http.RoundTripper
	balancer         Balancer
	middlewares      []Middleware
	baseURL          string // for NewSimple (static base URL)
	retryCfg         *retry.Config
	circuitBreaker   circuitbreaker.CircuitBreaker
	tracerName       string
	metricsCollector metrics.Collector
	tlsConfig        *tls.Config
	cacheTTL         time.Duration // 服务发现缓存 TTL
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		name:     "HTTP-Client",
		scheme:   "http",
		timeout:  30 * time.Second,
		headers:  make(map[string]string),
		cacheTTL: defaultDiscoveryCacheTTL,
	}
}

// WithName 设置客户端名称（用于日志）.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithServiceName 设置目标服务名称（必需）.
func WithServiceName(name string) Option {
	return func(o *options) {
		o.serviceName = name
	}
}

// WithScheme 设置 URL scheme，默认 http.
func WithScheme(scheme string) Option {
	return func(o *options) {
		o.scheme = scheme
	}
}

// WithDiscovery 设置服务发现实例（必需）.
func WithDiscovery(d discovery.Discovery) Option {
	return func(o *options) {
		o.discovery = d
	}
}

// WithLogger 设置日志实例.
func WithLogger(l logger.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

// WithTimeout 设置请求超时.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
	}
}

// WithHeader 添加默认请求头.
func WithHeader(key, value string) Option {
	return func(o *options) {
		o.headers[key] = value
	}
}

// WithHeaders 设置多个默认请求头.
func WithHeaders(headers map[string]string) Option {
	return func(o *options) {
		for k, v := range headers {
			o.headers[k] = v
		}
	}
}

// WithTransport 设置自定义底层 Transport（中间件链的最内层）.
func WithTransport(transport http.RoundTripper) Option {
	return func(o *options) {
		o.transport = transport
	}
}

// WithBalancer 设置负载均衡器，默认 RoundRobinBalancer.
func WithBalancer(b Balancer) Option {
	return func(o *options) {
		o.balancer = b
	}
}

// WithMiddlewares 添加 HTTP 中间件（按添加顺序从外到内执行）.
func WithMiddlewares(mws ...Middleware) Option {
	return func(o *options) {
		o.middlewares = append(o.middlewares, mws...)
	}
}

// WithBaseURL 设置静态 base URL（用于 NewSimple 模式）.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// WithRetry 设置重试配置.
func WithRetry(cfg *retry.Config) Option {
	return func(o *options) { o.retryCfg = cfg }
}

// WithCircuitBreaker 设置熔断器.
func WithCircuitBreaker(cb circuitbreaker.CircuitBreaker) Option {
	return func(o *options) { o.circuitBreaker = cb }
}

// WithTracing 设置链路追踪 tracer 名称.
func WithTracing(tracerName string) Option {
	return func(o *options) { o.tracerName = tracerName }
}

// WithMetrics 设置指标收集器.
func WithMetrics(c metrics.Collector) Option {
	return func(o *options) { o.metricsCollector = c }
}

// WithTLS 设置 TLS 配置.
//
// 如果已通过 WithTransport 设置了自定义 Transport，TLS 配置将应用到该 Transport 上.
// 否则创建新的 http.Transport 并配置 TLS.
func WithTLS(cfg *tls.Config) Option {
	return func(o *options) { o.tlsConfig = cfg }
}

// WithCacheTTL 设置服务发现缓存 TTL，默认 10 秒.
func WithCacheTTL(d time.Duration) Option {
	return func(o *options) { o.cacheTTL = d }
}

// Request 支持 auto JSON 和 per-request headers 的请求描述.
type Request struct {
	Method  string
	Path    string
	Body    any // 自动 JSON 序列化
	Headers map[string]string
	Query   map[string]string
}

// DoRequest 使用 Request 发送请求，返回 *Response.
func (c *Client) DoRequest(ctx context.Context, r *Request) (*Response, error) {
	// Marshal body to JSON if present
	var bodyReader io.Reader
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, ErrMarshalBody.WithCause(err)
		}
		bodyReader = bytes.NewReader(b)
	}

	// Use pick() for service discovery, or baseURL for simple mode
	var url string
	if c.opts.discovery != nil {
		addr, err := c.pick(ctx)
		if err != nil {
			return nil, err
		}
		url = fmt.Sprintf("%s://%s%s", c.opts.scheme, addr, r.Path)
	} else {
		url = c.opts.baseURL + r.Path
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, url, bodyReader)
	if err != nil {
		return nil, ErrRequestFailed.WithCause(err)
	}

	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Default headers
	for key, value := range c.opts.headers {
		req.Header.Set(key, value)
	}

	// Per-request headers (override defaults)
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	// Query parameters
	if len(r.Query) > 0 {
		q := req.URL.Query()
		for k, v := range r.Query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{Response: resp}, nil
}

// NewSimple 创建不需要服务发现的简单客户端.
// 适用于调用外部 API、固定地址服务等场景.
func NewSimple(opts ...Option) *Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	var rt http.RoundTripper = o.transport
	if rt == nil {
		rt = http.DefaultTransport
	}

	// Apply TLS configuration
	rt = applyTLSConfig(rt, o.tlsConfig)

	// 先应用用户自定义中间件（内层），再应用内置中间件（外层），与 New 保持一致
	for i := len(o.middlewares) - 1; i >= 0; i-- {
		rt = o.middlewares[i](rt)
	}

	// Build middleware chain: 内置中间件（最先注册 = 最外层）
	if o.logger != nil {
		rt = LoggingMiddleware(o.logger)(rt)
	}
	if o.retryCfg != nil {
		rt = RetryMiddleware(o.retryCfg)(rt)
	}
	if o.circuitBreaker != nil {
		rt = CircuitBreakerMiddleware(o.circuitBreaker)(rt)
	}
	if o.tracerName != "" {
		rt = TracingMiddleware(o.tracerName)(rt)
	}
	if o.metricsCollector != nil {
		rt = MetricsMiddleware(o.metricsCollector)(rt)
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   o.timeout,
			Transport: rt,
		},
		opts: o,
	}
}

// applyTLSConfig 将 TLS 配置应用到 RoundTripper.
//
// 如果 rt 是 *http.Transport，则 clone 后设置 TLSClientConfig.
// 否则创建新的 http.Transport 包裹 TLS 配置.
func applyTLSConfig(rt http.RoundTripper, tlsCfg *tls.Config) http.RoundTripper {
	if tlsCfg == nil {
		return rt
	}
	if t, ok := rt.(*http.Transport); ok {
		t2 := t.Clone()
		t2.TLSClientConfig = tlsCfg
		return t2
	}
	return &http.Transport{
		TLSClientConfig: tlsCfg,
	}
}
