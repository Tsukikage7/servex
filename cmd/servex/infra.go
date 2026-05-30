package main

import (
	"fmt"
	"os"
	"strings"
)

// ComponentDef 基础设施组件定义.
type ComponentDef struct {
	Key          string   // "mysql", "redis", "kafka"
	DisplayName  string   // "MySQL 数据库"
	Import       string   // 完整 import 路径
	Alias        string   // import 别名[仅真实命名冲突时使用]
	ConfigKey    string   // config.yaml 中的 key
	ConfigYAML   string   // 默认配置 YAML 片段[缩进好的]
	CloseCode    string   // defer 关闭代码[可选]
	ExtraImports []string // 额外 stdlib import[如 "context", "time", "net/http"]
	BlankImports []string // 额外副作用 import[如 provider 注册包]
	// Wire DI 字段
	ProviderFunc string // Wire provider 函数名，如 "provideMySQL"
	ProviderCode string // Wire provider 函数体
	ConfigField  string // Config 结构体字段，如 `Database rdbms.Config ...`
}

// componentRegistry 可用基础设施组件注册表.
var componentRegistry = map[string]ComponentDef{
	"mysql": {
		Key:         "mysql",
		DisplayName: "MySQL 数据库",
		Import:      "github.com/Tsukikage7/servex/v2/storage/rdbms",
		ConfigKey:   "database",
		ConfigYAML: `database:
  driver: mysql
  dsn: "root:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local"
  max_open_conns: 20
  max_idle_conns: 10
  max_lifetime: 300s`,
		ProviderFunc: "provideMySQL",
		ProviderCode: `func provideMySQL(cfg *Config, log logger.Logger) (*gorm.DB, func(), error) {
	db, err := rdbms.NewDatabase(&cfg.Database, log)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, _ := db.DB()
	return db, func() { sqlDB.Close() }, nil
}`,
		ConfigField: "Database rdbms.Config `yaml:\"database\" mapstructure:\"database\"`",
	},
	"postgres": {
		Key:         "postgres",
		DisplayName: "PostgreSQL 数据库",
		Import:      "github.com/Tsukikage7/servex/v2/storage/rdbms",
		ConfigKey:   "database",
		ConfigYAML: `database:
  driver: postgres
  dsn: "host=127.0.0.1 port=5432 user=postgres password=postgres dbname=mydb sslmode=disable"
  max_open_conns: 20
  max_idle_conns: 10
  max_lifetime: 300s`,
		ProviderFunc: "providePostgres",
		ProviderCode: `func providePostgres(cfg *Config, log logger.Logger) (*gorm.DB, func(), error) {
	db, err := rdbms.NewDatabase(&cfg.Database, log)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, _ := db.DB()
	return db, func() { sqlDB.Close() }, nil
}`,
		ConfigField: "Database rdbms.Config `yaml:\"database\" mapstructure:\"database\"`",
	},
	"sqlite": {
		Key:         "sqlite",
		DisplayName: "SQLite 数据库",
		Import:      "github.com/Tsukikage7/servex/v2/storage/rdbms",
		ConfigKey:   "database",
		ConfigYAML: `database:
  driver: sqlite
  dsn: "./data.db"`,
		ProviderFunc: "provideSQLite",
		ProviderCode: `func provideSQLite(cfg *Config, log logger.Logger) (*gorm.DB, func(), error) {
	db, err := rdbms.NewDatabase(&cfg.Database, log)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, _ := db.DB()
	return db, func() { sqlDB.Close() }, nil
}`,
		ConfigField: "Database rdbms.Config `yaml:\"database\" mapstructure:\"database\"`",
	},
	"redis": {
		Key:         "redis",
		DisplayName: "Redis",
		Import:      "github.com/Tsukikage7/servex/v2/storage/redis",
		ConfigKey:   "redis",
		ConfigYAML: `redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  pool_size: 10`,
		ProviderFunc: "provideRedis",
		ProviderCode: `func provideRedis(cfg *Config, log logger.Logger) (redis.Client, func(), error) {
	rdb, err := redis.NewClient(&cfg.Redis, log)
	if err != nil {
		return nil, nil, err
	}
	return rdb, func() { rdb.Close() }, nil
}`,
		ConfigField: "Redis redis.Config `yaml:\"redis\" mapstructure:\"redis\"`",
	},
	"mongo": {
		Key:         "mongo",
		DisplayName: "MongoDB",
		Import:      "github.com/Tsukikage7/servex/v2/storage/mongodb",
		ConfigKey:   "mongodb",
		ConfigYAML: `mongodb:
  uri: "mongodb://localhost:27017"
  database: "mydb"`,
		ProviderFunc: "provideMongo",
		ProviderCode: `func provideMongo(cfg *Config, log logger.Logger) (*mongodb.Client, func(), error) {
	client, err := mongodb.NewClient(&cfg.MongoDB, log)
	if err != nil {
		return nil, nil, err
	}
	return client, func() { client.Close() }, nil
}`,
		ConfigField: "MongoDB mongodb.Config `yaml:\"mongodb\" mapstructure:\"mongodb\"`",
	},
	"es": {
		Key:         "es",
		DisplayName: "Elasticsearch",
		Import:      "github.com/Tsukikage7/servex/v2/storage/elasticsearch",
		ConfigKey:   "elasticsearch",
		ConfigYAML: `elasticsearch:
  addresses:
    - "http://localhost:9200"
  username: ""
  password: ""`,
		ProviderFunc: "provideElasticsearch",
		ProviderCode: `func provideElasticsearch(cfg *Config, log logger.Logger) (*elasticsearch.Client, func(), error) {
	client, err := elasticsearch.NewClient(&cfg.Elasticsearch, log)
	if err != nil {
		return nil, nil, err
	}
	return client, func() { client.Close() }, nil
}`,
		ConfigField: "Elasticsearch elasticsearch.Config `yaml:\"elasticsearch\" mapstructure:\"elasticsearch\"`",
	},
	"clickhouse": {
		Key:         "clickhouse",
		DisplayName: "ClickHouse",
		Import:      "github.com/Tsukikage7/servex/v2/storage/clickhouse",
		ConfigKey:   "clickhouse",
		ConfigYAML: `clickhouse:
  addrs:
    - "localhost:9000"
  database: "default"
  max_open_conns: 10
  max_idle_conns: 5
  compression: "lz4"`,
		ProviderFunc: "provideClickHouse",
		ProviderCode: `func provideClickHouse(cfg *Config, log logger.Logger) (*clickhouse.Client, func(), error) {
	client, err := clickhouse.NewClient(&cfg.ClickHouse, log)
	if err != nil {
		return nil, nil, err
	}
	return client, func() { client.Close() }, nil
}`,
		ConfigField: "ClickHouse clickhouse.Config `yaml:\"clickhouse\" mapstructure:\"clickhouse\"`",
	},
	"s3": {
		Key:         "s3",
		DisplayName: "S3 对象存储",
		Import:      "github.com/Tsukikage7/servex/v2/storage/s3",
		ConfigKey:   "s3",
		ConfigYAML: `s3:
  endpoint: "http://localhost:9000"
  region: "us-east-1"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket: "my-bucket"`,
		ProviderFunc: "provideS3",
		ProviderCode: `func provideS3(cfg *Config, log logger.Logger) (*s3.Client, func(), error) {
	client, err := s3.NewClient(&cfg.S3, log)
	if err != nil {
		return nil, nil, err
	}
	return client, func() {}, nil
}`,
		ConfigField: "S3 s3.Config `yaml:\"s3\" mapstructure:\"s3\"`",
	},
	"minio": {
		Key:         "minio",
		DisplayName: "MinIO 对象存储",
		Import:      "github.com/Tsukikage7/servex/v2/storage/minio",
		ConfigKey:   "minio",
		ConfigYAML: `minio:
  endpoint: "localhost:9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket: "my-bucket"
  use_ssl: false`,
		ProviderFunc: "provideMinIO",
		ProviderCode: `func provideMinIO(cfg *Config, log logger.Logger) (*minio.Client, func(), error) {
	client, err := minio.NewClient(&cfg.MinIO, log, minio.WithLogger(log))
	if err != nil {
		return nil, nil, err
	}
	return client, func() {}, nil
}`,
		ConfigField: "MinIO minio.Config `yaml:\"minio\" mapstructure:\"minio\"`",
	},
	"neo4j": {
		Key:         "neo4j",
		DisplayName: "Neo4j 图数据库",
		Import:      "github.com/Tsukikage7/servex/v2/storage/neo4j",
		ConfigKey:   "neo4j",
		ConfigYAML: `neo4j:
  uri: "bolt://localhost:7687"
  username: "neo4j"
  password: "neo4j"
  database: "neo4j"`,
		ProviderFunc: "provideNeo4j",
		ProviderCode: `func provideNeo4j(cfg *Config, log logger.Logger) (*neo4j.Client, func(), error) {
	client, err := neo4j.NewClient(&cfg.Neo4j, neo4j.WithLogger(log))
	if err != nil {
		return nil, nil, err
	}
	return client, func() { client.Close() }, nil
}`,
		ConfigField: "Neo4j neo4j.Config `yaml:\"neo4j\" mapstructure:\"neo4j\"`",
	},
	"kafka": {
		Key:         "kafka",
		DisplayName: "Kafka 消息队列",
		Import:      "github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka",
		ConfigKey:   "kafka",
		ConfigYAML: `kafka:
  brokers:
    - "localhost:9092"
  group: "my-group"`,
		ProviderFunc: "provideKafka",
		ProviderCode: `func provideKafka(cfg *Config, log logger.Logger) (*kafka.Publisher, func(), error) {
	pub, err := kafka.NewPublisherFromConfig(cfg.Kafka.Brokers, log)
	if err != nil {
		return nil, nil, err
	}
	return pub, func() { pub.Close() }, nil
}`,
		ConfigField: "Kafka kafka.Config `yaml:\"kafka\" mapstructure:\"kafka\"`",
	},
	"rabbitmq": {
		Key:         "rabbitmq",
		DisplayName: "RabbitMQ 消息队列",
		Import:      "github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq",
		ConfigKey:   "rabbitmq",
		ConfigYAML: `rabbitmq:
  url: "amqp://guest:guest@localhost:5672/"`,
		ProviderFunc: "provideRabbitMQ",
		ProviderCode: `func provideRabbitMQ(cfg *Config, log logger.Logger) (*rabbitmq.Publisher, func(), error) {
	pub, err := rabbitmq.NewPublisherFromConfig(cfg.RabbitMQ.URL, log)
	if err != nil {
		return nil, nil, err
	}
	return pub, func() { pub.Close() }, nil
}`,
		ConfigField: "RabbitMQ rabbitmq.Config `yaml:\"rabbitmq\" mapstructure:\"rabbitmq\"`",
	},

	// ── Observability ───────────────────────────────────────────────────

	"metrics": {
		Key:         "metrics",
		DisplayName: "Prometheus 指标收集",
		Import:      "github.com/Tsukikage7/servex/v2/observability/metrics",
		ConfigKey:   "metrics",
		ConfigYAML: `metrics:
  namespace: "app"
  path: "/metrics"`,
		ExtraImports: []string{"net/http"},
		ProviderFunc: "provideMetrics",
		ProviderCode: `func provideMetrics(cfg *Config, log logger.Logger) (*metrics.Prometheus, func(), error) {
	collector, err := metrics.NewPrometheus(&cfg.Metrics)
	if err != nil {
		return nil, nil, err
	}
	return collector, func() {}, nil
}`,
		ConfigField: "Metrics metrics.Config `yaml:\"metrics\" mapstructure:\"metrics\"`",
	},
	"tracing": {
		Key:         "tracing",
		DisplayName: "OpenTelemetry 链路追踪",
		Import:      "github.com/Tsukikage7/servex/v2/observability/tracing",
		ConfigKey:   "tracing",
		ConfigYAML: `tracing:
  enabled: true
  sampling_rate: 1.0
  otlp:
    endpoint: "localhost:4318"`,
		ExtraImports: []string{"context"},
		ProviderFunc: "provideTracing",
		ProviderCode: `func provideTracing(cfg *Config, log logger.Logger) (*tracing.Tracer, func(), error) {
	tp, err := tracing.NewTracer(&cfg.Tracing)
	if err != nil {
		return nil, nil, err
	}
	return tp, func() { _ = tp.Shutdown(context.Background()) }, nil
}`,
		ConfigField: "Tracing tracing.TracingConfig `yaml:\"tracing\" mapstructure:\"tracing\"`",
	},
	"profiling": {
		Key:         "profiling",
		DisplayName: "持续性能剖析",
		Import:      "github.com/Tsukikage7/servex/v2/observability/profiling",
		ConfigKey:   "profiling",
		ConfigYAML: `profiling:
  enabled: true
  interval: 60s
  types:
    - cpu
    - heap`,
		ExtraImports: []string{"context", "time"},
		ProviderFunc: "provideProfiling",
		ProviderCode: `func provideProfiling(cfg *Config, log logger.Logger) (*profiling.Profiler, func(), error) {
	profiler, err := profiling.New(&cfg.Profiling)
	if err != nil {
		return nil, nil, err
	}
	if err := profiler.Start(context.Background()); err != nil {
		return nil, nil, err
	}
	return profiler, func() { _ = profiler.Stop(context.Background()) }, nil
}`,
		ConfigField: "Profiling profiling.Config `yaml:\"profiling\" mapstructure:\"profiling\"`",
	},
	// NOTE: slo[SLO/SLI 追踪]通常在代码中按业务逻辑配置，不适合通过 CLI flag 统一初始化.

	// ── Auth ─────────────────────────────────────────────────────────────

	"jwt": {
		Key:         "jwt",
		DisplayName: "JWT 认证",
		Import:      "github.com/Tsukikage7/servex/v2/auth/jwt",
		ConfigKey:   "jwt",
		ConfigYAML: `jwt:
  secret: "change-me"
  access_duration: 2h
  refresh_duration: 168h`,
		ProviderFunc: "provideJWT",
		ProviderCode: `func provideJWT(cfg *Config, log logger.Logger) (*jwt.JWT, func(), error) {
	jwtSvc := jwt.MustNew(
		jwt.WithSecretKey(cfg.JWT.Secret),
		jwt.WithLogger(log),
	)
	return jwtSvc, func() {}, nil
}`,
		ConfigField: "JWT jwt.Config `yaml:\"jwt\" mapstructure:\"jwt\"`",
	},
	// ── Service Discovery ────────────────────────────────────────────────

	"consul": {
		Key:         "consul",
		DisplayName: "Consul 服务发现",
		Import:      "github.com/Tsukikage7/servex/v2/discovery",
		BlankImports: []string{
			"github.com/Tsukikage7/servex/v2/discovery/consul",
		},
		ConfigKey: "discovery",
		ConfigYAML: `discovery:
  type: "consul"
  addr: "127.0.0.1:8500"`,
		ProviderFunc: "provideConsul",
		ProviderCode: `func provideConsul(cfg *Config, log logger.Logger) (*discovery.Discovery, func(), error) {
	disc, err := discovery.NewDiscovery(&cfg.Discovery, log)
	if err != nil {
		return nil, nil, err
	}
	return disc, func() {}, nil
}`,
		ConfigField: "Discovery discovery.Config `yaml:\"discovery\" mapstructure:\"discovery\"`",
	},
	"etcd": {
		Key:         "etcd",
		DisplayName: "etcd 服务发现",
		Import:      "github.com/Tsukikage7/servex/v2/discovery",
		BlankImports: []string{
			"github.com/Tsukikage7/servex/v2/discovery/etcd",
		},
		ConfigKey: "discovery",
		ConfigYAML: `discovery:
  type: "etcd"
  etcd_endpoints:
    - "127.0.0.1:2379"`,
		ProviderFunc: "provideEtcd",
		ProviderCode: `func provideEtcd(cfg *Config, log logger.Logger) (*discovery.Discovery, func(), error) {
	disc, err := discovery.NewDiscovery(&cfg.Discovery, log)
	if err != nil {
		return nil, nil, err
	}
	return disc, func() {}, nil
}`,
		ConfigField: "Discovery discovery.Config `yaml:\"discovery\" mapstructure:\"discovery\"`",
	},
	"nacos": {
		Key:         "nacos",
		DisplayName: "Nacos 服务发现",
		Import:      "github.com/Tsukikage7/servex/v2/discovery",
		BlankImports: []string{
			"github.com/Tsukikage7/servex/v2/discovery/nacos",
		},
		ConfigKey: "discovery",
		ConfigYAML: `discovery:
  type: "nacos"
  nacos_endpoints:
    - "127.0.0.1:8848"`,
		ProviderFunc: "provideNacos",
		ProviderCode: `func provideNacos(cfg *Config, log logger.Logger) (*discovery.Discovery, func(), error) {
	disc, err := discovery.NewDiscovery(&cfg.Discovery, log)
	if err != nil {
		return nil, nil, err
	}
	return disc, func() {}, nil
}`,
		ConfigField: "Discovery discovery.Config `yaml:\"discovery\" mapstructure:\"discovery\"`",
	},

	// ── Other ────────────────────────────────────────────────────────────

	"scheduler": {
		Key:          "scheduler",
		DisplayName:  "定时任务",
		Import:       "github.com/Tsukikage7/servex/v2/scheduler",
		ConfigKey:    "scheduler",
		ConfigYAML:   `# scheduler: 任务规则通常在代码中定义`,
		ProviderFunc: "provideScheduler",
		ProviderCode: `func provideScheduler(log logger.Logger) (*scheduler.Scheduler, func(), error) {
	sched, err := scheduler.NewScheduler()
	if err != nil {
		return nil, nil, err
	}
	return sched, func() { sched.Stop() }, nil
}`,
		ConfigField: "",
	},
	"i18n": {
		Key:         "i18n",
		DisplayName: "国际化",
		Import:      "github.com/Tsukikage7/servex/v2/i18n",
		ConfigKey:   "i18n",
		ConfigYAML: `i18n:
  default_lang: "zh"
  supported_langs:
    - "zh"
    - "en"`,
		ProviderFunc: "provideI18n",
		ProviderCode: `func provideI18n(cfg *Config, log logger.Logger) (*i18n.Bundle, func(), error) {
	bundle := i18n.NewBundle(cfg.I18n.DefaultLang)
	return bundle, func() {}, nil
}`,
		ConfigField: "I18n i18n.Config `yaml:\"i18n\" mapstructure:\"i18n\"`",
	},
	"tenant": {
		Key:          "tenant",
		DisplayName:  "多租户",
		Import:       "github.com/Tsukikage7/servex/v2/tenant",
		ConfigKey:    "tenant",
		ConfigYAML:   `# tenant: 租户解析通常在代码中配置`,
		ProviderFunc: "provideTenant",
		ProviderCode: `func provideTenant(log logger.Logger) (*tenant.Manager, func(), error) {
	mgr := tenant.NewManager()
	return mgr, func() {}, nil
}`,
		ConfigField: "",
	},
}

