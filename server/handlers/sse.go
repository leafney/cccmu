package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/auth"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
)

// SSEHandler SSE处理器
type SSEHandler struct {
	db          *database.BadgerDB
	scheduler   *services.SchedulerService
	authManager *auth.Manager
}

// NewSSEHandler 创建SSE处理器
func NewSSEHandler(db *database.BadgerDB, scheduler *services.SchedulerService, authManager *auth.Manager) *SSEHandler {
	handler := &SSEHandler{
		db:          db,
		scheduler:   scheduler,
		authManager: authManager,
	}

	// 注册会话事件监听器
	authManager.AddSessionEventHandler(handler.handleSessionEvent)

	return handler
}

// handleSessionEvent 处理会话事件
func (h *SSEHandler) handleSessionEvent(event auth.SessionEvent) {
	// 这里可以实现更复杂的逻辑，比如通知特定的SSE连接
	// 目前主要用于日志记录
	switch event.Type {
	case auth.SessionEventDeleted:
		log.Printf("SSE: 会话被删除，相关连接将在下次心跳时断开: %s", event.SessionID[:8]+"...")
	case auth.SessionEventExpired:
		log.Printf("SSE: 会话已过期，相关连接将在下次心跳时断开: %s", event.SessionID[:8]+"...")
	}
}

