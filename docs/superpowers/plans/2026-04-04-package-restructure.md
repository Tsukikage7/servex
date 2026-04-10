# Servex 包结构重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 servex 顶层目录从 40+ 精简到 ~20，通过合并碎片化工具包、按领域聚合相关包、消除功能重叠来改善包组织结构。

**Architecture:** 纯目录移动 + import 路径替换，不改变任何接口或实现逻辑。每个 Task 独立完成一个包的移动，确保每次 commit 后项目编译通过。按依赖关系从叶子节点到根节点的顺序执行。

**Tech Stack:** Go 1.26, `sed` 批量替换 import 路径, `go build ./...` 验证

**Spec 文件:** `docs/superpowers/specs/2026-04-04-package-restructure-design.md`

---

## 通用操作模式

每个 Task 遵循相同的模式：

1. `mkdir -p` 创建目标目录
2. `git mv` 移动文件（保留 git 历史）
3. `sed -i ''` 批量替换 import 路径（macOS sed）
4. 如果 package 名变了，同时替换 package 声明
5. `go build ./...` 验证编译
6. `go test ./...` 验证测试
7. `git commit`

**import 替换命令模板：**

```bash
# 替换所有 .go 文件中的 import 路径
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/OLD"|"github.com/Tsukikage7/servex/NEW"|g' {} +
```

---

### Task 1: 移动 ptrx → xutil/ptrx

零外部依赖，零内部引用。最安全的起步。

**Files:**
- Move: `ptrx/` → `xutil/ptrx/`

- [ ] **Step 1: 创建目标目录并移动文件**

```bash
mkdir -p xutil
git mv ptrx xutil/ptrx
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/ptrx"|"github.com/Tsukikage7/servex/xutil/ptrx"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./xutil/ptrx/...
```

Expected: 全部通过，无编译错误。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 ptrx → xutil/ptrx"
```

---

### Task 2: 批量移动其余零引用工具包 → xutil/

`optionx`, `valuex`, `strx`, `randx`, `iox`, `copier`, `syncx`, `crypto`, `version` 均无外部引用（仅自身测试）。批量处理。

**Files:**
- Move: `optionx/` → `xutil/optionx/`
- Move: `valuex/` → `xutil/valuex/`
- Move: `strx/` → `xutil/strx/`
- Move: `randx/` → `xutil/randx/`
- Move: `iox/` → `xutil/iox/`
- Move: `copier/` → `xutil/copier/`
- Move: `syncx/` → `xutil/syncx/`
- Move: `crypto/` → `xutil/crypto/`
- Move: `version/` → `xutil/version/`

- [ ] **Step 1: 批量移动**

```bash
for pkg in optionx valuex strx randx iox copier syncx crypto version; do
  git mv "$pkg" "xutil/$pkg"
done
```

- [ ] **Step 2: 批量替换 import 路径**

```bash
for pkg in optionx valuex strx randx iox copier syncx crypto version; do
  find . -name '*.go' -exec sed -i '' "s|\"github.com/Tsukikage7/servex/${pkg}\"|\"github.com/Tsukikage7/servex/xutil/${pkg}\"|g" {} +
