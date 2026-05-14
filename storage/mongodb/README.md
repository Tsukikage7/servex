# mongodb

`github.com/Tsukikage7/servex/v2/storage/mongodb` -- MongoDB 客户端封装。

## 概述

mongodb 包基于官方 mongo-driver v2 提供 MongoDB 客户端封装，通过接口抽象简化常用 CRUD 操作，支持连接池配置、会话管理与事务操作。

## 功能特性

- 基于官方 mongo-driver v2 实现
- 接口化设计，便于测试与替换
- 支持连接池配置（最大/最小连接数、空闲超时等）
- 支持会话与事务操作
- 提供 BSON 类型别名（D、M、A、E、ObjectID）
- 支持查询选项（投影、排序、分页）
- 支持索引管理

## API

### 接口

**Client** -- 客户端接口：

| 方法 | 说明 |
|------|------|
| `Database(name ...string) Database` | 获取数据库 |
| `Collection(name string) Collection` | 获取集合（默认数据库） |
| `Ping(ctx) error` | 测试连接 |
| `Close(ctx) error` | 关闭连接 |
| `Client() *mongo.Client` | 获取原生客户端 |
| `StartSession(opts...) (*mongo.Session, error)` | 开始会话 |
| `UseSession(ctx, fn) error` | 使用会话执行操作 |
| `UseTransaction(ctx, fn) error` | 使用事务执行操作 |

**Collection** -- 集合接口：

| 方法 | 说明 |
|------|------|
| `InsertOne / InsertMany` | 插入文档 |
| `FindOne / Find` | 查询文档 |
| `CountDocuments / EstimatedDocumentCount` | 统计文档数量 |
| `UpdateOne / UpdateMany / ReplaceOne` | 更新文档 |
| `DeleteOne / DeleteMany` | 删除文档 |
| `FindOneAndUpdate / FindOneAndDelete` | 查询并修改 |
| `Aggregate` | 聚合查询 |
| `CreateIndex / CreateIndexes / DropIndex` | 索引管理 |

### 构造函数

| 函数 | 说明 |
|------|------|
| `NewClient(config, logger) (Client, error)` | 创建客户端 |
| `MustNewClient(config, logger) Client` | 创建客户端，失败 panic |
| `DefaultConfig() *Config` | 返回默认配置 |

### 类型别名

| 别名 | 原始类型 | 说明 |
|------|----------|------|
| `D` | `bson.D` | 有序文档 |
| `M` | `bson.M` | 无序文档 |
| `A` | `bson.A` | 数组 |
| `E` | `bson.E` | 文档元素 |
| `ObjectID` | `bson.ObjectID` | 对象ID |

### 查询选项

| 函数 | 说明 |
|------|------|
| `WithProjection(projection)` | FindOne 设置投影 |
| `WithSort(sort)` | FindOne 设置排序 |
| `WithFindProjection(projection)` | Find 设置投影 |
| `WithFindSort(sort)` | Find 设置排序 |
| `WithFindSkip(skip)` | Find 设置跳过数量 |
| `WithFindLimit(limit)` | Find 设置限制数量 |
