// oauth2/github/provider_test.go
package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/oauth2"
)

func TestProvider_AuthURL(t *testing.T) {
	p := NewProvider(
		WithClientID("test-id"),
		WithRedirectURL("http://localhost/callback"),
		WithScopes("user:email"),
	)
	defer p.Close()

	url := p.AuthURL("test-state")
	if !strings.Contains(url, "client_id=test-id") {
		t.Error("missing client_id")
	}
	if !strings.Contains(url, "state=test-state") {
		t.Error("missing state")
	}
	if !strings.Contains(url, "redirect_uri=") {
		t.Error("missing redirect_uri")
	}
	if !strings.Contains(url, "code_challenge=") {
		t.Error("missing code_challenge (PKCE)")
	}
	if !strings.Contains(url, "code_challenge_method=S256") {
		t.Error("missing code_challenge_method (PKCE)")
	}
}

func TestProvider_AuthURL_PKCE_StoresVerifier(t *testing.T) {
	p := NewProvider(WithClientID("test-id"))
	defer p.Close()
	p.AuthURL("state-123")

	p.mu.Lock()
	entry, ok := p.verifiers["state-123"]
	p.mu.Unlock()

	if !ok {
		t.Fatal("verifier should be stored for state")
	}
	if len(entry.verifier) < 43 {
		t.Errorf("verifier length = %d, want >= 43", len(entry.verifier))
	}
}

func TestProvider_Exchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "gho_test123",
			"token_type":   "bearer",
			"scope":        "user:email",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	defer p.Close()

	token, err := p.Exchange(t.Context(), "test-code")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "gho_test123" {
		t.Errorf("access_token = %s", token.AccessToken)
	}
}

func TestProvider_ExchangeWithState_PKCE(t *testing.T) {
	var receivedVerifier string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		receivedVerifier = r.FormValue("code_verifier")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "gho_pkce",
			"token_type":   "bearer",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	defer p.Close()
	// Generate AuthURL to store verifier
	p.AuthURL("pkce-state")

	p.mu.Lock()
	storedVerifier := p.verifiers["pkce-state"].verifier
	p.mu.Unlock()

	token, err := p.ExchangeWithState(t.Context(), "test-code", "pkce-state")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "gho_pkce" {
		t.Errorf("access_token = %s", token.AccessToken)
	}
	if receivedVerifier != storedVerifier {
		t.Errorf("code_verifier mismatch: sent=%q, stored=%q", receivedVerifier, storedVerifier)
	}

	// Verifier should be consumed
	p.mu.Lock()
	_, exists := p.verifiers["pkce-state"]
	p.mu.Unlock()
	if exists {
		t.Error("verifier should be consumed after exchange")
	}
}

func TestProvider_Exchange_EmptyCode(t *testing.T) {
	p := NewProvider()
	defer p.Close()
	_, err := p.Exchange(t.Context(), "")
	if !errors.Is(err, oauth2.ErrInvalidCode) {
		t.Errorf("got %v, want ErrInvalidCode", err)
	}
}

func TestProvider_UserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":         12345,
			"login":      "octocat",
			"email":      "octocat@github.com",
			"avatar_url": "https://avatars.githubusercontent.com/u/12345",
		})
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	defer p.Close()
	token := &oauth2.Token{AccessToken: "test"}

	user, err := p.UserInfo(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	if user.Provider != "github" {
		t.Errorf("provider = %s", user.Provider)
	}
	if user.Name != "octocat" {
		t.Errorf("name = %s", user.Name)
	}
	if user.Email != "octocat@github.com" {
		t.Errorf("email = %s", user.Email)
	}
}

func TestProvider_ImplementsInterface(t *testing.T) {
	var _ oauth2.Provider = (*Provider)(nil)
}

func newTestProvider(baseURL string) *Provider {
	p := NewProvider(
		WithClientID("test-id"),
		WithClientSecret("test-secret"),
	)
	p.tokenURL = baseURL
	p.userInfoURL = baseURL
	return p
}
