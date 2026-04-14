package mongodb

import (
	"context"
	"net/url"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// mongoClient MongoDB 客户端实现.
type mongoClient struct {
	client   *mongo.Client
	database string
	log      logger.Logger
}

// newMongoClient 创建 MongoDB 客户端.
func newMongoClient(config *Config, log logger.Logger) (*mongoClient, error) {
	// 构建客户端选项
	opts := options.Client().ApplyURI(config.URI)

	// 连接超时
	opts.SetConnectTimeout(config.ConnectTimeout)
	opts.SetServerSelectionTimeout(config.ServerSelectionTimeout)
	// SocketTimeout 通过 Timeout 选项应用于操作级别超时
	if config.SocketTimeout > 0 {
		opts.SetTimeout(config.SocketTimeout)
	}

	// 连接池配置
	opts.SetMaxPoolSize(config.MaxPoolSize)
	opts.SetMinPoolSize(config.MinPoolSize)
	opts.SetMaxConnIdleTime(config.MaxConnIdleTime)

	// 副本集
	if config.ReplicaSet != "" {
		opts.SetReplicaSet(config.ReplicaSet)
	}

	// 直连模式
	if config.Direct {
		opts.SetDirect(true)
	}

	// 启用链路追踪
	if config.EnableTracing {
		opts.SetMonitor(otelmongo.NewMonitor())
	}

	// 创建客户端
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	log.Info("mongodb 连接成功", "uri", maskURI(config.URI), "database", config.Database)

	return &mongoClient{
		client:   client,
		database: config.Database,
		log:      log,
	}, nil
}

// maskURI 遮盖 URI 中的密码信息.
func maskURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return "***"
	}
	if u.User != nil {
		if _, hasPass := u.User.Password(); hasPass {
			u.User = url.UserPassword(u.User.Username(), "***")
		}
	}
	return u.String()
}

func (c *mongoClient) Database(name ...string) Database {
	dbName := c.database
	if len(name) > 0 && name[0] != "" {
		dbName = name[0]
	}
	return &mongoDatabase{
		database: c.client.Database(dbName),
	}
}

func (c *mongoClient) Collection(name string) Collection {
	return &mongoCollection{
		collection: c.client.Database(c.database).Collection(name),
	}
}

func (c *mongoClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, nil)
}

func (c *mongoClient) Close(ctx context.Context) error {
	c.log.Info("mongodb 连接关闭")
	return c.client.Disconnect(ctx)
}

func (c *mongoClient) Client() *mongo.Client {
	return c.client
}

func (c *mongoClient) StartSession(opts ...options.Lister[options.SessionOptions]) (*mongo.Session, error) {
	return c.client.StartSession(opts...)
}

func (c *mongoClient) UseSession(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.client.UseSession(ctx, func(sc context.Context) error {
		return fn(sc)
	})
}

func (c *mongoClient) UseTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := c.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		return nil, fn(sc)
	})
	return err
}

// mongoDatabase MongoDB 数据库实现.
type mongoDatabase struct {
	database *mongo.Database
}

func (d *mongoDatabase) Collection(name string) Collection {
	return &mongoCollection{
		collection: d.database.Collection(name),
	}
}

func (d *mongoDatabase) Name() string {
	return d.database.Name()
}

func (d *mongoDatabase) Drop(ctx context.Context) error {
	return d.database.Drop(ctx)
}

func (d *mongoDatabase) ListCollectionNames(ctx context.Context, filter any) ([]string, error) {
	return d.database.ListCollectionNames(ctx, filter)
}

func (d *mongoDatabase) Database() *mongo.Database {
	return d.database
}

// mongoCollection MongoDB 集合实现.
type mongoCollection struct {
	collection *mongo.Collection
}

func (c *mongoCollection) Name() string {
	return c.collection.Name()
}

func (c *mongoCollection) InsertOne(ctx context.Context, document any) (*InsertOneResult, error) {
	result, err := c.collection.InsertOne(ctx, document)
	if err != nil {
		return nil, err
	}
	return &InsertOneResult{InsertedID: result.InsertedID}, nil
}

func (c *mongoCollection) InsertMany(ctx context.Context, documents []any) (*InsertManyResult, error) {
	result, err := c.collection.InsertMany(ctx, documents)
	if err != nil {
		return nil, err
	}
	return &InsertManyResult{InsertedIDs: result.InsertedIDs}, nil
}

func (c *mongoCollection) FindOne(ctx context.Context, filter any, opts ...FindOneOption) SingleResult {
	o := &findOneOptions{}
	for _, opt := range opts {
		opt(o)
	}

	findOpts := options.FindOne()
	if o.projection != nil {
		findOpts.SetProjection(o.projection)
	}
	if o.sort != nil {
		findOpts.SetSort(o.sort)
	}
	if o.skip > 0 {
		findOpts.SetSkip(o.skip)
	}

	return c.collection.FindOne(ctx, filter, findOpts)
}

func (c *mongoCollection) Find(ctx context.Context, filter any, opts ...FindOption) (Cursor, error) {
	o := &findOptions{}
	for _, opt := range opts {
		opt(o)
	}

	findOpts := options.Find()
	if o.projection != nil {
		findOpts.SetProjection(o.projection)
	}
	if o.sort != nil {
		findOpts.SetSort(o.sort)
	}
	if o.skip > 0 {
		findOpts.SetSkip(o.skip)
	}
	if o.limit > 0 {
		findOpts.SetLimit(o.limit)
	}

	return c.collection.Find(ctx, filter, findOpts)
}

