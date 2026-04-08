package main

import (
	"fmt"
	"os"
	"strings"
)

// InfraDef 基础设施定义.
type InfraDef struct {
	Key          string   // "mysql", "redis", "kafka"
	DisplayName  string   // "MySQL 数据库"
	Import       string   // 完整 import 路径
	Alias        string   // import 别名[可选，如 srvredis]
	ConfigKey    string   // config.yaml 中的 key
	ConfigYAML   string   // 默认配置 YAML 片段[缩进好的]
	InitCode     string   // main.go 初始化代码
	CloseCode    string   // defer 关闭代码[可选]
	ExtraImports []string // 额外 stdlib import[如 "context", "time", "net/http"]
}

// infraRegistry 可用基础设施组件注册表.
var infraRegistry = map[string]InfraDef{
	"mysql": {
		Key:         "mysql",
		DisplayName: "MySQL 数据库",
		Import:      "github.com/Tsukikage7/servex/storage/rdbms",
		ConfigKey:   "database",
		ConfigYAML: `database:
  driver: mysql
  dsn: "root:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local"
  max_open_conns: 20
  max_idle_conns: 10
  max_lifetime: 300s`,
		InitCode: `// 初始化 MySQL 数据库
	db, err := rdbms.NewDatabase(&rdbms.Config{
		Driver: rdbms.DriverMySQL,
		DSN:    envOr("DB_DSN", "root:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local"),
	}, log)
	if err != nil {
		log.Fatalf("init mysql: %v", err)
	}
	_ = db`,
	},
	"postgres": {
		Key:         "postgres",
		DisplayName: "PostgreSQL 数据库",
		Import:      "github.com/Tsukikage7/servex/storage/rdbms",
		ConfigKey:   "database",
		ConfigYAML: `database:
  driver: postgres
  dsn: "host=127.0.0.1 port=5432 user=postgres password=postgres dbname=mydb sslmode=disable"
  max_open_conns: 20
  max_idle_conns: 10
  max_lifetime: 300s`,
		InitCode: `// 初始化 PostgreSQL 数据库
	db, err := rdbms.NewDatabase(&rdbms.Config{
		Driver: rdbms.DriverPostgres,
		DSN:    envOr("DB_DSN", "host=127.0.0.1 port=5432 user=postgres password=postgres dbname=mydb sslmode=disable"),
	}, log)
	if err != nil {
		log.Fatalf("init postgres: %v", err)
	}
	_ = db`,
	},
	"sqlite": {
		Key:         "sqlite",
		DisplayName: "SQLite 数据库",
		Import:      "github.com/Tsukikage7/servex/storage/rdbms",
		ConfigKey:   "database",
		ConfigYAML: `database:
  driver: sqlite
  dsn: "./data.db"`,
		InitCode: `// 初始化 SQLite 数据库
	db, err := rdbms.NewDatabase(&rdbms.Config{
		Driver: rdbms.DriverSQLite,
		DSN:    envOr("DB_DSN", "./data.db"),
	}, log)
	if err != nil {
		log.Fatalf("init sqlite: %v", err)
	}
	_ = db`,
	},
	"redis": {
		Key:         "redis",
		DisplayName: "Redis",
		Import:      "github.com/Tsukikage7/servex/storage/redis",
		Alias:       "srvredis",
		ConfigKey:   "redis",
		ConfigYAML: `redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  pool_size: 10`,
		InitCode: `// 初始化 Redis
	rdb, err := srvredis.NewClient(&srvredis.Config{
		Addr: envOr("REDIS_ADDR", "127.0.0.1:6379"),
	}, log)
	if err != nil {
		log.Fatalf("init redis: %v", err)
	}
	defer rdb.Close()`,
	},
	"mongo": {
		Key:         "mongo",
		DisplayName: "MongoDB",
		Import:      "github.com/Tsukikage7/servex/storage/mongodb",
		ConfigKey:   "mongodb",
		ConfigYAML: `mongodb:
  uri: "mongodb://localhost:27017"
  database: "mydb"`,
		InitCode: `// 初始化 MongoDB
	mongoClient, err := mongodb.NewClient(&mongodb.Config{
		URI:      envOr("MONGO_URI", "mongodb://localhost:27017"),
		Database: envOr("MONGO_DATABASE", "mydb"),
	}, log)
	if err != nil {
		log.Fatalf("init mongodb: %v", err)
	}
	defer mongoClient.Close()`,
	},
	"es": {
		Key:         "es",
		DisplayName: "Elasticsearch",
		Import:      "github.com/Tsukikage7/servex/storage/elasticsearch",
		ConfigKey:   "elasticsearch",
		ConfigYAML: `elasticsearch:
  addresses:
    - "http://localhost:9200"
  username: ""
  password: ""`,
		InitCode: `// 初始化 Elasticsearch
	esClient, err := elasticsearch.NewClient(&elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}, log)
	if err != nil {
		log.Fatalf("init elasticsearch: %v", err)
	}
	defer esClient.Close()`,
	},
	"clickhouse": {
		Key:         "clickhouse",
		DisplayName: "ClickHouse",
		Import:      "github.com/Tsukikage7/servex/storage/clickhouse",
		ConfigKey:   "clickhouse",
		ConfigYAML: `clickhouse:
  addrs:
    - "localhost:9000"
  database: "default"
  max_open_conns: 10
  max_idle_conns: 5
  compression: "lz4"`,
		InitCode: `// 初始化 ClickHouse
	chClient, err := clickhouse.NewClient(&clickhouse.Config{
		Addrs:    []string{"localhost:9000"},
		Database: envOr("CLICKHOUSE_DATABASE", "default"),
	}, log)
	if err != nil {
		log.Fatalf("init clickhouse: %v", err)
	}
	defer chClient.Close()`,
	},
	"s3": {
		Key:         "s3",
		DisplayName: "S3 对象存储",
		Import:      "github.com/Tsukikage7/servex/storage/s3",
		ConfigKey:   "s3",
		ConfigYAML: `s3:
  endpoint: "http://localhost:9000"
  region: "us-east-1"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket: "my-bucket"`,
		InitCode: `// 初始化 S3
	s3Client, err := s3.NewClient(&s3.Config{
		Endpoint:  envOr("S3_ENDPOINT", "http://localhost:9000"),
		Region:    envOr("S3_REGION", "us-east-1"),
		AccessKey: envOr("S3_ACCESS_KEY", "minioadmin"),
		SecretKey: envOr("S3_SECRET_KEY", "minioadmin"),
		Bucket:    envOr("S3_BUCKET", "my-bucket"),
	}, log)
	if err != nil {
		log.Fatalf("init s3: %v", err)
	}
	_ = s3Client`,
	},
	"minio": {
		Key:         "minio",
		DisplayName: "MinIO 对象存储",
		Import:      "github.com/Tsukikage7/servex/storage/minio",
		ConfigKey:   "minio",
		ConfigYAML: `minio:
  endpoint: "localhost:9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket: "my-bucket"
  use_ssl: false`,
		InitCode: `// 初始化 MinIO
	minioClient, err := minio.NewClient(&minio.Config{
		Endpoint:  envOr("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey: envOr("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey: envOr("MINIO_SECRET_KEY", "minioadmin"),
		Bucket:    envOr("MINIO_BUCKET", "my-bucket"),
	}, minio.WithLogger(log))
	if err != nil {
		log.Fatalf("init minio: %v", err)
	}
	_ = minioClient`,
	},
	"neo4j": {
		Key:         "neo4j",
		DisplayName: "Neo4j 图数据库",
		Import:      "github.com/Tsukikage7/servex/storage/neo4j",
		Alias:       "srvneo4j",
		ConfigKey:   "neo4j",
		ConfigYAML: `neo4j:
  uri: "bolt://localhost:7687"
  username: "neo4j"
  password: "neo4j"
  database: "neo4j"`,
		InitCode: `// 初始化 Neo4j
	neo4jClient, err := srvneo4j.NewClient(&srvneo4j.Config{
		URI:      envOr("NEO4J_URI", "bolt://localhost:7687"),
		Username: envOr("NEO4J_USERNAME", "neo4j"),
		Password: envOr("NEO4J_PASSWORD", "neo4j"),
		Database: envOr("NEO4J_DATABASE", "neo4j"),
	}, srvneo4j.WithLogger(log))
	if err != nil {
		log.Fatalf("init neo4j: %v", err)
	}
	_ = neo4jClient`,
	},
	"kafka": {
		Key:         "kafka",
		DisplayName: "Kafka 消息队列",
		Import:      "github.com/Tsukikage7/servex/messaging/pubsub/kafka",
		ConfigKey:   "kafka",
		ConfigYAML: `kafka:
  brokers:
    - "localhost:9092"
  group: "my-group"`,
		InitCode: `// 初始化 Kafka Publisher
	kafkaPub, err := kafka.NewPublisherFromConfig(
		[]string{"localhost:9092"}, log,
	)
	if err != nil {
		log.Fatalf("init kafka publisher: %v", err)
	}
	defer kafkaPub.Close()`,
	},
	"rabbitmq": {
		Key:         "rabbitmq",
		DisplayName: "RabbitMQ 消息队列",
		Import:      "github.com/Tsukikage7/servex/messaging/pubsub/rabbitmq",
		ConfigKey:   "rabbitmq",
		ConfigYAML: `rabbitmq:
  url: "amqp://guest:guest@localhost:5672/"`,
		InitCode: `// 初始化 RabbitMQ Publisher
	rmqPub, err := rabbitmq.NewPublisherFromConfig(
		envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), log,
	)
	if err != nil {
		log.Fatalf("init rabbitmq publisher: %v", err)
	}
	defer rmqPub.Close()`,
	},

	// ── Observability ───────────────────────────────────────────────────

	"metrics": {
		Key:         "metrics",
		DisplayName: "Prometheus 指标收集",
		Import:      "github.com/Tsukikage7/servex/observability/metrics",
		ConfigKey:   "metrics",
		ConfigYAML: `metrics:
  namespace: "app"
  path: "/metrics"`,
		InitCode: `// 初始化 Prometheus 指标收集器
	collector, err := metrics.NewPrometheus(&metrics.Config{
		Namespace: envOr("METRICS_NAMESPACE", "app"),
		Path:      envOr("METRICS_PATH", "/metrics"),
	})
	if err != nil {
		log.Fatalf("init metrics: %v", err)
	}
	http.Handle("/metrics", collector.GetHandler())
	_ = collector`,
		ExtraImports: []string{"net/http"},
	},
	"tracing": {
		Key:         "tracing",
		DisplayName: "OpenTelemetry 链路追踪",
		Import:      "github.com/Tsukikage7/servex/observability/tracing",
		ConfigKey:   "tracing",
		ConfigYAML: `tracing:
  enabled: true
  sampling_rate: 1.0
  otlp:
    endpoint: "localhost:4318"`,
		InitCode: `// 初始化 OpenTelemetry 链路追踪
	tp, err := tracing.NewTracer(&tracing.TracingConfig{
		Enabled:      true,
		SamplingRate: 1.0,
		OTLP: &tracing.OTLPConfig{
			Endpoint: envOr("OTEL_ENDPOINT", "localhost:4318"),
		},
	}, envOr("SERVICE_NAME", "my-service"), "1.0.0")
	if err != nil {
		log.Fatalf("init tracing: %v", err)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()`,
		ExtraImports: []string{"context"},
	},
	"profiling": {
		Key:         "profiling",
		DisplayName: "持续性能剖析",
		Import:      "github.com/Tsukikage7/servex/observability/profiling",
		ConfigKey:   "profiling",
		ConfigYAML: `profiling:
  enabled: true
  interval: 60s
  types:
    - cpu
    - heap`,
		InitCode: `// 初始化持续性能剖析
	profiler, err := profiling.New(&profiling.Config{
		Enabled:  true,
		Interval: 60 * time.Second,
		Types:    []profiling.ProfileType{profiling.ProfileCPU, profiling.ProfileHeap},
	})
	if err != nil {
		log.Fatalf("init profiling: %v", err)
	}
	if err := profiler.Start(context.Background()); err != nil {
		log.Fatalf("start profiling: %v", err)
	}
	defer func() { _ = profiler.Stop(context.Background()) }()`,
		ExtraImports: []string{"context", "time"},
	},
	// NOTE: slo[SLO/SLI 追踪]通常在代码中按业务逻辑配置，不适合通过 CLI flag 统一初始化.

	// ── Auth ─────────────────────────────────────────────────────────────

	"jwt": {
		Key:         "jwt",
		DisplayName: "JWT 认证",
		Import:      "github.com/Tsukikage7/servex/auth/jwt",
		Alias:       "srvjwt",
		ConfigKey:   "jwt",
		ConfigYAML: `jwt:
  secret: "change-me"
  access_duration: 2h
  refresh_duration: 168h`,
		InitCode: `// 初始化 JWT 认证
	jwtSvc := srvjwt.NewJWT(
		srvjwt.WithSecretKey(envOr("JWT_SECRET", "change-me")),
		srvjwt.WithLogger(log),
	)
	_ = jwtSvc`,
	},
	"rbac": {
		Key:         "rbac",
		DisplayName: "RBAC 权限",
		Import:      "github.com/Tsukikage7/servex/auth/rbac",
		ConfigKey:   "rbac",
		ConfigYAML: `# rbac: 权限规则通常在代码中定义`,
		InitCode: `// 初始化 RBAC 权限管理
	rbacStore := rbac.NewMemoryStore()
	rbacMgr := rbac.NewManager(rbacStore)
	_ = rbacMgr`,
	},

	// ── Service Discovery ────────────────────────────────────────────────

	"consul": {
		Key:         "consul",
		DisplayName: "Consul 服务发现",
		Import:      "github.com/Tsukikage7/servex/discovery",
		ConfigKey:   "discovery",
		ConfigYAML: `discovery:
  type: "consul"
  addr: "127.0.0.1:8500"`,
		InitCode: `// 初始化 Consul 服务发现
	disc, err := discovery.NewDiscovery(&discovery.Config{
		Type: discovery.TypeConsul,
		Addr: envOr("CONSUL_ADDR", "127.0.0.1:8500"),
	}, log)
	if err != nil {
		log.Fatalf("init consul discovery: %v", err)
	}
	_ = disc`,
	},
	"etcd": {
		Key:         "etcd",
		DisplayName: "etcd 服务发现",
		Import:      "github.com/Tsukikage7/servex/discovery",
		ConfigKey:   "discovery",
		ConfigYAML: `discovery:
  type: "etcd"
  etcd_endpoints:
    - "127.0.0.1:2379"`,
		InitCode: `// 初始化 etcd 服务发现
	disc, err := discovery.NewDiscovery(&discovery.Config{
		Type:          discovery.TypeEtcd,
		EtcdEndpoints: []string{envOr("ETCD_ENDPOINTS", "127.0.0.1:2379")},
	}, log)
	if err != nil {
		log.Fatalf("init etcd discovery: %v", err)
	}
	_ = disc`,
	},
	"nacos": {
		Key:         "nacos",
		DisplayName: "Nacos 服务发现",
		Import:      "github.com/Tsukikage7/servex/discovery",
		ConfigKey:   "discovery",
		ConfigYAML: `discovery:
  type: "nacos"
  nacos_endpoints:
    - "127.0.0.1:8848"`,
		InitCode: `// 初始化 Nacos 服务发现
	disc, err := discovery.NewDiscovery(&discovery.Config{
		Type:           discovery.TypeNacos,
		NacosEndpoints: []string{envOr("NACOS_ENDPOINTS", "127.0.0.1:8848")},
	}, log)
	if err != nil {
		log.Fatalf("init nacos discovery: %v", err)
	}
	_ = disc`,
	},

	// ── Other ────────────────────────────────────────────────────────────

	"scheduler": {
		Key:         "scheduler",
		DisplayName: "定时任务",
		Import:      "github.com/Tsukikage7/servex/scheduler",
		ConfigKey:   "scheduler",
		ConfigYAML: `# scheduler: 任务规则通常在代码中定义`,
		InitCode: `// 初始化定时任务调度器
	sched, err := scheduler.NewScheduler()
	if err != nil {
		log.Fatalf("init scheduler: %v", err)
	}
	defer sched.Stop()
	_ = sched`,
	},
	"i18n": {
		Key:         "i18n",
		DisplayName: "国际化",
		Import:      "github.com/Tsukikage7/servex/i18n",
		ConfigKey:   "i18n",
		ConfigYAML: `i18n:
  default_lang: "zh"
  supported_langs:
    - "zh"
    - "en"`,
		InitCode: `// 初始化国际化消息包
	i18nBundle := i18n.NewBundle(language.Chinese)
	_ = i18nBundle`,
	},
	"tenant": {
		Key:         "tenant",
		DisplayName: "多租户",
		Import:      "github.com/Tsukikage7/servex/tenant",
		ConfigKey:   "tenant",
		ConfigYAML: `# tenant: 租户解析通常在代码中配置`,
		InitCode: `// 多租户中间件[需要自定义 Resolver 实现]
	// resolver := tenant.NewXxxResolver(...)
	// tenantMW := tenant.Middleware(resolver)
	_ = tenant.ID // 占位：使用 tenant.FromContext(ctx) 获取当前租户`,
	},
}


