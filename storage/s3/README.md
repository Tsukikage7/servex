# s3

`github.com/Tsukikage7/servex/v2/storage/s3` -- S3 兼容对象存储客户端。

## 概述

s3 包基于 AWS SDK v2 提供 S3 兼容对象存储客户端封装，支持 MinIO、阿里云 OSS、腾讯云 COS 等 S3 兼容存储服务。提供对象上传/下载、分片上传、预签名 URL 等功能。

## 功能特性

- 基于 AWS SDK v2 实现，兼容所有 S3 协议存储
- 智能上传：自动根据文件大小选择普通上传或分片上传
- 支持分片上传与断点续传
- 支持预签名 URL（上传/下载）
- 支持自定义 ACL 与存储类型
- 支持路径风格与虚拟主机风格访问

## API

### Client 接口

**桶操作：**

| 方法 | 说明 |
|------|------|
| `CreateBucket(ctx, bucket) error` | 创建桶 |
| `DeleteBucket(ctx, bucket) error` | 删除桶 |
| `BucketExists(ctx, bucket) (bool, error)` | 检查桶是否存在 |
| `ListBuckets(ctx) ([]BucketInfo, error)` | 列出所有桶 |

**对象操作：**

| 方法 | 说明 |
|------|------|
| `PutObject(ctx, key, reader, size, opts...) (*PutObjectResult, error)` | 上传对象 |
| `GetObject(ctx, key) (*Object, error)` | 获取对象 |
| `DeleteObject(ctx, key) error` | 删除对象 |
| `DeleteObjects(ctx, keys) error` | 批量删除 |
| `CopyObject(ctx, srcKey, destKey) error` | 复制对象 |
| `HeadObject(ctx, key) (*ObjectInfo, error)` | 获取元信息 |
| `ListObjects(ctx, prefix, opts...) (*ListObjectsResult, error)` | 列出对象 |

**分片上传：**

| 方法 | 说明 |
|------|------|
| `CreateMultipartUpload(ctx, key, opts...) (*MultipartUpload, error)` | 创建分片上传 |
| `UploadPart(ctx, upload, partNumber, reader, size) (*UploadPartResult, error)` | 上传分片 |
| `CompleteMultipartUpload(ctx, upload, parts) (*PutObjectResult, error)` | 完成分片上传 |
| `AbortMultipartUpload(ctx, upload) error` | 取消分片上传 |

**预签名与工具：**

| 方法 | 说明 |
|------|------|
| `PresignGetObject(ctx, key, expires) (string, error)` | 生成下载预签名 URL |
| `PresignPutObject(ctx, key, expires) (string, error)` | 生成上传预签名 URL |
| `Upload(ctx, key, reader, size, opts...) (*PutObjectResult, error)` | 智能上传 |
| `Download(ctx, key, writer) (int64, error)` | 下载到 Writer |
| `UseBucket(bucket) Client` | 切换桶 |

### 上传选项 (PutOption)

| 函数 | 说明 |
|------|------|
| `WithContentType(contentType)` | 设置 Content-Type |
| `WithContentDisposition(disposition)` | 设置 Content-Disposition |
| `WithCacheControl(cacheControl)` | 设置 Cache-Control |
| `WithMetadata(metadata)` | 设置自定义元数据 |
| `WithACL(acl)` | 设置访问控制 |
| `WithStorageClass(storageClass)` | 设置存储类型 |

### ACL 常量

`ACLPrivate`、`ACLPublicRead`、`ACLPublicReadWrite`、`ACLAuthenticatedRead`

### StorageClass 常量

`StorageClassStandard`、`StorageClassStandardIA`、`StorageClassGlacier`、`StorageClassDeepArchive` 等
