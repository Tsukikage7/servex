# sorting

`github.com/Tsukikage7/servex/v2/xutil/sorting` -- 排序工具。

## 概述

sorting 包提供排序参数解析与处理功能，支持从字符串解析多字段排序条件，提供白名单过滤机制防止注入，并可直接应用于 GORM 查询。

## 功能特性

- 从字符串解析排序参数，支持多字段排序
- 支持升序（asc）和降序（desc），默认降序
- 白名单字段过滤，防止 SQL 注入
- 默认排序回退机制
- GORM 集成：Scope 函数与链式调用

## API

### 类型

| 类型 | 说明 |
|------|------|
| `Sort` | 单个排序条件，包含 Field 和 Order |
| `Sorting` | 排序参数集合，包含 Sorts 切片 |
| `Order` | 排序方向枚举 |

### 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `Asc` | `"asc"` | 升序 |
| `Desc` | `"desc"` | 降序 |
| `DefaultOrder` | `Desc` | 默认排序方向 |

### 函数

| 函数 | 说明 |
|------|------|
| `New(sort string) Sorting` | 解析排序字符串，支持 `"field:order,field:order"` 格式 |

### Sorting 方法

| 方法 | 说明 |
|------|------|
| `IsEmpty() bool` | 是否为空 |
| `First() Sort` | 返回第一个排序条件 |
| `String() string` | 返回完整排序字符串，如 `"created_time desc, id asc"` |
| `Filter(allowedFields ...string) Sorting` | 白名单过滤，只保留允许的字段 |
| `WithDefault(defaultSort string) Sorting` | 为空时使用默认排序 |
| `GORMScope() func(*gorm.DB) *gorm.DB` | 返回 GORM Scope 函数 |
| `Apply(db *gorm.DB) *gorm.DB` | 直接应用到 GORM 查询 |
