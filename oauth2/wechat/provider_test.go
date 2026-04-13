// oauth2/wechat/provider_test.go
package wechat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/oauth2"
)

func TestProvider_AuthURL(t *testing.T) {
	p := NewProvider(WithAppID("wx123"), WithRedirectURL("https://example.com/callback"))
	url := p.AuthURL("state1")
	if !strings.Contains(url, "appid=wx123") {
		t.Error("missing appid")
	}
	if !strings.Contains(url, "redirect_uri=https") {
		t.Error("missing redirect_uri")
	}
	if !strings.HasSuffix(url, "#wechat_redirect") {
		t.Error("missing #wechat_redirect")
	}
}

func TestProvider_Exchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "wx_token", "refresh_token": "wx_refresh",
			"expires_in": 7200, "openid": "oXXXX",
		})
	}))
	defer server.Close()

	p := NewProvider(WithAppID("wx"), WithAppSecret("secret"))
	p.tokenURL = server.URL

	token, err := p.Exchange(t.Context(), "code")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "wx_token" {
		t.Errorf("access_token = %s", token.AccessToken)
	}
}

func TestProvider_UserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"unionid": "u123", "nickname": "微信用户", "headimgurl": "https://pic.wx",
		})
	}))
	defer server.Close()

	p := NewProvider()
	p.userInfoURL = server.URL
	token := &oauth2.Token{AccessToken: "t", Raw: map[string]any{"openid": "o1"}}

	user, err := p.UserInfo(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	if user.Provider != "wechat" {
		t.Errorf("provider = %s", user.Provider)
	}
	if user.Name != "微信用户" {
		t.Errorf("name = %s", user.Name)
	}
}

func TestProvider_ImplementsInterface(t *testing.T) {
	var _ oauth2.Provider = (*Provider)(nil)
}
