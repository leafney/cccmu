# 项目介绍

当前项目通过图表的形式展示 Claude Code 积分使用量的变化曲线。

# 项目功能

- 通过SSE方式，展示 Claude Code 积分使用量的变化曲线
- 支持自定义时间范围，展示不同时间周期的积分使用情况
- 支持对比不同模型的积分使用情况

# 项目架构

- 前端：Bun + Vite + React + TypeScript + Echarts + TailwindCSS4
- 后端：Golang + Fiber v2
- 图表库：Echarts

# 项目部署

- 通过golang embed 内嵌前端项目，打包为单独可执行二进制文件
- 创建 Makefile 脚本文件，定义项目的编译、运行、打包等任务

# 项目逻辑

- 前端通过SSE方式，与后端建立连接，实时接收积分使用数据
- 后端将获取到的数据，通过SSE方式，发送给前端
- 前端收到数据后，通过Echarts库，展示积分使用曲线，曲线展示方式为折线图，显示出当前的积分值
- 后端定时请求接口，获取最新数据展示。前端可配置默认请求时间间隔，默认每1分钟请求一次，支持调整请求时间间隔，如1分钟、5分钟、10分钟等；同时前端支持立即刷新数据
- 前端支持自定义展示时间范围，默认展示最近1小时的积分使用情况，可选择展示最近1小时、2小时、3小时、6小时、12小时、24小时的积分使用情况
- 前端支持对比不同模型的积分使用情况，根据接口返回的模型类型，展示不同的模型曲线
- 前端的配置信息，在后端通过badgerdb存储
- 后端通过 go-resty 库调用指定的积分查询api接口，获取积分使用数据；调用时需要设置请求cookie信息，用户可以在前端设置页面中添加cookie信息，后端会将cookie信息存储在badgerdb中
- 后端实现一个异步任务，调用指定的验证cookie有效性接口，来验证cookie是否有效；如果cookie失效，则停止调用api接口获取数据，并弹窗提示用户重新填写cookie值；如果cookie有效，则继续调用api接口获取数据
- 前端通过switch切换获取积分数据任务的执行状态，默认关闭，用户需要手动开启获取数据任务，开启后，后端会定时调用api接口获取数据，前端会实时展示数据
- 前端支持手动刷新数据，用户可以在前端页面中点击刷新按钮，手动触发数据刷新
- 用户在前端页面中添加的cookie信息是从浏览器的cookie中获取的，需要考虑是否对cookie信息做转义处理，避免特殊字符导致的问题

# 接口定义

## 积分查询接口

- 接口地址：https://www.aicodemirror.com/api/user/usage
- 接口方法：GET
- 请求参数：
  - cookie：用户在前端设置页面中添加的cookie信息
  - referer：https://www.aicodemirror.com/dashboard/usage
  - user-agent：Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36
  - accept：application/json, text/plain, */*
- 响应参数：
  - code：响应状态码，200表示成功，其他值表示失败
  - data：积分使用数据，json格式

响应数据示例：

```json
[{"id":11048661,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":9,"createdAt":"2025-08-25T13:39:44.230Z","model":"claude-sonnet-4-20250514"},{"id":11048594,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":18,"createdAt":"2025-08-25T13:39:34.217Z","model":"claude-sonnet-4-20250514"},{"id":11048316,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":21,"createdAt":"2025-08-25T13:38:56.052Z","model":"claude-sonnet-4-20250514"},{"id":11047545,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":51,"createdAt":"2025-08-25T13:37:24.760Z","model":"claude-sonnet-4-20250514"},{"id":11045149,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":16,"createdAt":"2025-08-25T13:31:33.438Z","model":"claude-sonnet-4-20250514"},{"id":11044949,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":41,"createdAt":"2025-08-25T13:31:03.813Z","model":"claude-sonnet-4-20250514"},{"id":11044679,"type":"USAGE","endpoint":"v1/messages/count_tokens","statusCode":200,"creditsUsed":0,"createdAt":"2025-08-25T13:30:27.382Z","model":"claude-sonnet-4-20250514"},{"id":11044674,"type":"USAGE","endpoint":"v1/messages/count_tokens","statusCode":200,"creditsUsed":0,"createdAt":"2025-08-25T13:30:26.734Z","model":"claude-sonnet-4-20250514"},{"id":11044671,"type":"USAGE","endpoint":"v1/messages/count_tokens","statusCode":200,"creditsUsed":0,"createdAt":"2025-08-25T13:30:26.715Z","model":"claude-sonnet-4-20250514"},{"id":11044665,"type":"USAGE","endpoint":"v1/messages/count_tokens","statusCode":200,"creditsUsed":0,"createdAt":"2025-08-25T13:30:26.380Z","model":"claude-sonnet-4-20250514"},{"id":11044654,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":73,"createdAt":"2025-08-25T13:30:25.502Z","model":"claude-sonnet-4-20250514"},{"id":11044251,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":13,"createdAt":"2025-08-25T13:29:25.843Z","model":"claude-sonnet-4-20250514"},{"id":11044153,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":10,"createdAt":"2025-08-25T13:29:11.667Z","model":"claude-sonnet-4-20250514"},{"id":11044090,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":15,"createdAt":"2025-08-25T13:29:02.896Z","model":"claude-sonnet-4-20250514"},{"id":11043770,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":12,"createdAt":"2025-08-25T13:28:20.940Z","model":"claude-sonnet-4-20250514"},{"id":11041368,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":10,"createdAt":"2025-08-25T13:23:26.990Z","model":"claude-sonnet-4-20250514"},{"id":11041253,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":15,"createdAt":"2025-08-25T13:23:13.697Z","model":"claude-sonnet-4-20250514"},{"id":11041168,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":8,"createdAt":"2025-08-25T13:23:05.154Z","model":"claude-sonnet-4-20250514"},{"id":11041108,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":9,"createdAt":"2025-08-25T13:22:59.270Z","model":"claude-sonnet-4-20250514"},{"id":11041043,"type":"USAGE","endpoint":"v1/messages","statusCode":200,"creditsUsed":13,"createdAt":"2025-08-25T13:22:51.212Z","model":"claude-sonnet-4-20250514"}]
```

响应参数说明：

- `id`: 数据id
- `type`: 数据类型，当值为 `USAGE` 时，表示积分使用量，其他值时不考虑
- `endpoint`: 数据接口地址，当值为 `v1/messages` 时，表示积分使用量，其他值时不考虑
- `creditsUsed`: 积分使用量值，示例为 `100`
- `createdAt`: 数据时间，示例为 `2025-08-25T13:22:59.270Z` ，需要做时间转换，转换为 `2025-08-25 13:22:59` 格式
- `model`: 模型名称，示例为 `claude-sonnet-4-20250514`，相同的模型名称采用同一条折线展示，不同的模型名称采用不同的折线展示

## 验证cookie有效性

- 接口地址：https://www.aicodemirror.com/api/user/usage/chart
- 接口方法：GET
- 请求参数：
  - cookie：用户在前端设置页面中添加的cookie信息
  - referer：https://www.aicodemirror.com/dashboard/usage
  - user-agent：Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36
  - accept：application/json, text/plain, */*
- 响应参数：
  - code：响应状态码，200表示成功，其他值表示失败
  - 不需要关注响应数据
  - 例如：当cookie值失效后，响应状态码为 401，响应内容为 `{"error":"Unauthorized"}`

---
