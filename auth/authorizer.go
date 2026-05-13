package auth

import "context"

// AuthorizerFunc 将函数适配为 Authorizer。
type AuthorizerFunc func(ctx context.Context, principal *Principal, target Target) error

// Authorize 实现 Authorizer 接口。
func (f AuthorizerFunc) Authorize(ctx context.Context, principal *Principal, target Target) error {
	return f(ctx, principal, target)
}

// MethodRule 定义一个 gRPC 方法的授权规则。
type MethodRule struct {
	// Method 是 gRPC full method，例如 /package.Service/Method。
	Method string

	// Action 和 Resource 会覆盖传入 Target 的对应字段。
	Action   string
	Resource string

	// Authorizer 是该方法使用的授权器。
	Authorizer Authorizer
}

// MethodAuthorizer 根据 gRPC full method 分发授权规则。
type MethodAuthorizer struct {
	rules map[string]MethodRule
}

// NewMethodAuthorizer 创建按方法分发的授权器。
func NewMethodAuthorizer(rules ...MethodRule) *MethodAuthorizer {
	a := &MethodAuthorizer{rules: make(map[string]MethodRule, len(rules))}
	for _, rule := range rules {
		if rule.Method == "" || rule.Authorizer == nil {
			continue
		}
		a.rules[rule.Method] = rule
	}
	return a
}

// Authorize 实现 Authorizer 接口。未配置规则的方法默认放行。
func (a *MethodAuthorizer) Authorize(ctx context.Context, principal *Principal, target Target) error {
	if a == nil {
		return nil
	}
	rule, ok := a.rules[target.Method]
	if !ok {
		return nil
	}
	if rule.Action != "" {
		target.Action = rule.Action
	}
	if rule.Resource != "" {
		target.Resource = rule.Resource
	}
	return rule.Authorizer.Authorize(ctx, principal, target)
}
