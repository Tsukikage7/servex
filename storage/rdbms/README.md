# Database

数据库连接管理包，提供统一的数据库抽象层，目前支持 GORM。

## 特性

- 支持多种数据库驱动（MySQL、PostgreSQL、SQLite）
- 连接池配置
- 慢查询日志
- 自动迁移支持
- 统一的日志适配器

## 配置选项

| 配置项             | 类型     | 默认值  | 说明                 |
| ------------------ | -------- | ------- | -------------------- |
| `Type`             | string   | `gorm`  | ORM 类型             |
| `Driver`           | string   | -       | 数据库驱动（必需）   |
| `DSN`              | string   | -       | 连接字符串（必需）   |
| `AutoMigrate`      | bool     | `false` | 是否自动迁移         |
| `SlowThreshold`    | Duration | `200ms` | 慢查询阈值           |
| `LogLevel`         | string   | `info`  | 日志级别             |
| `Pool.MaxOpen`     | int      | `100`   | 最大打开连接数       |
| `Pool.MaxIdle`     | int      | `10`    | 最大空闲连接数       |
| `Pool.MaxLifetime` | Duration | `1h`    | 连接最大生命周期     |
| `Pool.MaxIdleTime` | Duration | `10m`   | 空闲连接最大存活时间 |

## 支持的驱动

| 常量               | 值           | 说明               |
| ------------------ | ------------ | ------------------ |
| `DriverMySQL`      | `mysql`      | MySQL              |
| `DriverPostgres`   | `postgres`   | PostgreSQL         |
| `DriverPostgreSQL` | `postgresql` | PostgreSQL（别名） |
| `DriverSQLite`     | `sqlite`     | SQLite             |
| `DriverSQLite3`    | `sqlite3`    | SQLite（别名）     |

## DSN 格式

### MySQL

```
user:pass@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
```

### PostgreSQL

```
host=localhost user=user password=pass dbname=dbname port=5432 sslmode=disable
```

### SQLite

```
/path/to/database.db
:memory:  # 内存数据库
```

## 基础模型

包提供了一个标准的 GORM 基础模型：

```go
type BaseModel struct {
    ID          uint           `gorm:"primaryKey"`
    CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
    UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime"`
    DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

// 在你的模型中使用
type User struct {
    database.BaseModel
    Name  string `gorm:"size:100"`
    Email string `gorm:"size:200;uniqueIndex"`
}
```

## 获取 GORM 实例

```go
// 方式一：通过 AsGORM 函数
if gormDB, ok := database.AsGORM(db); ok {
    gormDB.GORM().Find(&users)
}

// 方式二：通过 DB() 方法（需要类型断言）
gormDB := db.DB().(*gorm.DB)
gormDB.Find(&users)
```

## 日志级别

| 级别     | 说明                 |
| -------- | -------------------- |
| `silent` | 静默模式，不输出日志 |
| `error`  | 仅输出错误日志       |
| `warn`   | 输出警告和错误日志   |
| `info`   | 输出所有日志（默认） |

## 慢查询日志

当 SQL 执行时间超过 `SlowThreshold` 时，会记录警告日志：

```go
cfg := &database.Config{
    Driver:        database.DriverMySQL,
    DSN:           "...",
    SlowThreshold: 100 * time.Millisecond, // 100ms 以上记录为慢查询
}
```

## 连接池配置

```go
cfg := &database.Config{
    Driver: database.DriverMySQL,
    DSN:    "...",
    Pool: database.PoolConfig{
        MaxOpen:     200,              // 最大打开连接数
        MaxIdle:     20,               // 最大空闲连接数
        MaxLifetime: 30 * time.Minute, // 连接最大生命周期
        MaxIdleTime: 5 * time.Minute,  // 空闲连接最大存活时间
    },
}
```

## 错误处理

```go
var (
    ErrNilConfig         = errors.New("database: 配置为空")
    ErrNilLogger         = errors.New("database: 日志记录器为空")
    ErrEmptyDriver       = errors.New("database: 驱动类型为空")
    ErrEmptyDSN          = errors.New("database: 连接字符串为空")
    ErrUnsupportedDriver = errors.New("database: 不支持的驱动类型")
    ErrUnsupportedType   = errors.New("database: 不支持的 ORM 类型")
)
```

## 完整示例

```go
package main

import (
    "fmt"
    "time"

    "github.com/Tsukikage7/servex/database"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

type User struct {
    database.BaseModel
    Name  string `gorm:"size:100"`
    Email string `gorm:"size:200;uniqueIndex"`
    Age   int
}

func main() {
    // 初始化日志
    log := logger.MustNewLogger(logger.DefaultConfig())
    defer log.Close()

    // 配置数据库
    cfg := &database.Config{
        Driver:        database.DriverSQLite,
        DSN:           ":memory:",
        AutoMigrate:   true,
        SlowThreshold: 100 * time.Millisecond,
        LogLevel:      "info",
        Pool: database.PoolConfig{
            MaxOpen:     50,
            MaxIdle:     5,
            MaxLifetime: 30 * time.Minute,
            MaxIdleTime: 5 * time.Minute,
        },
    }

    // 创建数据库连接
    db, err := database.NewDatabase(cfg, log)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // 自动迁移
    if err := db.AutoMigrate(&User{}); err != nil {
        panic(err)
    }

    // 获取 GORM 实例
    gormDB, ok := database.AsGORM(db)
    if !ok {
        panic("failed to get GORM instance")
    }
    gorm := gormDB.GORM()

    // CRUD 操作
    user := &User{Name: "John", Email: "john@example.com", Age: 30}
    gorm.Create(user)

    var found User
    gorm.First(&found, user.ID)
    fmt.Printf("Found user: %+v\n", found)

    gorm.Model(&found).Update("age", 31)

    var users []User
    gorm.Find(&users)
    fmt.Printf("All users: %d\n", len(users))
}
```
