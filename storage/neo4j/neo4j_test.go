package neo4j_test

import (
	"testing"
	"time"

	neodriver "github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/Tsukikage7/servex/observability/logger"
	neo4jpkg "github.com/Tsukikage7/servex/storage/neo4j"
	"github.com/Tsukikage7/servex/testx"
)

// ---- 单元测试（不需要服务）----

func TestNewClient_NilConfig(t *testing.T) {
	_, err := neo4jpkg.NewClient(nil)
	if err != neo4jpkg.ErrNilConfig {
		t.Errorf("期望 ErrNilConfig，得到 %v", err)
	}
}

func TestNewClient_EmptyURI(t *testing.T) {
	cfg := &neo4jpkg.Config{Database: "neo4j"}
	_, err := neo4jpkg.NewClient(cfg)
	if err != neo4jpkg.ErrEmptyURI {
		t.Errorf("期望 ErrEmptyURI，得到 %v", err)
	}
}

func TestNewClient_EmptyDatabase(t *testing.T) {
	cfg := &neo4jpkg.Config{URI: "neo4j://localhost:7687", Database: ""}
	// ApplyDefaults 会设置默认数据库名，所以不会返回 ErrEmptyDatabase
	// 必须手动绕过
	cfg.Database = "" // 确保为空
	// ApplyDefaults 会将空数据库名填上默认值 "neo4j"，因此不能测到 ErrEmptyDatabase
	// 通过直接调用 Validate 来测试
	if err := cfg.Validate(); err != neo4jpkg.ErrEmptyDatabase {
		t.Errorf("期望 ErrEmptyDatabase，得到 %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := neo4jpkg.DefaultConfig()
	if cfg.MaxConnectionPoolSize == 0 {
		t.Error("MaxConnectionPoolSize 不应为 0")
	}
	if cfg.ConnectionAcquisitionTimeout == 0 {
		t.Error("ConnectionAcquisitionTimeout 不应为 0")
	}
	if cfg.MaxTransactionRetryTime == 0 {
		t.Error("MaxTransactionRetryTime 不应为 0")
	}
	if cfg.Database != "neo4j" {
		t.Errorf("期望默认 Database 为 neo4j，得到 %q", cfg.Database)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     neo4jpkg.Config
		wantErr error
	}{
		{
			name:    "空 URI",
			cfg:     neo4jpkg.Config{URI: "", Database: "db"},
			wantErr: neo4jpkg.ErrEmptyURI,
		},
		{
			name:    "空数据库名",
			cfg:     neo4jpkg.Config{URI: "neo4j://localhost:7687", Database: ""},
			wantErr: neo4jpkg.ErrEmptyDatabase,
		},
		{
			name:    "有效配置",
			cfg:     neo4jpkg.Config{URI: "neo4j://localhost:7687", Database: "neo4j"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err != tt.wantErr {
				t.Errorf("期望 %v，得到 %v", tt.wantErr, err)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	cfg := &neo4jpkg.Config{
		URI: "neo4j://localhost:7687",
	}
	cfg.ApplyDefaults()

	if cfg.Database != "neo4j" {
		t.Errorf("期望默认 Database 为 neo4j，得到 %q", cfg.Database)
	}
	if cfg.MaxConnectionPoolSize != 100 {
		t.Errorf("期望默认 MaxConnectionPoolSize 为 100，得到 %d", cfg.MaxConnectionPoolSize)
	}
	if cfg.ConnectionAcquisitionTimeout != 60*time.Second {
		t.Errorf("期望默认 ConnectionAcquisitionTimeout 为 60s，得到 %v", cfg.ConnectionAcquisitionTimeout)
	}
	if cfg.MaxTransactionRetryTime != 30*time.Second {
		t.Errorf("期望默认 MaxTransactionRetryTime 为 30s，得到 %v", cfg.MaxTransactionRetryTime)
	}
}

func TestConfig_ApplyDefaults_NoOverwrite(t *testing.T) {
	cfg := &neo4jpkg.Config{
		URI:                          "neo4j://localhost:7687",
		Database:                     "custom",
		MaxConnectionPoolSize:        50,
		ConnectionAcquisitionTimeout: 10 * time.Second,
		MaxTransactionRetryTime:      5 * time.Second,
	}
	cfg.ApplyDefaults()

	if cfg.Database != "custom" {
		t.Errorf("不应覆盖已设置的 Database，得到 %q", cfg.Database)
	}
	if cfg.MaxConnectionPoolSize != 50 {
		t.Errorf("不应覆盖已设置的 MaxConnectionPoolSize，得到 %d", cfg.MaxConnectionPoolSize)
	}
}

func TestRecord_GetByIndex(t *testing.T) {
	r := &neo4jpkg.Record{
		Keys:   []string{"name", "age"},
		Values: []any{"Alice", 30},
	}

	if v := r.GetByIndex(0); v != "Alice" {
		t.Errorf("期望 Alice，得到 %v", v)
	}
	if v := r.GetByIndex(1); v != 30 {
		t.Errorf("期望 30，得到 %v", v)
	}
	if v := r.GetByIndex(-1); v != nil {
		t.Errorf("期望 nil（负索引），得到 %v", v)
	}
	if v := r.GetByIndex(2); v != nil {
		t.Errorf("期望 nil（越界），得到 %v", v)
	}
}

func TestRecord_Get(t *testing.T) {
	r := &neo4jpkg.Record{
		Keys:   []string{"name", "age"},
		Values: []any{"Alice", 30},
	}

	v, ok := r.Get("name")
	if !ok || v != "Alice" {
		t.Errorf("期望 (Alice, true)，得到 (%v, %v)", v, ok)
	}

	v, ok = r.Get("age")
	if !ok || v != 30 {
		t.Errorf("期望 (30, true)，得到 (%v, %v)", v, ok)
	}

	v, ok = r.Get("missing")
	if ok || v != nil {
		t.Errorf("期望 (nil, false)，得到 (%v, %v)", v, ok)
	}
}

func TestWithLogger(t *testing.T) {
	// 验证 WithLogger 选项不会 panic
	log := testx.NopLogger()
	opt := neo4jpkg.WithLogger(log)
	if opt == nil {
		t.Error("WithLogger 不应返回 nil")
	}
}

func TestClose_NotConnected(t *testing.T) {
	// Client 零值应当返回 ErrNotConnected
	client := &neo4jpkg.Client{}
	err := client.Close(t.Context())
	if err != neo4jpkg.ErrNotConnected {
		t.Errorf("期望 ErrNotConnected，得到 %v", err)
	}
}

func TestRecord_Get_KeysMoreThanValues(t *testing.T) {
	// 测试 Keys 比 Values 多的情况（index < len(Values) 分支为 false）
	r := &neo4jpkg.Record{
		Keys:   []string{"name", "age", "extra"},
		Values: []any{"Alice", 30},
	}

	// "extra" 的 index 是 2，len(Values) 是 2，所以 i < len(r.Values) 为 false
	v, ok := r.Get("extra")
	if ok || v != nil {
		t.Errorf("期望 (nil, false)，得到 (%v, %v)", v, ok)
	}
}

func TestRecord_GetByIndex_EmptyRecord(t *testing.T) {
	r := &neo4jpkg.Record{}
	if v := r.GetByIndex(0); v != nil {
		t.Errorf("期望 nil，得到 %v", v)
	}
}

func TestRecord_Get_EmptyRecord(t *testing.T) {
	r := &neo4jpkg.Record{}
	v, ok := r.Get("any")
	if ok || v != nil {
		t.Errorf("期望 (nil, false)，得到 (%v, %v)", v, ok)
	}
}

func TestRecord_Get_NilValues(t *testing.T) {
	r := &neo4jpkg.Record{
		Keys:   []string{"name"},
		Values: []any{nil},
	}
	v, ok := r.Get("name")
	if !ok {
		t.Error("期望找到 key")
	}
	if v != nil {
		t.Errorf("期望 nil value，得到 %v", v)
	}
}

func TestRecord_GetByIndex_VariousTypes(t *testing.T) {
	r := &neo4jpkg.Record{
		Keys:   []string{"str", "int", "float", "bool", "nil"},
		Values: []any{"hello", 42, 3.14, true, nil},
	}

	if v := r.GetByIndex(0); v != "hello" {
		t.Errorf("期望 hello，得到 %v", v)
	}
	if v := r.GetByIndex(2); v != 3.14 {
		t.Errorf("期望 3.14，得到 %v", v)
	}
	if v := r.GetByIndex(3); v != true {
		t.Errorf("期望 true，得到 %v", v)
	}
	if v := r.GetByIndex(4); v != nil {
		t.Errorf("期望 nil，得到 %v", v)
	}
}

func TestNewClient_InvalidURI(t *testing.T) {
	// A URI that will be accepted by config validation but fail at driver creation
	// or connectivity check. The neo4j driver typically fails on VerifyConnectivity.
	cfg := &neo4jpkg.Config{
		URI:      "neo4j://localhost:7687",
		Database: "neo4j",
	}
	_, err := neo4jpkg.NewClient(cfg)
	// Should fail because no server is running — error is expected
	if err == nil {
		t.Error("期望连接失败的错误，得到 nil")
	}
}

func TestNewClient_WithCredentials(t *testing.T) {
	cfg := &neo4jpkg.Config{
		URI:      "neo4j://localhost:7687",
		Database: "neo4j",
		Username: "neo4j",
		Password: "password",
	}
	_, err := neo4jpkg.NewClient(cfg)
	// Should fail because no server is running
	if err == nil {
		t.Error("期望连接失败的错误，得到 nil")
	}
}

func TestNewClient_WithLogger(t *testing.T) {
	log := testx.NopLogger()
	cfg := &neo4jpkg.Config{
		URI:      "neo4j://localhost:7687",
		Database: "neo4j",
	}
	_, err := neo4jpkg.NewClient(cfg, neo4jpkg.WithLogger(log))
	// Should fail because no server is running
	if err == nil {
		t.Error("期望连接失败的错误，得到 nil")
	}
}

func TestNewClient_CustomPoolSettings(t *testing.T) {
	cfg := &neo4jpkg.Config{
		URI:                          "neo4j://localhost:7687",
		Database:                     "testdb",
		MaxConnectionPoolSize:        50,
		ConnectionAcquisitionTimeout: 10 * time.Second,
		MaxTransactionRetryTime:      5 * time.Second,
		Encrypted:                    true,
	}
	_, err := neo4jpkg.NewClient(cfg)
	if err == nil {
		t.Error("期望连接失败的错误，得到 nil")
	}
}

func TestConfig_Validate_ValidWithOptionalFields(t *testing.T) {
	cfg := neo4jpkg.Config{
		URI:      "bolt://localhost:7687",
		Database: "mydb",
		Username: "user",
		Password: "pass",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("期望无错误，得到 %v", err)
	}
}

func TestConfig_ApplyDefaults_PartialCustom(t *testing.T) {
	cfg := &neo4jpkg.Config{
		URI:                   "neo4j://localhost:7687",
		Database:              "customdb",
		MaxConnectionPoolSize: 200,
		// Leave timeouts at zero — should get defaults
	}
	cfg.ApplyDefaults()

	if cfg.Database != "customdb" {
		t.Errorf("不应覆盖已设置的 Database，得到 %q", cfg.Database)
	}
	if cfg.MaxConnectionPoolSize != 200 {
		t.Errorf("不应覆盖已设置的 MaxConnectionPoolSize，得到 %d", cfg.MaxConnectionPoolSize)
	}
	if cfg.ConnectionAcquisitionTimeout != 60*time.Second {
		t.Errorf("期望默认 ConnectionAcquisitionTimeout，得到 %v", cfg.ConnectionAcquisitionTimeout)
	}
	if cfg.MaxTransactionRetryTime != 30*time.Second {
		t.Errorf("期望默认 MaxTransactionRetryTime，得到 %v", cfg.MaxTransactionRetryTime)
	}
}

// 确保导入不报错（编译检查）.
var (
	_ neodriver.AccessMode
	_ logger.Logger
)
