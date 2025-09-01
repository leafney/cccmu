package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
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

// StreamUsageData SSE数据流端点
func (h *SSEHandler) StreamUsageData(c *fiber.Ctx) error {
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

		// 添加数据监听器
		listener := h.scheduler.AddDataListener()
		balanceListener := h.scheduler.AddBalanceListener()
		defer func() {
			h.scheduler.RemoveDataListener(listener)
			h.scheduler.RemoveBalanceListener(balanceListener)
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

			case <-ticker.C:
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