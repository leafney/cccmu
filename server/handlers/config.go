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

	// 不返回敏感的Cookie信息到前端，只返回是否已设置
	safeConfig := &models.UserConfig{
		Cookie:    "", // 不返回实际Cookie值
		Interval:  config.Interval,
		TimeRange: config.TimeRange,
		Enabled:   config.Enabled,
	}

	// 如果Cookie不为空，设置一个标识
	if config.Cookie != "" {
		safeConfig.Cookie = "已设置"
	}

	return c.JSON(models.Success(safeConfig))
}

// UpdateConfig 更新配置
func (h *ConfigHandler) UpdateConfig(c *fiber.Ctx) error {
	var newConfig models.UserConfig
	if err := c.BodyParser(&newConfig); err != nil {
		return c.Status(400).JSON(models.Error(400, "请求参数错误", err))
	}

	// 验证配置
	if err := newConfig.Validate(); err != nil {
		return c.Status(400).JSON(models.Error(400, "配置验证失败", err))
	}

	// 获取当前配置
	currentConfig, err := h.db.GetConfig()
	if err != nil {
		log.Printf("获取当前配置失败: %v", err)
		currentConfig = models.GetDefaultConfig()
	}

	// 如果新配置的Cookie为空但不是"已设置"标识，保持原有Cookie
	if newConfig.Cookie == "" || newConfig.Cookie == "已设置" {
		newConfig.Cookie = currentConfig.Cookie
	}

	// 更新调度器配置
	if err := h.scheduler.UpdateConfig(&newConfig); err != nil {
		log.Printf("更新调度器配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "更新配置失败", err))
	}

	log.Printf("配置已更新: 间隔=%d秒, 时间范围=%d分钟, 启用=%v", 
		newConfig.Interval, newConfig.TimeRange, newConfig.Enabled)

	return c.JSON(models.SuccessMessage("配置更新成功"))
}

// ClearCookie 清除Cookie
func (h *ConfigHandler) ClearCookie(c *fiber.Ctx) error {
	// 清除数据库中的Cookie
	if err := h.db.ClearCookie(); err != nil {
		log.Printf("清除Cookie失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "清除Cookie失败", err))
	}

	// 更新调度器，停止当前任务
	if err := h.scheduler.Stop(); err != nil {
		log.Printf("停止调度器失败: %v", err)
	}

	log.Printf("Cookie已清除，监控任务已停止")
	return c.JSON(models.SuccessMessage("Cookie已清除"))
}