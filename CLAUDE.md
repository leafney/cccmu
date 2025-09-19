# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

CCCMU (ACM Claude积分监控系统) 是一个用于实时监控和可视化 ACM Claude 积分使用量的 Web 应用程序，采用 Go + React 架构，支持 SSE 实时数据推送和单文件部署。

## 开发环境

### 必要依赖
- **Go**: >= 1.23
- **Bun**: >= 1.0 (包管理器和运行时)
- **Node.js**: >= 18 (可选，Bun可替代)

### 常用命令

#### 开发环境
```bash
# 安装所有依赖
make install

# 启动完整开发环境 (前后端并行)
make dev

# 单独启动前端开发服务器
make dev-frontend

# 单独启动后端开发服务器  
make dev-backend
```

#### 构建和运行
```bash
# 完整构建 (前端构建 + 后端编译 + 静态文件嵌入)
make build

# 运行应用 (默认端口8080)
make run

# 运行应用并开启调试日志
make run-debug

# 使用自定义端口运行
make run-port

# 直接运行可执行文件
./bin/cccmu -p 8080 -l
```

#### 代码检查
```bash
# 运行测试
make test

# 格式化代码
make fmt

# 代码检查
make lint

# 清理构建文件
make clean
```

## 架构设计

### 后端架构 (Go + Fiber)

**核心组件结构**:
- `main.go`: 应用启动入口，服务器配置，路由设置
- `database/`: BadgerDB 嵌入式数据库操作
- `handlers/`: HTTP 请求处理器 (config, control, sse)
- `services/`: 核心业务逻辑
  - `scheduler.go`: 数据抓取调度服务
  - `auto_scheduler.go`: 自动调度器
  - `auto_reset.go`: 自动重置功能  
  - `async_config_updater.go`: 异步配置更新
- `client/`: 外部API客户端和缓存
- `models/`: 数据模型定义 (config, usage, response)
- `web/`: 静态文件嵌入 (embed.go)

**关键技术特性**:
- 使用 `go:embed` 将前端构建产物嵌入到二进制文件中
- 基于 Gocron 的定时任务调度系统
- SSE (Server-Sent Events) 实时数据推送
- BadgerDB 本地存储配置和积分数据
- 优雅关闭和信号处理

### 前端架构 (React 19 + TypeScript)

**技术栈**:
- **构建工具**: Vite + Bun
- **框架**: React 19 + TypeScript 
- **样式**: TailwindCSS 4
- **图表**: ECharts (echarts-for-react)
- **图标**: Lucide React
- **通知**: react-hot-toast

**组件结构**:
- `Dashboard.tsx`: 主页面组件
- `SettingsModal.tsx`: 设置弹窗 
- `SettingsPanel.tsx`: 设置面板
- `StatusIndicator.tsx`: 状态指示器
- `UsageChart.tsx`: 积分使用趋势图表
- `api/client.ts`: API 客户端封装

### 部署模式

1. **单文件部署**: 前端静态文件通过 `go:embed` 嵌入到 Go 二进制文件中
2. **跨平台支持**: 支持 Windows、macOS、Linux (amd64/arm64)
3. **Docker 部署**: 提供 multi-arch Docker 镜像
4. **数据持久化**: BadgerDB 数据存储在 `.b/` 目录

## 特殊注意事项

### 构建流程
1. 前端构建: `cd web && bun run build` 生成 `web/dist/`
2. 文件复制: 将 `web/dist/` 复制到 `server/web/dist/`  
3. 后端编译: Go 编译时将 `server/web/dist/` 嵌入二进制文件
4. 最终产物: 包含前后端的单一可执行文件 `bin/cccmu`

### 开发模式差异
- **开发环境**: 前后端独立运行，前端通过代理访问后端 API
- **生产环境**: 后端直接服务前端静态文件，所有请求统一处理

### 数据流架构
- **外部 API**: ACM Claude Dashboard API
- **数据收集**: 定时任务 -> API 客户端 -> BadgerDB 存储
- **实时推送**: SSE 连接 -> 数据变化监听 -> 客户端更新
- **状态管理**: React 组件状态 + SSE 事件驱动更新

### Cookie 验证机制
- 通过积分数据获取接口隐式验证 Cookie 有效性
- API 返回 401 时自动检测 Cookie 失效
- 前端提示用户更新无效 Cookie

### 端口配置优先级
1. 命令行参数 `-p` 或 `--port`
2. 环境变量 `PORT`
3. 默认端口 `:8080`

## 开发备忘录

### 命令相关

- 通过 make 命令编译前后端