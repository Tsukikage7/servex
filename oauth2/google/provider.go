// Package google 实现 Google OAuth2 登录.
package google

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/oauth2"
)

const (
	defaultAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL    = "https://oauth2.googleapis.com/token"
	defaultUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

	// maxResponseBody 限制响应体最大读取大小 (1MB).
	maxResponseBody = 1 << 20

	// pkceVerifierTTL PKCE code_verifier 的过期时间.
	pkceVerifierTTL = 10 * time.Minute

	// pkceCleanupInterval PKCE code_verifier 清理间隔.
	pkceCleanupInterval = 2 * time.Minute
)

// verifierEntry 带时间戳的 PKCE code_verifier.
type verifierEntry struct {
	verifier  string
	createdAt time.Time
}

// Provider 实现 Google OAuth2 登录.
type Provider struct {
	opts        options
	authBaseURL string
	tokenURL    string
	userInfoURL string

	mu        sync.Mutex
	verifiers map[string]verifierEntry // state -> verifierEntry

	closeOnce sync.Once
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewProvider 创建 Google OAuth2 Provider 实例.
func NewProvider(opts ...Option) *Provider {
	o := options{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		scopes:     []string{"openid", "profile", "email"},
	}
	for _, opt := range opts {
		opt(&o)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Provider{
		opts:        o,
		authBaseURL: defaultAuthURL,
		tokenURL:    defaultTokenURL,
		userInfoURL: defaultUserInfoURL,
		verifiers:   make(map[string]verifierEntry),
		cancel:      cancel,
		done:        make(chan struct{}),
	}
	go p.cleanupLoop(ctx)
	return p
}

// Close 停止后台清理协程.
func (p *Provider) Close() {
	p.closeOnce.Do(func() {
		p.cancel()
		<-p.done
	})
}

// AuthURL 生成 Google OAuth2 授权跳转链接（含 PKCE）.
func (p *Provider) AuthURL(state string, opts ...oauth2.AuthURLOption) string {
	extra := oauth2.ApplyAuthURLOptions(opts)
	params := url.Values{
		"client_id":     {p.opts.clientID},
		"redirect_uri":  {p.opts.redirectURL},
		"response_type": {"code"},
		"state":         {state},
		"access_type":   {"offline"},
	}
	scopes := append(p.opts.scopes, extra.Scopes...)
	params.Set("scope", strings.Join(scopes, " "))
	if extra.Prompt != "" {
		params.Set("prompt", extra.Prompt)
	}

	// PKCE: 生成 code_verifier 和 code_challenge
	verifier := generateCodeVerifier()
	challenge := computeCodeChallenge(verifier)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")

	p.mu.Lock()
	p.verifiers[state] = verifierEntry{verifier: verifier, createdAt: time.Now()}
	p.mu.Unlock()

	return p.authBaseURL + "?" + params.Encode()
}

// Exchange 使用授权码换取访问令牌.
func (p *Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.ExchangeWithState(ctx, code, "")
}

// ExchangeWithState 使用授权码和 state 换取访问令牌（含 PKCE code_verifier）.
func (p *Provider) ExchangeWithState(ctx context.Context, code, state string) (*oauth2.Token, error) {
	if code == "" {
		return nil, oauth2.ErrInvalidCode
	}
	data := url.Values{
		"client_id":     {p.opts.clientID},
		"client_secret": {p.opts.clientSecret},
		"code":          {code},
		"redirect_uri":  {p.opts.redirectURL},
		"grant_type":    {"authorization_code"},
	}

	// PKCE: 如果有对应 state 的 code_verifier 则附加到请求中
	if state != "" {
		p.mu.Lock()
		if entry, ok := p.verifiers[state]; ok {
			data.Set("code_verifier", entry.verifier)
			delete(p.verifiers, state)
		}
		p.mu.Unlock()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.opts.httpClient.Do(req)
	if err != nil {
		return nil, errors.Join(oauth2.ErrExchangeFailed, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, errors.Join(oauth2.ErrExchangeFailed, err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.Join(oauth2.ErrExchangeFailed, err)
	}
	if _, ok := result["error"]; ok {
		return nil, fmt.Errorf("%w: %v", oauth2.ErrExchangeFailed, result["error_description"])
	}

	token := &oauth2.Token{
		AccessToken:  getString(result, "access_token"),
		TokenType:    getString(result, "token_type"),
		RefreshToken: getString(result, "refresh_token"),
		Raw:          result,
	}
	if exp, ok := result["expires_in"].(float64); ok {
		token.ExpiresAt = time.Now().Add(time.Duration(exp) * time.Second)
	}
	if scope := getString(result, "scope"); scope != "" {
		token.Scopes = strings.Split(scope, " ")
	}
	return token, nil
}

// Refresh 使用 refresh token 刷新访问令牌.
func (p *Provider) Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	if refreshToken == "" {
		return nil, oauth2.ErrRefreshFailed
	}
	data := url.Values{
		"client_id":     {p.opts.clientID},
		"client_secret": {p.opts.clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.opts.httpClient.Do(req)
	if err != nil {
		return nil, errors.Join(oauth2.ErrRefreshFailed, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Join(oauth2.ErrRefreshFailed, err)
	}

	token := &oauth2.Token{
		AccessToken:  getString(result, "access_token"),
		TokenType:    getString(result, "token_type"),
		RefreshToken: refreshToken,
		Raw:          result,
	}
	if exp, ok := result["expires_in"].(float64); ok {
		token.ExpiresAt = time.Now().Add(time.Duration(exp) * time.Second)
	}
	return token, nil
}

// UserInfo 获取 Google 用户信息.
func (p *Provider) UserInfo(ctx context.Context, token *oauth2.Token) (*oauth2.UserInfo, error) {
	if token == nil || token.AccessToken == "" {
		return nil, oauth2.ErrInvalidToken
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.opts.httpClient.Do(req)
	if err != nil {
		return nil, errors.Join(oauth2.ErrUserInfoFailed, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Join(oauth2.ErrUserInfoFailed, err)
	}

	return &oauth2.UserInfo{
		ProviderID: getString(result, "id"),
		Provider:   "google",
		Name:       getString(result, "name"),
		Email:      getString(result, "email"),
		AvatarURL:  getString(result, "picture"),
		Extra:      result,
	}, nil
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// generateCodeVerifier 生成 PKCE code_verifier（43-128 字符，URL-safe）.
func generateCodeVerifier() string {
	b := make([]byte, 32) // 32 bytes -> 43 chars base64url
	if _, err := rand.Read(b); err != nil {
		panic("oauth2/google: failed to generate random bytes: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// computeCodeChallenge 计算 PKCE code_challenge = base64url(sha256(verifier)).
func computeCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// cleanupLoop 定期清理过期的 PKCE code_verifier，防止未完成流程导致的内存泄漏.
func (p *Provider) cleanupLoop(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(pkceCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.removeExpiredVerifiers()
		}
	}
}

// removeExpiredVerifiers 删除超过 TTL 的 code_verifier.
func (p *Provider) removeExpiredVerifiers() {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	for state, entry := range p.verifiers {
		if now.Sub(entry.createdAt) > pkceVerifierTTL {
			delete(p.verifiers, state)
		}
	}
}
