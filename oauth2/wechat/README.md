# oauth2/wechat

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/oauth2/wechat"
```

## 简介

`oauth2/wechat` 提供微信 OAuth2 登录的 `Provider` 实现。封装微信网页授权流程，支持获取授权 URL、通过授权码换取 access_token、刷新令牌及获取微信用户信息（OpenID、昵称、头像等）。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Provider` | 微信 OAuth2 Provider，实现 `oauth2.Provider` |
| `NewProvider(appID, appSecret, redirectURL)` | 创建微信 Provider |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/Tsukikage7/servex/v2/oauth2/state"
    "github.com/Tsukikage7/servex/v2/oauth2/wechat"
)

func main() {
    provider := wechat.NewProvider(
        "wx1234567890abcdef",     // AppID
        "your-wechat-app-secret", // AppSecret
        "https://myapp.example.com/auth/wechat/callback",
    )

    stateStore := state.NewMemoryStore()
    ctx := context.Background()

    http.HandleFunc("/auth/wechat", func(w http.ResponseWriter, r *http.Request) {
        stateToken, _ := stateStore.Generate(ctx)
        authURL := provider.AuthURL(ctx, stateToken)
        http.Redirect(w, r, authURL, http.StatusFound)
    })

    http.HandleFunc("/auth/wechat/callback", func(w http.ResponseWriter, r *http.Request) {
        stateToken := r.URL.Query().Get("state")
        if err := stateStore.Validate(ctx, stateToken); err != nil {
            http.Error(w, "invalid state", http.StatusBadRequest)
            return
        }

        code := r.URL.Query().Get("code")
        token, err := provider.Exchange(ctx, code)
        if err != nil {
            http.Error(w, "exchange failed", http.StatusInternalServerError)
            return
        }

        userInfo, err := provider.UserInfo(ctx, token)
        if err != nil {
            http.Error(w, "get user info failed", http.StatusInternalServerError)
            return
        }

        // userInfo.ID 为 OpenID
        fmt.Fprintf(w, "欢迎，%s！OpenID：%s", userInfo.Name, userInfo.ID)
    })

    http.ListenAndServe(":8080", nil)
}
```
