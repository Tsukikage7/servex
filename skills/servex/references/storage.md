# servex 存储

## storage/cache — 缓存（Redis / 内存）

```go
// Redis 缓存
c, err := cache.NewCache(cache.NewRedisConfig("localhost:6379"), log)
if err != nil { ... }
defer c.Close()

// 写入（TTL 1 小时）
_ = c.Set(ctx, "user:1", `{"name":"alice"}`, time.Hour)

// 读取
val, err := c.Get(ctx, "user:1")

// 删除（方法名是 Del，不是 Delete）
_ = c.Del(ctx, "user:1")

// 内存缓存（开发/测试，无外部依赖）
memCache, err := cache.NewCache(cache.NewMemoryConfig(), log)
```

完整示例：`docs/superpowers/examples/storage/main.go`

**注意：** 删除方法是 `Del`（不是 `Delete`）。

## storage/rdbms — GORM 数据库

```go
// 支持 DriverPostgres / DriverMySQL / DriverSQLite / DriverClickHouse
db, err := database.NewDatabase(&database.Config{
    Driver:      database.DriverPostgres,
    DSN:         "host=localhost user=postgres password=postgres dbname=mydb port=5432 sslmode=disable",
    AutoMigrate: true,
}, log)
if err != nil { ... }
defer db.Close()

// 模型嵌入 BaseModel[T] 获得 ID、CreatedAt、UpdatedAt、DeletedAt（软删除）
// 字段名使用 `_at` 后缀：`CreatedAt`、`UpdatedAt`、`DeletedAt`
type User struct {
    database.BaseModel[uint]
    Name  string
    Email string
}

// 自动建表（仅 Config.AutoMigrate=true 时生效）
db.AutoMigrate(&User{})

// 获取带 context 的 *gorm.DB（推荐，确保链路追踪生效）
gdb := database.DB(ctx, db)

// 标准 GORM 操作
gdb.Create(&user)
gdb.First(&found, id)
gdb.Model(&found).Update("name", "new-name")
gdb.Delete(&found)  // 软删除
```

**注意：**
- `database.DB(ctx, db)` 推荐，而非 `db.AsGORM()`（后者无 context）
- `BaseModel` 字段名含 `Time` 后缀：`CreatedAt`、`UpdatedAt`、`DeletedAt`

## storage/mongodb — MongoDB 客户端

```go
// MustNewClient 初始化失败直接 panic（适合 main 函数）
client := mongodb.MustNewClient(mongodb.Config{
    URI:      "mongodb://localhost:27017",
    Database: "mydb",
})
defer client.Disconnect(ctx)

// NewClient 返回 error
client, err := mongodb.NewClient(mongodb.Config{...})
```

## storage/s3 — S3/MinIO 对象存储

```go
// MustNewClient 初始化失败直接 panic
s3 := s3.MustNewClient(s3.Config{
    Endpoint:  "localhost:9000",
    AccessKey: "minioadmin",
    SecretKey: "minioadmin",
    UseSSL:    false,
})

// NewClient 返回 error
s3, err := s3.NewClient(s3.Config{...})
```

## storage/elasticsearch — Elasticsearch 客户端

```go
// 创建 Elasticsearch 客户端
client, err := elasticsearch.NewClient(&elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
    Username:  "elastic",
    Password:  "changeme",
    MaxRetries: 3,
    EnableTracing: true,
}, log)
if err != nil { ... }
defer client.Close()

// MustNewClient 失败时 panic
client := elasticsearch.MustNewClient(elasticsearch.DefaultConfig(), log)

// 索引操作
idx := client.Index("my-index")
idx.Create(ctx, map[string]any{
    "mappings": map[string]any{
        "properties": map[string]any{
            "name": map[string]any{"type": "text"},
        },
    },
})

// 文档 CRUD
doc := idx.Document()
doc.Index(ctx, "1", map[string]any{"name": "Alice"})
result, err := doc.Get(ctx, "1")
doc.Update(ctx, "1", map[string]any{"doc": map[string]any{"name": "Bob"}})
doc.Delete(ctx, "1")

// 批量操作
doc.Bulk(ctx, []elasticsearch.BulkAction{
    {Type: "index", ID: "1", Body: map[string]any{"name": "Alice"}},
    {Type: "delete", ID: "2"},
})

// 搜索
search := idx.Search()
results, err := search.Query(ctx, map[string]any{
    "match": map[string]any{"name": "Alice"},
}, elasticsearch.WithSize(10), elasticsearch.WithFrom(0))
```

