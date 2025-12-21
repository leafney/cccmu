# SSE 客户端连接优化功能

## 功能概述

为了减少服务器资源消耗和 API 请求压力，CCCMU 实现了智能的 SSE 客户端连接优化功能。当系统检测到没有活跃的 SSE 客户端连接时，定时监控任务将跳过实际的 API 调用，从而节省资源。

## 工作原理

### 核心机制
- **连接计数器**: 实时统计活跃的 SSE 客户端连接数量
- **智能跳过**: 无客户端连接时，跳过 `fetchAndSaveData()` 和 `fetchAndSaveBalance()` 的 API 调用
- **任务状态维持**: 定时任务保持运行状态，仅跳过具体的网络请求操作
- **即时恢复**: 客户端重新连接时，立即恢复正常的数据获取

### 监听器类型统计
系统会统计以下类型的监听器连接：
- 数据监听器 (DataListener)
- 积分余额监听器 (BalanceListener) 
- 错误监听器 (ErrorListener)
- 重置状态监听器 (ResetStatusListener)
- 自动调度监听器 (AutoScheduleListener)
- 每日使用监听器 (DailyUsageListener)

## 配置选项

### SkipWhenNoConnections
- **类型**: `boolean`
- **默认值**: `true`
- **描述**: 控制是否在无客户端连接时跳过 API 请求
- **API字段**: `skipWhenNoConnections`

```json
{
  "skipWhenNoConnections": true
}
```

## 日志记录

### 连接管理日志
```
[连接管理] ➕ 添加数据监听器，当前活跃连接数: 1
[连接管理] ➖ 移除数据监听器，当前活跃连接数: 0
```

### 任务优化日志
```
[任务优化] 🚫 无活跃连接，跳过使用数据获取任务 (已跳过: 15次)
[任务优化] 🚫 无活跃连接，跳过积分余额获取任务 (已跳过: 16次)
```

### 任务恢复日志
```
[任务恢复] 🔄 检测到活跃连接，恢复使用数据获取任务 (当前连接数: 1)
[任务恢复] 🔄 检测到活跃连接，恢复积分余额获取任务 (当前连接数: 1)
```

### 任务执行日志
```
[任务执行] ✅ 成功获取使用数据，当前连接数: 2
[任务执行] ✅ 成功获取积分余额，当前连接数: 2
```

## 统计信息

系统提供优化统计信息接口：

```go
scheduler.GetOptimizationStats()
```

返回数据格式：
```json
{
  "activeConnections": 0,
  "skippedTaskCount": 25,
  "skipWhenNoConnections": true,
  "optimizationEnabled": true
}
```

### 字段说明
- `activeConnections`: 当前活跃连接数
- `skippedTaskCount`: 累计跳过的任务次数
- `skipWhenNoConnections`: 优化功能是否启用
- `optimizationEnabled`: 当前是否正在优化（无连接且功能启用）

## 使用场景

### 适用场景
1. **长时间无人监控**: 夜间或非工作时间无人查看监控面板
2. **资源节约**: 减少不必要的 API 请求，降低服务器负载
3. **网络优化**: 减少外部 API 调用，节省网络带宽

### 不适用场景
1. **数据完整性要求**: 需要持续收集历史数据用于分析
2. **第三方集成**: 其他系统依赖持续的数据更新
3. **监管合规**: 需要完整的操作记录和数据链

## 配置建议

### 推荐配置
```json
{
  "skipWhenNoConnections": true,
  "interval": 60
}
```

### 特殊需求配置
如需要持续数据收集（如用于数据分析或合规要求）：
```json
{
  "skipWhenNoConnections": false,
  "interval": 60
}
```

## 技术实现

### 核心组件
- `SchedulerService.activeConnections`: 连接计数器
- `SchedulerService.shouldSkipTaskWhenNoConnections()`: 跳过检查逻辑
- `UserConfig.SkipWhenNoConnections`: 配置开关

### 线程安全
- 使用 `sync.RWMutex` 保护连接计数器
- 监听器添加/移除操作的原子性
- 配置更新的线程安全处理

### 性能影响
- **CPU**: 几乎无额外开销，仅增加简单的计数操作
- **内存**: 增加两个整型字段（`activeConnections`, `skippedTaskCount`）
- **网络**: 显著减少无效的 API 请求

## 监控与维护

### 运行状态检查
通过日志观察优化功能运行状态：
```bash
# 查看连接管理日志
grep "连接管理" logs/app.log

# 查看任务优化日志  
grep "任务优化" logs/app.log

# 查看跳过统计
grep "已跳过" logs/app.log
```

### 性能评估
- 监控 `skippedTaskCount` 了解优化效果
- 观察服务器资源使用情况变化
- 记录 API 请求频率的降低程度

## 总结

SSE 客户端连接优化功能通过智能检测客户端连接状态，在无连接时跳过不必要的 API 调用，有效降低了系统资源消耗和服务器请求压力。该功能设计简洁、性能优异，并提供完善的日志记录和统计信息，是一个实用的系统优化特性。