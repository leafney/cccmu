# Claude Code 项目开发规范

## 项目信息
- **项目名称**: Claude Code积分监控系统 (CCCMU)
- **技术栈**: Go + Fiber v2 (后端) + React + TypeScript (前端)
- **开发工具**: Bun + Vite + TailwindCSS4 + ECharts

---

## 开发环境要求

### 必备工具
- **Go**: >= 1.21
- **Bun**: 最新稳定版 (用于前端包管理和构建)
- **Make**: 用于执行构建脚本

### 推荐编辑器配置
- VS Code 或 GoLand
- Go 语言插件和 TypeScript 插件
- 代码格式化：`gofmt` 和 `prettier`

---

## 项目结构规范

```
cccmu/
├── server/                 # 后端Go代码
│   ├── main.go            # 程序入口
│   ├── config/            # 配置管理
│   ├── database/          # 数据库操作
│   ├── handlers/          # HTTP处理器
│   ├── services/          # 业务逻辑
│   ├── models/            # 数据模型
│   ├── client/            # API客户端
│   ├── middleware/        # 中间件
│   └── utils/             # 工具函数
├── web/                   # 前端React代码
│   ├── src/
│   │   ├── components/    # React组件
│   │   ├── pages/         # 页面组件
│   │   ├── utils/         # 工具函数
│   │   ├── types/         # TypeScript类型定义
│   │   ├── api/           # API接口封装
│   │   └── assets/        # 静态资源
│   ├── public/            # 公共静态文件
│   └── package.json       # 前端依赖配置
├── docs/                  # 项目文档
├── Makefile              # 构建脚本
├── go.mod                # Go依赖管理
├── .gitignore           # Git忽略配置
├── idea.md              # 项目需求文档
├── plan.md              # 开发计划文档
└── claude.md            # 本开发规范文档
```

---

## 代码规范

### Go 代码规范

#### 命名规范
- **包名**: 小写字母，简洁明了 (`config`, `handlers`, `services`)
- **文件名**: 小写字母加下划线 (`user_config.go`, `usage_data.go`)
- **结构体**: 大驼峰命名 (`UserConfig`, `UsageData`)
- **函数**: 大驼峰命名 (公开) 或小驼峰命名 (私有)
- **常量**: 全大写加下划线 (`API_BASE_URL`, `DEFAULT_TIMEOUT`)

#### 代码组织
```go
// 标准包导入顺序
package main

import (
    // 标准库
    "fmt"
    "time"
    
    // 第三方库
    "github.com/gofiber/fiber/v2"
    "github.com/dgraph-io/badger/v4"
    
    // 本地包
    "./config"
    "./services"
)
```

#### 错误处理
```go
// 统一错误处理模式
func GetUsageData() (*UsageData, error) {
    data, err := apiClient.FetchData()
    if err != nil {
        return nil, fmt.Errorf("获取数据失败: %w", err)
    }
    return data, nil
}
```

### TypeScript/React 代码规范

#### 命名规范
- **组件名**: 大驼峰命名 (`UsageChart`, `SettingsPanel`)
- **文件名**: 大驼峰命名，与组件名一致 (`UsageChart.tsx`)
- **变量/函数**: 小驼峰命名 (`usageData`, `fetchData`)
- **类型接口**: 大驼峰命名，以I开头 (`IUsageData`, `IConfig`)

#### 组件结构
```tsx
// 组件模板
import React from 'react';
import { IUsageData } from '../types';

interface UsageChartProps {
  data: IUsageData[];
  className?: string;
}

export const UsageChart: React.FC<UsageChartProps> = ({ 
  data, 
  className = '' 
}) => {
  return (
    <div className={`usage-chart ${className}`}>
      {/* 组件内容 */}
    </div>
  );
};
```

---

## API 设计规范

### RESTful 接口规范
```
GET    /api/config          # 获取配置
PUT    /api/config          # 更新配置
GET    /api/usage/stream    # SSE数据流
POST   /api/control/start   # 启动任务
POST   /api/control/stop    # 停止任务  
POST   /api/refresh         # 手动刷新
```

### 请求/响应格式
```go
// 统一响应格式
type APIResponse struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// 错误响应
type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Error   string `json:"error,omitempty"`
}
```

---

## 数据库设计规范

### BadgerDB 存储键值规范
```
config:cookie        # 用户Cookie配置
config:interval      # 刷新时间间隔
config:timerange     # 时间范围设置
config:enabled       # 任务启用状态
usage:{timestamp}    # 积分使用数据 (按时间戳存储)
```

