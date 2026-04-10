# Servex 包结构重构设计

## 目标

1. 消除包碎片化 — 顶层目录从 40+ 降到 ~20
2. 消除功能重叠 — 合并概念相近的包
3. 明确 public/internal 边界 — 通过命名和组织让用户知道该用哪些包
4. 按领域聚合 — 相关功能放在一起

## 约束

- 无外部用户，import path 可自由调整
- 不引入 `pkg/` 或 `internal/` 层级，保持 Go 惯例的扁平结构
- 接口不变，只做包移动和重组织

## 最终目录结构

```
servex/
├── app/
├── ai/
├── auth/
├── collections/
├── config/
├── discovery/
├── domain/          # +cqrs, saga, outbox
├── encoding/        # +pbjson
├── endpoint/
├── errors/
├── httputil/        # 原 request/
├── i18n/
├── messaging/       # pubsub + jobqueue
├── middleware/
├── notify/          # notification + webhook
├── oauth2/
├── observability/   # +logger
├── openapi/
├── scheduler/
├── storage/         # database→rdbms, +sqlx
├── tenant/
├── transport/
└── xutil/           # 小工具包集合
```

## 各项变更详细设计

### 1. 小工具包合并 → xutil/

将单文件/少文件的工具包合并到 `xutil/` 命名空间下，各子包保持独立 package 声明：

```
xutil/
├── ptrx/          # 原 ptrx/
├── optionx/       # 原 optionx/
├── valuex/        # 原 valuex/
├── strx/          # 原 strx/
├── randx/         # 原 randx/
├── iox/           # 原 iox/
├── copier/        # 原 copier/
├── syncx/         # 原 syncx/
├── sorting/       # 原 sorting/
├── pagination/    # 原 pagination/
├── version/       # 原 version/
└── crypto/        # 原 crypto/
```

import 路径变化：`servex/<pkg>` → `servex/xutil/<pkg>`

### 2. encoding + pbjson 合并

将 `pbjson/` 的 3 个文件移入 `encoding/pbjson/`：

```
encoding/
├── codec.go        # 不变
├── json/           # 不变
├── proto/          # 不变
├── xml/            # 不变
└── pbjson/         # 原顶层 pbjson/ 移入
```

import 路径变化：`servex/pbjson` → `servex/encoding/pbjson`

### 3. messaging/ — pubsub + jobqueue 合并

```
messaging/
├── pubsub/            # 原 pubsub/ 整体移入
│   ├── pubsub.go
│   ├── factory/
│   ├── kafka/
│   ├── rabbitmq/
│   └── redis/
└── jobqueue/          # 原 jobqueue/ 整体移入
    ├── jobqueue.go
    ├── factory/
    ├── database/
    ├── kafka/
    ├── rabbitmq/
    └── redis/
```

`messaging/` 本身为纯命名空间，无顶层 .go 文件。

import 路径变化：
- `servex/pubsub` → `servex/messaging/pubsub`
- `servex/jobqueue` → `servex/messaging/jobqueue`

### 4. observability/ — 合并 logger

```
observability/
├── logger/        # 原顶层 logger/ 移入
├── metrics/       # 不变
└── tracing/       # 不变
```

import 路径变化：`servex/logger` → `servex/observability/logger`

注意：logger 被广泛引用（app、transport、notification 等），此项涉及最多的 import 替换。

### 5. domain/ — DDD 相关聚合

```
domain/
├── aggregate.go       # 不变
├── event.go           # 不变
├── eventbus.go        # 不变
├── async_eventbus.go  # 不变
├── errors.go          # 不变
├── cqrs/              # 原顶层 cqrs/ 移入
│   ├── cqrs.go
│   └── middleware/     # CQRS 专用中间件，保留在此
├── saga/              # 原顶层 saga/ 移入
└── outbox/            # 原顶层 outbox/ 移入
```

`cqrs/middleware/` 保留在 `domain/cqrs/middleware/` — 其装饰器操作 CommandHandler/QueryHandler，类型签名与通用 endpoint middleware 不同。

import 路径变化：
- `servex/cqrs` → `servex/domain/cqrs`
- `servex/saga` → `servex/domain/saga`
- `servex/outbox` → `servex/domain/outbox`

### 6. notify/ — notification + webhook 合并

```
notify/
├── notification.go       # 原 notification/ 根文件
├── dispatcher.go
├── email/
├── sms/
├── push/
├── nwebhook/             # 原 notification/webhook/ 改名，避免路径冲突
├── factory/
├── testdata/
└── webhook/              # 原顶层 webhook/ 移入，通用 webhook 基础设施
    ├── webhook.go
    ├── dispatcher.go
    ├── receiver.go
    ├── signer.go
    └── store/
```

