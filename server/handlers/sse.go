package handlers

import (
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

	// 获取查询参数
	hours := c.QueryInt("hours", 1)
	if hours <= 0 {
		hours = 1
	}

	log.Printf("新的SSE连接，时间范围: %d小时", hours)

	// 立即发送当前数据
	if err := h.sendCurrentData(c, hours); err != nil {
		log.Printf("发送当前数据失败: %v", err)
		return err
	}

	// 添加数据监听器
	listener := h.scheduler.AddDataListener()
	defer h.scheduler.RemoveDataListener(listener)

	// 设置连接保活
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 监听新数据和保活
	for {
		select {
		case _, ok := <-listener:
			if !ok {
				return nil // 监听器已关闭
			}

			// 获取完整的时间范围数据
			allData, err := h.db.GetUsageData(hours)
			if err != nil {
				log.Printf("获取数据失败: %v", err)
				continue
			}

			if err := h.sendData(c, allData); err != nil {
				log.Printf("发送数据失败: %v", err)
				return err
			}

		case <-ticker.C:
			// 发送心跳保活
			if err := h.sendHeartbeat(c); err != nil {
				log.Printf("发送心跳失败: %v", err)
				return err
			}

		case <-c.Context().Done():
			log.Println("SSE连接已断开")
			return nil
		}
	}
}

// sendCurrentData 发送当前数据
func (h *SSEHandler) sendCurrentData(c *fiber.Ctx, hours int) error {
	data, err := h.db.GetUsageData(hours)
	if err != nil {
		return fmt.Errorf("获取数据失败: %w", err)
	}

	return h.sendData(c, data)
}

// sendData 发送数据
func (h *SSEHandler) sendData(c *fiber.Ctx, data models.UsageDataList) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	message := fmt.Sprintf("event: usage\ndata: %s\n\n", jsonData)
	
	if _, err := c.Write([]byte(message)); err != nil {
		return fmt.Errorf("写入响应失败: %w", err)
	}

	// Fiber v2中返回即可

	return nil
}

// sendHeartbeat 发送心跳
func (h *SSEHandler) sendHeartbeat(c *fiber.Ctx) error {
	heartbeat := map[string]interface{}{
		"type":      "heartbeat",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(heartbeat)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("event: heartbeat\ndata: %s\n\n", jsonData)
	
	if _, err := c.Write([]byte(message)); err != nil {
		return err
	}

	return nil
}

// GetUsageData 获取历史数据
func (h *SSEHandler) GetUsageData(c *fiber.Ctx) error {
	// 获取查询参数
	hours := c.QueryInt("hours", 1)
	if hours <= 0 {
		hours = 1
	}

	data, err := h.db.GetUsageData(hours)
	if err != nil {
		log.Printf("获取数据失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "获取数据失败", err))
	}

	return c.JSON(models.Success(data))
}