package auth

import "strings"

// MethodPolicyProvider 提供 transport 方法对应的授权策略。
type MethodPolicyProvider interface {
	PolicyForMethod(method string) (*MethodAuthInfo, bool)
}

// MethodPolicyMap 是内存版 MethodPolicyProvider。
type MethodPolicyMap map[string]*MethodAuthInfo

// PolicyForMethod 返回指定方法的授权策略。
func (m MethodPolicyMap) PolicyForMethod(method string) (*MethodAuthInfo, bool) {
	if m == nil {
		return nil, false
	}
	policy, ok := m[method]
	return policy, ok
}

func applyMethodPolicy(target Target, provider MethodPolicyProvider) Target {
	if provider == nil || target.Method == "" {
		return target
	}
	policy, ok := provider.PolicyForMethod(target.Method)
	if !ok || policy == nil {
		return target
	}
	if len(policy.Permissions) == 0 {
		return target
	}
	target.Permissions = append([]string(nil), policy.Permissions...)
	target.AllPermissions = policy.AllPermissions
	return target
}

func targetForPermission(base Target, permission string) Target {
	resource, action := splitPermission(permission)
	base.Resource = resource
	base.Action = action
	return base
}

func splitPermission(permission string) (resource, action string) {
	resource, action, ok := strings.Cut(permission, ":")
	if !ok {
		return permission, "*"
	}
	return resource, action
}
