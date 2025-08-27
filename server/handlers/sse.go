package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
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
	hours := c.QueryInt("hours", 1)
	if hours <= 0 {
		hours = 1
	}
	
	// 获取上下文，避免在goroutine中访问可能已释放的context
	ctx := c.Context()
	
	log.Printf("新的SSE连接，时间范围: %d小时", hours)

	// 使用Fiber的流式响应
	c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		// 立即发送连接确认
		fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
		w.Flush()
		log.Printf("已发送SSE连接确认")

		// 立即发送当前数据
		allData := h.scheduler.GetLatestData()
		filteredData := models.UsageDataList(allData).FilterByTimeRange(hours)
		log.Printf("发送初始数据: %d条记录 (过滤前: %d条)", len(filteredData), len(allData))
		
		if len(filteredData) > 0 {
			jsonData, err := json.Marshal(filteredData)
			if err != nil {
				log.Printf("序列化初始数据失败: %v", err)
				return
			}
			fmt.Fprintf(w, "event: usage\ndata: %s\n\n", jsonData)
			w.Flush()
			log.Printf("已发送初始数据")
		}

		// 添加数据监听器
		listener := h.scheduler.AddDataListener()
		defer func() {
			h.scheduler.RemoveDataListener(listener)
			log.Printf("SSE监听器已清理")
		}()
		
		log.Printf("SSE连接已建立监听器，缓冲区大小: %d", cap(listener))

		// 设置连接保活
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// 监听新数据和保活
		for {
			select {
			case data, ok := <-listener:
				if !ok {
					log.Println("SSE监听器通道已关闭")
					return // 监听器已关闭
				}

				log.Printf("SSE监听器收到数据: %d条记录", len(data))
				// 按时间范围过滤数据后发送
				filteredData := models.UsageDataList(data).FilterByTimeRange(hours)
				log.Printf("过滤后发送数据: %d条记录 (过滤前: %d条, 时间范围: %d小时)", len(filteredData), len(data), hours)
				
				if len(filteredData) > 0 {
					jsonData, err := json.Marshal(filteredData)
					if err != nil {
						log.Printf("序列化数据失败: %v", err)
						continue
					}
					fmt.Fprintf(w, "event: usage\ndata: %s\n\n", jsonData)
					if err := w.Flush(); err != nil {
						log.Printf("刷新数据到客户端失败: %v", err)
						return
					}
					log.Printf("已发送数据到客户端")
				}

			case <-ticker.C:
				// 发送心跳保活
				heartbeat := map[string]any{
					"type":      "heartbeat",
					"timestamp": time.Now().Format(time.RFC3339),
				}
				jsonData, err := json.Marshal(heartbeat)
				if err != nil {
					log.Printf("序列化心跳失败: %v", err)
					continue
				}
				fmt.Fprintf(w, "event: heartbeat\ndata: %s\n\n", jsonData)
				if err := w.Flush(); err != nil {
					log.Printf("发送心跳失败: %v", err)
					return
				}

			case <-ctx.Done():
				log.Println("SSE连接上下文已取消")
				return
			}
		}
	})

	return nil
}


// GetUsageData 获取历史数据
func (h *SSEHandler) GetUsageData(c *fiber.Ctx) error {
	// 获取时间范围参数
	hours := c.QueryInt("hours", 1)
	if hours <= 0 {
		hours = 1
	}

	// 从调度器获取最新数据并按时间范围过滤
	allData := h.scheduler.GetLatestData()
	filteredData := models.UsageDataList(allData).FilterByTimeRange(hours)
	log.Printf("API获取数据: %d条记录 (过滤前: %d条, 时间范围: %d小时)", len(filteredData), len(allData), hours)

	return c.JSON(models.Success(filteredData))
}