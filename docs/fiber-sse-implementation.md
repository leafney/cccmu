# Fiber + SSE 实时数据通信实现指南

本文档总结了在 Go Fiber 框架中实现 Server-Sent Events (SSE) 的完整方案，包含关键步骤、最佳实践和常见问题解决方案。

## 目录

- [概述](#概述)
- [后端实现](#后端实现)
  - [SSE 处理器结构](#sse-处理器结构)
  - [流式响应实现](#流式响应实现)
  - [数据监听器模式](#数据监听器模式)
  - [错误处理和资源清理](#错误处理和资源清理)
- [前端实现](#前端实现)
  - [EventSource 连接管理](#eventsource-连接管理)
  - [事件监听和状态管理](#事件监听和状态管理)
  - [重连机制](#重连机制)
- [最佳实践](#最佳实践)
- [常见问题与解决方案](#常见问题与解决方案)
- [性能优化](#性能优化)

---

## 概述

Server-Sent Events (SSE) 是一种让服务器向客户端推送实时数据的技术。相比 WebSocket，SSE 更轻量且单向通信，适合实时数据推送场景。

### 适用场景
- 实时数据监控仪表板
- 实时日志流
- 进度更新推送
- 实时通知系统

### 技术栈
- **后端**: Go + Fiber v2
- **前端**: TypeScript + EventSource API
- **数据格式**: JSON
- **通信协议**: HTTP/1.1 或 HTTP/2

---

## 后端实现

### SSE 处理器结构

```go
// handlers/sse.go
package handlers

import (
    "bufio"
    "encoding/json"
    "fmt"
    "log"
    "time"
    
    "github.com/gofiber/fiber/v2"
)

// SSEHandler SSE处理器
type SSEHandler struct {
    db        *database.BadgerDB
    scheduler *services.SchedulerService
}

// NewSSEHandler 创建SSE处理器
func NewSSEHandler(db *database.BadgerDB, scheduler *services.SchedulerService) *SSEHandler {
    return &SSEHandler{
        db:        db,
        scheduler: scheduler,
    }
}
```

### 流式响应实现

**核心要点**：
1. 使用 `SetBodyStreamWriter` 而非直接写入响应
2. 在流式函数外获取查询参数和上下文
3. 立即发送连接确认事件
4. 使用 defer 确保资源清理

```go
// StreamUsageData SSE数据流端点
func (h *SSEHandler) StreamUsageData(c *fiber.Ctx) error {
    // 设置SSE响应头
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Access-Control-Allow-Origin", "*")
    c.Set("Access-Control-Allow-Headers", "Cache-Control")
    
    // ⚠️ 关键：在流式响应外获取查询参数，避免context访问问题
    hours := c.QueryInt("hours", 1)
    if hours <= 0 {
        hours = 1
    }
    
    // ⚠️ 关键：获取上下文，避免在goroutine中访问可能已释放的context
    ctx := c.Context()
    
    log.Printf("新的SSE连接，时间范围: %d小时", hours)

    // ⚠️ 关键：使用Fiber的流式响应，而非直接写入
    c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
        // 1. 立即发送连接确认
        fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
        w.Flush()
        log.Printf("已发送SSE连接确认")

        // 2. 立即发送当前数据
        h.sendInitialData(w, hours)

        // 3. 设置数据监听器
        listener := h.scheduler.AddDataListener()
        defer func() {
            h.scheduler.RemoveDataListener(listener)
            log.Printf("SSE监听器已清理")
        }()
        
        // 4. 设置心跳保活
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        // 5. 监听循环
        h.listenLoop(w, listener, ticker, ctx, hours)
    })

    return nil
}
```

### 数据监听器模式

**设计思路**：
- 使用通道 (channel) 进行数据分发
- 支持多个客户端同时连接
- 自动清理断开的监听器

```go
// services/scheduler.go
type SchedulerService struct {
    listeners []chan []models.UsageData
    mu        sync.RWMutex
    lastData  []models.UsageData
}

// AddDataListener 添加数据监听器
func (s *SchedulerService) AddDataListener() chan []models.UsageData {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // 创建缓冲通道，避免阻塞
    listener := make(chan []models.UsageData, 10)
    s.listeners = append(s.listeners, listener)
    
    log.Printf("添加数据监听器，当前监听器数量: %d", len(s.listeners))
    return listener
}

// RemoveDataListener 移除数据监听器
func (s *SchedulerService) RemoveDataListener(target chan []models.UsageData) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    for i, listener := range s.listeners {
        if listener == target {
            close(listener)
            s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
            log.Printf("移除数据监听器，剩余监听器数量: %d", len(s.listeners))
            break
        }
    }
}

// NotifyListeners 通知所有监听器
func (s *SchedulerService) notifyListeners(data []models.UsageData) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    for i, listener := range s.listeners {
        select {
        case listener <- data:
            log.Printf("成功向监听器 %d 发送数据", i)
        default:
            // ⚠️ 重要：如果通道已满，跳过这次通知避免阻塞
            log.Printf("监听器 %d 通道已满，跳过通知", i)
        }
    }
}
```

### 错误处理和资源清理

```go
func (h *SSEHandler) listenLoop(w *bufio.Writer, listener chan []models.UsageData, ticker *time.Ticker, ctx context.Context, hours int) {
    for {
        select {
        case data, ok := <-listener:
            if !ok {
                log.Println("SSE监听器通道已关闭")
                return
            }
            
            // 发送数据
            if err := h.sendUsageData(w, data, hours); err != nil {
                log.Printf("发送数据失败: %v", err)
                return
            }

        case <-ticker.C:
            // 发送心跳保活
            if err := h.sendHeartbeat(w); err != nil {
                log.Printf("发送心跳失败: %v", err)
                return
            }

        case <-ctx.Done():
            log.Println("SSE连接上下文已取消")
            return
        }
    }
}

// sendUsageData 发送数据并处理错误
func (h *SSEHandler) sendUsageData(w *bufio.Writer, data []models.UsageData, hours int) error {
    filteredData := models.UsageDataList(data).FilterByTimeRange(hours)
    
    if len(filteredData) > 0 {
        jsonData, err := json.Marshal(filteredData)
        if err != nil {
            return fmt.Errorf("序列化数据失败: %w", err)
        }
        
        fmt.Fprintf(w, "event: usage\ndata: %s\n\n", jsonData)
        
        // ⚠️ 重要：检查 Flush 错误，客户端断开时会返回错误
        if err := w.Flush(); err != nil {
            return fmt.Errorf("刷新数据到客户端失败: %w", err)
        }
        
        log.Printf("已发送数据到客户端")
    }
    
    return nil
}
```

---

## 前端实现

### EventSource 连接管理

```typescript
// api/client.ts
class APIClient {
  createSSEConnection(
    onMessage: (data: IUsageData[]) => void, 
    onError?: (error: Event) => void, 
    onOpen?: () => void,
    timeRange: number = 1
  ): EventSource {
    const eventSource = new EventSource(`${API_BASE}/usage/stream?hours=${timeRange}`);
    
    // 1. 监听连接确认事件
    eventSource.addEventListener('connected', (event) => {
      console.log('SSE连接已确认:', event.data);
    });

    // 2. 监听数据事件
    eventSource.addEventListener('usage', (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch (error) {
        console.error('解析SSE数据失败:', error, event.data);
      }
    });

    // 3. 监听心跳事件
    eventSource.addEventListener('heartbeat', (event) => {
      console.debug('收到SSE心跳:', event.data);
    });

    // 4. 错误处理
    eventSource.onerror = (error) => {
      console.error('SSE连接错误:', error);
      if (onError) {
        onError(error);
      }
    };

    // 5. 连接打开事件
    eventSource.onopen = () => {
      console.log('SSE连接已建立 - onopen事件');
      if (onOpen) {
        onOpen();
      }
    };

    return eventSource;
  }
}
```

### 事件监听和状态管理

```typescript
// pages/Dashboard.tsx (React Hook 示例)
export function Dashboard() {
  const [isConnected, setIsConnected] = useState(false);
  const [eventSource, setEventSource] = useState<EventSource | null>(null);
  const retryTimeoutRef = useRef<number | null>(null);

  // 建立SSE连接
  const connectSSE = useCallback(() => {
    // 清理现有连接和重试计时器
    if (retryTimeoutRef.current) {
      clearTimeout(retryTimeoutRef.current);
      retryTimeoutRef.current = null;
    }
    
    setEventSource(prev => {
      if (prev) {
        prev.close();
        setIsConnected(false);
      }
      return null;
    });

    // 创建新连接
    const newEventSource = apiClient.createSSEConnection(
      (data) => {
        setUsageData(data);
        setIsConnected(true); // ⚠️ 重要：收到数据时确认连接状态
      },
      (error) => {
        setIsConnected(false);
        // 自动重连
        retryTimeoutRef.current = setTimeout(() => {
          connectSSE();
        }, 5000);
      },
      () => {
        setIsConnected(true); // ⚠️ 重要：连接建立时更新状态
      },
      timeRange
    );

    setEventSource(newEventSource);
  }, [timeRange]);

  // 组件卸载时清理
  useEffect(() => {
    return () => {
      if (retryTimeoutRef.current) {
        clearTimeout(retryTimeoutRef.current);
      }
      if (eventSource) {
        eventSource.close();
      }
    };
  }, [eventSource]);
}
```

### 重连机制

```typescript
// 指数退避重连策略
class SSEReconnector {
  private retryCount = 0;
  private maxRetries = 5;
  private baseDelay = 1000; // 1秒
  private maxDelay = 30000; // 30秒

  scheduleReconnect(connectFn: () => void): void {
    if (this.retryCount >= this.maxRetries) {
      console.error('SSE重连次数已达上限');
      return;
    }

    const delay = Math.min(
      this.baseDelay * Math.pow(2, this.retryCount),
      this.maxDelay
    );

    console.log(`SSE将在${delay}ms后重连 (第${this.retryCount + 1}次)`);
    
    setTimeout(() => {
      this.retryCount++;
      connectFn();
    }, delay);
  }

  reset(): void {
    this.retryCount = 0;
  }
}
```

---

## 最佳实践

### 1. 后端最佳实践

#### 响应头设置
```go
// 必需的SSE响应头
c.Set("Content-Type", "text/event-stream")
c.Set("Cache-Control", "no-cache")
c.Set("Connection", "keep-alive")

// CORS支持（如果需要）
c.Set("Access-Control-Allow-Origin", "*")
c.Set("Access-Control-Allow-Headers", "Cache-Control")
```

#### 事件格式规范
```go
// 标准SSE事件格式
fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)

// 多行数据支持
fmt.Fprintf(w, "event: usage\ndata: %s\ndata: %s\n\n", line1, line2)

// 包含ID和重试间隔
fmt.Fprintf(w, "id: %d\nretry: 3000\nevent: usage\ndata: %s\n\n", id, data)
```

#### 资源管理
```go
// 1. 使用缓冲通道避免阻塞
listener := make(chan []models.UsageData, 10)

// 2. 必须使用defer清理资源
defer func() {
    h.scheduler.RemoveDataListener(listener)
    ticker.Stop()
}()

// 3. 监听上下文取消
case <-ctx.Done():
    return
```

### 2. 前端最佳实践

#### 连接管理
```typescript
// 1. 单例模式管理连接
class SSEManager {
  private eventSource: EventSource | null = null;
  
  connect(url: string): void {
    this.disconnect(); // 先关闭现有连接
    this.eventSource = new EventSource(url);
  }
  
  disconnect(): void {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
  }
}

// 2. 页面卸载时关闭连接
window.addEventListener('beforeunload', () => {
  sseManager.disconnect();
});
```

#### 错误处理
```typescript
// 1. 区分错误类型
eventSource.onerror = (error) => {
  if (eventSource.readyState === EventSource.CONNECTING) {
    console.log('SSE正在重连...');
  } else if (eventSource.readyState === EventSource.CLOSED) {
    console.log('SSE连接已关闭');
  }
};

// 2. 自动重连with限制
let reconnectAttempts = 0;
const maxReconnectAttempts = 3;

function handleError() {
  if (reconnectAttempts < maxReconnectAttempts) {
    reconnectAttempts++;
    setTimeout(connect, 2000);
  }
}
```

### 3. 数据处理最佳实践

```go
// 1. 数据过滤和处理
func (u UsageDataList) FilterByTimeRange(hours int) UsageDataList {
    if hours <= 0 {
        return u
    }
    
    cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
    var filtered UsageDataList
    
    for _, data := range u {
        if data.CreatedAt.After(cutoff) {
            filtered = append(filtered, data)
        }
    }
    
    return filtered
}

// 2. JSON序列化优化
type UsageData struct {
    ID          int       `json:"id"`
    CreditsUsed int       `json:"creditsUsed"`
    CreatedAt   time.Time `json:"createdAt"`
    Model       string    `json:"model,omitempty"` // 空值时省略
}
```

---

## 常见问题与解决方案

### 1. 连接状态显示错误

**问题**：前端显示连接断开，但实际有数据传输

**解决方案**：
```typescript
// 在onopen事件中设置连接状态，而不是等待数据
eventSource.onopen = () => {
  setIsConnected(true); // ✅ 正确
};

// 错误做法：只在收到数据时设置连接状态
// setIsConnected(true); // ❌ 错误
```

### 2. Nil指针错误

**问题**：`panic: runtime error: invalid memory address or nil pointer dereference`

**解决方案**：
```go
// ❌ 错误：在goroutine中直接访问context
c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
    select {
    case <-c.Context().Done(): // 可能导致panic
        return
    }
})

// ✅ 正确：预先获取context
ctx := c.Context()
c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
    select {
    case <-ctx.Done():
        return
    }
})
```

### 3. 数据发送失败

**问题**：客户端断开连接后服务端继续发送数据

**解决方案**：
```go
// 检查Flush错误
if err := w.Flush(); err != nil {
    log.Printf("客户端已断开连接: %v", err)
    return // 退出发送循环
}
```

### 4. 内存泄漏

**问题**：监听器未正确清理导致内存泄漏

**解决方案**：
```go
// 1. 使用defer确保清理
listener := h.scheduler.AddDataListener()
defer func() {
    h.scheduler.RemoveDataListener(listener)
}()

// 2. 监听器管理使用切片而非map
func (s *SchedulerService) RemoveDataListener(target chan []models.UsageData) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    for i, listener := range s.listeners {
        if listener == target {
            close(listener) // 关闭通道
            s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
            break
        }
    }
}
```

### 5. 跨域问题

**问题**：浏览器阻止SSE连接

**解决方案**：
```go
// 设置CORS头
c.Set("Access-Control-Allow-Origin", "*")
c.Set("Access-Control-Allow-Headers", "Cache-Control")
c.Set("Access-Control-Allow-Methods", "GET, OPTIONS")

// 处理OPTIONS预检请求
if c.Method() == "OPTIONS" {
    return c.SendStatus(204)
}
```

---

## 性能优化

### 1. 后端优化

```go
// 1. 使用对象池减少GC压力
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 1024)
    },
}

// 2. 批量发送数据
func (h *SSEHandler) sendBatchData(w *bufio.Writer, data []models.UsageData) error {
    const batchSize = 100
    for i := 0; i < len(data); i += batchSize {
        end := i + batchSize
        if end > len(data) {
            end = len(data)
        }
        
        batch := data[i:end]
        if err := h.sendUsageData(w, batch, hours); err != nil {
            return err
        }
    }
    return nil
}

// 3. 压缩数据（gzip）
func (h *SSEHandler) compressData(data []byte) ([]byte, error) {
    var buf bytes.Buffer
    gz := gzip.NewWriter(&buf)
    
    if _, err := gz.Write(data); err != nil {
        return nil, err
    }
    
    if err := gz.Close(); err != nil {
        return nil, err
    }
    
    return buf.Bytes(), nil
}
```

### 2. 前端优化

```typescript
// 1. 数据节流处理
const throttle = (func: Function, limit: number) => {
  let inThrottle: boolean;
  return function(this: any, ...args: any[]) {
    if (!inThrottle) {
      func.apply(this, args);
      inThrottle = true;
      setTimeout(() => inThrottle = false, limit);
    }
  };
};

// 2. 大数据量虚拟化
const [visibleData, setVisibleData] = useState<IUsageData[]>([]);
const ITEMS_PER_PAGE = 100;

useEffect(() => {
  // 只显示最新的100条数据
  setVisibleData(allData.slice(-ITEMS_PER_PAGE));
}, [allData]);

// 3. 使用Web Worker处理数据
// worker.js
self.onmessage = function(e) {
  const { data, operation } = e.data;
  
  if (operation === 'filter') {
    const filtered = data.filter(item => item.creditsUsed > 0);
    self.postMessage({ result: filtered });
  }
};
```

### 3. 网络优化

```go
// 1. 启用HTTP/2
app := fiber.New(fiber.Config{
    Network: fiber.NetworkTCP,
})

// 2. 设置合适的缓冲区大小
c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
    // 使用较大的缓冲区减少系统调用
    w = bufio.NewWriterSize(w, 8192)
    defer w.Flush()
    
    // ... 处理逻辑
})

// 3. 启用压缩（middleware级别）
app.Use(compress.New(compress.Config{
    Level: compress.LevelBestSpeed,
}))
```

---

## 监控和调试

### 1. 日志记录

```go
// 结构化日志
log.Printf("SSE连接建立: 客户端=%s, 参数=%v", 
    c.IP(), 
    map[string]interface{}{
        "hours": hours,
        "userAgent": c.Get("User-Agent"),
    },
)

// 性能监控
start := time.Now()
defer func() {
    log.Printf("SSE连接持续时间: %v", time.Since(start))
}()
```

### 2. 指标收集

```go
// 连接数统计
var (
    activeConnections int64
    totalMessages     int64
    errorCount       int64
)

func (h *SSEHandler) StreamUsageData(c *fiber.Ctx) error {
    atomic.AddInt64(&activeConnections, 1)
    defer atomic.AddInt64(&activeConnections, -1)
    
    // ... 实现逻辑
}

// 健康检查端点
func (h *SSEHandler) HealthCheck(c *fiber.Ctx) error {
    stats := map[string]interface{}{
        "activeConnections": atomic.LoadInt64(&activeConnections),
        "totalMessages":     atomic.LoadInt64(&totalMessages),
        "errorCount":        atomic.LoadInt64(&errorCount),
        "uptime":           time.Since(startTime),
    }
    
    return c.JSON(stats)
}
```

---

## 总结

本文档涵盖了 Fiber + SSE 实现的完整方案，关键要点包括：

1. **正确使用** `SetBodyStreamWriter` 进行流式响应
2. **避免** context 和参数访问错误 
3. **实现** 完善的监听器管理和资源清理机制
4. **处理** 各种边界情况和错误场景
5. **优化** 性能和用户体验

遵循这些最佳实践，可以构建稳定、高性能的实时数据推送系统。

---

**参考资料**：
- [Server-Sent Events MDN文档](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [Fiber框架官方文档](https://docs.gofiber.io/)
- [EventSource API文档](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)