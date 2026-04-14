package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// esClient Elasticsearch 客户端实现.
type esClient struct {
	client *es.Client
	log    logger.Logger
}

// newESClient 创建 Elasticsearch 客户端.
func newESClient(config *Config, log logger.Logger) (*esClient, error) {
	cfg := es.Config{
		Addresses:  config.Addresses,
		Username:   config.Username,
		Password:   config.Password,
		APIKey:     config.APIKey,
		CloudID:    config.CloudID,
		MaxRetries: config.MaxRetries,
	}

	// CA 证书
	if config.CACert != "" {
		cfg.CACert = []byte(config.CACert)
	}

	// 自定义 Transport 设置（基于默认 Transport 克隆，保留 TLS、DialContext 等默认配置）
	if config.MaxIdleConnsPerHost > 0 || config.ResponseHeaderTimeout > 0 {
		tp := http.DefaultTransport.(*http.Transport).Clone()
		if config.MaxIdleConnsPerHost > 0 {
			tp.MaxIdleConnsPerHost = config.MaxIdleConnsPerHost
		}
		if config.ResponseHeaderTimeout > 0 {
			tp.ResponseHeaderTimeout = config.ResponseHeaderTimeout
		}
		cfg.Transport = tp
	}

	client, err := es.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: create client: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), config.PingTimeout)
	defer cancel()

	res, err := client.Ping(client.Ping.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: ping: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch: ping returned status %s", res.Status())
	}

	log.Info("elasticsearch 连接成功", "addrs", config.Addresses)

	return &esClient{
		client: client,
		log:    log,
	}, nil
}

func (c *esClient) Index(name string) Index {
	return &esIndex{
		client: c.client,
		name:   name,
		log:    c.log,
	}
}

func (c *esClient) Ping(ctx context.Context) error {
	res, err := c.client.Ping(c.client.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%w: %s", ErrRequestFailed, res.Status())
	}
	return nil
}

func (c *esClient) Close() error {
	c.log.Info("elasticsearch 连接关闭")
	// go-elasticsearch 客户端无需显式关闭
	return nil
}

func (c *esClient) Client() *es.Client {
	return c.client
}

// encodeBody 将任意类型编码为 JSON Reader.
func encodeBody(v any) (io.Reader, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, fmt.Errorf("elasticsearch: encode body: %w", err)
	}
	return &buf, nil
}

// decodeResponse 解码 ES 响应.
func decodeResponse(res *esapi.Response, v any) error {
	if res.IsError() {
		// 尝试读取错误信息
		var body map[string]any
		if err := json.NewDecoder(res.Body).Decode(&body); err == nil {
			if errInfo, ok := body["error"]; ok {
				return fmt.Errorf("%w: [%s] %v", ErrRequestFailed, res.Status(), errInfo)
			}
		}
		return fmt.Errorf("%w: %s", ErrRequestFailed, res.Status())
	}

	if v != nil {
		if err := json.NewDecoder(res.Body).Decode(v); err != nil {
			return fmt.Errorf("elasticsearch: decode response: %w", err)
		}
	}
	return nil
}