// infraCategory 基础设施分类.
type infraCategory struct {
	Name string
	Keys []string
}

// infraCategories 分类列表[保持展示顺序].
var infraCategories = []infraCategory{
	{Name: "存储", Keys: []string{"mysql", "postgres", "sqlite", "mongo", "es", "clickhouse", "s3", "minio", "neo4j"}},
	{Name: "缓存", Keys: []string{"redis"}},
	{Name: "消息队列", Keys: []string{"kafka", "rabbitmq"}},
}

// observeRegistry 可观测性组件注册表.
var observeRegistry = map[string]InfraDef{}

// authRegistry 认证组件注册表.
var authRegistry = map[string]InfraDef{}

// discoveryRegistry 服务发现组件注册表.
var discoveryRegistry = map[string]InfraDef{}

// otherRegistry 其他组件注册表.
var otherRegistry = map[string]InfraDef{}

func init() {
	// 从 infraRegistry 中分离各类组件到独立 registry
	observeKeys := map[string]bool{"metrics": true, "tracing": true, "profiling": true}
	authKeys := map[string]bool{"jwt": true, "rbac": true}
	discoveryKeys := map[string]bool{"consul": true, "etcd": true, "nacos": true}
	otherKeys := map[string]bool{"scheduler": true, "i18n": true, "tenant": true}

	for k, v := range infraRegistry {
		switch {
		case observeKeys[k]:
			observeRegistry[k] = v
			delete(infraRegistry, k)
		case authKeys[k]:
			authRegistry[k] = v
			delete(infraRegistry, k)
		case discoveryKeys[k]:
			discoveryRegistry[k] = v
			delete(infraRegistry, k)
		case otherKeys[k]:
			otherRegistry[k] = v
			delete(infraRegistry, k)
		}
	}
}

