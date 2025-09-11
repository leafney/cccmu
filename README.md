# ACM Claude 积分监控系统 (CCCMU)

一个用于实时监控和可视化 ACM Claude 积分使用量的 Web 应用程序。

## 📸 项目截图

![ACM Claude 积分监控系统界面](docs/image2.png)

*实时监控 ACM Claude 积分使用量，支持多时间范围查看和趋势分析*

## 🚀 功能特性

- **实时监控**: 通过 SSE (Server-Sent Events) 实时展示积分使用量变化曲线
- **积分余额显示**: 实时展示当前剩余积分数量，支持自动刷新
- **可视化图表**: 使用 ECharts 生成美观的折线图展示积分使用趋势
- **多时间范围**: 支持查看最近 1小时、2小时、3小时、6小时、12小时、24小时的使用情况
- **多模型对比**: 支持同时展示不同 Claude 模型的积分使用情况
- **智能Cookie验证**: 通过数据获取接口隐式验证，无需额外验证请求
- **配置管理**: 支持自定义数据获取间隔和Cookie配置
- **单文件部署**: 前端静态文件内嵌到后端二进制文件中，部署简单

## 🏗️ 技术架构

### 前端技术栈
- **Bun** - 现代 JavaScript 运行时和包管理器
- **Vite** - 现代前端构建工具
- **React 19** + **TypeScript** - UI 框架
- **TailwindCSS 4** - 原子化 CSS 框架
- **ECharts** - 数据可视化图表库
- **Lucide React** - 现代图标库

### 后端技术栈
- **Go 1.23** - 后端编程语言
- **Fiber v2** - 高性能 Web 框架
- **BadgerDB** - 嵌入式 NoSQL 数据库
- **Resty** - HTTP 客户端库
- **Gocron** - 定时任务调度器
- **Go Embed** - 静态文件嵌入

## 📦 项目结构

```
cccmu/
├── server/                 # 后端代码
│   ├── client/            # API 客户端
│   ├── database/          # 数据库操作
│   ├── handlers/          # HTTP 处理器
│   ├── models/            # 数据模型
│   ├── services/          # 业务服务
│   ├── web/               # 静态文件嵌入
│   └── main.go           # 程序入口
├── web/                   # 前端代码
│   ├── src/
│   │   ├── components/   # React 组件
│   │   ├── pages/        # 页面组件
│   │   ├── api/          # API 客户端
│   │   └── types/        # TypeScript 类型定义
│   ├── dist/             # 前端构建输出
│   └── package.json
├── docs/                  # 项目文档
├── Makefile              # 构建脚本
└── README.md
```

## 🔧 快速开始

### 环境要求

- **Go**: >= 1.23
- **Bun**: >= 1.0
- **Node.js**: >= 18 (可选，Bun 可替代)

### 安装依赖

```bash
# 安装项目依赖
make install
```

### 开发模式

```bash
# 启动前端开发服务器
make dev-frontend

# 启动后端开发服务器
make dev-backend

# 同时启动前后端开发环境
make dev
```

### 生产构建

```bash
# 完整构建项目
make build
```

构建完成后会生成 `cccmu` 可执行文件，包含完整的前后端应用。

### 运行应用

```bash
# 使用默认端口 8080 运行
./cccmu

# 使用自定义端口运行
./cccmu -port 9090

# 启用详细日志输出（用于调试）
./cccmu --log

# 同时指定端口和启用日志
./cccmu --port 9090 --log

# 查看帮助信息
./cccmu -h
```

默认访问地址: `http://localhost:8080`

#### 命令行参数

| 参数 | 描述 | 示例 |
|------|------|------|
| `--port` / `-port` | 指定服务器端口号 | `./cccmu --port 9090` |
| `--log` | 启用详细日志输出（用于调试和维护） | `./cccmu --log` |
| `--help` / `-h` | 显示帮助信息 | `./cccmu --help` |

**日志控制说明：**
- **默认模式**：静默运行，仅显示必要的启动和错误信息
- **调试模式**：使用 `--log` 参数启用详细日志，包括：
  - API 请求和响应详情
  - 积分计算过程
  - 数据处理状态
  - 定时任务执行情况
- **性能影响**：未启用日志时，调试输出被完全禁用，不影响运行性能

#### 端口配置方式

支持多种端口配置方式，按优先级排序：

1. **命令行参数**（最高优先级）
   ```bash
   ./cccmu -port 9090
   ./cccmu -port :3000
   ```

2. **环境变量**
   ```bash
   PORT=9090 ./cccmu
   export PORT=3000 && ./cccmu
   ```

3. **默认端口**：`:8080`（如果未配置其他方式）

命令行参数会覆盖环境变量设置：
```bash
# 最终使用端口 9090（命令行参数优先）
PORT=8080 ./cccmu -port 9090
```

## ⚙️ 配置说明

### Cookie 配置

1. 访问 [ACM Claude Dashboard](https://www.aicodemirror.com/dashboard/usage)
2. 在浏览器开发者工具中复制完整的 Cookie 字符串
3. 在应用设置页面中粘贴 Cookie 信息

### 数据获取间隔

支持配置以下时间间隔：
- 30 秒
- 1 分钟（默认）
- 5 分钟
- 10 分钟
- 30 分钟
- 1 小时

### Cookie验证机制

**智能隐式验证**：
- 通过积分数据获取和余额查询接口自动验证Cookie有效性
- 当API返回401状态码时自动检测Cookie失效
- 验证失败时会通过前端提示用户更新Cookie
- 无需额外的验证请求，减少API调用频次

### 数据展示范围

支持以下时间范围：
- 最近 1 小时（默认）
- 最近 2 小时
- 最近 3 小时
- 最近 6 小时
- 最近 12 小时
- 最近 24 小时

## 📊 数据格式

### 积分使用数据结构

```json
{
  "id": 11048661,
  "type": "USAGE", 
  "endpoint": "v1/messages",
  "statusCode": 200,
  "creditsUsed": 9,
  "createdAt": "2025-08-25T13:39:44.230Z",
  "model": "claude-sonnet-4-20250514"
}
```

### 积分余额数据结构

```json
{
  "remaining": 7542,
  "updatedAt": "2025-09-01T10:30:45.123Z"
}
```

## 🔐 安全说明

- Cookie 信息本地存储在 BadgerDB 中，确保数据安全
- **隐式Cookie验证**：通过数据获取接口自动验证Cookie有效性，减少API调用
- **自动失败处理**：Cookie失效时前端会提示用户更新，保护系统稳定性
- **智能错误处理**：401状态码自动识别Cookie过期，及时反馈给用户

## 📝 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🤝 贡献指南

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📞 支持

如果您遇到任何问题或有功能建议，请在 [GitHub Issues](https://github.com/leafney/cccmu/issues) 中提出。
