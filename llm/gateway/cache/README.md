# llm/gateway/cache

`github.com/Tsukikage7/servex/v2/llm/gateway/cache` — 基于语义相似度的 AI 响应缓存，通过向量余弦相似度比对命中缓存，避免重复调用语言模型。

## 核心类型

- `Config` — 缓存配置，包含 EmbeddingModel、Store、Threshold（相似度阈值，默认 0.95）、TTL（默认 1h）
- `Store` — 缓存存储接口，方法包括 Put、Search、Clear
- `MemoryStore` — 基于内存切片的缓存实现，线性扫描余弦相似度，自动清理过期条目
- `Middleware(cfg)` — 校验配置并返回语义缓存中间件，对 Generate 和 Stream 均生效
- `NewCachedModel(model, cfg)` — 校验配置并将缓存中间件应用到 model，返回带缓存能力的 ChatModel

## 使用示例

```go
import "github.com/Tsukikage7/servex/v2/llm/gateway/cache"

store := cache.NewMemoryStore()

cfg := &cache.Config{
    EmbeddingModel: embModel,
    Store:          store,
    Threshold:      0.95,
    TTL:            time.Hour,
}

// 方式一：通过中间件
cachedMw, err := cache.Middleware(cfg)
if err != nil {
    return err
}
cachedModel := cachedMw(myModel)

// 方式二：快捷函数
cachedModel, err = cache.NewCachedModel(myModel, cfg)
if err != nil {
    return err
}

// 语义相似的问题会命中缓存
resp1, _ := cachedModel.Generate(ctx, messages1)  // 调用模型
resp2, _ := cachedModel.Generate(ctx, messages2)  // 命中缓存（若相似度 >= 0.95）
_ = resp1
_ = resp2

// 清空缓存
_ = store.Clear(ctx)
```
