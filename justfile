# servex justfile
# 使用: just <recipe>
# 安装 just: https://github.com/casey/just

# Docker Compose 文件路径
compose_file := "tests/docker-compose.yml"

# 默认 recipe
default: check

# ── 检查 ──────────────────────────────────────────────

# 完整检查lint + deps-check + test + build + vuln
check: lint deps-check test-unit build vuln

# 发布前工作区检查覆盖 go.work 中的所有 module
check-workspace:
    ./scripts/check-workspace.sh

# ── 构建 ──────────────────────────────────────────────

# 编译所有包
build:
    go build ./...

# 编译并安装
install:
    go install ./...

# 清理构建产物
clean:
    rm -f coverage*.out coverage*.html
    go clean -cache -testcache

# ── 测试 ──────────────────────────────────────────────

# 单元测试不需要外部服务
test-unit:
    go test -short -race -coverprofile=coverage.out ./...
    @echo ""
    @echo "Coverage:"
    @go tool cover -func=coverage.out | tail -1

# 集成测试需要先 just services-up 启动依赖
test-integration:
    #!/usr/bin/env bash
    set -euo pipefail
    set -a; source tests/.env.test; set +a
    go test -race -count=1 -tags=integration ./tests/integration/...

# 在 Docker 中运行全量测试自动启动服务 + 跑测试 + 清理
test-docker:
    #!/usr/bin/env bash
    set -e
    echo "Starting services..."
    docker compose -f tests/docker-compose.yml up -d
    echo "Waiting for services..."
    sleep 15
    echo "Running tests..."
    set -a; source tests/.env.test; set +a
    go test -race -count=1 -tags=integration ./tests/integration/... || { docker compose -f tests/docker-compose.yml down -v; exit 1; }
    echo "Cleaning up..."
    docker compose -f tests/docker-compose.yml down -v
    echo "Done."

# 全量测试单元 + 集成
test-all: test-unit test-integration

# 运行指定包的测试
test pkg:
    go test -v -race ./{{pkg}}/...

# 运行匹配名称的测试
test-run pattern:
    go test -v -race -run {{pattern}} ./...

# ── 覆盖率 ──────────────────────────────────────────────

# 覆盖率报告终端
coverage:
    go test -short -race -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out | tail -20

# 覆盖率报告HTML
coverage-html:
    go test -short -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Report: coverage.html"
    open coverage.html 2>/dev/null || xdg-open coverage.html 2>/dev/null || echo "Open coverage.html in browser"

# 覆盖率分布统计
coverage-stats:
    #!/usr/bin/env bash
    go test -short -cover ./... 2>&1 | tee /tmp/servex-cov.txt
    echo ""
    echo "=== 覆盖率分布 ==="
    zero=$(grep 'coverage: 0.0%' /tmp/servex-cov.txt | wc -l | tr -d ' ')
    low=$(grep 'coverage:' /tmp/servex-cov.txt | sed 's/.*coverage: //' | sed 's/%.*//' | awk '$1 > 0 && $1 < 50' | wc -l | tr -d ' ')
    mid=$(grep 'coverage:' /tmp/servex-cov.txt | sed 's/.*coverage: //' | sed 's/%.*//' | awk '$1 >= 50 && $1 < 80' | wc -l | tr -d ' ')
    high=$(grep 'coverage:' /tmp/servex-cov.txt | sed 's/.*coverage: //' | sed 's/%.*//' | awk '$1 >= 80 && $1 < 100' | wc -l | tr -d ' ')
    full=$(grep 'coverage: 100.0%' /tmp/servex-cov.txt | wc -l | tr -d ' ')
    echo "  0%:      $zero"
    echo "  1-49%:   $low"
    echo "  50-79%:  $mid"
    echo "  80-99%:  $high"
    echo "  100%:    $full"
    echo ""
    grep 'coverage:' /tmp/servex-cov.txt | sed 's/.*coverage: //' | sed 's/% of statements//' | awk '{sum+=$1; n++} END {printf "平均: %.1f%% (%d 个包)\n", sum/n, n}'
    grep 'coverage:' /tmp/servex-cov.txt | sed 's/.*coverage: //' | sed 's/% of statements//' | awk '$1 > 0 {sum+=$1; n++} END {printf "排除 0%%: %.1f%% (%d 个包)\n", sum/n, n}'

# ── 代码质量 ──────────────────────────────────────────────

# 代码检查golangci-lint
lint:
    golangci-lint run ./... 2>/dev/null || (echo "golangci-lint not found, falling back to go vet" && go vet ./...)

# go vet
vet:
    go vet ./...