关键决策：原 `notification/webhook/` 改名为 `notify/nwebhook/`，避免和 `notify/webhook/` 路径冲突。

import 路径变化：
- `servex/notification` → `servex/notify`
- `servex/notification/webhook` → `servex/notify/nwebhook`
- `servex/webhook` → `servex/notify/webhook`

### 7. storage/ 内部整理 + sqlx 归入

```
storage/
├── cache/         # 不变
├── rdbms/         # 原 database/ 改名
├── mongodb/       # 不变
├── lock/          # 不变
├── s3/            # 不变
└── sqlx/          # 原顶层 sqlx/ 移入
```

import 路径变化：
- `servex/storage/database` → `servex/storage/rdbms`
- `servex/sqlx` → `servex/storage/sqlx`

### 8. request/ → httputil/ 改名

```
httputil/
├── request.go
├── activity/
├── botdetect/
├── clientip/
├── deviceinfo/
├── locale/
├── referer/
└── useragent/
```

纯目录改名。import 路径 `servex/request/*` → `servex/httputil/*`。

### 9. middleware/ 职责明确

顶层 `middleware/` 结构不变，保持通用 endpoint + HTTP handler 中间件。

职责边界明确化：
- `middleware/` = 通用 endpoint + HTTP handler 中间件
- `ai/middleware/` = AI pipeline 装饰器（保留在 ai/ 内）
- `domain/cqrs/middleware/` = CQRS handler 装饰器（保留在 domain/cqrs/ 内）
- `transport/httpclient/middleware.go` = RoundTripper 链（保留在 transport/ 内）

### 10. 不变的包

以下包结构不变：
- `app/` — 应用生命周期编排
- `config/` — 配置管理（含 source/ 子包）
- `transport/` — HTTP/gRPC/WS/SSE server+client
- `endpoint/` — endpoint 抽象
- `errors/` — HTTP+gRPC 统一错误映射
- `auth/` — 认证授权（jwt/apikey/proto）
- `oauth2/` — 第三方 OAuth（github/google/wechat）
- `ai/` — AI 集成（middleware 留在内部）
- `collections/` — 数据结构集合
- `discovery/` — 服务发现
- `i18n/` — 国际化
- `openapi/` — OpenAPI 工具
- `scheduler/` — Cron 调度
- `tenant/` — 多租户支持

## import 路径变更汇总

| 旧路径 | 新路径 |
|--------|--------|
| `servex/ptrx` | `servex/xutil/ptrx` |
| `servex/optionx` | `servex/xutil/optionx` |
| `servex/valuex` | `servex/xutil/valuex` |
| `servex/strx` | `servex/xutil/strx` |
| `servex/randx` | `servex/xutil/randx` |
| `servex/iox` | `servex/xutil/iox` |
| `servex/copier` | `servex/xutil/copier` |
| `servex/syncx` | `servex/xutil/syncx` |
| `servex/sorting` | `servex/xutil/sorting` |
| `servex/pagination` | `servex/xutil/pagination` |
| `servex/version` | `servex/xutil/version` |
| `servex/crypto` | `servex/xutil/crypto` |
| `servex/pbjson` | `servex/encoding/pbjson` |
| `servex/pubsub` | `servex/messaging/pubsub` |
| `servex/jobqueue` | `servex/messaging/jobqueue` |
| `servex/logger` | `servex/observability/logger` |
| `servex/cqrs` | `servex/domain/cqrs` |
| `servex/saga` | `servex/domain/saga` |
| `servex/outbox` | `servex/domain/outbox` |
| `servex/notification` | `servex/notify` |
| `servex/notification/*` | `servex/notify/*` |
| `servex/notification/webhook` | `servex/notify/nwebhook` |
| `servex/webhook` | `servex/notify/webhook` |
| `servex/storage/database` | `servex/storage/rdbms` |
| `servex/sqlx` | `servex/storage/sqlx` |
| `servex/request` | `servex/httputil` |
| `servex/request/*` | `servex/httputil/*` |

## 实施策略

每个变更作为独立的 commit，按依赖顺序执行：
1. 先移动无内部依赖的包（xutil 相关）
2. 再移动被依赖少的包（encoding/pbjson, storage/sqlx）
3. 最后处理被广泛依赖的包（logger, notification）

每个 commit 后确保 `go build ./...` 和 `go test ./...` 通过。
