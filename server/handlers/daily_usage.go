package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/auth"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
)

// DailyUsageHandler 每日积分使用统计处理器
type DailyUsageHandler struct {
	scheduler   *services.SchedulerService
	authManager *auth.Manager
}

// NewDailyUsageHandler 创建每日积分统计处理器
func NewDailyUsageHandler(scheduler *services.SchedulerService, authManager *auth.Manager) *DailyUsageHandler {
	return &DailyUsageHandler{
		scheduler:   scheduler,
		authManager: authManager,
	}
}

// GetWeeklyUsage 触发积分历史统计数据获取（通过SSE推送）
func (h *DailyUsageHandler) GetWeeklyUsage(c *fiber.Ctx) error {
	// 验证认证状态
	sessionID := c.Cookies("cccmu_session")
	if _, valid := h.authManager.ValidateSession(sessionID); !valid {
		return c.Status(401).JSON(models.Error(401, "认证无效", nil))
	}

	// 获取数据并通过SSE推送
	go func() {
		// 获取一周数据，如果失败则返回7天0数据
		weeklyUsage, err := h.scheduler.GetWeeklyUsage()
		if err != nil || len(weeklyUsage) == 0 {
			// 生成7天的0数据
			weekDates := models.GetWeekDates()
			weeklyUsage = make([]models.DailyUsage, len(weekDates))
			for i, date := range weekDates {
				weeklyUsage[i] = models.DailyUsage{
					Date:         date,
					TotalCredits: 0,
				}
			}
		} else {
			// 填充缺失日期
			weeklyUsageList := models.DailyUsageList(weeklyUsage)
			weeklyUsage = weeklyUsageList.FillMissingDates()
		}

		// 推送数据
		h.scheduler.BroadcastDailyUsage(weeklyUsage)
	}()

	// 立即返回成功响应
	return c.JSON(models.Success("ok"))
}