# 检查基础包依赖边界
deps-check:
    ./scripts/check-deps.sh

# 格式化修改文件
fmt:
    gofmt -w -s .
    goimports -w -local github.com/Tsukikage7/servex/v2 .

# 格式化检查只检查不修改，适用于 CI
fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    bad=$(gofmt -l -s . | head -20)
    if [ -n "$bad" ]; then
      echo "gofmt: 以下文件格式不符合规范:"
      echo "$bad"
      exit 1
    fi
    bad=$(goimports -l -local github.com/Tsukikage7/servex/v2 . | head -20)
    if [ -n "$bad" ]; then
      echo "goimports: 以下文件 import 顺序不符合规范:"
      echo "$bad"
      exit 1
    fi

# 依赖整理
tidy:
    go mod tidy

# 检查依赖漏洞
vuln:
    ./scripts/check-vuln.sh

# 检查过时依赖
outdated:
    go list -m -u all 2>/dev/null | grep '\[' || echo "All dependencies up to date"

# ── 生成 ──────────────────────────────────────────────

# 生成 protobuf (buf)
proto:
    buf generate auth/proto

# ── 文档 ──────────────────────────────────────────────

# 启动本地 godoc
doc:
    @echo "http://localhost:6060/pkg/github.com/Tsukikage7/servex/v2/"
    godoc -http=:6060

# 统计项目规模
stats:
    #!/usr/bin/env bash
    echo "=== 项目统计 ==="
    echo -n "Go 包数:      "; find . -name '*.go' -not -path './.git/*' | xargs dirname | sort -u | wc -l | tr -d ' '
    echo -n "Go 文件数:    "; find . -name '*.go' -not -path './.git/*' | wc -l | tr -d ' '
    echo -n "测试文件数:   "; find . -name '*_test.go' -not -path './.git/*' | wc -l | tr -d ' '
    echo -n "Go 代码行数:  "; find . -name '*.go' -not -name '*_test.go' -not -path './.git/*' | xargs wc -l 2>/dev/null | tail -1 | tr -d ' '
    echo -n "测试代码行数: "; find . -name '*_test.go' -not -path './.git/*' | xargs wc -l 2>/dev/null | tail -1 | tr -d ' '
    echo -n "README 数:    "; find . -name 'README.md' -not -path './.git/*' | wc -l | tr -d ' '
    echo -n "Skill 数:     "; find skills -name 'SKILL.md' | wc -l | tr -d ' '

# ── 服务管理 ──────────────────────────────────────────────

# 启动所有测试依赖服务
services-up:
    docker compose -f {{compose_file}} up -d
    @echo ""
    @echo "等待服务就绪..."
    @sleep 5
    @just services-check

# 停止所有测试依赖服务
services-down:
    docker compose -f {{compose_file}} down -v

# 查看服务日志
services-logs *args:
    docker compose -f {{compose_file}} logs {{args}}

# 检查服务可用性
services-check:
    #!/usr/bin/env bash
    echo "=== 服务状态检查 ==="
    check() {
      if eval "$2" >/dev/null 2>&1; then echo "  [OK] $1"; else echo "  [--] $1"; fi
    }
    check "Redis        (6379)" "redis-cli -h localhost -p 6379 ping"
    check "PostgreSQL   (5432)" "pg_isready -h localhost -p 5432"
    check "MongoDB      (27017)" "mongosh --host localhost --port 27017 --eval 'db.runCommand({ping:1})' --quiet"
    check "Elasticsearch(9200)" "curl -sf http://localhost:9200/_cluster/health"
    check "ClickHouse   (8123)" "curl -sf 'http://localhost:8123/?query=SELECT%201'"
    check "MySQL        (3306)" "mysqladmin -h localhost -P 3306 -u root -p123456 ping 2>/dev/null"
    check "MinIO        (9000)" "curl -sf http://localhost:9000/minio/health/live"
    check "Kafka        (9092)" "nc -z localhost 9092"
    check "Consul       (8500)" "curl -sf http://localhost:8500/v1/status/leader"
    check "Etcd         (2379)" "curl -sf http://localhost:2379/health"

# 启动指定服务如 just service redis postgres
service *names:
    docker compose -f {{compose_file}} up -d {{names}}

# ── CI 本地模拟 ──────────────────────────────────────────────

# 模拟 CI 完整流程仅单元测试
ci: tidy fmt-check lint vet test-unit build
    @echo ""
    @echo "CI 检查全部通过"

# 模拟 CI 完整流程含集成测试，需要先 just services-up
ci-full: tidy fmt-check lint vet test-unit test-integration build
    @echo ""
    @echo "CI 全量检查通过"