**关键接口：**
- `elasticsearch.Client` — 顶层客户端（`Index`, `Ping`, `Close`, `Client`）
- `elasticsearch.Index` — 索引操作（`Create`, `Delete`, `Exists`, `Document`, `Search`）
- `elasticsearch.Document` — 文档 CRUD（`Index`, `Get`, `Update`, `Delete`, `Bulk`）
- `elasticsearch.Search` — 搜索（`Query`, `Count`, `Aggregate`, `Scroll`）
- 搜索选项：`WithSize`, `WithFrom`, `WithSort`, `WithHighlight`, `WithSourceIncludes`

## storage/lock — 分布式锁

```go
// 创建 Redis 分布式锁
locker := lock.NewRedis(cacheClient)

// 方式1: WithLock 辅助函数（推荐，自动获取/释放）
err := lock.WithLock(ctx, locker, "order:123", 30*time.Second, func() error {
    return processOrder(123)
})

// 方式2: TryWithLock 非阻塞版本
err := lock.TryWithLock(ctx, locker, "order:123", 30*time.Second, func() error {
    return processOrder(123)
})
if errors.Is(err, lock.ErrLockNotAcquired) {
    // 锁被占用
}

// 方式3: 手动管理
acquired, err := locker.TryLock(ctx, "my-resource", 30*time.Second)
if acquired {
    defer locker.Unlock(ctx, "my-resource")
    // ...
}

// 阻塞获取 + 延长锁
err := locker.Lock(ctx, "my-resource", 30*time.Second)
defer locker.Unlock(ctx, "my-resource")
locker.Extend(ctx, "my-resource", 30*time.Second) // 长操作时延长
```

**关键类型：**
- `lock.Locker` — 分布式锁接口（`TryLock`, `Lock`, `Unlock`, `Extend`）
- `lock.NewRedis(cacheClient)` — 基于 Redis 的实现
- `lock.WithLock(ctx, locker, key, ttl, fn)` — 自动管理锁生命周期
- `lock.TryWithLock(...)` — 非阻塞版本，获取失败立即返回
- 错误：`ErrLockNotAcquired`、`ErrLockNotHeld`、`ErrLockExpired`

## storage/sqlx — SQL 类型辅助工具

```go
// Nullable[T] 泛型 nullable 包装，支持 JSON/SQL 双向序列化
type User struct {
    Name  string
    Phone sqlx.Nullable[string] // 数据库中可为 NULL
    Age   sqlx.Nullable[int64]
}

// 创建 Nullable 值
phone := sqlx.Of("13800138000")  // Valid=true
noPhone := sqlx.Null[string]()   // Valid=false（NULL）

// 读取值（带默认值）
val := phone.ValueOr("未知")

// JSON 序列化：Valid=false → null，Valid=true → 值
// SQL 读写：自动实现 sql.Scanner 和 driver.Valuer

// 与标准库互转
ns := sqlx.NullableString(sql.NullString{String: "hello", Valid: true})
ni := sqlx.NullableInt64(sql.NullInt64{Int64: 42, Valid: true})
```

**关键类型：**
- `sqlx.Nullable[T]` — 泛型 nullable 包装（`Val T`, `Valid bool`）
- `sqlx.Of[T](v)` — 创建 Valid=true 的 Nullable
- `sqlx.Null[T]()` — 创建 Valid=false 的 Nullable（NULL）
- `n.ValueOr(def)` — 安全取值，NULL 时返回默认值
- 自动实现 `json.Marshaler`、`json.Unmarshaler`、`sql.Scanner`、`driver.Valuer`

## storage/migration — 数据库迁移（Go DSL）

