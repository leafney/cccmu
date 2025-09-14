package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
)

// ConfigHandler 配置处理器
type ConfigHandler struct {
	db        *database.BadgerDB
	scheduler *services.SchedulerService
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(db *database.BadgerDB, scheduler *services.SchedulerService) *ConfigHandler {
	return &ConfigHandler{
		db:        db,
		scheduler: scheduler,
	}
}

// GetConfig 获取配置
func (h *ConfigHandler) GetConfig(c *fiber.Ctx) error {
	config, err := h.db.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "获取配置失败", err))
	}

	// 转换为API响应格式，Cookie字段自动转为布尔值
	responseConfig := config.ToResponse()
	
	return c.JSON(models.Success(responseConfig))
}

// UpdateConfig 更新配置
func (h *ConfigHandler) UpdateConfig(c *fiber.Ctx) error {
	var requestConfig models.UserConfigRequest
	if err := c.BodyParser(&requestConfig); err != nil {
		return c.Status(400).JSON(models.Error(400, "请求参数错误", err))
	}

	// 获取当前配置
	currentConfig, err := h.db.GetConfig()
	if err != nil {
		log.Printf("获取当前配置失败: %v", err)
		currentConfig = models.GetDefaultConfig()
	}

	// 构建新的配置，保留内部字段
	newConfig := &models.UserConfig{
		Cookie:                  currentConfig.Cookie, // 默认保持原有Cookie
		Interval:                requestConfig.Interval,
		TimeRange:               requestConfig.TimeRange,
		Enabled:                 requestConfig.Enabled,
		LastCookieValidTime:     currentConfig.LastCookieValidTime,
		CookieValidationInterval: currentConfig.CookieValidationInterval,
		DailyResetUsed:          currentConfig.DailyResetUsed,
		AutoSchedule:            currentConfig.AutoSchedule, // 默认保持原有自动调度配置
	}

	// 如果请求中包含新的Cookie，则更新（使用指针判断是否设置了Cookie字段）
	if requestConfig.Cookie != nil {
		newConfig.Cookie = *requestConfig.Cookie
	}

	// 如果请求中包含自动调度配置，则更新
	if requestConfig.AutoSchedule != nil {
		oldAutoSchedule := currentConfig.AutoSchedule
		newConfig.AutoSchedule = *requestConfig.AutoSchedule
		
		log.Printf("[配置更新] 自动调度配置变更:")
		log.Printf("[配置更新] - 启用状态: %v -> %v", oldAutoSchedule.Enabled, newConfig.AutoSchedule.Enabled)
		if newConfig.AutoSchedule.Enabled {
			log.Printf("[配置更新] - 时间范围: %s-%s", newConfig.AutoSchedule.StartTime, newConfig.AutoSchedule.EndTime)
			log.Printf("[配置更新] - 范围内监控: %v", newConfig.AutoSchedule.MonitoringOn)
		}
		
		// 如果启用了自动调度，强制开启主监控开关
		if newConfig.AutoSchedule.Enabled {
			newConfig.Enabled = true
			log.Printf("[配置更新] 启用自动调度，强制开启监控开关")
		}
	}

	// 验证配置
	if err := newConfig.Validate(); err != nil {
		return c.Status(400).JSON(models.Error(400, "配置验证失败", err))
	}

	// 更新调度器配置
	if err := h.scheduler.UpdateConfig(newConfig); err != nil {
		log.Printf("更新调度器配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "更新配置失败", err))
	}

	log.Printf("[配置更新] 配置已更新完成:")
	log.Printf("[配置更新] - 间隔: %d秒", newConfig.Interval)
	log.Printf("[配置更新] - 时间范围: %d分钟", newConfig.TimeRange)
	log.Printf("[配置更新] - 监控启用: %v", newConfig.Enabled)
	log.Printf("[配置更新] - 自动调度: %v", newConfig.AutoSchedule.Enabled)

	// 通过SSE通知前端配置已更新
	log.Printf("[配置更新] 通知前端配置变更...")
	h.scheduler.NotifyConfigChange()
	
	// 通知自动调度状态变化
	log.Printf("[配置更新] 通知前端自动调度状态变更...")
	h.scheduler.NotifyAutoScheduleChange()

	return c.JSON(models.SuccessMessage("配置更新成功"))
}

// ClearCookie 清除Cookie
func (h *ConfigHandler) ClearCookie(c *fiber.Ctx) error {
	// 获取当前配置
	config, err := h.db.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "获取配置失败", err))
	}

	// 清除Cookie
	config.Cookie = ""

	// 保存更新的配置
	if err := h.db.SaveConfig(config); err != nil {
		log.Printf("保存清除后的配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "清除Cookie失败", err))
	}

	// 更新调度器，停止当前任务
	if err := h.scheduler.Stop(); err != nil {
		log.Printf("停止调度器失败: %v", err)
	}

	log.Printf("Cookie已清除，监控任务已停止")
	return c.JSON(models.SuccessMessage("Cookie已清除"))
}