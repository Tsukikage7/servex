package auth

// MethodAuthInfo 方法的认证信息.
type MethodAuthInfo struct {
	// FullMethod gRPC 完整方法名，如 "/api.user.v1.AuthService/Login".
	FullMethod string

	// Public 是否为公开方法（无需认证）.
	Public bool

	// Permissions 该方法需要的权限列表.
	Permissions []string

	// AllPermissions 是否需要拥有所有权限（AND 逻辑）.
	AllPermissions bool
}