done
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./xutil/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 批量移动工具包 → xutil/"
```

---

### Task 3: 移动 sorting → xutil/sorting

`sorting/` 引用了 `gorm.io/gorm`，但无项目内部引用。

**Files:**
- Move: `sorting/` → `xutil/sorting/`

- [ ] **Step 1: 移动**

```bash
git mv sorting xutil/sorting
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/sorting"|"github.com/Tsukikage7/servex/xutil/sorting"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./xutil/sorting/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 sorting → xutil/sorting"
```

---

### Task 4: 移动 pagination → xutil/pagination

`pagination/` 被 `transport/response/response.go` 引用（1 处）。

**Files:**
- Move: `pagination/` → `xutil/pagination/`
- Modify: `transport/response/response.go` (import 路径)

- [ ] **Step 1: 移动**

```bash
git mv pagination xutil/pagination
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/pagination"|"github.com/Tsukikage7/servex/xutil/pagination"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./xutil/pagination/... ./transport/response/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 pagination → xutil/pagination"
```

---

### Task 5: 移动 sqlx → storage/sqlx

零引用。

**Files:**
- Move: `sqlx/` → `storage/sqlx/`

- [ ] **Step 1: 移动**

```bash
git mv sqlx storage/sqlx
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/sqlx"|"github.com/Tsukikage7/servex/storage/sqlx"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./storage/sqlx/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 sqlx → storage/sqlx"
```

---

### Task 6: 移动 pbjson → encoding/pbjson

被 `encoding/proto/proto.go` 引用（1 处）。

**Files:**
- Move: `pbjson/` → `encoding/pbjson/`
- Modify: `encoding/proto/proto.go` (import 路径)

- [ ] **Step 1: 移动**

```bash
git mv pbjson encoding/pbjson
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/pbjson"|"github.com/Tsukikage7/servex/encoding/pbjson"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./encoding/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 pbjson → encoding/pbjson"
```

---

### Task 7: 重命名 storage/database → storage/rdbms

被 2 个文件引用：`outbox/store.go` 和 `webhook/store/gorm/store.go`。

**Files:**
- Move: `storage/database/` → `storage/rdbms/`
- Modify: 所有引用 `servex/storage/database` 的文件

- [ ] **Step 1: 移动并重命名**

```bash
git mv storage/database storage/rdbms
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/storage/database"|"github.com/Tsukikage7/servex/storage/rdbms"|g' {} +
```

- [ ] **Step 3: 替换 package 声明**

`storage/database/` 下所有文件的 `package database` 需改为 `package rdbms`：

```bash
find storage/rdbms -name '*.go' -exec sed -i '' 's|^package database$|package rdbms|g' {} +
```

- [ ] **Step 4: 替换引用方 package 别名**

引用方可能用 `database.XXX` 调用，需替换为 `rdbms.XXX`：

```bash
find . -name '*.go' -exec sed -i '' 's|database\.New|rdbms.New|g; s|database\.Config|rdbms.Config|g; s|database\.Driver|rdbms.Driver|g; s|database\.Type|rdbms.Type|g; s|database\.Err|rdbms.Err|g' {} +
```

注意：此步需要人工检查，`database.` 可能在非 servex 上下文中出现（如 `gorm.io`）。仅在引用了 `servex/storage/database` 的文件中做替换更安全：

```bash
# 更精确的替换 — 只在引用了该包的文件中
for f in outbox/store.go webhook/store/gorm/store.go; do
  sed -i '' 's|database\.|rdbms.|g' "$f"
done
```

- [ ] **Step 5: 验证编译和测试**

```bash
go build ./...
go test ./storage/rdbms/... ./outbox/... ./webhook/...
```

Expected: 全部通过。

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor: 重命名 storage/database → storage/rdbms"
```

---

### Task 8: 移动 saga → domain/saga

零外部引用（只有自身文件）。

**Files:**
- Move: `saga/` → `domain/saga/`

- [ ] **Step 1: 移动**

```bash
git mv saga domain/saga
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/saga"|"github.com/Tsukikage7/servex/domain/saga"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./domain/saga/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 saga → domain/saga"
```

---

### Task 9: 移动 cqrs → domain/cqrs

被 3 个文件引用（全部是自身 `cqrs/middleware/` 下的文件）。

**Files:**
- Move: `cqrs/` → `domain/cqrs/`

- [ ] **Step 1: 移动**

```bash
git mv cqrs domain/cqrs
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/cqrs"|"github.com/Tsukikage7/servex/domain/cqrs"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./domain/cqrs/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 cqrs → domain/cqrs"
```

---

### Task 10: 移动 outbox → domain/outbox

零外部引用。但 outbox 内部引用了 `servex/pubsub` 和 `servex/storage/database`（已在 Task 7 改为 `storage/rdbms`）。

**Files:**
- Move: `outbox/` → `domain/outbox/`

- [ ] **Step 1: 移动**

```bash
git mv outbox domain/outbox
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/outbox"|"github.com/Tsukikage7/servex/domain/outbox"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./domain/outbox/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 outbox → domain/outbox"
```

---

### Task 11: 移动 webhook → notify/webhook

被 2 个文件引用（`webhook/store/gorm/store.go` 和 `webhook/store/memory/store.go`，都是自身子包）。

**Files:**
- Move: `webhook/` → `notify/webhook/`

- [ ] **Step 1: 创建目标目录并移动**

```bash
mkdir -p notify
git mv webhook notify/webhook
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/webhook"|"github.com/Tsukikage7/servex/notify/webhook"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./notify/webhook/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 webhook → notify/webhook"
```

---

### Task 12: 移动 notification → notify/ 并重命名 notification/webhook → notify/nwebhook

`notification/` 被 5 个文件引用（全是自身子包）。需要：
1. 移动整个 `notification/` 到 `notify/`
2. 将 `notify/webhook/` (原 `notification/webhook/`) 改名为 `notify/nwebhook/` 避免与 Task 11 中的 `notify/webhook/` 冲突