```go
// 创建迁移注册表，链式添加迁移
registry := migration.NewRegistry().
    Add(migration.Migration{
        Version:     20240101000001,
        Description: "创建 users 表",
        Up: func(tx *gorm.DB) error {
            return tx.Exec(`CREATE TABLE users (
                id BIGSERIAL PRIMARY KEY,
                name VARCHAR(255) NOT NULL,
                email VARCHAR(255) UNIQUE NOT NULL,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )`).Error
        },
        Down: func(tx *gorm.DB) error {
            return tx.Exec(`DROP TABLE IF EXISTS users`).Error
        },
    }).
    Add(migration.Migration{
        Version:     20240101000002,
        Description: "添加 users.phone 列",
        Up: func(tx *gorm.DB) error {
            return tx.Exec(`ALTER TABLE users ADD COLUMN phone VARCHAR(20)`).Error
        },
        Down: func(tx *gorm.DB) error {
            return tx.Exec(`ALTER TABLE users DROP COLUMN IF EXISTS phone`).Error
        },
    })

// 创建执行器（需要 *gorm.DB）
runner, err := migration.NewRunner(gormDB, registry, log)
if err != nil { ... }

// 执行所有未应用的迁移
if err := runner.Up(ctx); err != nil { ... }

// 回滚最后一次迁移
if err := runner.Down(ctx); err != nil { ... }

// 迁移到指定版本（含）
if err := runner.UpTo(ctx, 20240101000001); err != nil { ... }

// 回滚到指定版本（不含）
if err := runner.DownTo(ctx, 20240101000001); err != nil { ... }

// 查看迁移状态
statuses, err := runner.Status(ctx)
for _, s := range statuses {
    fmt.Printf("版本 %d: %s — applied=%v\n", s.Version, s.Description, s.Applied)
}

// 当前版本号
version, err := runner.CurrentVersion(ctx)
```

**关键类型：**
- `migration.Migration` — 迁移定义（`Version int64`, `Description string`, `Up func(*gorm.DB) error`, `Down func(*gorm.DB) error`）
- `migration.Registry` — 注册表（`NewRegistry()`, `Add(m)`, `Migrations()`）
- `migration.Runner` — 执行器接口（`Up`, `Down`, `UpTo`, `DownTo`, `Status`, `CurrentVersion`）
- `migration.NewRunner(db, registry, log)` — 创建执行器
- `migration.MigrationStatus` — 状态（`Version`, `Description`, `Applied bool`, `AppliedAt *time.Time`）

**注意：**
- `Version` 通常使用时间戳格式（如 `20240101000001`），自动按升序排列执行
- `Up`/`Down` 函数在事务中执行，失败自动回滚
- 迁移历史记录在数据库 `schema_migrations` 表中

## storage/clickhouse — ClickHouse 客户端

```go
// 创建客户端（推荐：带 logger，自动应用默认值）
client, err := clickhouse.NewClient(&clickhouse.Config{
    Addrs:         []string{"localhost:9000"},
    Database:      "analytics",
    Username:      "default",
    Password:      "",
    MaxOpenConns:  20,
    MaxIdleConns:  10,
    Compression:   "lz4",    // "lz4"、"zstd"、"none"
    EnableTracing: true,
}, log)
if err != nil { ... }
defer client.Close()

// MustNewClient 失败时 panic（适合 main）
client := clickhouse.MustNewClient(clickhouse.DefaultConfig(), log)

// Exec — DDL / INSERT（不返回行）
err = client.Exec(ctx, `
    CREATE TABLE IF NOT EXISTS events (
        ts       DateTime,
        user_id  UInt64,
        action   String
    ) ENGINE = MergeTree()
    ORDER BY (ts, user_id)
`)

// Query — 返回多行
rows, err := client.Query(ctx, "SELECT ts, user_id FROM events WHERE user_id = ?", userID)
defer rows.Close()
for rows.Next() {
    var ts time.Time
    var uid uint64
    rows.Scan(&ts, &uid)
}

// Select — 直接扫描到结构体切片
type EventRow struct {
    TS     time.Time `ch:"ts"`
    UserID uint64    `ch:"user_id"`
    Action string    `ch:"action"`
}
var result []EventRow
err = client.Select(ctx, &result,
    "SELECT ts, user_id, action FROM events WHERE action = ?", "click")

// PrepareBatch — 高性能批量写入
batch, err := client.PrepareBatch(ctx, "INSERT INTO events (ts, user_id, action)")
if err != nil { ... }
for _, e := range events {
    batch.Append(e.TS, e.UserID, e.Action)
}
err = batch.Send()

// Ping — 连接检查
if err := client.Ping(ctx); err != nil { ... }
```

