package auth

import (
	"context"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

const (
	// AuthorizationHeader HTTP Authorization 请求头.
	AuthorizationHeader = "Authorization"
	// BearerPrefix Bearer 令牌前缀.
	BearerPrefix = "Bearer "
	// APIKeyHeader API Key 请求头.
	APIKeyHeader = "X-API-Key"
)

const (
	// GRPCAuthorizationMetadata gRPC authorization 元数据键.
	GRPCAuthorizationMetadata = "authorization"
	// GRPCAPIKeyMetadata gRPC API Key 元数据键.
	GRPCAPIKeyMetadata = "x-api-key"
)

// options 中间件配置.
type options struct {
	authenticator        Authenticator
	authorizer           Authorizer
	credentialsExtractor CredentialsExtractor
	skipper              Skipper
	errorHandler         ErrorHandler
	logger               logger.Logger
	target               Target
	policyProvider       MethodPolicyProvider
}

// Option 中间件配置选项.
type Option func(*options)

// defaultOptions 返回默认配置.
func defaultOptions(authenticator Authenticator) *options {
	return &options{
		authenticator: authenticator,
	}
}

// WithAuthorizer 设置授权器.
func WithAuthorizer(authorizer Authorizer) Option {
	return func(o *options) {
		o.authorizer = authorizer
	}
}

// WithCredentialsExtractor 设置凭据提取器.
func WithCredentialsExtractor(extractor CredentialsExtractor) Option {
	return func(o *options) {
		o.credentialsExtractor = extractor
	}
}

// WithSkipper 设置跳过函数.
func WithSkipper(skipper Skipper) Option {
	return func(o *options) {
		if o.skipper == nil {
			o.skipper = skipper
			return
		}
		previous := o.skipper
		o.skipper = func(ctx context.Context, request any) bool {
			return previous(ctx, request) || skipper(ctx, request)
		}
	}
}

// WithErrorHandler 设置错误处理函数.
func WithErrorHandler(handler ErrorHandler) Option {
	return func(o *options) {
		o.errorHandler = handler
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = log
	}
}

// WithResource 设置授权检查的资源名.
func WithResource(resource string) Option {
	return func(o *options) {
		o.target.Resource = resource
	}
}

// WithAction 设置授权检查的操作名.
func WithAction(action string) Option {
	return func(o *options) {
		o.target.Action = action
	}
}

// WithTarget 设置授权检查目标.
func WithTarget(target Target) Option {
	return func(o *options) {
		o.target = target
	}
}

// WithPolicyProvider 设置方法级认证授权策略提供者.
func WithPolicyProvider(provider MethodPolicyProvider) Option {
	return func(o *options) {
		o.policyProvider = provider
	}
}
