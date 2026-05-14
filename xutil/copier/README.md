# copier

`github.com/Tsukikage7/servex/v2/xutil/copier` -- 结构体复制。

## 概述

copier 包提供基于反射的结构体对象复制功能，支持泛型 API，可按字段名自动匹配复制，支持忽略字段与字段映射。适用于 DTO 转换、模型映射等场景。

## 功能特性

- 泛型 API：`Copy[Dst, Src]` 创建新对象，`CopyTo[Dst, Src]` 复制到已有对象
- 按导出字段名自动匹配复制
- 支持忽略指定字段
- 支持字段名映射（源字段名 -> 目标字段名）
- 支持类型自动转换（AssignableTo、ConvertibleTo）
- 支持嵌套结构体与指针类型的递归复制

## API

### 函数

| 函数 | 说明 |
|------|------|
| `Copy[Dst, Src](src *Src) (*Dst, error)` | 从 src 创建 Dst 类型的新对象 |
| `CopyTo[Dst, Src](src *Src, dst *Dst) error` | 将 src 复制到已有的 dst |
| `CopyWithOptions[Dst, Src](src *Src, opts...) (*Dst, error)` | 带选项创建副本 |
| `CopyToWithOptions[Dst, Src](src *Src, dst *Dst, opts...) error` | 带选项复制到目标 |

### 配置选项 (CopyOption)

| 选项 | 说明 |
|------|------|
| `IgnoreFields(fields ...string)` | 忽略指定字段，不进行复制 |
| `FieldMapping(srcField, dstField string)` | 字段名映射，将源字段复制到不同名的目标字段 |

### 预定义错误

| 错误 | 说明 |
|------|------|
| `ErrNilSource` | 源对象为 nil |
| `ErrNilDestination` | 目标对象为 nil |
| `ErrNotStruct` | 参数不是结构体类型 |
