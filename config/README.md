# Config

基于 Viper 的配置加载库，支持多种格式、环境变量覆盖和自动验证。

## 特性

- **多格式支持**：YAML、JSON、TOML、INI、ENV、Properties
- **泛型 API**：类型安全的配置加载
- **环境变量**：自动绑定环境变量，支持前缀
- **自动验证**：实现 `Validatable` 接口自动验证配置
- **多种加载方式**：文件路径、字节数组、搜索路径

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/config
```

## API 参考

### Load - 从文件加载

```go
// 基础加载
cfg, err := config.Load[MyConfig]("config.yaml")

// 带选项加载
cfg, err := config.Load[MyConfig]("config.yaml",
    config.WithEnvPrefix("MYAPP"),
    config.WithDefaults(map[string]any{
        "app.port": 3000,
    }),
)
```

### MustLoad - 加载失败时 panic

```go
cfg := config.MustLoad[MyConfig]("config.yaml")
```

### LoadFromBytes - 从字节加载

```go
data := []byte(`
app:
  name: test
  port: 8080
`)

cfg, err := config.LoadFromBytes[MyConfig](data, "yaml")
```

### LoadWithSearch - 搜索多个目录

```go
// 在多个目录中搜索 app.yaml 或 app.json 等
cfg, err := config.LoadWithSearch[MyConfig]("app", []string{
    ".",
    "/etc/myapp",
    "$HOME/.config/myapp",
})
```

## 配置选项

### WithEnvPrefix - 环境变量前缀

```go
// APP_DATABASE_HOST 会映射到 database.host
cfg, err := config.Load[MyConfig]("config.yaml",
    config.WithEnvPrefix("APP"),
)
```

### WithDefaults - 设置默认值

```go
cfg, err := config.Load[MyConfig]("config.yaml",
    config.WithDefaults(map[string]any{
        "app.port":      3000,
        "database.host": "localhost",
        "database.port": 5432,
    }),
)
```

### WithAutomaticEnv - 自动绑定环境变量

```go
// 默认已启用
cfg, err := config.Load[MyConfig]("config.yaml",
    config.WithAutomaticEnv(),
)
```

### WithConfigType - 显式指定配置类型

```go
// 当文件没有扩展名时使用
cfg, err := config.Load[MyConfig]("/etc/myapp/config",
    config.WithConfigType("yaml"),
)
```

## 配置验证

实现 `Validatable` 接口，配置加载后自动验证：

```go
type ServerConfig struct {
    Host string `mapstructure:"host"`
    Port int    `mapstructure:"port"`
}

// 实现 Validatable 接口
func (c *ServerConfig) Validate() error {
    if c.Host == "" {
        return errors.New("host 不能为空")
    }
    if c.Port <= 0 || c.Port > 65535 {
        return errors.New("port 必须在 1-65535 之间")
    }
    return nil
}

// 加载时自动验证
cfg, err := config.Load[ServerConfig]("config.yaml")
if err != nil {
    // 可能是 "配置验证失败: host 不能为空"
    log.Fatal(err)
}
```

## 环境变量覆盖

环境变量可以覆盖配置文件中的值：

```yaml
# config.yaml
database:
  host: localhost
  port: 5432
```

```bash
# 环境变量覆盖
export DATABASE_HOST=production-db.example.com
export DATABASE_PORT=5433
```

```go
cfg, err := config.Load[MyConfig]("config.yaml")
// cfg.Database.Host = "production-db.example.com"
// cfg.Database.Port = 5433
```

带前缀的环境变量：

```bash
export MYAPP_DATABASE_HOST=production-db.example.com
```

```go
cfg, err := config.Load[MyConfig]("config.yaml",
    config.WithEnvPrefix("MYAPP"),
)
```

## 支持的文件格式

| 格式       | 扩展名          |
| ---------- | --------------- |
| YAML       | `.yaml`, `.yml` |
| JSON       | `.json`         |
| TOML       | `.toml`         |
| INI        | `.ini`          |
| ENV        | `.env`          |
| Properties | `.properties`   |

## 工具函数

### GetConfigType - 获取配置类型

```go
configType := config.GetConfigType("config.yaml")  // "yaml"
configType := config.GetConfigType("config.json")  // "json"
configType := config.GetConfigType("config.toml")  // "toml"
```

## 错误处理

```go
cfg, err := config.Load[MyConfig]("config.yaml")
if err != nil {
    switch {
    case errors.Is(err, config.ErrFileNotFound):
        log.Fatal("配置文件不存在")
    case errors.Is(err, config.ErrNilConfig):
        log.Fatal("配置为空")
    default:
        log.Fatalf("加载配置失败: %v", err)
    }
}
```

## 错误常量

| 常量              | 说明                 |
| ----------------- | -------------------- |
| `ErrNilConfig`    | 配置为空             |
| `ErrFileNotFound` | 配置文件不存在       |
| `ErrInvalidType`  | 不支持的配置文件类型 |

## 最佳实践

### 1. 结构化配置

```go
type Config struct {
    App      AppConfig      `mapstructure:"app"`
    Database DatabaseConfig `mapstructure:"database"`
    Redis    RedisConfig    `mapstructure:"redis"`
    Logger   LoggerConfig   `mapstructure:"logger"`
}

type AppConfig struct {
    Name        string `mapstructure:"name"`
    Environment string `mapstructure:"environment"`
    Port        int    `mapstructure:"port"`
}
```

### 2. 分环境配置

```go
func LoadConfig() (*Config, error) {
    env := os.Getenv("APP_ENV")
    if env == "" {
        env = "development"
    }

    configFile := fmt.Sprintf("config.%s.yaml", env)

    return config.Load[Config](configFile,
        config.WithDefaults(map[string]any{
            "app.environment": env,
        }),
    )
}
```

### 3. 配置验证

```go
func (c *Config) Validate() error {
    if c.App.Name == "" {
        return errors.New("app.name 不能为空")
    }
    if c.App.Port <= 0 {
        return errors.New("app.port 必须大于 0")
    }
    if c.Database.Host == "" {
        return errors.New("database.host 不能为空")
    }
    return nil
}
```

### 4. 单例模式

```go
var (
    cfg  *Config
    once sync.Once
)

func GetConfig() *Config {
    once.Do(func() {
        var err error
        cfg, err = config.Load[Config]("config.yaml")
        if err != nil {
            panic(err)
        }
    })
    return cfg
}
```

## 测试

```bash
go test ./config/... -v -cover
```

## License

MIT License
