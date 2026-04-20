# ai/prompt

`ai/prompt` 包提供基于 Go `text/template` 的 AI 消息模板引擎，将模板渲染为 `llm.Message`。

## 功能特性

- 支持 Go `text/template` 全部语法（条件、循环、管道等）
- `Render(data)` 接受 struct、map 或任意类型
- `MustNew` / `MustRender` 变体在 panic 场景下简化代码

## 安装

```bash
go get github.com/Tsukikage7/servex/llm
```

## API

```go
func New(role llm.Role, text string) (*Template, error)
func MustNew(role llm.Role, text string) *Template

func (t *Template) Render(data any) (llm.Message, error)
func (t *Template) MustRender(data any) llm.Message
```

## 使用示例

```go
// 系统提示词模板
systemTmpl := prompt.MustNew(llm.RoleSystem,
    "你是一个专业的 {{.Language}} 工程师，擅长 {{.Domain}}。",
)

// 用户消息模板
userTmpl := prompt.MustNew(llm.RoleUser,
    `请审查以下代码并指出问题：
{{range .Files}}
文件：{{.Name}}
\`\`\`{{$.Language}}
{{.Content}}
\`\`\`
{{end}}`,
)

// 渲染
sysMsg := systemTmpl.MustRender(map[string]string{
    "Language": "Go",
    "Domain":   "微服务架构",
})

userMsg, err := userTmpl.Render(struct {
    Language string
    Files    []struct{ Name, Content string }
}{
    Language: "go",
    Files: []struct{ Name, Content string }{
        {"main.go", "package main\n..."},
    },
})

resp, _ := client.Generate(ctx, []llm.Message{sysMsg, userMsg})
```

## Registry：版本管理 / AB / 回滚

`Registry` 按逻辑标识（name）管理同一 prompt 的多个版本，支持 AB 分流与一键回滚，持久化后端通过 `Store` 接口插拔。

### 核心类型

- `Registry` — 注册表接口：`Register`/`Get`/`GetVersion`/`SetActive`/`SetABWeights`/`List`
- `Version` — 版本元数据（含 Role、Text、Active、Weight、CreatedAt），也是 GORM 持久化实体
- `Store` — 持久化接口：`Save`/`LoadAll`/`LoadAllNames`/`UpdateFlags`
- `NewMemoryStore()` — 内存实现（测试、单机无持久化场景）
- `NewRegistry(store, opts...)` — 创建 Registry；`WithRand(rng)` 可注入可重现的随机源
- `ErrNilStore` / `ErrNilTemplate` / `ErrEmptyName` / `ErrNotFound` / `ErrInvalidWeights` — 哨兵错误

### 语义

- `Register`：首次注册自动 Active（version=1）；后续追加 version=N+1 为新 Active，旧版本 Active 置 false
- `Get`：若任一版本 `Weight > 0` 视为启用 AB，按权重随机分流；否则返回 Active 版本
- `SetActive`：切换 Active（回滚），**同时清空所有 AB 权重**（AB 与单一 Active 互斥）
- `SetABWeights(name, map[version]weight)`：权重之和必须等于 `100`，所有 key 必须存在的版本；传 `nil` 或空 map 关闭 AB
- `GetVersion(name, v)`：按版本号取指定版本；不存在返回 `ErrNotFound`
- `List(name)`：返回所有版本（按 version 升序），未注册的 name 返回 `(nil, nil)`

### 使用示例

```go
import (
    "github.com/Tsukikage7/servex/v2/llm"
    "github.com/Tsukikage7/servex/v2/llm/prompt"
)

reg, _ := prompt.NewRegistry(prompt.NewMemoryStore())

// 注册首个版本
tmpl := prompt.MustNew(llm.RoleSystem, "你是客服助手 v1：{{.Product}}")
_, _ = reg.Register(ctx, "chat.default_system", tmpl)

// 发布 v2（自动 Active，v1 降级）
tmpl2 := prompt.MustNew(llm.RoleSystem, "你是客服助手 v2：{{.Product}}")
_, _ = reg.Register(ctx, "chat.default_system", tmpl2)

// 线上拿当前版本
current, _ := reg.Get(ctx, "chat.default_system")
msg, _ := current.Render(map[string]string{"Product": "VPS"})
// msg.Content == "你是客服助手 v2：VPS"

// AB 分流：70% v1、30% v2
_ = reg.SetABWeights(ctx, "chat.default_system", map[int]int{1: 70, 2: 30})

// 出问题回滚到 v1（同时清空 AB）
_ = reg.SetActive(ctx, "chat.default_system", 1)
```

### GORM 持久化

GORM Store 放在子包 `llm/prompt/gorm` 中：

```go
import (
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    promptgorm "github.com/Tsukikage7/servex/v2/llm/prompt/gorm"
    "github.com/Tsukikage7/servex/v2/llm/prompt"
)

db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
_ = promptgorm.AutoMigrate(ctx, db) // 建 prompt_versions 表

store := promptgorm.NewGORMStore(db)
reg, _ := prompt.NewRegistry(store)
// 之后 Register/Get 与 MemoryStore 完全一致，数据持久化到数据库
```

表结构：`prompt_versions(name, version)` 复合主键 + 索引，Role/Text/Active/Weight/CreatedAt 字段。

## 许可证

详见项目根目录 LICENSE 文件。
