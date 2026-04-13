package tenant

import (
	"net/http"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/auth"
)

func TestBearerTokenExtractor(t *testing.T) {
	extractor := BearerTokenExtractor()

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer my-token")

	token, err := extractor(t.Context(), r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "my-token" {
		t.Fatalf("token = %q, want %q", token, "my-token")
	}
}

func TestBearerTokenExtractor_Missing(t *testing.T) {
	extractor := BearerTokenExtractor()
	r, _ := http.NewRequest("GET", "/", nil)

	_, err := extractor(t.Context(), r)
	if err != ErrMissingToken {
		t.Fatalf("err = %v, want ErrMissingToken", err)
	}
}

func TestBearerTokenExtractor_NotHTTP(t *testing.T) {
	extractor := BearerTokenExtractor()
	_, err := extractor(t.Context(), "not-http")
	if err != ErrMissingToken {
		t.Fatalf("err = %v, want ErrMissingToken", err)
	}
}

func TestHeaderTokenExtractor(t *testing.T) {
	extractor := HeaderTokenExtractor("X-Tenant-ID")

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "tenant-123")

	token, err := extractor(t.Context(), r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tenant-123" {
		t.Fatalf("token = %q, want %q", token, "tenant-123")
	}
}

func TestHeaderTokenExtractor_Missing(t *testing.T) {
	extractor := HeaderTokenExtractor("X-Tenant-ID")
	r, _ := http.NewRequest("GET", "/", nil)

	_, err := extractor(t.Context(), r)
	if err != ErrMissingToken {
		t.Fatalf("err = %v, want ErrMissingToken", err)
	}
}

func TestQueryTokenExtractor(t *testing.T) {
	extractor := QueryTokenExtractor("tenant")

	r, _ := http.NewRequest("GET", "/?tenant=xyz", nil)
	token, err := extractor(t.Context(), r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "xyz" {
		t.Fatalf("token = %q, want %q", token, "xyz")
	}
}

func TestQueryTokenExtractor_Missing(t *testing.T) {
	extractor := QueryTokenExtractor("tenant")
	r, _ := http.NewRequest("GET", "/", nil)

	_, err := extractor(t.Context(), r)
	if err != ErrMissingToken {
		t.Fatalf("err = %v, want ErrMissingToken", err)
	}
}

func TestMetadataTokenExtractor(t *testing.T) {
	extractor := MetadataTokenExtractor("x-tenant-token")

	md := metadata.New(map[string]string{"x-tenant-token": "grpc-tenant"})
	ctx := metadata.NewIncomingContext(t.Context(), md)

	token, err := extractor(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "grpc-tenant" {
		t.Fatalf("token = %q, want %q", token, "grpc-tenant")
	}
}

func TestMetadataTokenExtractor_NoMetadata(t *testing.T) {
	extractor := MetadataTokenExtractor("x-tenant-token")
	_, err := extractor(t.Context(), nil)
	if err != ErrMissingToken {
		t.Fatalf("err = %v, want ErrMissingToken", err)
	}
}

func TestPrincipalTokenExtractor(t *testing.T) {
	extractor := PrincipalTokenExtractor()

	ctx := auth.WithPrincipal(t.Context(), &auth.Principal{ID: "user-456"})
	token, err := extractor(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "user-456" {
		t.Fatalf("token = %q, want %q", token, "user-456")
	}
}

func TestPrincipalTokenExtractor_NoPrincipal(t *testing.T) {
	extractor := PrincipalTokenExtractor()
	_, err := extractor(t.Context(), nil)
	if err != ErrMissingToken {
		t.Fatalf("err = %v, want ErrMissingToken", err)
	}
}
