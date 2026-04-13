package auth

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	authpb "github.com/Tsukikage7/servex/v2/auth/proto"
)

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

// DiscoveryResult 发现结果.
type DiscoveryResult struct {
	// PublicMethods 公开方法列表.
	PublicMethods []string

	// MethodAuthInfos 所有方法的认证信息.
	MethodAuthInfos map[string]*MethodAuthInfo
}

// DiscoverFromServer 从 gRPC 服务器发现方法的认证配置.
//
// 该函数通过反射读取注册到 gRPC 服务器的所有服务，
// 解析 proto 中定义的 auth options，返回发现结果.
//
// 使用示例:
//
//	server := grpc.NewServer()
//	userService.RegisterGRPC(server)
//
//	result := auth.DiscoverFromServer(server)
//	fmt.Println("Public methods:", result.PublicMethods)
func DiscoverFromServer(server *grpc.Server) *DiscoveryResult {
	result := &DiscoveryResult{
		PublicMethods:   make([]string, 0),
		MethodAuthInfos: make(map[string]*MethodAuthInfo),
	}

	info := server.GetServiceInfo()
	for serviceName, serviceInfo := range info {
		// 获取服务级别的 auth options
		servicePublic, serviceDefaultPerms := getServiceAuthOptions(serviceName)

		for _, method := range serviceInfo.Methods {
			fullMethod := fmt.Sprintf("/%s/%s", serviceName, method.Name)

			// 获取方法级别的 auth options
			methodOpts := getMethodAuthOptions(serviceName, method.Name)

			authInfo := &MethodAuthInfo{
				FullMethod: fullMethod,
			}

			// 方法级别配置优先于服务级别配置
			if methodOpts != nil {
				authInfo.Public = methodOpts.GetPublic()
				authInfo.Permissions = methodOpts.GetPermissions()
				authInfo.AllPermissions = methodOpts.GetAllPermissions()
			} else {
				// 使用服务级别的默认配置
				authInfo.Public = servicePublic
				authInfo.Permissions = serviceDefaultPerms
			}

			result.MethodAuthInfos[fullMethod] = authInfo

			if authInfo.Public {
				result.PublicMethods = append(result.PublicMethods, fullMethod)
			}
		}
	}

	return result
}

// DiscoverPublicMethods 从 gRPC 服务器发现公开方法列表.
//
// 这是 DiscoverFromServer 的便捷方法，仅返回公开方法列表.
func DiscoverPublicMethods(server *grpc.Server) []string {
	return DiscoverFromServer(server).PublicMethods
}

// getServiceAuthOptions 获取服务级别的认证选项.
func getServiceAuthOptions(serviceName string) (public bool, defaultPerms []string) {
	// 通过服务名查找服务描述符
	fd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return false, nil
	}

	sd, ok := fd.(protoreflect.ServiceDescriptor)
	if !ok {
		return false, nil
	}

	// 获取服务选项
	opts := sd.Options()
	if opts == nil {
		return false, nil
	}

	// 解析 auth.service 扩展
	ext := proto.GetExtension(opts, authpb.E_Service)
	if ext == nil {
		return false, nil
	}

	serviceOpts, ok := ext.(*authpb.ServiceAuthOptions)
	if !ok || serviceOpts == nil {
		return false, nil
	}

	return serviceOpts.GetPublic(), serviceOpts.GetDefaultPermissions()
}

// getMethodAuthOptions 获取方法级别的认证选项.
func getMethodAuthOptions(serviceName, methodName string) *authpb.MethodAuthOptions {
	// 通过服务名查找服务描述符
	fd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil
	}

	sd, ok := fd.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil
	}

	// 查找方法描述符
	md := sd.Methods().ByName(protoreflect.Name(methodName))
	if md == nil {
		return nil
	}

	// 获取方法选项
	opts := md.Options()
	if opts == nil {
		return nil
	}

	// 解析 auth.method 扩展
	ext := proto.GetExtension(opts, authpb.E_Method)
	if ext == nil {
		return nil
	}

	methodOpts, ok := ext.(*authpb.MethodAuthOptions)
	if !ok {
		return nil
	}

	return methodOpts
}

// BuildSkipperFromDiscovery 根据发现结果构建 Skipper.
//
// 返回的 Skipper 会跳过所有标记为 public 的方法.
func BuildSkipperFromDiscovery(result *DiscoveryResult) Skipper {
	publicSet := make(map[string]bool, len(result.PublicMethods))
	for _, m := range result.PublicMethods {
		publicSet[m] = true
	}

	return func(ctx context.Context, _ any) bool {
		method, ok := grpc.Method(ctx)
		if !ok {
			return false
		}
		return publicSet[method]
	}
}
