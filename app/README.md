# app

`github.com/Tsukikage7/servex/v2/app`

应用生命周期钩子管理，提供启动和停止阶段的前置/后置钩子注册与执行能力。

## 功能特性

- 四阶段生命周期钩子：启动前、启动后、停止前、停止后
- 链式构建器模式，支持流式注册多个钩子
- 钩子按注册顺序依次执行，遇错即停

## API

### 类型定义

```go
// Hook 生命周期钩子函数
type Hook func(ctx context.Context) error
```

### Hooks 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `BeforeStart` | `[]Hook` | 启动前钩子列表 |
| `AfterStart` | `[]Hook` | 启动后钩子列表 |
| `BeforeStop` | `[]Hook` | 停止前钩子列表 |
| `AfterStop` | `[]Hook` | 停止后钩子列表 |

`Hooks` 提供内部执行方法 `runBeforeStart`、`runAfterStart`、`runBeforeStop`、`runAfterStop`，按顺序执行对应阶段的钩子，任一钩子返回错误则立即终止后续执行。

### HooksBuilder

通过 `NewHooks()` 创建构建器，支持链式调用：

| 方法 | 说明 |
|------|------|
| `BeforeStart(hook Hook) *HooksBuilder` | 添加启动前钩子 |
| `AfterStart(hook Hook) *HooksBuilder` | 添加启动后钩子 |
| `BeforeStop(hook Hook) *HooksBuilder` | 添加停止前钩子 |
| `AfterStop(hook Hook) *HooksBuilder` | 添加停止后钩子 |
| `Build() *Hooks` | 构建并返回 Hooks 实例 |
