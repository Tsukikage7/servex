package clickhouse_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/clickhouse"
	"github.com/Tsukikage7/servex/v2/testx"
)

// chAddrs 从环境变量读取，默认指向本地.
var (
	chAddrs     = []string{"localhost:9000"}
	chAvailable bool
)

func TestMain(m *testing.M) {
	if addrs := os.Getenv("CH_ADDRS"); addrs != "" {
		chAddrs = strings.Split(addrs, ",")
	}

	// 统一探测 ClickHouse 连通性（一次即可）
	chAvailable = probeCH()

	os.Exit(m.Run())
}

// probeCH 探测 ClickHouse 是否可用.
func probeCH() bool {
	cfg := &clickhouse.Config{
		Addrs:       chAddrs,
		Database:    "default",
		DialTimeout: 2 * time.Second,
	}
	client, err := clickhouse.NewClient(cfg, testx.NopLogger())
	if err != nil {
		return false
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return client.Ping(ctx) == nil
}

func skipIfNoCH(t *testing.T) {
	t.Helper()
	if !chAvailable {
		t.Skip("ClickHouse 不可用，跳过集成测试")
	}
}

func newTestClient(t *testing.T) clickhouse.Client {
	t.Helper()

	cfg := &clickhouse.Config{
		Addrs:    chAddrs,
		Database: "default",
	}

	client, err := clickhouse.NewClient(cfg, testx.NopLogger())
	if err != nil {
		t.Fatalf("创建 ClickHouse 客户端失败: %v", err)
	}
	return client
}

// ---- 单元测试（不需要服务）----

func TestNewClient_NilConfig(t *testing.T) {
	_, err := clickhouse.NewClient(nil, testx.NopLogger())
	if err != clickhouse.ErrNilConfig {
		t.Errorf("期望 ErrNilConfig，得到 %v", err)
	}
}

func TestNewClient_NilLogger(t *testing.T) {
	cfg := &clickhouse.Config{Addrs: []string{"localhost:9000"}, Database: "default"}
	_, err := clickhouse.NewClient(cfg, nil)
	if err != clickhouse.ErrNilLogger {
		t.Errorf("期望 ErrNilLogger，得到 %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := clickhouse.DefaultConfig()
	if len(cfg.Addrs) == 0 {
		t.Error("Addrs 不应为空")
	}
	if cfg.MaxOpenConns == 0 {
		t.Error("MaxOpenConns 不应为 0")
	}
	if cfg.DialTimeout == 0 {
		t.Error("DialTimeout 不应为 0")
	}
	if cfg.Compression == "" {
		t.Error("Compression 不应为空")
	}
}

// ---- 集成测试，需要 ClickHouse 实例 ----

func TestPing(t *testing.T) {
	skipIfNoCH(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

func TestExecAndQuery(t *testing.T) {
	skipIfNoCH(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()

	// 创建临时表
	err := client.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS servex_test_ch (
			id   UInt64,
			name String
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("创建表失败: %v", err)
	}
	defer client.Exec(ctx, "DROP TABLE IF EXISTS servex_test_ch") //nolint

	// 插入数据
	if err := client.Exec(ctx, "INSERT INTO servex_test_ch VALUES (?, ?)", uint64(1), "Alice"); err != nil {
		t.Fatalf("INSERT 失败: %v", err)
	}

	// 查询数据
	row := client.QueryRow(ctx, "SELECT name FROM servex_test_ch WHERE id = ?", uint64(1))
	if row.Err() != nil {
		t.Fatalf("QueryRow 失败: %v", row.Err())
	}
	var name string
	if err := row.Scan(&name); err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}
	if name != "Alice" {
		t.Errorf("期望 name=Alice，得到 %s", name)
	}

	// Select 查询
	type Record struct {
		ID   uint64 `ch:"id"`
		Name string `ch:"name"`
	}
	var records []Record
	if err := client.Select(ctx, &records, "SELECT id, name FROM servex_test_ch"); err != nil {
		t.Fatalf("Select 失败: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("期望 1 条记录，得到 %d", len(records))
	}
}
