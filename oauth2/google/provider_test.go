// oauth2/google/provider_test.go
package google

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/oauth2"
)

func TestProvider_AuthURL(t *testing.T) {
	p := NewProvider(WithClientID("goog-id"), WithRedirectURL("http://localhost/cb"))
	defer p.Close()
	url := p.AuthURL("s1")
	if !strings.Contains(url, "client_id=goog-id") {
		t.Error("missing client_id")
	}
	if !strings.Contains(url, "response_type=code") {
		t.Error("missing response_type")
	}
	if !strings.Contains(url, "access_type=offline") {
		t.Error("missing access_type")
	}
	if !strings.Contains(url, "code_challenge=") {
		t.Error("missing code_challenge (PKCE)")
	}
	if !strings.Contains(url, "code_challenge_method=S256") {
		t.Error("missing code_challenge_method (PKCE)")
	}
}

func TestProvider_AuthURL_PKCE_StoresVerifier(t *testing.T) {
	p := NewProvider(WithClientID("goog-id"))
	defer p.Close()
	p.AuthURL("state-abc")

	p.mu.Lock()
	entry, ok := p.verifiers["state-abc"]
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
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ya29.test",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "1//test",
		})
	}))
	defer server.Close()

	p := NewProvider(WithClientID("id"), WithClientSecret("secret"))
	defer p.Close()
	p.tokenURL = server.URL

	token, err := p.Exchange(t.Context(), "code")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "ya29.test" {
		t.Errorf("access_token = %s", token.AccessToken)
	}
	if token.RefreshToken != "1//test" {
		t.Errorf("refresh_token = %s", token.RefreshToken)
	}
}

func TestProvider_ExchangeWithState_PKCE(t *testing.T) {
	var receivedVerifier string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		receivedVerifier = r.FormValue("code_verifier")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "ya29.pkce",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	p := NewProvider(WithClientID("id"), WithClientSecret("secret"))
	defer p.Close()
	p.tokenURL = server.URL

	// Generate AuthURL to store verifier
	p.AuthURL("pkce-state")

	p.mu.Lock()
	storedVerifier := p.verifiers["pkce-state"].verifier
	p.mu.Unlock()

	token, err := p.ExchangeWithState(t.Context(), "test-code", "pkce-state")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "ya29.pkce" {
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

func TestProvider_UserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id": "123", "name": "Test User", "email": "test@gmail.com", "picture": "https://pic.test",
		})
	}))
	defer server.Close()

	p := NewProvider()
	defer p.Close()
	p.userInfoURL = server.URL

	user, err := p.UserInfo(t.Context(), &oauth2.Token{AccessToken: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if user.Provider != "google" {
		t.Errorf("provider = %s", user.Provider)
	}
	if user.Email != "test@gmail.com" {
		t.Errorf("email = %s", user.Email)
	}
}

func TestProvider_ImplementsInterface(t *testing.T) {
	var _ oauth2.Provider = (*Provider)(nil)
}
