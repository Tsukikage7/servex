package elasticsearch_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/elasticsearch"
	"github.com/Tsukikage7/servex/v2/testx"
)

// esAddresses 从环境变量读取，默认指向本地.
var (
	esAddresses = []string{"http://localhost:9200"}
	esAvailable bool
)

func TestMain(m *testing.M) {
	if addr := os.Getenv("ES_ADDRESSES"); addr != "" {
		esAddresses = []string{addr}
	}

	// 探测 Elasticsearch 连通性
	esAvailable = probeES()

	os.Exit(m.Run())
}

// probeES 探测 Elasticsearch 是否可用.
func probeES() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(esAddresses[0])
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func skipIfNoES(t *testing.T) {
	t.Helper()
	if !esAvailable {
		t.Skip("Elasticsearch 不可用，跳过集成测试")
	}
}

func newTestClient(t *testing.T) elasticsearch.Client {
	t.Helper()

	cfg := &elasticsearch.Config{
		Addresses: esAddresses,
	}

	client, err := elasticsearch.NewClient(cfg, testx.NopLogger())
	if err != nil {
		t.Fatalf("创建 Elasticsearch 客户端失败: %v", err)
	}
	return client
}

// ---- 单元测试不需要服务----

func TestNewClient_NilConfig(t *testing.T) {
	_, err := elasticsearch.NewClient(nil, testx.NopLogger())
	if err != elasticsearch.ErrNilConfig {
		t.Errorf("期望 ErrNilConfig，得到 %v", err)
	}
}

func TestNewClient_NilLogger(t *testing.T) {
	cfg := &elasticsearch.Config{Addresses: []string{"http://localhost:9200"}}
	_, err := elasticsearch.NewClient(cfg, nil)
	if err != elasticsearch.ErrNilLogger {
		t.Errorf("期望 ErrNilLogger，得到 %v", err)
	}
}

