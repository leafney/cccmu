package handlers

import (
	"log"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
)

// 版本信息变量，通过编译时注入
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// SetVersionInfo 设置版本信息（从main函数调用）
func SetVersionInfo(version, gitCommit, buildTime string) {
	Version = version
	GitCommit = gitCommit
	BuildTime = buildTime
}

// ConfigHandler 配置处理器
type ConfigHandler struct {
	db               *database.BadgerDB
	scheduler        *services.SchedulerService
	autoResetService *services.AutoResetService
	asyncUpdater     *services.AsyncConfigUpdater
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(db *database.BadgerDB, scheduler *services.SchedulerService, autoResetService *services.AutoResetService, asyncUpdater *services.AsyncConfigUpdater) *ConfigHandler {
	return &ConfigHandler{
		db:               db,
		scheduler:        scheduler,
		autoResetService: autoResetService,
		asyncUpdater:     asyncUpdater,
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

	// 添加版本信息
	responseConfig.Version = models.VersionInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
	}

	// 添加订阅等级信息，优先从BadgerDB获取持久化数据
	if balance, err := h.db.GetCreditBalance(); err == nil && balance != nil {
		responseConfig.Plan = balance.Plan
	} else if balance := h.scheduler.GetLatestBalance(); balance != nil {
		responseConfig.Plan = balance.Plan
	} else {
		responseConfig.Plan = ""
	}

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
		Cookie:                   currentConfig.Cookie, // 默认保持原有Cookie
		Interval:                 requestConfig.Interval,
		TimeRange:                requestConfig.TimeRange,
		Enabled:                  requestConfig.Enabled,
		LastCookieValidTime:      currentConfig.LastCookieValidTime,
		CookieValidationInterval: currentConfig.CookieValidationInterval,
		DailyResetUsed:           currentConfig.DailyResetUsed,
		DailyUsageEnabled:        currentConfig.DailyUsageEnabled,     // 默认保持原有每日统计配置
		SkipWhenNoConnections:    currentConfig.SkipWhenNoConnections, // 默认保持原有连接优化配置
		AutoSchedule:             currentConfig.AutoSchedule,          // 默认保持原有自动调度配置
		AutoReset:                currentConfig.AutoReset,             // 默认保持原有自动重置配置
	}

	// 如果请求中包含新的Cookie，则更新（使用指针判断是否设置了Cookie字段）
	if requestConfig.Cookie != nil {
		newConfig.Cookie = *requestConfig.Cookie
	}

	// 如果请求中包含每日积分统计配置，则更新
	if requestConfig.DailyUsageEnabled != nil {
		oldDailyUsageEnabled := currentConfig.DailyUsageEnabled
		newConfig.DailyUsageEnabled = *requestConfig.DailyUsageEnabled

		log.Printf("[配置更新] 每日积分统计配置变更: %v -> %v", oldDailyUsageEnabled, newConfig.DailyUsageEnabled)
	}

	// 如果请求中包含连接优化配置，则更新
	if requestConfig.SkipWhenNoConnections != nil {
		oldSkipWhenNoConnections := currentConfig.SkipWhenNoConnections
		newConfig.SkipWhenNoConnections = *requestConfig.SkipWhenNoConnections

		log.Printf("[配置更新] 无连接时跳过API请求配置变更: %v -> %v", oldSkipWhenNoConnections, newConfig.SkipWhenNoConnections)
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

	// 如果请求中包含自动重置配置，则更新
	if requestConfig.AutoReset != nil {
		oldAutoReset := currentConfig.AutoReset
		newConfig.AutoReset = *requestConfig.AutoReset

		log.Printf("[配置更新] 自动重置配置变更:")
		log.Printf("[配置更新] - 启用状态: %v -> %v", oldAutoReset.Enabled, newConfig.AutoReset.Enabled)
		if newConfig.AutoReset.Enabled && newConfig.AutoReset.ResetTime != "" {
			log.Printf("[配置更新] - 重置时间: %s", newConfig.AutoReset.ResetTime)
		}
	}

	// 验证配置
	if err := newConfig.Validate(); err != nil {
		return c.Status(400).JSON(models.Error(400, "配置验证失败", err))
	}

	// 先同步保存配置到数据库（快速操作）
	if err := h.scheduler.UpdateConfigSync(newConfig); err != nil {
		log.Printf("同步保存配置失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "保存配置失败", err))
	}

	// 异步提交重型操作任务
	if h.asyncUpdater != nil && h.asyncUpdater.IsRunning() {
		// 提交调度器更新任务
		if h.scheduler.NeedsTaskRestart(currentConfig, newConfig) {
			jobID, err := h.asyncUpdater.SubmitJob(services.JobTypeScheduler, currentConfig, newConfig)
			if err != nil {
				log.Printf("提交调度器异步更新任务失败: %v", err)
				// 降级到同步模式
				if err := h.scheduler.UpdateConfig(newConfig); err != nil {
					log.Printf("降级同步更新调度器配置失败: %v", err)
					return c.Status(500).JSON(models.Error(500, "更新配置失败", err))
				}
			} else {
				log.Printf("调度器异步更新任务已提交: %s", jobID)
			}
		}

		// 提交自动调度配置更新任务
		if requestConfig.AutoSchedule != nil {
			jobID, err := h.asyncUpdater.SubmitJob(services.JobTypeAutoSchedule, &currentConfig.AutoSchedule, &newConfig.AutoSchedule)
			if err != nil {
				log.Printf("提交自动调度异步更新任务失败: %v", err)
			} else {
				log.Printf("自动调度异步更新任务已提交: %s", jobID)
			}
		}

		// 提交自动重置配置更新任务
		if requestConfig.AutoReset != nil && h.autoResetService != nil {
			jobID, err := h.asyncUpdater.SubmitJob(services.JobTypeAutoReset, &currentConfig.AutoReset, &newConfig.AutoReset)
			if err != nil {
				log.Printf("提交自动重置异步更新任务失败: %v", err)
				// 降级到同步模式
				if err := h.autoResetService.UpdateConfig(&newConfig.AutoReset); err != nil {
					log.Printf("降级同步更新自动重置配置失败: %v", err)
					return c.Status(500).JSON(models.Error(500, "更新自动重置配置失败", err))
				}
			} else {
				log.Printf("自动重置异步更新任务已提交: %s", jobID)
			}
		}
	} else {
		// 异步服务不可用，降级到同步模式
		log.Printf("异步配置更新服务不可用，降级到同步模式")

		// 更新调度器配置
		if err := h.scheduler.UpdateConfig(newConfig); err != nil {
			log.Printf("更新调度器配置失败: %v", err)
			return c.Status(500).JSON(models.Error(500, "更新配置失败", err))
		}

		// 更新自动重置服务配置
		if h.autoResetService != nil {
			if err := h.autoResetService.UpdateConfig(&newConfig.AutoReset); err != nil {
				log.Printf("更新自动重置服务配置失败: %v", err)
				return c.Status(500).JSON(models.Error(500, "更新自动重置配置失败", err))
			}
		}
	}

	log.Printf("[配置更新] 配置已更新完成:")
	log.Printf("[配置更新] - 间隔: %d秒", newConfig.Interval)
	log.Printf("[配置更新] - 时间范围: %d分钟", newConfig.TimeRange)
	log.Printf("[配置更新] - 监控启用: %v", newConfig.Enabled)
	log.Printf("[配置更新] - 每日积分统计: %v", newConfig.DailyUsageEnabled)
	log.Printf("[配置更新] - 自动调度: %v", newConfig.AutoSchedule.Enabled)
	log.Printf("[配置更新] - 自动重置: %v", newConfig.AutoReset.Enabled)

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
