package mongodb_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/mongodb"
	"github.com/Tsukikage7/servex/v2/testx"
)

// mongoURI 从环境变量读取，默认指向本地.
var (
	mongoURI       = "mongodb://localhost:27017"
	mongoAvailable bool
)

func TestMain(m *testing.M) {
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		mongoURI = uri
	}

	// 统一探测 MongoDB 连通性（一次即可）
	mongoAvailable = probeMongoDB()

	os.Exit(m.Run())
}

// probeMongoDB 探测 MongoDB 是否可用.
func probeMongoDB() bool {
	cfg := &mongodb.Config{
		URI:            mongoURI,
		Database:       "servex_probe",
		ConnectTimeout: 2 * time.Second,
	}
	client, err := mongodb.NewClient(cfg, testx.NopLogger())
	if err != nil {
		return false
	}
	defer client.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return client.Ping(ctx) == nil
}

func skipIfNoMongoDB(t *testing.T) {
	t.Helper()
	if !mongoAvailable {
		t.Skip("MongoDB 不可用，跳过集成测试")
	}
}

func newTestClient(t *testing.T) mongodb.Client {
	t.Helper()

	cfg := &mongodb.Config{
		URI:      mongoURI,
		Database: "servex_test",
	}

	client, err := mongodb.NewClient(cfg, testx.NopLogger())
	if err != nil {
		t.Fatalf("创建 MongoDB 客户端失败: %v", err)
	}
	return client
}

// ---- 单元测试（不需要服务）----

func TestNewClient_NilConfig(t *testing.T) {
	_, err := mongodb.NewClient(nil, testx.NopLogger())
	if err != mongodb.ErrNilConfig {
		t.Errorf("期望 ErrNilConfig，得到 %v", err)
	}
}

func TestNewClient_NilLogger(t *testing.T) {
	cfg := &mongodb.Config{URI: "mongodb://localhost", Database: "db"}
	_, err := mongodb.NewClient(cfg, nil)
	if err != mongodb.ErrNilLogger {
		t.Errorf("期望 ErrNilLogger，得到 %v", err)
	}
}

func TestNewClient_EmptyURI(t *testing.T) {
	cfg := &mongodb.Config{Database: "db"}
	_, err := mongodb.NewClient(cfg, testx.NopLogger())
	if err != mongodb.ErrEmptyURI {
		t.Errorf("期望 ErrEmptyURI，得到 %v", err)
	}
}

