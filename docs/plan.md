# ACM Claude 积分监控系统 - 开发任务计划

## 项目概述
ACM Claude 积分使用量监控系统，通过SSE方式实时展示积分使用曲线，支持多模型对比和自定义时间范围查看。

---

## 阶段一：项目结构初始化

### ✅ 任务 1.1 创建项目目录结构
- [ ] 创建后端目录：`server/`
- [ ] 创建前端目录：`web/`
- [ ] 创建文档目录：`docs/`
- [ ] 创建构建脚本：`Makefile`
- [ ] 配置忽略文件：`.gitignore`

### ✅ 任务 1.2 项目文档初始化
- [x] 项目介绍文档：`idea.md`
- [x] 开发计划文档：`plan.md`
- [x] 开发规范文档：`claude.md`

---

## 阶段二：后端开发 (Golang + Fiber v2)

### ✅ 任务 2.1 Go项目初始化
- [ ] 初始化Go模块：`go mod init github.com/leafney/cccmu`
- [ ] 安装核心依赖：
  - `github.com/gofiber/fiber/v2` (Web框架)
  - `github.com/dgraph-io/badger/v4` (本地数据库)
  - `github.com/go-resty/resty/v2` (HTTP客户端)
  - `github.com/go-co-op/gocron/v2` (定时任务)

### ✅ 任务 2.2 核心模块开发
- [ ] 配置管理模块 (`config/config.go`)
  - 应用配置结构定义
  - 环境变量读取
  - 配置验证
- [ ] 数据库模块 (`database/badger.go`)
  - BadgerDB初始化
  - 数据存储接口定义
  - Cookie和配置数据存储
- [ ] API客户端模块 (`client/api.go`)
  - 积分查询API封装
  - Cookie验证API封装
  - HTTP请求头设置

### ✅ 任务 2.3 数据模型定义
- [ ] 用户配置模型 (`models/config.go`)
- [ ] 积分数据模型 (`models/usage.go`)
- [ ] API响应模型 (`models/response.go`)

### ✅ 任务 2.4 业务逻辑层
- [x] 积分数据服务 (`services/usage.go`)
  - 数据获取和处理
  - 时间格式转换
  - 数据过滤逻辑
- [x] 配置管理服务 (`services/config.go`)
  - Cookie管理
  - 用户设置管理
- [x] 任务调度服务 (`services/scheduler.go`)
  - 定时积分数据获取
  - Cookie有效性验证
  - 使用gocron/v2实现
  - **智能Cookie验证机制**：
    - 基于时间戳的验证策略
    - API成功请求自动更新验证时间
    - 可配置验证间隔（默认10分钟）
    - Cookie失效自动停止任务并清理状态

### ✅ 任务 2.5 API接口层
- [ ] SSE接口 (`handlers/sse.go`)
  - 实时数据推送
  - 客户端连接管理
- [ ] 配置接口 (`handlers/config.go`)
  - Cookie设置接口
  - 配置查询和更新接口
- [ ] 控制接口 (`handlers/control.go`)
  - 任务启动/停止接口
  - 手动刷新接口

### ✅ 任务 2.6 中间件和工具
- [ ] CORS中间件配置
- [ ] 静态文件服务中间件
- [ ] 错误处理中间件
- [ ] 日志中间件

---

## 阶段三：前端开发 (React + TypeScript)

### ✅ 任务 3.1 前端项目初始化
- [ ] 使用Bun初始化项目：`bun create vite web --template react-ts`
- [ ] 安装依赖：
  - `echarts` (图表库)
  - `echarts-for-react` (React封装)
  - `@tailwindcss/typography` (样式)
  - `lucide-react` (图标)
- [ ] 配置TailwindCSS4
- [ ] 配置Vite构建选项

### ✅ 任务 3.2 项目结构搭建
- [ ] 创建组件目录结构：`src/components/`
- [ ] 创建页面目录：`src/pages/`
- [ ] 创建工具目录：`src/utils/`
- [ ] 创建类型定义：`src/types/`
- [ ] 创建API接口：`src/api/`

### ✅ 任务 3.3 核心组件开发
- [ ] 积分曲线图组件 (`components/UsageChart.tsx`)
  - ECharts折线图配置
  - 多模型数据处理
  - 时间轴配置
  - 响应式设计
- [ ] 设置面板组件 (`components/SettingsPanel.tsx`)
  - Cookie输入表单
  - 时间间隔选择
  - 任务控制开关
