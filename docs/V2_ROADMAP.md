# servex v2.0.0 规划

> 综合来源: CODE_REVIEW.md (154 个问题) + 3 份调研报告(Go 生态对标/安全+可观测/架构短板)

---

## 一、CODE_REVIEW 问题修复（必须全部完成）

### 第一批：Critical + High（44 个，必须在 v2.0.0-alpha 前完成）

**Critical (10 个):**
- C-01: RabbitMQ 并发发布确认错乱
- C-02: crypto 包验证码用非安全随机数
- C-03: 可重入锁未基于 goroutine 身份
- C-04: 验证码比对时序攻击
- C-05: 微信 OAuth redirect_uri bug
- C-06: memoryCache.Close() 重复调用 panic
- C-07: app.start() 100ms 后错误丢弃（持续消费 errCh）
- C-08: 服务 ID 使用不安全随机数
- C-09: Gemini API Key 暴露在 URL
- C-10: LLM 护栏只检查 Content 不检查 Parts

**High (34 个):** H-01 ~ H-34 全部

### 第二批：Medium + Low（110 个，在 v2.0.0-beta 前完成）

M-01 ~ M-79, L-01 ~ L-31 全部

---

## 二、Go 现代特性升级

| 优先级 | 特性 | 改动范围 |
|--------|------|---------|
| P0 | collections 包添加 `iter.Seq`/`iter.Seq2` 迭代器方法 | collections/ 全部子包 |
| P0 | `log/slog` 适配器（Logger 接口桥接 slog.Handler） | observability/logger/ |
| P0 | `testing/synctest` 替代 time.Sleep 式并发测试 | middleware/ratelimit, circuitbreaker, scheduler 测试 |
| P1 | `sync.WaitGroup.Go()` 替代手动 Add/Done | app/, syncx/ |
| P1 | `runtime/trace.FlightRecorder` 集成 | observability/profiling/ |
| P2 | `new(expr)` 简化 ptrx.Of | xutil/ptrx/ |

---

## 三、安全增强

| 优先级 | 特性 | 说明 |
|--------|------|------|
| P0 | JWT 非对称签名（RS256/ES256/EdDSA） | auth/jwt 目前仅 HMAC |
| P0 | OIDC 支持 | 新增 auth/oidc 包 |
| P0 | Secrets 管理（Vault/KMS） | 新增 storage/secrets 包 |
| P1 | ABAC / OPA 集成 | auth/policy 包 |
| P1 | 自动证书轮换 | transport/tls 增强 |
| P1 | WAF 中间件（OWASP Top 10） | 新增 middleware/waf |
| P2 | 敏感字段日志脱敏（密码/URI/Key） | 跨模块统一处理 |

---

## 四、可观测性增强

| 优先级 | 特性 | 说明 |
|--------|------|------|
| P0 | OpenTelemetry Metrics（OTLP 导出） | 当前仅 Prometheus |
| P0 | OTel Logs 集成 | observability/logs 新包 |
| P1 | 调试面板 HTTP handler（/debug/） | 路由/中间件/config/metrics/pprof 聚合 |
| P1 | goroutine 泄漏检测（Go 1.26 profile） | testx + profiling |
| P2 | 内置依赖健康检查器（DB/Redis/外部服务） | transport/health 增强 |

---

## 五、架构改进

| 优先级 | 特性 | 说明 |
|--------|------|------|
| P0 | gRPC 错误用标准 errdetails（替代 JSON-in-message） | errors/grpc.go |
| P0 | protoc-gen-servex 完整代码生成链 | proto → HTTP+gRPC+OpenAPI+client SDK |
| P1 | API 版本化中间件 | path/header 双模式 |
| P1 | 优雅降级：fallback + load shedding 中间件 | middleware/fallback, middleware/loadshed |
| P1 | OpenAPI 3.1 升级 | openapi/ |
| P2 | 子模块拆分（可选依赖轻量化） | 重依赖包独立 go.mod |
| P2 | 服务网格感知（Istio header 传播） | transport/mesh |

---

## 六、开发体验（DX）

| 优先级 | 特性 | 说明 |
|--------|------|------|
| P0 | `servex dev` 热重载命令（集成 air） | cmd/servex |
| P0 | 配置错误增强（key path + source + expected type） | config/ |
| P1 | K8s manifest 生成（`servex gen k8s`） | Deployment+Service+HPA |
| P1 | 迁移指南 v1→v2 | 文档 |
| P2 | 合约测试（Pact） | testing/contract |
| P2 | 混沌工程 hooks | testing/chaos |

---

## 七、里程碑

| 阶段 | 内容 | 预计 |
|------|------|------|
| **v2.0.0-alpha** | CODE_REVIEW Critical+High 全修 + Go 现代特性 P0 + 安全 P0 | 第 1-2 周 |
| **v2.0.0-beta** | CODE_REVIEW Medium+Low 全修 + 可观测 P0 + 架构 P0 + DX P0 | 第 3-4 周 |
| **v2.0.0-rc** | P1 全部 + 迁移指南 + 全量测试 | 第 5-6 周 |
| **v2.0.0** | P2 + 文档完善 + 性能基准 | 第 7-8 周 |