**Files:**
- Move: `notification/*.go` → `notify/` (根文件)
- Move: `notification/email/` → `notify/email/`
- Move: `notification/sms/` → `notify/sms/`
- Move: `notification/push/` → `notify/push/`
- Move: `notification/webhook/` → `notify/nwebhook/`
- Move: `notification/factory/` → `notify/factory/`
- Move: `notification/testdata/` → `notify/testdata/`

- [ ] **Step 1: 移动根文件和子目录**

注意 `notify/webhook/` 已经存在（Task 11 放入的），所以 `notification/webhook/` 不能直接 `git mv` 到 `notify/webhook/`，需要改名为 `nwebhook`：

```bash
# 先移动 notification/webhook → notify/nwebhook（改名）
git mv notification/webhook notify/nwebhook

# 移动其余子目录
for dir in email sms push factory testdata; do
  git mv "notification/$dir" "notify/$dir"
done

# 移动根 .go 文件
git mv notification/*.go notify/
# 移动 README 等非 .go 文件（如果有）
git mv notification/README.md notify/ 2>/dev/null || true

# 删除空的 notification 目录
rmdir notification 2>/dev/null || true
```

- [ ] **Step 2: 替换 package 声明**

`notify/` 根下的 .go 文件 package 名从 `notification` 改为 `notify`：

```bash
find notify -maxdepth 1 -name '*.go' -exec sed -i '' 's|^package notification$|package notify|g' {} +
```

`notify/nwebhook/` 下的 .go 文件 package 名从 `webhook` 改为 `nwebhook`：

```bash
find notify/nwebhook -name '*.go' -exec sed -i '' 's|^package webhook$|package nwebhook|g' {} +
```

- [ ] **Step 3: 替换 import 路径**

```bash
# notification 根包
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/notification/webhook"|"github.com/Tsukikage7/servex/notify/nwebhook"|g' {} +
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/notification"|"github.com/Tsukikage7/servex/notify"|g' {} +
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/notification/|"github.com/Tsukikage7/servex/notify/|g' {} +
```

- [ ] **Step 4: 替换代码中的 package 引用**

引用方代码中 `notification.XXX` 需改为 `notify.XXX`，`webhook.XXX`（来自 notification/webhook）需改为 `nwebhook.XXX`：

```bash
# 在引用了 notify 包的文件中替换 notification. 前缀
# 精确替换：仅在 notify/ 子包的文件中
for f in notify/email/sender.go notify/sms/sender.go notify/push/sender.go notify/nwebhook/sender.go notify/factory/factory.go; do
  sed -i '' 's|notification\.|notify.|g' "$f"
done

# nwebhook 包内如果有 webhook. 自引用（不太可能，但检查）
# 主要是外部引用 notification/webhook 的地方，现在改为 nwebhook
```

- [ ] **Step 5: 验证编译和测试**

```bash
go build ./...
go test ./notify/...
```

Expected: 全部通过。

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor: 移动 notification → notify/，webhook 渠道改名 nwebhook"
```

---

### Task 13: 移动 pubsub → messaging/pubsub

被 12 个文件引用（domain/async_eventbus, outbox/*, pubsub/子包, request/activity）。

**Files:**
- Move: `pubsub/` → `messaging/pubsub/`
- Modify: 所有引用 `servex/pubsub` 的文件

- [ ] **Step 1: 创建目标目录并移动**

```bash
mkdir -p messaging
git mv pubsub messaging/pubsub
```

- [ ] **Step 2: 替换 import 路径**

注意要先替换子包路径再替换根路径，避免部分匹配：

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/pubsub/|"github.com/Tsukikage7/servex/messaging/pubsub/|g' {} +
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/pubsub"|"github.com/Tsukikage7/servex/messaging/pubsub"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./messaging/pubsub/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 pubsub → messaging/pubsub"
```

---

### Task 14: 移动 jobqueue → messaging/jobqueue

被 7 个文件引用（jobqueue/子包 + notification/dispatcher + notification/options）。注意 notification 此时已经变成 notify。

**Files:**
- Move: `jobqueue/` → `messaging/jobqueue/`
- Modify: 所有引用 `servex/jobqueue` 的文件

- [ ] **Step 1: 移动**

```bash
git mv jobqueue messaging/jobqueue
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/jobqueue/|"github.com/Tsukikage7/servex/messaging/jobqueue/|g' {} +
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/jobqueue"|"github.com/Tsukikage7/servex/messaging/jobqueue"|g' {} +
```

- [ ] **Step 3: 验证编译和测试**

```bash
go build ./...
go test ./messaging/jobqueue/...
```