func (c *mongoCollection) CountDocuments(ctx context.Context, filter any) (int64, error) {
	return c.collection.CountDocuments(ctx, filter)
}

func (c *mongoCollection) EstimatedDocumentCount(ctx context.Context) (int64, error) {
	return c.collection.EstimatedDocumentCount(ctx)
}

func (c *mongoCollection) Distinct(ctx context.Context, fieldName string, filter any) ([]any, error) {
	result := c.collection.Distinct(ctx, fieldName, filter)
	if result.Err() != nil {
		return nil, result.Err()
	}
	var values []any
	if err := result.Decode(&values); err != nil {
		return nil, err
	}
	return values, nil
}

func (c *mongoCollection) UpdateOne(ctx context.Context, filter, update any) (*UpdateResult, error) {
	result, err := c.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return &UpdateResult{
		MatchedCount:  result.MatchedCount,
		ModifiedCount: result.ModifiedCount,
		UpsertedCount: result.UpsertedCount,
		UpsertedID:    result.UpsertedID,
	}, nil
}

func (c *mongoCollection) UpdateMany(ctx context.Context, filter, update any) (*UpdateResult, error) {
	result, err := c.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return &UpdateResult{
		MatchedCount:  result.MatchedCount,
		ModifiedCount: result.ModifiedCount,
		UpsertedCount: result.UpsertedCount,
		UpsertedID:    result.UpsertedID,
	}, nil
}

func (c *mongoCollection) ReplaceOne(ctx context.Context, filter, replacement any) (*UpdateResult, error) {
	result, err := c.collection.ReplaceOne(ctx, filter, replacement)
	if err != nil {
		return nil, err
	}
	return &UpdateResult{
		MatchedCount:  result.MatchedCount,
		ModifiedCount: result.ModifiedCount,
		UpsertedCount: result.UpsertedCount,
		UpsertedID:    result.UpsertedID,
	}, nil
}

func (c *mongoCollection) DeleteOne(ctx context.Context, filter any) (*DeleteResult, error) {
	result, err := c.collection.DeleteOne(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &DeleteResult{DeletedCount: result.DeletedCount}, nil
}

func (c *mongoCollection) DeleteMany(ctx context.Context, filter any) (*DeleteResult, error) {
	result, err := c.collection.DeleteMany(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &DeleteResult{DeletedCount: result.DeletedCount}, nil
}

func (c *mongoCollection) FindOneAndUpdate(ctx context.Context, filter, update any) SingleResult {
	return c.collection.FindOneAndUpdate(ctx, filter, update)
}

func (c *mongoCollection) FindOneAndReplace(ctx context.Context, filter, replacement any) SingleResult {
	return c.collection.FindOneAndReplace(ctx, filter, replacement)
}

func (c *mongoCollection) FindOneAndDelete(ctx context.Context, filter any) SingleResult {
	return c.collection.FindOneAndDelete(ctx, filter)
}

func (c *mongoCollection) Aggregate(ctx context.Context, pipeline any) (Cursor, error) {
	return c.collection.Aggregate(ctx, pipeline)
}

func (c *mongoCollection) CreateIndex(ctx context.Context, model IndexModel) (string, error) {
	indexModel := mongo.IndexModel{
		Keys: model.Keys,
	}

	if model.Options != nil {
		opts := options.Index()
		if model.Options.Name != "" {
			opts.SetName(model.Options.Name)
		}
		if model.Options.Unique {
			opts.SetUnique(true)
		}
		if model.Options.Sparse {
			opts.SetSparse(true)
		}
		if model.Options.ExpireAfterSeconds > 0 {
			opts.SetExpireAfterSeconds(model.Options.ExpireAfterSeconds)
		}
		indexModel.Options = opts
	}

	return c.collection.Indexes().CreateOne(ctx, indexModel)
}

func (c *mongoCollection) CreateIndexes(ctx context.Context, models []IndexModel) ([]string, error) {
	indexModels := make([]mongo.IndexModel, len(models))
	for i, model := range models {
		indexModels[i] = mongo.IndexModel{
			Keys: model.Keys,
		}
		if model.Options != nil {
			opts := options.Index()
			if model.Options.Name != "" {
				opts.SetName(model.Options.Name)
			}
			if model.Options.Unique {
				opts.SetUnique(true)
			}
			if model.Options.Sparse {
				opts.SetSparse(true)
			}
			if model.Options.ExpireAfterSeconds > 0 {
				opts.SetExpireAfterSeconds(model.Options.ExpireAfterSeconds)
			}
			indexModels[i].Options = opts
		}
	}

	return c.collection.Indexes().CreateMany(ctx, indexModels)
}

func (c *mongoCollection) DropIndex(ctx context.Context, name string) error {
	return c.collection.Indexes().DropOne(ctx, name)
}

func (c *mongoCollection) DropAllIndexes(ctx context.Context) error {
	return c.collection.Indexes().DropAll(ctx)
}

func (c *mongoCollection) ListIndexes(ctx context.Context) (Cursor, error) {
	return c.collection.Indexes().List(ctx)
}

func (c *mongoCollection) Drop(ctx context.Context) error {
	return c.collection.Drop(ctx)
}

func (c *mongoCollection) Collection() *mongo.Collection {
	return c.collection
}