### 数据模型定义
```go
// 积分使用数据
type UsageData struct {
    ID          int       `json:"id"`
    Type        string    `json:"type"`
    Endpoint    string    `json:"endpoint"`
    StatusCode  int       `json:"statusCode"`
    CreditsUsed int       `json:"creditsUsed"`
    CreatedAt   time.Time `json:"createdAt"`
    Model       string    `json:"model"`
}

// 用户配置
type UserConfig struct {
    Cookie      string `json:"cookie"`
    Interval    int    `json:"interval"`    // 刷新间隔(分钟)
    TimeRange   int    `json:"timeRange"`   // 时间范围(小时)
    Enabled     bool   `json:"enabled"`     // 任务启用状态
}
```

---

## 构建和部署规范

### Makefile 命令规范
```makefile
# 开发环境
dev-frontend:    # 启动前端开发服务器
dev-backend:     # 启动后端开发服务器
dev:            # 同时启动前后端开发环境

# 构建相关
build-frontend:  # 构建前端生产版本
build-backend:   # 编译后端二进制文件
build:          # 完整构建 (先前端后后端)

# 工具命令
clean:          # 清理构建文件
test:           # 运行测试
lint:           # 代码检查
fmt:            # 代码格式化
```

### 构建参数
```bash
# Go 构建参数 (减小二进制文件大小)
go build -ldflags="-s -w" -o cccmu main.go

# 前端构建优化
bun run build --minify --sourcemap=false
```

---

## 错误处理和日志规范

### 错误分类
```go
const (
    ErrInvalidCookie    = "INVALID_COOKIE"
    ErrAPIRequest       = "API_REQUEST_ERROR"
    ErrDataParsing      = "DATA_PARSING_ERROR" 
    ErrDatabaseOperation = "DATABASE_ERROR"
)
```

### 日志格式
```go
// 使用结构化日志
log.Info("数据获取成功", 
    "endpoint", "/api/user/usage",
    "records", len(data),
    "duration", time.Since(start),
)

log.Error("Cookie验证失败",
    "error", err,
    "cookie_prefix", cookie[:20]+"...",
)
```

---

## 性能优化规范

### 后端优化
- 使用连接池管理HTTP请求
- BadgerDB读写操作异步处理
- SSE连接数量限制和清理
- 定时任务合理的时间间隔设置

### 前端优化
- ECharts图表数据虚拟化（大数据量时）
- React组件懒加载
- 使用useMemo和useCallback优化渲染
- SSE连接重连机制

### 数据处理优化
- 只保留必要的历史数据
- 定期清理过期数据
- 数据压缩存储
- 增量数据更新

---

## 安全规范

### Cookie 处理
- 用户Cookie信息加密存储
- Cookie有效性定期验证
- 敏感信息不记录到日志
- Cookie传输使用HTTPS

### 输入验证
- 所有用户输入进行验证和转义
- API参数类型和范围检查
- SQL注入和XSS防护
- 文件上传安全检查

---

## 测试规范

### 单元测试
```go
// Go 测试文件命名: xxx_test.go
func TestGetUsageData(t *testing.T) {
    // 测试逻辑
}
```

### 集成测试
- API接口测试
- SSE连接测试
- 数据库操作测试
- 定时任务测试

### 前端测试
```tsx
// React 组件测试
import { render, screen } from '@testing-library/react';
import { UsageChart } from './UsageChart';

test('renders usage chart', () => {
    // 测试逻辑
});
```

---

## Git 提交规范

### 提交信息格式
```
类型(作用域): 简短描述

详细说明 (可选)

相关Issue: #123
```

### 提交类型
- `feat`: 新功能
- `fix`: 错误修复
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建工具或辅助工具变动

### 示例
```
feat(backend): 添加SSE数据推送接口

- 实现实时积分数据推送
- 支持多客户端连接管理
- 添加连接断开处理机制

相关Issue: #1
```

---

## 开发工作流

### 分支管理
- `main`: 主分支，稳定代码
- `develop`: 开发分支
- `feature/*`: 功能分支
- `hotfix/*`: 紧急修复分支

### 开发步骤
1. 从`develop`分支创建`feature`分支
2. 完成功能开发和测试
3. 提交代码并创建PR
4. 代码评审通过后合并到`develop`
5. 测试无误后合并到`main`

---

## 文档维护规范

### 必要文档
- `idea.md`: 项目需求和功能说明
- `plan.md`: 开发计划和任务清单  
- `claude.md`: 开发规范 (本文档)
- `README.md`: 项目说明和使用指南
- `API.md`: API接口文档

### 文档更新
- 功能变更时同步更新相关文档
- 保持文档与代码实现一致
- 定期检查文档的准确性和完整性

---

## 部署和运维

### 生产环境要求
- Linux服务器环境
- 至少1GB内存和10GB存储空间
- 网络访问第三方API的能力
- SSL证书配置 (推荐)

### 监控指标
- 应用程序运行状态
- API响应时间
- 数据获取成功率
- 内存和CPU使用情况
- 错误日志监控

---

**注意**: 本规范文档会随着项目开发过程不断完善和更新，请开发过程中及时维护。