// StreamUsageData SSE数据流端点
func (h *SSEHandler) StreamUsageData(c *fiber.Ctx) error {
	// 验证认证状态（由于已经通过中间件，这里再次检查以确保安全）
	sessionID := c.Cookies("cccmu_session")
	if _, valid := h.authManager.ValidateSession(sessionID); !valid {
		return c.Status(401).JSON(models.Error(401, "认证无效", nil))
	}

	// 设置SSE响应头
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "Cache-Control")

	// 获取查询参数（在流式响应外获取）
	minutes := c.QueryInt("minutes", 60)
	if minutes <= 0 {
		minutes = 60
	}

	// 获取上下文，避免在goroutine中访问可能已释放的context
	ctx := c.Context()

	// 使用Fiber的流式响应
	c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		// 立即发送连接确认
		fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
		w.Flush()

		// 立即发送当前数据
		allData := h.scheduler.GetLatestData()
		filteredData := models.UsageDataList(allData).FilterByTimeRange(minutes)

		if len(filteredData) > 0 {
			jsonData, err := json.Marshal(filteredData)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "event: usage\ndata: %s\n\n", jsonData)
			w.Flush()
		}

		// 立即发送当前积分余额
		balance := h.scheduler.GetLatestBalance()
		if balance != nil {
			jsonData, err := json.Marshal(balance)
			if err == nil {
				fmt.Fprintf(w, "event: balance\ndata: %s\n\n", jsonData)
				w.Flush()
			}
		}

		// 立即发送当前重置状态
		config, err := h.db.GetConfig()
		if err == nil {
			resetData := map[string]any{
				"type":      "reset_status",
				"resetUsed": config.DailyResetUsed,
				"timestamp": time.Now().Format(time.RFC3339),
			}
			jsonData, err := json.Marshal(resetData)
			if err == nil {
				fmt.Fprintf(w, "event: reset_status\ndata: %s\n\n", jsonData)
				w.Flush()
			}
		}

		// 立即发送当前监控状态和自动调度状态
		statusData := map[string]any{
			"type":                "monitoring_status",
			"isMonitoring":        h.scheduler.IsRunning(),
			"autoScheduleEnabled": h.scheduler.IsAutoScheduleEnabled(),
			"autoScheduleActive":  h.scheduler.IsInAutoScheduleTimeRange(),
			"timestamp":           time.Now().Format(time.RFC3339),
		}
		jsonData, err := json.Marshal(statusData)
		if err == nil {
			fmt.Fprintf(w, "event: monitoring_status\ndata: %s\n\n", jsonData)
			w.Flush()
		}

		// 添加数据监听器
		listener := h.scheduler.AddDataListener()
		balanceListener := h.scheduler.AddBalanceListener()
		errorListener := h.scheduler.AddErrorListener()
		resetStatusListener := h.scheduler.AddResetStatusListener()
		autoScheduleListener := h.scheduler.AddAutoScheduleListener()
		dailyUsageListener := h.scheduler.AddDailyUsageListener()
		defer func() {
			h.scheduler.RemoveDataListener(listener)
			h.scheduler.RemoveBalanceListener(balanceListener)
			h.scheduler.RemoveErrorListener(errorListener)
			h.scheduler.RemoveResetStatusListener(resetStatusListener)
			h.scheduler.RemoveAutoScheduleListener(autoScheduleListener)
			h.scheduler.RemoveDailyUsageListener(dailyUsageListener)
		}()

		// 设置连接保活
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// 监听新数据和保活
		for {
			select {
			case data, ok := <-listener:
				if !ok {
					return // 监听器已关闭
				}

				// 按时间范围过滤数据后发送
				filteredData := models.UsageDataList(data).FilterByTimeRange(minutes)

				if len(filteredData) > 0 {
					jsonData, err := json.Marshal(filteredData)
					if err != nil {
						continue
					}
					fmt.Fprintf(w, "event: usage\ndata: %s\n\n", jsonData)
					if err := w.Flush(); err != nil {
						return
					}
				}

			case balance, ok := <-balanceListener:
				if !ok {
					return // 监听器已关闭
				}

				// 发送积分余额数据
				jsonData, err := json.Marshal(balance)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: balance\ndata: %s\n\n", jsonData)
				if err := w.Flush(); err != nil {
					return
				}

			case errorMsg, ok := <-errorListener:
				if !ok {
					return // 监听器已关闭
				}

				// 发送错误信息
				errorData := map[string]any{
					"type":      "error",
					"message":   errorMsg,
					"timestamp": time.Now().Format(time.RFC3339),
				}
				jsonData, err := json.Marshal(errorData)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonData)
				if err := w.Flush(); err != nil {
					return
				}

			case resetStatus, ok := <-resetStatusListener:
				if !ok {
					return // 监听器已关闭
				}

				// 发送重置状态信息
				resetData := map[string]any{
					"type":      "reset_status",
					"resetUsed": resetStatus,
					"timestamp": time.Now().Format(time.RFC3339),
				}
				jsonData, err := json.Marshal(resetData)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: reset_status\ndata: %s\n\n", jsonData)
				if err := w.Flush(); err != nil {
					return
				}

			case <-autoScheduleListener:
				// 自动调度状态变化，发送完整的监控状态
				statusData := map[string]any{
					"type":                "monitoring_status",
					"isMonitoring":        h.scheduler.IsRunning(),
					"autoScheduleEnabled": h.scheduler.IsAutoScheduleEnabled(),
					"autoScheduleActive":  h.scheduler.IsInAutoScheduleTimeRange(),
					"timestamp":           time.Now().Format(time.RFC3339),
				}
				jsonData, err := json.Marshal(statusData)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: monitoring_status\ndata: %s\n\n", jsonData)
				if err := w.Flush(); err != nil {
					return
				}

			case dailyUsageData, ok := <-dailyUsageListener:
				if !ok {
					return // 监听器已关闭
				}

				// 发送每日积分统计数据
				jsonData, err := json.Marshal(dailyUsageData)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: daily_usage\ndata: %s\n\n", jsonData)
				if err := w.Flush(); err != nil {
					return
				}

			case <-ticker.C:
				// 检查认证状态
				if _, valid := h.authManager.ValidateSession(sessionID); !valid {
					// 发送认证过期事件
					authExpired := map[string]any{
						"type":      "auth_expired",
						"message":   "登录已过期",
						"timestamp": time.Now().Format(time.RFC3339),
					}
					jsonData, err := json.Marshal(authExpired)
					if err == nil {
						fmt.Fprintf(w, "event: auth_expired\ndata: %s\n\n", jsonData)
						w.Flush()
					}
					return // 关闭连接
				}

				// 发送心跳保活
				heartbeat := map[string]any{
					"type":      "heartbeat",
					"timestamp": time.Now().Format(time.RFC3339),
				}
				jsonData, err := json.Marshal(heartbeat)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: heartbeat\ndata: %s\n\n", jsonData)
				if err := w.Flush(); err != nil {
					return
				}

			case <-ctx.Done():
				return
			}
		}
	})

	return nil
}

// GetUsageData 获取历史数据
func (h *SSEHandler) GetUsageData(c *fiber.Ctx) error {
	// 获取时间范围参数
	minutes := c.QueryInt("minutes", 60)
	if minutes <= 0 {
		minutes = 60
	}

	// 从调度器获取最新数据并按时间范围过滤
	allData := h.scheduler.GetLatestData()
	filteredData := models.UsageDataList(allData).FilterByTimeRange(minutes)

	return c.JSON(models.Success(filteredData))
}