func TestNewClient_EmptyDatabase(t *testing.T) {
	cfg := &mongodb.Config{URI: "mongodb://localhost"}
	_, err := mongodb.NewClient(cfg, testx.NopLogger())
	if err != mongodb.ErrEmptyDatabase {
		t.Errorf("期望 ErrEmptyDatabase，得到 %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := mongodb.DefaultConfig()
	if cfg.MaxPoolSize == 0 {
		t.Error("MaxPoolSize 不应为 0")
	}
	if cfg.ConnectTimeout == 0 {
		t.Error("ConnectTimeout 不应为 0")
	}
}

// ---- 集成测试，需要 MongoDB 实例 ----

func TestPing(t *testing.T) {
	skipIfNoMongoDB(t)
	client := newTestClient(t)
	defer client.Close(t.Context())

	ctx := t.Context()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

func TestInsertOneAndFindOne(t *testing.T) {
	skipIfNoMongoDB(t)
	client := newTestClient(t)
	ctx := t.Context()
	defer client.Close(ctx)

	coll := client.Collection("test_users")
	defer coll.Drop(ctx) //nolint

	type User struct {
		Name string `bson:"name"`
		Age  int    `bson:"age"`
	}

	res, err := coll.InsertOne(ctx, mongodb.M{"name": "Alice", "age": 30})
	if err != nil {
		t.Fatalf("InsertOne 失败: %v", err)
	}
	if res.InsertedID == nil {
		t.Fatal("InsertedID 不应为 nil")
	}

	var user User
	if err = coll.FindOne(ctx, mongodb.M{"name": "Alice"}).Decode(&user); err != nil {
		t.Fatalf("FindOne 失败: %v", err)
	}
	if user.Name != "Alice" || user.Age != 30 {
		t.Errorf("期望 Alice/30，得到 %v/%v", user.Name, user.Age)
	}
}

func TestInsertManyAndCount(t *testing.T) {
	skipIfNoMongoDB(t)
	client := newTestClient(t)
	ctx := t.Context()
	defer client.Close(ctx)

	coll := client.Collection("test_many")
	defer coll.Drop(ctx) //nolint

	docs := []any{
		mongodb.M{"tag": "bulk", "n": 1},
		mongodb.M{"tag": "bulk", "n": 2},
		mongodb.M{"tag": "bulk", "n": 3},
	}
	res, err := coll.InsertMany(ctx, docs)
	if err != nil {
		t.Fatalf("InsertMany 失败: %v", err)
	}
	if len(res.InsertedIDs) != 3 {
		t.Errorf("期望 3 个 InsertedID，得到 %d", len(res.InsertedIDs))
	}

	count, err := coll.CountDocuments(ctx, mongodb.M{"tag": "bulk"})
	if err != nil {
		t.Fatalf("CountDocuments 失败: %v", err)
	}
	if count != 3 {
		t.Errorf("期望 count=3，得到 %d", count)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	skipIfNoMongoDB(t)
	client := newTestClient(t)
	ctx := t.Context()
	defer client.Close(ctx)

	coll := client.Collection("test_update")
	defer coll.Drop(ctx) //nolint

	if _, err := coll.InsertOne(ctx, mongodb.M{"x": 1, "v": "old"}); err != nil {
		t.Fatalf("InsertOne 失败: %v", err)
	}

	ur, err := coll.UpdateOne(ctx, mongodb.M{"x": 1}, mongodb.M{"$set": mongodb.M{"v": "new"}})
	if err != nil {
		t.Fatalf("UpdateOne 失败: %v", err)
	}
	if ur.ModifiedCount != 1 {
		t.Errorf("期望 ModifiedCount=1，得到 %d", ur.ModifiedCount)
	}

	dr, err := coll.DeleteOne(ctx, mongodb.M{"x": 1})
	if err != nil {
		t.Fatalf("DeleteOne 失败: %v", err)
	}
	if dr.DeletedCount != 1 {
		t.Errorf("期望 DeletedCount=1，得到 %d", dr.DeletedCount)
	}
}

func TestCreateIndex(t *testing.T) {
	skipIfNoMongoDB(t)
	client := newTestClient(t)
	ctx := t.Context()
	defer client.Close(ctx)

	coll := client.Collection("test_index")
	defer coll.Drop(ctx) //nolint

	name, err := coll.CreateIndex(ctx, mongodb.IndexModel{
		Keys: mongodb.D{{Key: "email", Value: 1}},
		Options: &mongodb.IndexOptions{
			Name:   "email_idx",
			Unique: true,
		},
	})
	if err != nil {
		t.Fatalf("CreateIndex 失败: %v", err)
	}
	if name == "" {
		t.Error("索引名不应为空")
	}
}

func TestTransaction(t *testing.T) {
	skipIfNoMongoDB(t)
	client := newTestClient(t)
	ctx := t.Context()
	defer client.Close(ctx)

	coll := client.Collection("test_tx")
	defer coll.Drop(ctx) //nolint

	err := client.UseTransaction(ctx, func(txCtx context.Context) error {
		_, err := coll.InsertOne(txCtx, mongodb.M{"tx": "ok"})
		return err
	})
	if err != nil {
		// 单节点模式不支持事务，跳过
		t.Skipf("事务不可用（可能为单节点模式）: %v", err)
	}

	count, _ := coll.CountDocuments(ctx, mongodb.M{"tx": "ok"})
	if count != 1 {
		t.Errorf("期望 count=1，得到 %d", count)
	}
}
