# servex 测试工具包（testx）

## NopLogger / TestLogger — Mock Logger

```go
// NopLogger：丢弃所有日志输出，适合不关心日志的单元测试
log := testx.NopLogger()
svc := NewMyService(log)

// TestLogger：将日志输出转发到 testing.T，失败时可看到完整日志
func TestMyService(t *testing.T) {
    log := testx.TestLogger(t)
    svc := NewMyService(log)
    // ...
}
```

**关键函数：**
- `testx.NopLogger() logger.Logger` — 返回空操作日志，实现 `logger.Logger` 全部方法但不输出
- `testx.TestLogger(t *testing.T) logger.Logger` — 将日志通过 `t.Log` 输出，支持 `With(fields...)` 附加字段

## 测试容器（Container）

基于 [testcontainers-go](https://github.com/testcontainers/testcontainers-go)，自动启动并管理 Docker 容器。

```go
func TestWithRedis(t *testing.T) {
    ctx := context.Background()

    // 启动 Redis 容器（redis:7-alpine）
    redis, err := testx.NewRedis(ctx)
    require.NoError(t, err)
    defer redis.Close(ctx)

    // 获取地址
    addr := redis.Addr()   // "localhost:49153"（动态端口）
    host := redis.Host()
    port := redis.Port()

    // 用于初始化真实客户端
    cache, _ := cache.NewCache(cache.NewRedisConfig(addr), testx.NopLogger())
    // ...
}

// PostgreSQL 容器（支持选项）
pg, err := testx.NewPostgres(ctx,
    testx.WithPostgresUser("myuser"),
    testx.WithPostgresPassword("mypass"),
    testx.WithPostgresDB("mydb"),
    testx.WithPostgresImage("postgres:15-alpine"), // 可选，默认 postgres:16-alpine
)

dsn := fmt.Sprintf("host=%s port=%s user=myuser password=mypass dbname=mydb sslmode=disable",
    pg.Host(), pg.Port())

// MySQL 容器
mysql, err := testx.NewMySQL(ctx,
    testx.WithMySQLRootPassword("root"),
    testx.WithMySQLDatabase("testdb"),
    testx.WithMySQLImage("mysql:8"),
)

// MongoDB 容器（mongo:7）
mongo, err := testx.NewMongoDB(ctx)

// Kafka 容器（KRaft 模式，confluentinc/cp-kafka:7.6.0）
kafka, err := testx.NewKafka(ctx)

// ClickHouse 容器
ch, err := testx.NewClickHouse(ctx)
```

**关键类型：**
- `testx.Container` — 封装容器（`Addr() string`, `Host() string`, `Port() string`, `Close(ctx) error`）
- `testx.NewRedis(ctx)` — Redis 7 Alpine
- `testx.NewPostgres(ctx, opts...)` — PostgreSQL 16 Alpine，支持 `WithPostgresUser/Password/DB/Image`
- `testx.NewMySQL(ctx, opts...)` — MySQL 8，支持 `WithMySQLRootPassword/Database/Image`
- `testx.NewMongoDB(ctx)` — MongoDB 7
- `testx.NewKafka(ctx)` — Kafka（KRaft 模式）
- `testx.NewClickHouse(ctx)` — ClickHouse（最新版）

**注意：** 需要本地 Docker 环境；`Close` 会终止并移除容器。

## HTTP 测试 Helper（HTTPTestServer）

```go
func TestMyHandler(t *testing.T) {
    // 创建测试服务器（可选中间件链）
    srv := testx.NewHTTPTestServer(myHandler,
        requestid.New().HTTPMiddleware,
        logging.NewHTTP(testx.TestLogger(t)).Middleware,
    )
    defer srv.Close() // 来自 httptest.Server

    // GET 请求
    resp := srv.Get("/api/users")
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // POST JSON 请求
    resp = srv.PostJSON("/api/users", map[string]any{
        "name": "Alice",
    })
    assert.Equal(t, http.StatusCreated, resp.StatusCode)

    // 自定义请求（用于需要设置 Headers 等场景）
    req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/users/1", nil)
    req.Header.Set("Authorization", "Bearer token")
    resp = srv.Do(req)
    assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
```

**关键类型：**
- `testx.NewHTTPTestServer(handler, middlewares...) *HTTPTestServer` — 创建测试服务器，中间件按声明顺序包裹（第一个最外层）
- `srv.Get(path) *http.Response` — 发送 GET 请求，失败时 panic
- `srv.PostJSON(path, body) *http.Response` — 发送 JSON POST 请求，失败时 panic
- `srv.Do(req) *http.Response` — 执行任意 `*http.Request`，失败时 panic
- `srv.URL` — 服务器地址（来自内嵌的 `*httptest.Server`）
- `srv.Close()` — 关闭服务器（来自内嵌的 `*httptest.Server`）

## gRPC 测试 Helper（GRPCTestServer）

使用 `bufconn` 内存连接，无需真实网络端口。

```go
func TestMyGRPCService(t *testing.T) {
    // 创建测试服务器（registerFn 注册 gRPC 服务，可选拦截器）
    conn, cleanup := testx.NewGRPCTestServer(
        func(s *grpc.Server) {
            pb.RegisterMyServiceServer(s, &myServiceImpl{})
        },
        // 可选：添加一元拦截器
        grpc_recovery.UnaryServerInterceptor(),
    )
    defer cleanup() // 关闭连接、优雅停止服务器

    // 创建 gRPC 客户端 stub
    client := pb.NewMyServiceClient(conn)
    resp, err := client.GetItem(context.Background(), &pb.GetItemRequest{Id: "1"})
    require.NoError(t, err)
    assert.Equal(t, "1", resp.Item.Id)
}
```

**关键函数：**
- `testx.NewGRPCTestServer(registerFn, interceptors...) (*grpc.ClientConn, func())` — 创建基于内存连接的 gRPC 测试服务器
  - `registerFn func(*grpc.Server)` — 注册 gRPC 服务的回调
  - `interceptors ...grpc.UnaryServerInterceptor` — 可选一元拦截器
  - 返回 `conn` 和 `cleanup` 函数，测试结束后必须调用 `cleanup()`

## Fixture 工具（LoadJSON / LoadYAML / Golden）

```go
// LoadJSON — 从文件加载 JSON 并反序列化
type UserFixture struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func TestCreateUser(t *testing.T) {
    user := testx.LoadJSON[UserFixture](t, "testdata/fixtures/user.json")
    // user.ID == "u-1", user.Name == "Alice"
}

// LoadYAML — 从文件加载 YAML 并反序列化
config := testx.LoadYAML[AppConfig](t, "testdata/config.yaml")

// Golden — 对比 actual 与 golden 文件（快照测试）
// 文件路径格式: testdata/<name>.golden
func TestRenderResponse(t *testing.T) {
    actual := renderJSON(myData)
    testx.Golden(t, "render_response", actual)
    // 首次运行使用 -update 生成: go test ./... -update
}

// GoldenJSON — 自动序列化为格式化 JSON 后对比
func TestAPIResponse(t *testing.T) {
    resp := callAPI()
    testx.GoldenJSON(t, "api_response", resp)
    // 生成 testdata/api_response.golden（格式化 JSON + 换行）
}
```

**关键函数：**
- `testx.LoadJSON[T](t, path) T` — 加载 JSON 文件，失败时 `t.Fatalf`
- `testx.LoadYAML[T](t, path) T` — 加载 YAML 文件，失败时 `t.Fatalf`
- `testx.Golden(t, name, actual []byte)` — 快照对比，golden 文件路径为 `testdata/<name>.golden`
- `testx.GoldenJSON(t, name, actual any)` — 序列化为 `json.MarshalIndent` 后对比

**更新 Golden 文件：**

```bash
# 首次生成 golden 文件，或变更预期输出时更新
go test ./... -update
```

**注意：** `-update` 标志通过 `flag.Bool("update", false, ...)` 注册，使用前需在测试中调用 `flag.Parse()`（通常由 `TestMain` 或测试框架自动处理）。

## Bot 测试工具（bottest）（v2.0.1+）

Bot handler 的测试工具在 `transport/botserver/bottest` 包中，详见 `references/transport.md` 的 `transport/botserver/bottest` 章节。

```go
import "github.com/Tsukikage7/servex/v2/transport/botserver/bottest"

func TestBotCommand(t *testing.T) {
    bot, recorder := bottest.NewTestBot()
    bot.Handle("ping", func(ctx botserver.Context) error {
        return ctx.Reply("pong!")
    })

    err := bot.Dispatch("/ping")
    require.NoError(t, err)
    assert.Equal(t, "pong!", recorder.Messages[0].Text)
}
```
