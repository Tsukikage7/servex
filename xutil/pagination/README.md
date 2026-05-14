# pagination

`github.com/Tsukikage7/servex/v2/xutil/pagination` -- 分页工具。

## 概述

pagination 包提供通用分页参数处理与分页结果封装，支持 Page/Offset 分页和 Cursor/Keyset 分页。GORM 游标分页适配放在 `xutil/pagination/gorm` 子包中，避免基础分页模型引入 GORM。

## 功能特性

- 自动规范化分页参数（页码最小为 1，每页数量限制在 1-100 之间）
- 提供 Offset/Limit 方法，方便数据库查询
- 泛型分页结果 `Result[T]`，支持任意数据类型
- 计算总页数、判断是否有上/下一页
- 游标分页请求/响应模型，适合无限滚动、大数据量列表和实时数据流
- 游标编码/解码使用 base64url + JSON，支持组合游标字段

## API

### Page/Offset 类型

| 类型 | 说明 |
|------|------|
| `Pagination` | 分页参数，包含 Page（int32）和 PageSize（int32） |
| `Result[T]` | 分页结果，包含 Items、Total、Page、PageSize |

### Page/Offset 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `DefaultPage` | `1` | 默认页码 |
| `DefaultPageSize` | `20` | 默认每页数量 |
| `MaxPageSize` | `100` | 最大每页数量 |

### Page/Offset 函数

| 函数 | 说明 |
|------|------|
| `New(page, pageSize int32) Pagination` | 创建分页参数，自动规范化 |
| `NewResult[T](items, total, pagination) Result[T]` | 创建分页结果 |

### Pagination 方法

| 方法 | 说明 |
|------|------|
| `Offset() int` | 计算偏移量 `(Page - 1) * PageSize` |
| `Limit() int` | 返回每页数量 |

### Result[T] 方法

| 方法 | 说明 |
|------|------|
| `TotalPages() int32` | 计算总页数 |
| `HasNext() bool` | 是否有下一页 |
| `HasPrev() bool` | 是否有上一页 |

### Cursor/Keyset 类型

| 类型 | 说明 |
|------|------|
| `CursorRequest` | 游标分页请求，包含 Cursor、Limit、Direction |
| `CursorResponse[T]` | 游标分页响应，包含 Items、NextCursor、PrevCursor、HasMore |
| `Direction` | 游标方向，取值为 `Forward` 或 `Backward` |

### Cursor/Keyset 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `DefaultCursorLimit` | `20` | 默认游标分页数量 |
| `MaxCursorLimit` | `100` | 最大游标分页数量 |
| `Forward` | `"forward"` | 向前分页 |
| `Backward` | `"backward"` | 向后分页 |

### Cursor/Keyset 函数

| 函数 | 说明 |
|------|------|
| `EncodeCursor(values ...any) string` | 将字段值编码为游标 |
| `DecodeCursor(cursor string) ([]any, error)` | 解码游标 |

## 示例

### Page/Offset

```go
p := pagination.New(2, 10)
offset := p.Offset()
limit := p.Limit()

result := pagination.NewResult(users, 35, p)
_ = result.TotalPages()
_ = result.HasNext()
```

### Cursor/Keyset

```go
req := (&pagination.CursorRequest{
    Cursor: r.URL.Query().Get("cursor"),
    Limit:  20,
}).Apply()

var nextCursor string
if len(items) > 0 {
    nextCursor = pagination.EncodeCursor(items[len(items)-1].ID)
}

resp := pagination.CursorResponse[User]{
    Items:      items,
    NextCursor: nextCursor,
    HasMore:    hasMore,
}
```

### GORM Cursor Adapter

```go
import paginationgorm "github.com/Tsukikage7/servex/v2/xutil/pagination/gorm"

paginationgorm.Paginate(db.Model(&User{}), req, "id").Find(&users)
```
