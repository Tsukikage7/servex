# oauth2/github

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/oauth2/github"
```

## 简介

`oauth2/github` 提供 GitHub OAuth2 登录的 `Provider` 实现。封装 GitHub OAuth2 授权流程，支持获取授权 URL、交换授权码、刷新令牌及获取用户信息（ID、用户名、邮箱、头像）。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Provider` | GitHub OAuth2 Provider，实现 `oauth2.Provider` |
| `NewProvider(clientID, clientSecret, redirectURL)` | 创建 GitHub Provider |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/Tsukikage7/servex/v2/oauth2/github"
    "github.com/Tsukikage7/servex/v2/oauth2/state"
)

func main() {
    provider := github.NewProvider(
        "your-github-client-id",
        "your-github-client-secret",
        "https://myapp.example.com/auth/github/callback",
    )

    stateStore := state.NewMemoryStore()
    ctx := context.Background()

    http.HandleFunc("/auth/github", func(w http.ResponseWriter, r *http.Request) {
        stateToken, _ := stateStore.Generate(ctx)
        authURL := provider.AuthURL(ctx, stateToken)
        http.Redirect(w, r, authURL, http.StatusFound)
    })

    http.HandleFunc("/auth/github/callback", func(w http.ResponseWriter, r *http.Request) {
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

        fmt.Fprintf(w, "欢迎，%s！邮箱：%s", userInfo.Name, userInfo.Email)
    })

    http.ListenAndServe(":8080", nil)
}
```
