package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
)

// ControlHandler 控制处理器
type ControlHandler struct {
	scheduler *services.SchedulerService
}

// NewControlHandler 创建控制处理器
func NewControlHandler(scheduler *services.SchedulerService) *ControlHandler {
	return &ControlHandler{
		scheduler: scheduler,
	}
}

// StartTask 启动任务
func (h *ControlHandler) StartTask(c *fiber.Ctx) error {
	if err := h.scheduler.Start(); err != nil {
		log.Printf("启动任务失败: %v", err)
		return c.Status(400).JSON(models.Error(400, "启动任务失败", err))
	}

	log.Println("定时任务已启动")
	return c.JSON(models.SuccessMessage("任务启动成功"))
}

// StopTask 停止任务
func (h *ControlHandler) StopTask(c *fiber.Ctx) error {
	if err := h.scheduler.Stop(); err != nil {
		log.Printf("停止任务失败: %v", err)
		return c.Status(400).JSON(models.Error(400, "停止任务失败", err))
	}

	log.Println("定时任务已停止")
	return c.JSON(models.SuccessMessage("任务停止成功"))
}

// GetTaskStatus 获取任务状态
func (h *ControlHandler) GetTaskStatus(c *fiber.Ctx) error {
	status := map[string]interface{}{
		"running": h.scheduler.IsRunning(),
	}
	
	return c.JSON(models.Success(status))
}


// GetCreditBalance 获取积分余额
func (h *ControlHandler) GetCreditBalance(c *fiber.Ctx) error {
	balance := h.scheduler.GetLatestBalance()
	
	return c.JSON(models.Success(balance))
}


// RefreshAll 手动刷新所有数据（使用数据 + 积分余额）
func (h *ControlHandler) RefreshAll(c *fiber.Ctx) error {
	if err := h.scheduler.FetchAllDataManually(); err != nil {
		log.Printf("手动刷新所有数据失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "刷新数据失败", err))
	}

	log.Println("所有数据已手动刷新")
	return c.JSON(models.SuccessMessage("数据刷新成功"))
}