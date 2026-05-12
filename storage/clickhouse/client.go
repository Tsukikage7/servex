package clickhouse

import (
	"context"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// chClient ClickHouse 客户端实现.
type chClient struct {
	conn driver.Conn
	log  logger.Logger
}

// newCHClient 创建 ClickHouse 客户端.
func newCHClient(config *Config, log logger.Logger) (*chClient, error) {
	opts := &clickhouse.Options{
		Addr: config.Addrs,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		MaxOpenConns: config.MaxOpenConns,
		MaxIdleConns: config.MaxIdleConns,
		DialTimeout:  config.DialTimeout,
	}

	// 压缩设置
	switch strings.ToLower(config.Compression) {
	case "lz4":
		opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionLZ4}
	case "zstd":
		opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionZSTD}
	case "none", "":
		// 不启用压缩
	default:
		// 默认使用 LZ4
		opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionLZ4}
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}

	log.Info("[ClickHouse] 连接成功", "addrs", config.Addrs, "database", config.Database)

	return &chClient{
		conn: conn,
		log:  log,
	}, nil
}

func (c *chClient) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}

func (c *chClient) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return c.conn.Query(ctx, query, args...)
}

func (c *chClient) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return c.conn.QueryRow(ctx, query, args...)
}

func (c *chClient) Select(ctx context.Context, dest any, query string, args ...any) error {
	return c.conn.Select(ctx, dest, query, args...)
}

func (c *chClient) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	return c.conn.PrepareBatch(ctx, query)
}

func (c *chClient) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c *chClient) Close() error {
	c.log.Info("[ClickHouse] 连接关闭")
	return c.conn.Close()
}

func (c *chClient) Conn() driver.Conn {
	return c.conn
}