**关键类型：**
- `clickhouse.Client` — 客户端接口（`Exec`, `Query`, `QueryRow`, `Select`, `PrepareBatch`, `Ping`, `Close`, `Conn`）
- `clickhouse.Config` — 配置（`Addrs`, `Database`, `Username`, `Password`, `MaxOpenConns`, `MaxIdleConns`, `DialTimeout`, `Compression`, `EnableTracing`）
- `clickhouse.DefaultConfig()` — 默认配置（localhost:9000, lz4 压缩）
- `clickhouse.NewClient(config, log)` — 返回 `(Client, error)`
- `clickhouse.MustNewClient(config, log)` — 失败时 panic

## storage/minio — MinIO 对象存储

```go
import "github.com/Tsukikage7/servex/storage/minio"

// 创建 MinIO 客户端
client, err := minio.NewClient(&minio.Config{
    Endpoint:  "localhost:9000",
    AccessKey: "minioadmin",
    SecretKey: "minioadmin",
    Bucket:    "my-bucket",
    UseSSL:    false,
    Region:    "us-east-1",       // 默认 "us-east-1"
}, minio.WithLogger(log))
if err != nil { ... }

// 上传对象
info, err := client.PutObject(ctx, "path/to/file.txt", reader, size, "application/octet-stream")

// 下载对象
obj, err := client.GetObject(ctx, "path/to/file.txt")

// 删除对象
err = client.DeleteObject(ctx, "path/to/file.txt")

// 列出对象
for objInfo := range client.ListObjects(ctx, "prefix/", true) {
    fmt.Println(objInfo.Key, objInfo.Size)
}

// 获取对象元信息
info, err := client.StatObject(ctx, "path/to/file.txt")

// 预签名 URL（下载/上传）
downloadURL, err := client.PresignGetObject(ctx, "path/to/file.txt", 1*time.Hour)
uploadURL, err := client.PresignPutObject(ctx, "path/to/file.txt", 15*time.Minute)

// 复制对象
client.CopyObject(ctx, "src/file.txt", "dst/file.txt")

// 文件级别操作
client.FPutObject(ctx, "remote/key", "/local/path", "text/plain")
client.FGetObject(ctx, "remote/key", "/local/path")

// 桶操作
exists, _ := client.BucketExists(ctx)
client.MakeBucket(ctx)
```

**关键类型：**
- `minio.Client` — MinIO 客户端（`PutObject`, `GetObject`, `DeleteObject`, `ListObjects`, `StatObject`, `PresignGetObject`, `PresignPutObject`, `CopyObject`, `FPutObject`, `FGetObject`, `BucketExists`, `MakeBucket`）
- `minio.Config` — 配置（`Endpoint`, `AccessKey`, `SecretKey`, `Bucket`, `UseSSL`, `Region`, `ConnectTimeout`）
- `minio.DefaultConfig()` — 默认配置（Region: us-east-1, ConnectTimeout: 10s）
- `minio.WithLogger(log)` — 设置日志记录器
- 错误：`ErrNilConfig`, `ErrEmptyEndpoint`, `ErrEmptyBucket`, `ErrEmptyKey`, `ErrObjectNotFound`

## storage/neo4j — Neo4j 图数据库

