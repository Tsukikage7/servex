// Package external 外部服务适配器.
package external

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// UserClient 用户服务 HTTP 客户端，实现 order.UserProvider 接口.
type UserClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewUserClient 创建用户服务客户端.
func NewUserClient(baseURL string) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// userResponse 用户服务响应结构.
type userResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// UserExists 检查用户是否存在.
func (c *UserClient) UserExists(ctx context.Context, userID uint64) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/users/%d", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.FromContext(ctx).With(
			logger.String("url", url),
			logger.Err(err),
		).Error("[UserClient] 请求用户服务失败")
		return false, fmt.Errorf("请求用户服务失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("用户服务返回异常状态码: %d", resp.StatusCode)
	}

	var result userResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("解析用户服务响应失败: %w", err)
	}

	return result.Code == 0, nil
}
