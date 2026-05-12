package auth

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/transport/health"
	"google.golang.org/grpc"
)

func TestDiscoverFromServer_NoProtoAuthOptionDefaultsPrivate(t *testing.T) {
	server := grpc.NewServer()
	health.NewGRPCServer(health.New()).Register(server)

	result := DiscoverFromServer(server)

	info, ok := result.MethodAuthInfos["/grpc.health.v1.Health/Check"]
	if !ok {
		t.Fatal("expected health check method to be discovered")
	}
	if info.Public {
		t.Fatal("expected method without proto auth option to stay private")
	}
	if len(info.Permissions) != 0 {
		t.Fatalf("expected no permissions, got %v", info.Permissions)
	}
	if len(result.PublicMethods) != 0 {
		t.Fatalf("expected no discovered public methods, got %v", result.PublicMethods)
	}
}
