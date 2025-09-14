package handlers

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/client"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
)

// ControlHandler 控制处理器
type ControlHandler struct {
	scheduler *services.SchedulerService
	db        *database.BadgerDB
}

// NewControlHandler 创建控制处理器
func NewControlHandler(scheduler *services.SchedulerService, db *database.BadgerDB) *ControlHandler {
	return &ControlHandler{
		scheduler: scheduler,
		db:        db,
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

// ResetCredits 重置积分
func (h *ControlHandler) ResetCredits(c *fiber.Ctx) error {
	// 获取当前配置
	config, err := h.db.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "获取配置失败", err))
	}

	// 检查Cookie是否配置
	if config.Cookie == "" {
		return c.Status(400).JSON(models.Error(400, "请先配置Cookie", nil))
	}

	// 调用积分重置API，通过状态码判断重置状态
	apiClient := client.NewClaudeAPIClient(config.Cookie)
	resetSuccess, resetInfo, err := apiClient.ResetCredits()
	if err != nil {
		log.Printf("调用重置积分API失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "重置积分失败", err))
	}

	if !resetSuccess {
		log.Printf("重置积分API返回失败")
		return c.Status(400).JSON(models.Error(400, "重置积分失败，请稍后重试", nil))
	}

	// API调用成功后，标记今日已使用重置
	config.DailyResetUsed = true

	// 保存配置
	if err := h.db.SaveConfig(config); err != nil {
		log.Printf("保存配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "保存配置失败", err))
	}

	log.Printf("积分重置成功，已标记今日已使用重置。重置信息: %s", resetInfo)

	// 通过调度器通知重置状态变化（SSE推送给前端）
	h.scheduler.NotifyResetStatusChange(true)

	// 触发数据刷新，获取最新的积分余额
	// 延迟2秒后查询，确保服务端处理完重置操作
	go func() {
		time.Sleep(2 * time.Second)
		if err := h.scheduler.FetchBalanceManually(); err != nil {
			log.Printf("重置后刷新积分余额失败: %v", err)
		}
	}()

	return c.JSON(models.SuccessMessage("积分重置成功"))
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