// componentCategory 基础设施分类.
type componentCategory struct {
	Name string
	Keys []string
}

// componentCategories 分类列表[保持展示顺序].
var componentCategories = []componentCategory{
	{Name: "存储", Keys: []string{"mysql", "postgres", "sqlite", "mongo", "es", "clickhouse", "s3", "minio", "neo4j"}},
	{Name: "缓存", Keys: []string{"redis"}},
	{Name: "消息队列", Keys: []string{"kafka", "rabbitmq"}},
}

// observeComponents 可观测性组件注册表.
var observeComponents = map[string]ComponentDef{}

// authComponents 认证组件注册表.
var authComponents = map[string]ComponentDef{}

// discoveryComponents 服务发现组件注册表.
var discoveryComponents = map[string]ComponentDef{}

// otherComponents 其他组件注册表.
var otherComponents = map[string]ComponentDef{}

func init() {
	// 从 componentRegistry 中分离各类组件到独立 registry.
	// 注意：此操作会修改 componentRegistry 的内容删除已分类的条目，
	// 使得 componentRegistry 仅保留基础设施组件.
	observeKeys := map[string]bool{"metrics": true, "tracing": true, "profiling": true}
	authKeys := map[string]bool{"jwt": true}
	discoveryKeys := map[string]bool{"consul": true, "etcd": true, "nacos": true}
	otherKeys := map[string]bool{"scheduler": true, "i18n": true, "tenant": true}

	for k, v := range componentRegistry {
		switch {
		case observeKeys[k]:
			observeComponents[k] = v
			delete(componentRegistry, k)
		case authKeys[k]:
			authComponents[k] = v
			delete(componentRegistry, k)
		case discoveryKeys[k]:
			discoveryComponents[k] = v
			delete(componentRegistry, k)
		case otherKeys[k]:
			otherComponents[k] = v
			delete(componentRegistry, k)
		}
	}
}

