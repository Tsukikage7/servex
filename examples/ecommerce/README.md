# Servex 电商微服务示例

基于 [servex](https://github.com/Tsukikage7/servex) 框架构建的电商微服务示例项目，展示 DDD + 六边形架构 + monorepo 的最佳实践。

## 架构说明

```
┌─────────────────────────────────────────────────────────────┐
│                        Monorepo                             │
│                                                             │
│  ┌───────────┐  ┌───────────────┐  ┌──────────────────────┐│
│  │  domain/   │  │  application/ │  │     services/        ││
│  │           │  │               │  │                      ││
│  │  user/    │  │  user/        │  │  user-service/       ││
│  │  order/   │  │  order/       │  │  order-service/      ││
│  └───────────┘  └───────────────┘  └──────────────────────┘│
│                                                             │
│  领域层           应用层              基础设施层               │
│  (纯业务逻辑)      (用例编排)          (HTTP/gRPC/DB/外部)     │
└─────────────────────────────────────────────────────────────┘
```

- **领域层 (`domain/`)**: 聚合根、领域事件、仓储接口、命令/查询对象，不依赖任何框架
- **应用层 (`application/`)**: 编排领域对象完成用例，注入事件总线发布领域事件
- **基础设施层 (`services/`)**: 每个微服务独立部署，包含 HTTP/gRPC 端口、数据库适配器、外部服务客户端

### 技术栈

| 组件 | 使用 |
|------|------|
| HTTP 服务器 | `servex/transport/httpserver` |
| gRPC 服务器 | `servex/transport/grpcserver` |
| 应用生命周期 | `servex/app` |
| 日志 | `servex/observability/logger` (zap) |
| 数据库 | `servex/storage/rdbms` (GORM + MySQL) |
| 缓存 | `servex/storage/redis` |
| 认证 | `servex/auth/jwt` |
| 领域驱动 | `servex/domain` (AggregateRoot, EventBus) |

## 快速启动

### 1. 启动基础设施

```bash
cd examples/ecommerce
docker compose -f deploy/docker-compose.yaml up -d
```

等待 MySQL 和 Redis 就绪（约 10 秒）。

### 2. 启动用户服务

```bash
go run ./services/user-service/cmd/server/
# 监听 :8081
```

### 3. 启动订单服务（另开终端）

```bash
go run ./services/order-service/cmd/server/
# 监听 :8082
```

### 4. 测试 API

```bash
# 创建用户
curl -X POST http://localhost:8081/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@example.com","password":"123456"}'

# 用户登录
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"123456"}'

# 下单（将 user_id 替换为实际值）
curl -X POST http://localhost:8082/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{"user_id":1234,"items":[{"product_id":1,"product_name":"Go 编程指南","quantity":2,"unit_price":59.9}]}'
```

## API 列表

### 用户服务 (`:8081`)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/users` | 创建用户 |
| GET | `/api/v1/users/{id}` | 查询用户 |
| PUT | `/api/v1/users/{id}` | 更新用户 |
| GET | `/api/v1/users` | 用户列表 |
| POST | `/api/v1/auth/login` | 用户登录 |

### 订单服务 (`:8082`)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/orders` | 下单 |
| GET | `/api/v1/orders/{id}` | 查询订单 |
| PUT | `/api/v1/orders/{id}/cancel` | 取消订单 |
| PUT | `/api/v1/orders/{id}/ship` | 发货 |
| PUT | `/api/v1/orders/{id}/complete` | 完成订单 |
| GET | `/api/v1/orders?user_id=xxx` | 订单列表 |

## 目录结构

```
examples/ecommerce/
├── go.mod                      # 模块定义，通过 replace 引用本地 servex
├── justfile                    # 常用命令
├── README.md
├── domain/                     # 领域层：纯业务逻辑，零框架依赖
│   ├── user/
│   │   ├── aggregate.go        # User 聚合根
│   │   ├── event.go            # UserCreated / UserUpdated 事件
│   │   ├── repository.go       # Repository 接口 + ErrNotFound
│   │   ├── command.go          # CreateUser / UpdateUser 命令
│   │   └── query.go            # GetUser / ListUsers 查询 + UserView
│   └── order/
│       ├── aggregate.go        # Order 聚合根（Place/Cancel/Ship/Complete）
│       ├── event.go            # 订单领域事件
│       ├── repository.go       # Repository 接口
│       ├── command.go          # 订单命令对象
│       ├── query.go            # 订单查询 + OrderView
│       ├── order_item.go       # OrderItem 值对象
│       └── ports.go            # UserProvider 防腐层接口
├── application/                # 应用层：用例编排
│   ├── user/service.go         # 用户 CRUD + JWT 登录
│   └── order/service.go        # 订单全生命周期管理
├── services/                   # 基础设施层：各微服务独立部署
│   ├── user-service/
│   │   ├── cmd/server/main.go  # 入口：组装依赖并启动
│   │   ├── internal/
│   │   │   ├── port/
│   │   │   │   ├── http.go     # REST 路由注册
│   │   │   │   └── grpc.go     # gRPC 服务（预留）
│   │   │   └── adapter/
│   │   │       └── persistence/
│   │   │           ├── user_repo.go   # GORM 实现
│   │   │           └── user_model.go  # PO 与聚合转换
│   │   └── configs/config.yaml
│   └── order-service/
│       ├── cmd/server/main.go
│       ├── internal/
│       │   ├── port/
│       │   │   └── http.go
│       │   └── adapter/
│       │       ├── persistence/
│       │       │   ├── order_repo.go
│       │       │   └── order_model.go
│       │       └── external/
│       │           └── user_client.go  # HTTP 调用 user-service
│       └── configs/config.yaml
├── api/                        # Proto 定义
│   ├── user/v1/user.proto
│   └── order/v1/order.proto
└── deploy/
    └── docker-compose.yaml     # MySQL + Redis
```
