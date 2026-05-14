package container

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Container 封装 testcontainers 容器，提供简洁的地址获取能力.
type Container struct {
	container testcontainers.Container
	host      string
	port      string
}

// Addr 返回 host:port 形式的地址.
func (c *Container) Addr() string {
	return c.host + ":" + c.port
}

// Host 返回容器映射后的主机地址.
func (c *Container) Host() string {
	return c.host
}

// Port 返回容器映射后的端口.
func (c *Container) Port() string {
	return c.port
}

// Close 终止并移除容器.
// 注意：ctx 参数当前未使用（testcontainers.TerminateContainer 不接受 context），保留签名以备未来兼容.
func (c *Container) Close(_ context.Context) error {
	if c.container == nil {
		return nil
	}
	return testcontainers.TerminateContainer(c.container)
}

// newContainer 从 testcontainers.Container 中提取映射后的主机和端口并构造 Container.
func newContainer(ctx context.Context, c testcontainers.Container, exposedPort string) (*Container, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("testx: 获取容器主机失败: %w", err)
	}
	mp, err := c.MappedPort(ctx, exposedPort+"/tcp")
	if err != nil {
		return nil, fmt.Errorf("testx: 获取容器映射端口失败: %w", err)
	}
	return &Container{
		container: c,
		host:      host,
		port:      mp.Port(),
	}, nil
}

// NewRedis 启动一个 Redis 测试容器.
func NewRedis(ctx context.Context) (*Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testx: 启动 Redis 容器失败: %w", err)
	}
	return newContainer(ctx, c, "6379")
}

// PostgresOption 配置 Postgres 测试容器的选项.
type PostgresOption func(*postgresConfig)

type postgresConfig struct {
	user     string
	password string
	dbName   string
	image    string
}

func defaultPostgresConfig() *postgresConfig {
	return &postgresConfig{
		user:     "test",
		password: "test",
		dbName:   "testdb",
		image:    "postgres:16-alpine",
	}
}

// WithPostgresUser 设置 Postgres 用户名.
func WithPostgresUser(user string) PostgresOption {
	return func(c *postgresConfig) { c.user = user }
}

// WithPostgresPassword 设置 Postgres 密码.
func WithPostgresPassword(password string) PostgresOption {
	return func(c *postgresConfig) { c.password = password }
}

// WithPostgresDB 设置 Postgres 数据库名.
func WithPostgresDB(dbName string) PostgresOption {
	return func(c *postgresConfig) { c.dbName = dbName }
}

// WithPostgresImage 设置 Postgres 镜像.
func WithPostgresImage(image string) PostgresOption {
	return func(c *postgresConfig) { c.image = image }
}

// NewPostgres 启动一个 PostgreSQL 测试容器.
func NewPostgres(ctx context.Context, opts ...PostgresOption) (*Container, error) {
	cfg := defaultPostgresConfig()
	for _, o := range opts {
		o(cfg)
	}
	req := testcontainers.ContainerRequest{
		Image:        cfg.image,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     cfg.user,
			"POSTGRES_PASSWORD": cfg.password,
			"POSTGRES_DB":       cfg.dbName,
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testx: 启动 Postgres 容器失败: %w", err)
	}
	return newContainer(ctx, c, "5432")
}

// MySQLOption 配置 MySQL 测试容器的选项.
type MySQLOption func(*mysqlConfig)

type mysqlConfig struct {
	rootPassword string
	database     string
	image        string
}

func defaultMySQLConfig() *mysqlConfig {
	return &mysqlConfig{
		rootPassword: "test",
		database:     "testdb",
		image:        "mysql:8",
	}
}

// WithMySQLRootPassword 设置 MySQL root 密码.
func WithMySQLRootPassword(password string) MySQLOption {
	return func(c *mysqlConfig) { c.rootPassword = password }
}

// WithMySQLDatabase 设置 MySQL 数据库名.
func WithMySQLDatabase(database string) MySQLOption {
	return func(c *mysqlConfig) { c.database = database }
}

// WithMySQLImage 设置 MySQL 镜像.
func WithMySQLImage(image string) MySQLOption {
	return func(c *mysqlConfig) { c.image = image }
}

// NewMySQL 启动一个 MySQL 测试容器.
func NewMySQL(ctx context.Context, opts ...MySQLOption) (*Container, error) {
	cfg := defaultMySQLConfig()
	for _, o := range opts {
		o(cfg)
	}
	req := testcontainers.ContainerRequest{
		Image:        cfg.image,
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": cfg.rootPassword,
			"MYSQL_DATABASE":      cfg.database,
		},
		WaitingFor: wait.ForLog("port: 3306  MySQL Community Server"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testx: 启动 MySQL 容器失败: %w", err)
	}
	return newContainer(ctx, c, "3306")
}

// NewMongoDB 启动一个 MongoDB 测试容器.
func NewMongoDB(ctx context.Context) (*Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "mongo:7",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForLog("Waiting for connections"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testx: 启动 MongoDB 容器失败: %w", err)
	}
	return newContainer(ctx, c, "27017")
}

// NewKafka 启动一个 Kafka 测试容器（使用 KRaft 模式）.
func NewKafka(ctx context.Context) (*Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:7.6.0",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT",
			"KAFKA_ADVERTISED_LISTENERS":             "PLAINTEXT://localhost:29092,PLAINTEXT_HOST://localhost:9092",
			"KAFKA_PROCESS_ROLES":                    "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:29093",
			"KAFKA_LISTENERS":                        "PLAINTEXT://0.0.0.0:29092,CONTROLLER://0.0.0.0:29093,PLAINTEXT_HOST://0.0.0.0:9092",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"CLUSTER_ID":                             "MkU3OEVBNTcwNTJENDM2Qk",
		},
		WaitingFor: wait.ForLog("Kafka Server started"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testx: 启动 Kafka 容器失败: %w", err)
	}
	return newContainer(ctx, c, "9092")
}

// NewClickHouse 启动一个 ClickHouse 测试容器.
func NewClickHouse(ctx context.Context) (*Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:latest",
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor:   wait.ForLog("Ready for connections"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testx: 启动 ClickHouse 容器失败: %w", err)
	}
	return newContainer(ctx, c, "9000")
}