```go
import "github.com/Tsukikage7/servex/storage/neo4j"

// 创建客户端（自动验证连通性）
client, err := neo4j.NewClient(&neo4j.Config{
    URI:                         "neo4j://localhost:7687",
    Username:                    "neo4j",
    Password:                    "password",
    Database:                    "neo4j",       // 默认 "neo4j"
    MaxConnectionPoolSize:       100,           // 默认 100
    ConnectionAcquisitionTimeout: 60 * time.Second,
    MaxTransactionRetryTime:     30 * time.Second,
}, neo4j.WithLogger(log))
if err != nil { ... }
defer client.Close(ctx)

// 执行 Cypher 查询
records, err := client.Run(ctx, "MATCH (n:Person) WHERE n.age > $age RETURN n.name", map[string]any{"age": 25})
for _, rec := range records {
    name, _ := rec.Get("n.name")
    fmt.Println(name)
}

// 写事务
_, err = client.WriteTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
    _, err := tx.Run(ctx, "CREATE (n:Person {name: $name, age: $age})", map[string]any{
        "name": "Alice", "age": 30,
    })
    return nil, err
})

// 读事务
result, err := client.ReadTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
    res, err := tx.Run(ctx, "MATCH (n:Person {name: $name}) RETURN n", map[string]any{"name": "Alice"})
    if err != nil { return nil, err }
    // 处理结果...
    return records, nil
})

// 获取原生驱动或会话
driver := client.Driver()
session := client.Session(ctx, neo4j.AccessModeRead)
```

**关键类型：**
- `neo4j.Client` — 客户端（`Run`, `ReadTransaction`, `WriteTransaction`, `Session`, `Driver`, `Close`）
- `neo4j.Config` — 配置（`URI`, `Username`, `Password`, `Database`, `MaxConnectionPoolSize`, `ConnectionAcquisitionTimeout`, `MaxTransactionRetryTime`, `Encrypted`）
- `neo4j.DefaultConfig()` — 默认配置（Database: neo4j, 连接池 100, 获取超时 60s, 重试 30s）
- `neo4j.Record` — 查询结果记录（`Get(key)`, `GetByIndex(i)`, `Keys`, `Values`）
- `neo4j.WithLogger(log)` — 设置日志记录器
- 错误：`ErrNilConfig`, `ErrEmptyURI`, `ErrEmptyDatabase`, `ErrNotConnected`

## storage/redis — Redis 客户端

```go
// 创建客户端
client, err := redis.NewClient(redis.DefaultConfig(), log)
if err != nil { ... }
defer client.Close()

// 或使用自定义配置
client, err = redis.NewClient(&redis.Config{
    Addr:          "localhost:6379",
    Password:      "",
    DB:            0,
    MaxRetries:    3,
    PoolSize:      10,
    MinIdleConns:  2,
    DialTimeout:   5 * time.Second,
    ReadTimeout:   3 * time.Second,
    WriteTimeout:  3 * time.Second,
    EnableTracing: true,
}, log)

// MustNewClient 失败时 panic
client = redis.MustNewClient(redis.DefaultConfig(), log)
```

```go
// String 操作
_ = client.Set(ctx, "key", "value", time.Hour)
val, _ := client.Get(ctx, "key")
count, _ := client.Del(ctx, "k1", "k2")
n, _ := client.Incr(ctx, "counter")

// Hash
client.HSet(ctx, "user:1", "name", "Alice", "age", 30)
name, _ := client.HGet(ctx, "user:1", "name")
all, _  := client.HGetAll(ctx, "user:1")

// List（队列）
client.RPush(ctx, "queue", "task1", "task2")
task, _ := client.LPop(ctx, "queue")
items, _ := client.LRange(ctx, "list", 0, -1)

// Set
client.SAdd(ctx, "tags", "go", "redis")
ok, _ := client.SIsMember(ctx, "tags", "go") // true

// Sorted Set（排行榜）
client.ZAdd(ctx, "board", goredis.Z{Score: 100, Member: "alice"})
top, _ := client.ZRangeWithScores(ctx, "board", 0, 9)

// Pipeline 批量
_ = client.PipelineExec(ctx, func(pipe goredis.Pipeliner) error {
    pipe.Set(ctx, "k1", "v1", time.Minute)
    pipe.Incr(ctx, "counter")
    return nil
})

// Pub/Sub
sub := client.Subscribe(ctx, "channel")
defer sub.Close()
for msg := range sub.Channel() { ... }

// 访问底层 go-redis 客户端
rdb := client.Underlying()
```

**关键类型：**
- `redis.Client` — 完整操作接口
- `redis.Config` — 配置（`Addr` 必填，其他有默认值）
- `redis.DefaultConfig()` — 默认配置（localhost:6379，连接池 10，超时 3s）
- `redis.NewClient(config, log)` — 返回 `(Client, error)`
- `redis.MustNewClient(config, log)` — 失败时 panic