- [ ] 时间范围选择器 (`components/TimeRangeSelector.tsx`)
- [ ] 状态指示器组件 (`components/StatusIndicator.tsx`)

### ✅ 任务 3.4 页面开发
- [ ] 主仪表盘页面 (`pages/Dashboard.tsx`)
- [ ] 设置页面 (`pages/Settings.tsx`)
- [ ] 路由配置 (`App.tsx`)

### ✅ 任务 3.5 数据处理和状态管理
- [ ] SSE连接管理 (`utils/sse.ts`)
- [ ] 数据格式化工具 (`utils/dataProcessor.ts`)
- [ ] 状态管理 (使用React Context或Zustand)
- [ ] API接口封装 (`api/client.ts`)

### ✅ 任务 3.6 用户体验优化
- [ ] 加载状态处理
- [ ] 错误边界和错误处理
- [ ] 响应式布局适配
- [ ] 主题色彩配置

---

## 阶段四：集成和部署

### ✅ 任务 4.1 前后端集成
- [ ] 配置Go embed前端静态资源
- [ ] 处理前端路由和API路由冲突
- [ ] 静态资源服务配置
- [ ] 开发环境代理配置

### ✅ 任务 4.2 构建系统
- [ ] 编写Makefile构建脚本：
  - `make frontend` - 构建前端
  - `make backend` - 编译后端
  - `make build` - 完整构建
  - `make dev` - 开发环境启动
  - `make clean` - 清理构建文件
- [ ] 配置构建优化参数

### ✅ 任务 4.3 测试和验证
- [ ] 功能测试：
  - SSE连接和数据推送
  - Cookie验证机制
  - 定时任务执行
  - 图表数据展示
- [ ] 边界情况测试：
  - 网络连接异常
  - Cookie失效处理
  - 数据格式异常
  - 并发访问测试

### ✅ 任务 4.4 文档和部署
- [ ] API接口文档
- [ ] 部署说明文档
- [ ] 用户使用手册
- [ ] 性能优化和监控

---

## 新增功能特性（Cookie验证优化）

### ✅ 任务 5.1 Cookie智能验证机制
- [x] **数据模型扩展**：
  - 添加`LastCookieValidTime`字段记录最后验证成功时间
  - 添加`CookieValidationInterval`字段配置验证间隔
- [x] **数据库操作扩展**：
  - 实现`UpdateCookieValidTime()`方法
  - 实现`ShouldValidateCookie()`智能判断逻辑
- [x] **API客户端优化**：
  - 集成回调机制，成功请求自动更新验证时间戳
  - `FetchUsageData`、`FetchCreditBalance`、`ValidateCookie`自动触发回调
- [x] **调度器服务增强**：
  - 新增独立的Cookie验证定时任务
  - 实现`validateCookieIfNeeded()`智能验证逻辑
  - 实现`handleCookieValidationFailure()`失败处理机制
  - Cookie失效时自动停止任务、清理状态、禁用配置

### ✅ 任务 5.2 性能和体验优化
- [x] **减少API调用频次**：
  - 只有超过验证间隔时间才进行Cookie验证
  - 避免频繁的验证请求影响API配额
- [x] **智能失败恢复**：
  - Cookie失效时自动清理无效状态
  - 用户只需重新配置Cookie即可快速恢复
- [x] **详细日志记录**：
  - Cookie验证过程的详细日志
  - 失败处理流程的状态跟踪

---

## 开发优先级

### 高优先级 (核心功能) ✅
1. 后端API客户端和数据获取
2. SSE数据推送机制
3. 前端图表展示组件
4. Cookie管理和验证
5. **Cookie智能验证机制（新增）**

### 中优先级 (重要功能) ✅
1. 定时任务调度
2. 用户配置管理
3. 时间范围选择
4. 多模型对比
5. 积分余额显示功能

### 低优先级 (优化功能)
1. 用户界面美化
2. 错误处理完善
3. 性能优化
4. 部署文档
5. 前端Cookie验证间隔配置界面（待实现）

---

## 预估时间

- **阶段一**: 0.5天
- **阶段二**: 1.5天
- **阶段三**: 1.5天
- **阶段四**: 0.5天

**总计**: 4天 (包含测试和调试时间)

---

## 风险控制

### 技术风险
- ECharts图表性能优化
- SSE连接稳定性
- Cookie转义字符处理

### 进度风险
- 前端样式调整时间
- 接口联调时间
- 部署环境配置

### 解决方案
- 分阶段开发，确保核心功能优先
- 预留20%的调试和优化时间
- 及时测试和验证功能