// parseFromRegistry 从指定注册表解析逗号分隔的组件列表.
func parseFromRegistry(s string, registry map[string]InfraDef, category string) []InfraDef {
	if s == "" {
		return nil
	}

	seen := make(map[string]bool)
	var result []InfraDef

	for _, key := range strings.Split(s, ",") {
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
func parseInfra(s string) []InfraDef {
	return parseFromRegistry(s, infraRegistry, "infra")
}

// parseObserve 解析 --observe 可观测性组件列表.
func parseObserve(s string) []InfraDef {
	return parseFromRegistry(s, observeRegistry, "observe")
}

// parseAuth 解析 --auth 认证组件列表.
func parseAuth(s string) []InfraDef {
	return parseFromRegistry(s, authRegistry, "auth")
}

// parseDiscovery 解析 --discovery 服务发现组件.
func parseDiscovery(s string) []InfraDef {
	return parseFromRegistry(s, discoveryRegistry, "discovery")
}

// parseOther 解析 --other 其他组件列表.
func parseOther(s string) []InfraDef {
	return parseFromRegistry(s, otherRegistry, "other")
}

// infraList 返回所有可用组件的分类描述字符串.
func infraList() string {
	var sb strings.Builder
	sb.WriteString("可用组件:\n\n")
	sb.WriteString("  --infra:      mysql, postgres, sqlite, mongo, es, clickhouse, s3, minio, neo4j, redis, kafka, rabbitmq\n")
	sb.WriteString("  --observe:    metrics, tracing, profiling\n")
	sb.WriteString("  --auth:       jwt, rbac\n")
	sb.WriteString("  --discovery:  consul, etcd, nacos\n")
	sb.WriteString("  --other:      scheduler, i18n, tenant\n")
	return sb.String()
}

// extraImports 收集所有 InfraDef 的 ExtraImports 并去重[模板函数].
func extraImports(defs []InfraDef) []string {
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