func TestNewClient_EmptyAddresses(t *testing.T) {
	cfg := &elasticsearch.Config{}
	_, err := elasticsearch.NewClient(cfg, testx.NopLogger())
	if err != elasticsearch.ErrEmptyAddresses {
		t.Errorf("期望 ErrEmptyAddresses，得到 %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := elasticsearch.DefaultConfig()
	if cfg.MaxRetries == 0 {
		t.Error("MaxRetries 不应为 0")
	}
	if len(cfg.Addresses) == 0 {
		t.Error("Addresses 不应为空")
	}
	if cfg.ResponseHeaderTimeout == 0 {
		t.Error("ResponseHeaderTimeout 不应为 0")
	}
}

// ---- 集成测试，需要 Elasticsearch 实例 ----

func TestPing(t *testing.T) {
	skipIfNoES(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

func TestIndexCreateAndDelete(t *testing.T) {
	skipIfNoES(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	indexName := "servex_test_index"
	idx := client.Index(indexName)

	// 清理可能残留的测试索引
	_ = idx.Delete(ctx)

	// 创建索引
	err := idx.Create(ctx, map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
	})
	if err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}

	// 检查索引存在
	exists, err := idx.Exists(ctx)
	if err != nil {
		t.Fatalf("检查索引失败: %v", err)
	}
	if !exists {
		t.Fatal("索引应该存在")
	}

	// 删除索引
	err = idx.Delete(ctx)
	if err != nil {
		t.Fatalf("删除索引失败: %v", err)
	}

	// 确认已删除
	exists, err = idx.Exists(ctx)
	if err != nil {
		t.Fatalf("检查索引失败: %v", err)
	}
	if exists {
		t.Fatal("索引不应该存在")
	}
}

func TestDocumentCRUD(t *testing.T) {
	skipIfNoES(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	indexName := "servex_test_doc_crud"
	idx := client.Index(indexName)

	// 创建索引
	_ = idx.Delete(ctx)
	err := idx.Create(ctx, map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
	})
	if err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}
	defer idx.Delete(ctx) //nolint

	doc := idx.Document()

	// Index 文档
	indexRes, err := doc.Index(ctx, "1", map[string]any{
		"name": "Alice",
		"age":  30,
	})
	if err != nil {
		t.Fatalf("索引文档失败: %v", err)
	}
	if indexRes.ID != "1" {
		t.Errorf("期望 ID=1，得到 %s", indexRes.ID)
	}

	// Get 文档
	getRes, err := doc.Get(ctx, "1")
	if err != nil {
		t.Fatalf("获取文档失败: %v", err)
	}
	if !getRes.Found {
		t.Fatal("文档应该存在")
	}

	var source map[string]any
	if err := json.Unmarshal(getRes.Source, &source); err != nil {
		t.Fatalf("解析 source 失败: %v", err)
	}
	if source["name"] != "Alice" {
		t.Errorf("期望 name=Alice，得到 %v", source["name"])
	}

	// Exists
	exists, err := doc.Exists(ctx, "1")
	if err != nil {
		t.Fatalf("检查文档存在失败: %v", err)
	}
	if !exists {
		t.Fatal("文档应该存在")
	}

	// Update 文档
	updateRes, err := doc.Update(ctx, "1", map[string]any{"age": 31})
	if err != nil {
		t.Fatalf("更新文档失败: %v", err)
	}
	if updateRes.Result != "updated" {
		t.Errorf("期望 result=updated，得到 %s", updateRes.Result)
	}

	// Delete 文档
	deleteRes, err := doc.Delete(ctx, "1")
	if err != nil {
		t.Fatalf("删除文档失败: %v", err)
	}
	if deleteRes.Result != "deleted" {
		t.Errorf("期望 result=deleted，得到 %s", deleteRes.Result)
	}

	// 确认已删除
	_, err = doc.Get(ctx, "1")
	if err != elasticsearch.ErrDocumentNotFound {
		t.Errorf("期望 ErrDocumentNotFound，得到 %v", err)
	}
}

func TestBulk(t *testing.T) {
	skipIfNoES(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	indexName := "servex_test_bulk"
	idx := client.Index(indexName)

	_ = idx.Delete(ctx)
	err := idx.Create(ctx, map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
	})
	if err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}
	defer idx.Delete(ctx) //nolint

	doc := idx.Document()

	actions := []elasticsearch.BulkAction{
		{Type: "index", ID: "1", Body: map[string]any{"name": "Alice", "age": 30}},
		{Type: "index", ID: "2", Body: map[string]any{"name": "Bob", "age": 25}},
		{Type: "index", ID: "3", Body: map[string]any{"name": "Charlie", "age": 35}},
	}

	result, err := doc.Bulk(ctx, actions)
	if err != nil {
		t.Fatalf("批量操作失败: %v", err)
	}
	if result.Errors {
		t.Errorf("批量操作有错误: %+v", result.Items)
	}
	if len(result.Items) != 3 {
		t.Errorf("期望 3 个结果，得到 %d", len(result.Items))
	}
}

func TestSearchQuery(t *testing.T) {
	skipIfNoES(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	indexName := "servex_test_search"
	idx := client.Index(indexName)

	_ = idx.Delete(ctx)
	err := idx.Create(ctx, map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
	})
	if err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}
	defer idx.Delete(ctx) //nolint

	doc := idx.Document()

	// 写入测试数据
	actions := []elasticsearch.BulkAction{
		{Type: "index", ID: "1", Body: map[string]any{"name": "Alice", "age": 30}},
		{Type: "index", ID: "2", Body: map[string]any{"name": "Bob", "age": 25}},
		{Type: "index", ID: "3", Body: map[string]any{"name": "Charlie", "age": 35}},
	}
	if _, err := doc.Bulk(ctx, actions); err != nil {
		t.Fatalf("批量写入失败: %v", err)
	}

	// 等待索引刷新
	_, _ = client.Client().Indices.Refresh(
		client.Client().Indices.Refresh.WithIndex(indexName),
	)

	search := idx.Search()

	// 测试 match_all 查询
	result, err := search.Query(ctx, map[string]any{
		"match_all": map[string]any{},
	}, elasticsearch.WithSize(10))
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if result.TotalHits != 3 {
		t.Errorf("期望 TotalHits=3，得到 %d", result.TotalHits)
	}
	if len(result.Hits) != 3 {
		t.Errorf("期望 3 个命中，得到 %d", len(result.Hits))
	}

	// 测试 Count
	count, err := search.Count(ctx, map[string]any{
		"match_all": map[string]any{},
	})
	if err != nil {
		t.Fatalf("Count 失败: %v", err)
	}
	if count != 3 {
		t.Errorf("期望 count=3，得到 %d", count)
	}

	// 测试范围查询
	result, err = search.Query(ctx, map[string]any{
		"range": map[string]any{
			"age": map[string]any{"gte": 30},
		},
	})
	if err != nil {
		t.Fatalf("范围搜索失败: %v", err)
	}
	if result.TotalHits != 2 {
		t.Errorf("期望 TotalHits=2，得到 %d", result.TotalHits)
	}
}

func TestAggregate(t *testing.T) {
	skipIfNoES(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	indexName := "servex_test_agg"
	idx := client.Index(indexName)

	_ = idx.Delete(ctx)
	err := idx.Create(ctx, map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
	})
	if err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}
	defer idx.Delete(ctx) //nolint

	doc := idx.Document()

	actions := []elasticsearch.BulkAction{
		{Type: "index", ID: "1", Body: map[string]any{"name": "Alice", "age": 30}},
		{Type: "index", ID: "2", Body: map[string]any{"name": "Bob", "age": 25}},
		{Type: "index", ID: "3", Body: map[string]any{"name": "Charlie", "age": 35}},
	}
	if _, err := doc.Bulk(ctx, actions); err != nil {
		t.Fatalf("批量写入失败: %v", err)
	}

	// 等待索引刷新
	_, _ = client.Client().Indices.Refresh(
		client.Client().Indices.Refresh.WithIndex(indexName),
	)

	search := idx.Search()

	// 聚合查询
	result, err := search.Aggregate(ctx, map[string]any{
		"avg_age": map[string]any{
			"avg": map[string]any{"field": "age"},
		},
	})
	if err != nil {
		t.Fatalf("聚合查询失败: %v", err)
	}

	if result.Aggregations == nil {
		t.Fatal("聚合结果不应为空")
	}
	if _, ok := result.Aggregations["avg_age"]; !ok {
		t.Error("应包含 avg_age 聚合结果")
	}
}
