// Package wechat 实现微信 OAuth2 登录.
package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Tsukikage7/servex/v2/oauth2"
)

const (
	defaultAuthURL     = "https://open.weixin.qq.com/connect/qrconnect"
	defaultTokenURL    = "https://api.weixin.qq.com/sns/oauth2/access_token"
	defaultRefreshURL  = "https://api.weixin.qq.com/sns/oauth2/refresh_token"
	defaultUserInfoURL = "https://api.weixin.qq.com/sns/userinfo"
)

// Provider 实现微信 OAuth2 登录.
type Provider struct {
	opts        options
	authBaseURL string
	tokenURL    string
	refreshURL  string
	userInfoURL string
}

// NewProvider 创建微信 OAuth2 Provider 实例.
func NewProvider(opts ...Option) *Provider {
	o := options{httpClient: &http.Client{Timeout: 10 * time.Second}}
	for _, opt := range opts {
		opt(&o)
	}
	return &Provider{
		opts: o, authBaseURL: defaultAuthURL,
		tokenURL: defaultTokenURL, refreshURL: defaultRefreshURL,
		userInfoURL: defaultUserInfoURL,
	}
}

// AuthURL 生成微信 OAuth2 授权跳转链接.
func (p *Provider) AuthURL(state string, _ ...oauth2.AuthURLOption) string {
	params := url.Values{
		"appid":         {p.opts.appID},
		"redirect_uri":  {p.opts.redirectURL},
		"response_type": {"code"},
		"scope":         {"snsapi_login"},
		"state":         {state},
	}
	return p.authBaseURL + "?" + params.Encode() + "#wechat_redirect"
}

// Exchange 使用授权码换取访问令牌.
func (p *Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if code == "" {
		return nil, oauth2.ErrInvalidCode
	}
	// 微信 API 要求在 URL 中传递 appSecret（而非 Header），这是微信官方接口规范要求。
	// 参考: https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html
	u := fmt.Sprintf("%s?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		p.tokenURL, p.opts.appID, p.opts.appSecret, code)

	var result map[string]any
	if err := p.get(ctx, u, &result); err != nil {
		return nil, errors.Join(oauth2.ErrExchangeFailed, err)
	}
	if _, ok := result["errcode"]; ok {
		return nil, fmt.Errorf("%w: %v", oauth2.ErrExchangeFailed, result["errmsg"])
	}

	token := &oauth2.Token{
		AccessToken:  getString(result, "access_token"),
		RefreshToken: getString(result, "refresh_token"),
		Raw:          result,
	}
	if exp, ok := result["expires_in"].(float64); ok {
		token.ExpiresAt = time.Now().Add(time.Duration(exp) * time.Second)
	}
	return token, nil
}

// Refresh 使用 refresh token 刷新访问令牌.
func (p *Provider) Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	if refreshToken == "" {
		return nil, oauth2.ErrRefreshFailed
	}
	u := fmt.Sprintf("%s?appid=%s&grant_type=refresh_token&refresh_token=%s",
		p.refreshURL, p.opts.appID, refreshToken)

	var result map[string]any
	if err := p.get(ctx, u, &result); err != nil {
		return nil, errors.Join(oauth2.ErrRefreshFailed, err)
	}

	token := &oauth2.Token{
		AccessToken:  getString(result, "access_token"),
		RefreshToken: getString(result, "refresh_token"),
		Raw:          result,
	}
	if exp, ok := result["expires_in"].(float64); ok {
		token.ExpiresAt = time.Now().Add(time.Duration(exp) * time.Second)
	}
	return token, nil
}

// UserInfo 获取微信用户信息.
func (p *Provider) UserInfo(ctx context.Context, token *oauth2.Token) (*oauth2.UserInfo, error) {
	if token == nil || token.AccessToken == "" {
		return nil, oauth2.ErrInvalidToken
	}
	openid := getString(token.Raw, "openid")
	u := fmt.Sprintf("%s?access_token=%s&openid=%s", p.userInfoURL, token.AccessToken, openid)

	var result map[string]any
	if err := p.get(ctx, u, &result); err != nil {
		return nil, errors.Join(oauth2.ErrUserInfoFailed, err)
	}

	return &oauth2.UserInfo{
		ProviderID: getString(result, "unionid"),
		Provider:   "wechat",
		Name:       getString(result, "nickname"),
		AvatarURL:  getString(result, "headimgurl"),
		Extra:      result,
	}, nil
}

func (p *Provider) get(ctx context.Context, url string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.opts.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 检查 HTTP 状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 限制响应体大小为 1MB，防止异常响应导致内存耗尽
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
