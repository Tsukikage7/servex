package grpcx

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/Tsukikage7/servex/v2/auth"
	authpb "github.com/Tsukikage7/servex/v2/auth/proto"
)

// DiscoveryResult 发现结果.
type DiscoveryResult struct {
	// PublicMethods 公开方法列表.
	PublicMethods []string

	// MethodAuthInfos 所有方法的认证信息.
	MethodAuthInfos map[string]*auth.MethodAuthInfo
}

// DiscoverFromServer 从 gRPC 服务器发现方法的认证配置.
//
// 该函数通过反射读取注册到 gRPC 服务器的所有服务，
// 解析 proto 中定义的 auth options，返回发现结果.
func DiscoverFromServer(server *grpc.Server) *DiscoveryResult {
	result := &DiscoveryResult{
		PublicMethods:   make([]string, 0),
		MethodAuthInfos: make(map[string]*auth.MethodAuthInfo),
	}

	info := server.GetServiceInfo()
	for serviceName, serviceInfo := range info {
		servicePublic, serviceDefaultPerms := serviceAuthOptions(serviceName)

		for _, method := range serviceInfo.Methods {
			fullMethod := fmt.Sprintf("/%s/%s", serviceName, method.Name)
			methodOpts := methodAuthOptions(serviceName, method.Name)
			authInfo := &auth.MethodAuthInfo{
				FullMethod: fullMethod,
			}

			if methodOpts != nil {
				authInfo.Public = methodOpts.GetPublic()
				authInfo.Permissions = methodOpts.GetPermissions()
				authInfo.AllPermissions = methodOpts.GetAllPermissions()
			} else {
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

func serviceAuthOptions(serviceName string) (public bool, defaultPerms []string) {
	fd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return false, nil
	}

	sd, ok := fd.(protoreflect.ServiceDescriptor)
	if !ok {
		return false, nil
	}

	opts := sd.Options()
	if opts == nil {
		return false, nil
	}

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

func methodAuthOptions(serviceName, methodName string) *authpb.MethodAuthOptions {
	fd, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil
	}

	sd, ok := fd.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil
	}

	md := sd.Methods().ByName(protoreflect.Name(methodName))
	if md == nil {
		return nil
	}

	opts := md.Options()
	if opts == nil {
		return nil
	}

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