// parseFromRegistry 从指定注册表解析逗号分隔的组件列表.
func parseFromRegistry(s string, registry map[string]ComponentDef, category string) []ComponentDef {
	if s == "" {
		return nil
	}

	seen := make(map[string]bool)
	var result []ComponentDef

	for key := range strings.SplitSeq(s, ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		def, ok := registry[key]
		if !ok {
			fmt.Fprintf(os.Stderr, "警告: 未知%s %q\n", category, key)
			continue
		}
		if seen[def.Import] {
			continue
		}
		seen[def.Import] = true
		result = append(result, def)
	}
	return result
}

// parseInfra 解析 --infra 基础设施组件列表.
func parseInfra(s string) []ComponentDef {
	return parseFromRegistry(s, componentRegistry, "infra")
}

// parseObserve 解析 --observe 可观测性组件列表.
func parseObserve(s string) []ComponentDef {
	return parseFromRegistry(s, observeComponents, "observe")
}

// parseAuth 解析 --auth 认证组件列表.
func parseAuth(s string) []ComponentDef {
	return parseFromRegistry(s, authComponents, "auth")
}

// parseDiscovery 解析 --discovery 服务发现组件.
func parseDiscovery(s string) []ComponentDef {
	return parseFromRegistry(s, discoveryComponents, "discovery")
}

// parseOther 解析 --other 其他组件列表.
func parseOther(s string) []ComponentDef {
	return parseFromRegistry(s, otherComponents, "other")
}

// componentList 返回所有可用组件的分类描述字符串.
func componentList() string {
	var sb strings.Builder
	sb.WriteString("可用组件:\n\n")
	sb.WriteString("  --infra:      mysql, postgres, sqlite, mongo, es, clickhouse, s3, minio, neo4j, redis, kafka, rabbitmq\n")
	sb.WriteString("  --observe:    metrics, tracing, profiling\n")
	sb.WriteString("  --auth:       jwt\n")
	sb.WriteString("  --discovery:  consul, etcd, nacos\n")
	sb.WriteString("  --other:      scheduler, i18n, tenant\n")
	return sb.String()
}

// extraImports 收集所有 ComponentDef 的 ExtraImports 并去重[模板函数].
func extraImports(defs []ComponentDef) []string {
	seen := make(map[string]bool)
	var result []string
	for _, d := range defs {
		for _, imp := range d.ExtraImports {
			if !seen[imp] {
				seen[imp] = true
				result = append(result, imp)
			}
		}
	}
	return result
}