Expected: 全部通过。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: 移动 jobqueue → messaging/jobqueue"
```

---

### Task 15: 移动 request → httputil

被 1 个文件外部引用（`request/request.go` 自引用），子包被 `transport/grpcserver/` 和 `transport/httpserver/` 引用。

**Files:**
- Move: `request/` → `httputil/`
- Modify: 所有引用 `servex/request` 的文件

- [ ] **Step 1: 移动**

```bash
git mv request httputil
```

- [ ] **Step 2: 替换 package 声明**

根 package 名从 `request` 改为 `httputil`：

```bash
find httputil -maxdepth 1 -name '*.go' -exec sed -i '' 's|^package request$|package httputil|g' {} +
```

- [ ] **Step 3: 替换 import 路径**

```bash
# 先替换子包路径
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/request/|"github.com/Tsukikage7/servex/httputil/|g' {} +
# 再替换根包
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/request"|"github.com/Tsukikage7/servex/httputil"|g' {} +
```

- [ ] **Step 4: 替换代码中的 package 引用**

```bash
# request. 前缀较通用，需精确匹配。检查哪些文件引用了 request 包：
grep -rl '"github.com/Tsukikage7/servex/httputil"' --include='*.go' . | while read f; do
  sed -i '' 's|request\.|httputil.|g' "$f"
done
```

注意：`httputil/request.go` 自身内部不需要改（它是包定义文件），但如果 `httputil/` 子包有 `import "servex/request"` 则那个 import 已被 Step 3 改掉。

- [ ] **Step 5: 验证编译和测试**

```bash
go build ./...
go test ./httputil/...
```

Expected: 全部通过。

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "refactor: 移动 request → httputil"
```

---

### Task 16: 移动 logger → observability/logger

被 76 个文件引用 — 影响面最大的一步。但操作本身是纯机械替换。

**Files:**
- Move: `logger/` → `observability/logger/`
- Modify: 76 个引用 `servex/logger` 的 .go 文件

- [ ] **Step 1: 移动**

```bash
git mv logger observability/logger
```

- [ ] **Step 2: 替换 import 路径**

```bash
find . -name '*.go' -exec sed -i '' 's|"github.com/Tsukikage7/servex/logger"|"github.com/Tsukikage7/servex/observability/logger"|g' {} +
```

包名仍然是 `logger`，所以代码中 `logger.XXX` 的调用无需修改。

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```

Expected: 全部通过。由于只改了 import 路径，package 名不变，引用方代码无需任何修改。

- [ ] **Step 4: 运行测试**

```bash
go test ./...
```

Expected: 全部通过。

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "refactor: 移动 logger → observability/logger"
```

---

### Task 17: 最终验证 + 清理

确认所有移动完成，项目结构正确，无残留空目录。

- [ ] **Step 1: 检查残留空目录**

```bash
find . -type d -empty -not -path './.git/*' -not -path './.claude/*'
```

如有空目录，删除：

```bash
find . -type d -empty -not -path './.git/*' -not -path './.claude/*' -delete
```

- [ ] **Step 2: 验证顶层目录数**

```bash
ls -d */ | grep -v '^\.' | wc -l
```

Expected: ~20 个目录。

- [ ] **Step 3: 验证目录结构符合设计**

```bash
ls -d */
```

Expected 输出应包含且仅包含：
```
ai/  app/  auth/  collections/  config/  discovery/  domain/  encoding/
endpoint/  errors/  httputil/  i18n/  messaging/  middleware/  notify/
oauth2/  observability/  openapi/  scheduler/  storage/  tenant/
transport/  xutil/
```

- [ ] **Step 4: 完整编译和测试**

```bash
go build ./...
go test ./...
```

Expected: 全部通过。

- [ ] **Step 5: 检查无旧 import 残留**

```bash
# 确认没有遗漏的旧 import 路径
grep -r '"github.com/Tsukikage7/servex/ptrx"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/logger"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/notification"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/pubsub"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/jobqueue"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/request"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/webhook"' --include='*.go' . | grep -v 'notify/webhook'
grep -r '"github.com/Tsukikage7/servex/cqrs"' --include='*.go' . | grep -v 'domain/cqrs'
grep -r '"github.com/Tsukikage7/servex/saga"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/outbox"' --include='*.go' . | grep -v 'domain/outbox'
grep -r '"github.com/Tsukikage7/servex/pbjson"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/sqlx"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/sorting"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/pagination"' --include='*.go' .
grep -r '"github.com/Tsukikage7/servex/storage/database"' --include='*.go' .
```

Expected: 全部无输出（零匹配）。

- [ ] **Step 6: Commit 清理（如果有）**

```bash
git add -A && git status
# 如果有变更：
git commit -m "refactor: 清理重构残留"
```
