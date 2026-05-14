# crypto

`github.com/Tsukikage7/servex/v2/xutil/crypto` -- 加密与随机生成工具。

## 概述

crypto 包提供常用的随机数生成与密码哈希功能，包括唯一 ID 生成、验证码生成、范围随机数生成以及 bcrypt 密码哈希。

## 功能特性

- 基于 `crypto/rand` 的安全随机 ID 生成
- 6 位数字验证码生成
- 指定范围的随机 int32/int64 生成
- 固定长度的业务 ID 生成（9 位 / 18 位）
- bcrypt 密码哈希与验证

## API

### 函数

| 函数 | 说明 |
|------|------|
| `GenerateID() (string, error)` | 生成 32 位十六进制随机 ID（16 字节） |
| `GenerateVerificationCode() string` | 生成 6 位数字验证码 |
| `GenerateRandomInt32(min, max int32) (int32, error)` | 生成 [min, max] 范围内的随机 int32 |
| `GenerateRandomInt64(min, max int64) (int64, error)` | 生成 [min, max] 范围内的随机 int64 |
| `GenerateBusinessID() int32` | 生成 9 位随机数字 ID（100000000-999999999） |
| `GenerateBusinessID64() int64` | 生成 18 位随机数字 ID |
| `HashPassword(password string) (string, error)` | 使用 bcrypt 哈希密码 |
| `VerifyPassword(hashedPassword, password string) error` | 验证密码是否匹配哈希值 |
