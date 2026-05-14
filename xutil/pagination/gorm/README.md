# xutil/pagination/gorm

`github.com/Tsukikage7/servex/v2/xutil/pagination/gorm` -- GORM 游标分页适配。

## 概述

该子包只提供 GORM 查询辅助，基础分页模型仍在 `xutil/pagination`。需要 GORM 时按需导入本包，避免只使用分页数据结构的项目被动引入 GORM。

## 示例

```go
import (
    "github.com/Tsukikage7/servex/v2/xutil/pagination"
    paginationgorm "github.com/Tsukikage7/servex/v2/xutil/pagination/gorm"
)

req := (&pagination.CursorRequest{
    Cursor: r.URL.Query().Get("cursor"),
    Limit:  20,
}).Apply()

var users []User
if err := paginationgorm.Paginate(db.Model(&User{}), req, "id").Find(&users).Error; err != nil {
    return err
}
```
