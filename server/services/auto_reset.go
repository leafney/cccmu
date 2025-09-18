package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/leafney/cccmu/server/client"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/utils"
)

// AutoResetService 自动重置服务
type AutoResetService struct {
	scheduler          gocron.Scheduler        // 时间任务调度器
	resetJob           gocron.Job              // 重置任务
	config             *models.AutoResetConfig // 当前配置
	db                 *database.BadgerDB      // 数据库访问
	schedulerSvc       *SchedulerService       // 调度器服务（用于通知和重置）
	mu                 sync.RWMutex            // 并发保护
	tasksCreated       bool                    // 标记任务是否已创建
	tasksRunning       bool                    // 标记任务是否正在运行
	thresholdScheduler gocron.Scheduler        // 阈值检查专用调度器
	thresholdJob       gocron.Job              // 阈值检查任务
	thresholdRunning   bool                    // 阈值任务运行状态
	apiClient          *client.ClaudeAPIClient // API客户端实例

	// 动态时间范围管理
	thresholdTimerJob gocron.Job // 时间范围管理任务
	thresholdActive   bool       // 当前是否在阈值检查时间范围内
}

// NewAutoResetService 创建自动重置服务
func NewAutoResetService(db *database.BadgerDB, schedulerSvc *SchedulerService) *AutoResetService {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		utils.Logf("[自动重置] 创建调度器失败: %v", err)
		return nil
	}

	// 创建阈值检查专用调度器
	thresholdScheduler, err := gocron.NewScheduler()
	if err != nil {
		utils.Logf("[自动重置] 创建阈值调度器失败: %v", err)
		return nil
	}

	return &AutoResetService{
		scheduler:          scheduler,
		thresholdScheduler: thresholdScheduler,
		db:                 db,
		schedulerSvc:       schedulerSvc,
		tasksCreated:       false,
		tasksRunning:       false,
		thresholdRunning:   false,
	}
}

// UpdateConfig 更新自动重置配置
func (s *AutoResetService) UpdateConfig(config *models.AutoResetConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldConfig := s.config
	s.config = config

	utils.Logf("[自动重置] 配置更新:")
	utils.Logf("[自动重置] - 启用状态: %v", config.Enabled)
	utils.Logf("[自动重置] - 时间触发条件: %v", config.TimeEnabled)
	utils.Logf("[自动重置] - 阈值触发条件: %v", config.ThresholdEnabled)

	if config.Enabled && config.TimeEnabled && config.ResetTime != "" {
		utils.Logf("[自动重置] - 重置时间: %s", config.ResetTime)
	}

	if config.Enabled && config.ThresholdEnabled {
		utils.Logf("[自动重置] - 积分阈值: %d", config.Threshold)
		if config.ThresholdTimeEnabled && config.ThresholdStartTime != "" && config.ThresholdEndTime != "" {
			utils.Logf("[自动重置] - 阈值检查时间: %s-%s", config.ThresholdStartTime, config.ThresholdEndTime)
		}
	}

	// 判断启用状态是否变化
	enabledChanged := (oldConfig == nil && config.Enabled) ||
		(oldConfig != nil && oldConfig.Enabled != config.Enabled)

	// 判断时间配置是否变化
	timeConfigChanged := oldConfig != nil && (oldConfig.TimeEnabled != config.TimeEnabled || oldConfig.ResetTime != config.ResetTime)

	// 判断阈值触发配置是否变化
	thresholdConfigChanged := oldConfig != nil && (oldConfig.ThresholdEnabled != config.ThresholdEnabled ||
		oldConfig.Threshold != config.Threshold ||
		oldConfig.ThresholdTimeEnabled != config.ThresholdTimeEnabled ||
		oldConfig.ThresholdStartTime != config.ThresholdStartTime ||
		oldConfig.ThresholdEndTime != config.ThresholdEndTime)

	// 处理阈值触发任务
	if config.Enabled && config.ThresholdEnabled {
		if !s.thresholdRunning || thresholdConfigChanged {
			utils.Logf("[自动重置] 启动/重启阈值触发任务")
			if s.thresholdRunning {
				s.stopThresholdTask()
			}
			if err := s.startThresholdTask(); err != nil {
				utils.Logf("[自动重置] 启动阈值触发任务失败: %v", err)
				return err
			}
		}
	} else {
		if s.thresholdRunning {
			utils.Logf("[自动重置] 停止阈值触发任务")
			s.stopThresholdTask()
		}
	}

	// 处理时间触发任务（保持原有逻辑）
	if timeConfigChanged {
		// 时间配置变化：必须重建任务
		utils.Logf("[自动重置] 检测到时间配置变化，重建任务")
		s.rebuildTasks(config)
	} else if enabledChanged {
		// 只是启用状态变化：控制任务启停
		if config.Enabled {
			utils.Logf("[自动重置] 启用自动重置")
			s.startTasks(config)
		} else {
			utils.Logf("[自动重置] 禁用自动重置")
			s.stopTasks()
		}
	} else {
		utils.Logf("[自动重置] 配置无实质性变化，保持当前状态")
	}

	return nil
}

// Start 启动自动重置服务
func (s *AutoResetService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从数据库加载配置
	config, err := s.db.GetConfig()
	if err != nil {
		log.Printf("[自动重置] 加载配置失败: %v", err)
		return err
	}

	s.config = &config.AutoReset

	if s.config.Enabled {
		log.Printf("[自动重置] 启动时自动重置已启用，开始初始化")
		s.startTasks(s.config)
	} else {
		log.Printf("[自动重置] 启动时自动重置未启用")
	}

	return nil
}

// Stop 停止自动重置服务
func (s *AutoResetService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopTasks()

	// 停止阈值任务
	if s.thresholdRunning {
		s.stopThresholdTaskInternal()
	}

	// 关闭调度器
	if s.scheduler != nil {
		utils.Logf("[自动重置] 关闭调度器")
		s.scheduler.Shutdown()
	}

	// 关闭阈值调度器
	if s.thresholdScheduler != nil {
		utils.Logf("[自动重置] 关闭阈值调度器")
		s.thresholdScheduler.Shutdown()
	}

	return nil
}

// IsEnabled 检查是否启用了自动重置
func (s *AutoResetService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config != nil && s.config.Enabled
}

// GetConfig 获取当前自动重置配置
func (s *AutoResetService) GetConfig() *models.AutoResetConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil {
		return &models.AutoResetConfig{}
	}
	return s.config
}

// IsThresholdTaskRunning 检查阈值触发任务是否正在运行
func (s *AutoResetService) IsThresholdTaskRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.thresholdRunning
}

// generateCronExpression 根据时间字符串生成cron表达式
// timeStr格式: "HH:MM" (如 "18:30")
// 返回格式: "MM HH * * *" (分 时 日 月 星期)
func (s *AutoResetService) generateCronExpression(timeStr string) (string, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("时间格式错误，应为 HH:MM 格式")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return "", fmt.Errorf("小时格式错误: %s", parts[0])
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return "", fmt.Errorf("分钟格式错误: %s", parts[1])
	}

	// gocron v2使用标准5字段格式: "分 时 日 月 星期"
	return fmt.Sprintf("%d %d * * *", minute, hour), nil
}

// isAlreadyReset 检查今日是否已重置过（复用现有的DailyResetUsed字段）
func (s *AutoResetService) isAlreadyReset() bool {
	config, err := s.db.GetConfig()
	if err != nil {
		log.Printf("[自动重置] 获取配置失败: %v", err)
		return true // 获取失败时跳过重置
	}
	return config.DailyResetUsed
}

// createTimeJob 创建时间触发任务
func (s *AutoResetService) createTimeJob() error {
	if s.config == nil || !s.config.TimeEnabled || s.config.ResetTime == "" {
		return fmt.Errorf("时间触发条件未启用或重置时间未配置")
	}

	log.Printf("[自动重置] 创建时间触发任务: %s", s.config.ResetTime)

	cronExpr, err := s.generateCronExpression(s.config.ResetTime)
	if err != nil {
		return fmt.Errorf("生成cron表达式失败: %w", err)
	}

	log.Printf("[自动重置] Cron表达式: %s -> %s", s.config.ResetTime, cronExpr)

	job, err := s.scheduler.NewJob(
		gocron.CronJob(cronExpr, false),
		gocron.NewTask(s.handleTimeResetTask),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建时间任务失败: %w", err)
	}

	s.resetJob = job
	s.tasksCreated = true

	log.Printf("[自动重置] ✅ 时间触发任务创建成功, ID: %v", job.ID())
	return nil
}

// removeTimeJob 删除时间触发任务
func (s *AutoResetService) removeTimeJob() {
	if !s.tasksCreated {
		log.Printf("[自动重置] 无任务需要删除")
		return
	}

	if s.resetJob != nil {
		log.Printf("[自动重置] 删除时间任务 (ID: %v)", s.resetJob.ID())
		if err := s.scheduler.RemoveJob(s.resetJob.ID()); err != nil {
			log.Printf("[自动重置] ❌ 删除时间任务失败: %v", err)
		} else {
			log.Printf("[自动重置] ✅ 时间任务删除成功")
		}
		s.resetJob = nil
	}

	s.tasksCreated = false
}

// startTasksInternal 启动任务（内部方法，无锁）
func (s *AutoResetService) startTasksInternal() error {
	if !s.tasksCreated {
		return fmt.Errorf("任务未创建")
	}

	if s.tasksRunning {
		log.Printf("[自动重置] 任务已在运行中")
		return nil
	}

	log.Printf("[自动重置] 启动调度器...")
	s.scheduler.Start()
	s.tasksRunning = true

	log.Printf("[自动重置] ✅ 定时任务启动完成")
	return nil
}

// stopTasksInternal 停止任务（内部方法，无锁）
func (s *AutoResetService) stopTasksInternal() {
	if !s.tasksRunning {
		log.Printf("[自动重置] 任务已经停止")
		return
	}

	log.Printf("[自动重置] 停止调度器...")
	s.scheduler.StopJobs()
	s.tasksRunning = false

	log.Printf("[自动重置] ✅ 定时任务停止完成")
}

// handleTimeResetTask 处理时间触发的重置任务
func (s *AutoResetService) handleTimeResetTask() {
	// 检查服务是否正在关闭
	if !s.tasksRunning {
		log.Printf("[自动重置] ⚠️  时间任务触发但服务正在关闭，跳过执行")
		return
	}

	now := time.Now()
	log.Printf("[自动重置] 🚀 时间触发任务执行!")
	log.Printf("[自动重置]   ⏰ 触发时间: %s", now.Format("2006-01-02 15:04:05"))
	log.Printf("[自动重置]   📋 配置时间: %s", s.config.ResetTime)

	// 检查今日是否已重置（手动或自动）
	if s.isAlreadyReset() {
		log.Printf("[自动重置]   ⚠️  今日已重置过，跳过时间触发的自动重置")
		log.Printf("[自动重置]   📋 每日重置限制: 最多执行一次")
		return
	}

	s.executeAutoReset("time_trigger")
}

// handleThresholdCheckTask 处理阈值检查任务（仅在时间范围内执行的版本）
func (s *AutoResetService) handleThresholdCheckTask() {
	now := time.Now()
	utils.Logf("[阈值触发] 🔍 执行阈值检查任务")
	utils.Logf("[阈值触发]   ⏰ 检查时间: %s", now.Format("2006-01-02 15:04:05"))

	// 检查今日是否已重置
	if s.isAlreadyReset() {
		utils.Logf("[阈值触发] ✅ 今日已重置，任务目标达成，提前结束阈值检查")
		s.deactivateThresholdCheck()
		return
	}

	// 获取积分余额（使用现有缓存逻辑）
	balance, err := s.apiClient.FetchCreditBalance()
	if err != nil {
		utils.Logf("[阈值触发]   ❌ 获取积分余额失败: %v", err)
		return
	}

	utils.Logf("[阈值触发]   💰 当前积分余额: %d", balance.Remaining)
	utils.Logf("[阈值触发]   🎯 设定阈值: %d", s.config.Threshold)

	// 通过SchedulerService推送积分到前端（SSE）
	s.schedulerSvc.NotifyBalanceUpdate(balance)
	utils.Logf("[阈值触发]   📡 已推送积分余额到前端")

	// 判断是否低于阈值
	if balance.Remaining > s.config.Threshold {
		utils.Logf("[阈值触发]   ✅ 积分余额充足，无需重置 (%d > %d)", balance.Remaining, s.config.Threshold)
		return
	}

	utils.Logf("[阈值触发]   🚨 积分余额低于阈值 (%d <= %d)，准备触发重置", balance.Remaining, s.config.Threshold)
	s.executeAutoReset("threshold_trigger")
}

// createThresholdJob 创建阈值检查任务
func (s *AutoResetService) createThresholdJob() error {
	utils.Logf("[阈值触发] 🔨 创建阈值检查任务")
	utils.Logf("[阈值触发]   📋 检查间隔: 30秒")
	utils.Logf("[阈值触发]   🎯 触发阈值: %d", s.config.Threshold)

	job, err := s.thresholdScheduler.NewJob(
		gocron.DurationJob(30*time.Second),
		gocron.NewTask(s.handleThresholdCheckTask),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		utils.Logf("[阈值触发] ❌ 创建阈值检查任务失败: %v", err)
		return err
	}

	s.thresholdJob = job
	utils.Logf("[阈值触发] ✅ 阈值检查任务创建成功, ID: %v", job.ID())
	return nil
}

// startThresholdTask 启动阈值检查任务
func (s *AutoResetService) startThresholdTask() error {
	utils.Logf("[阈值触发] 🚀 启动阈值检查任务")
	utils.Logf("[阈值触发]   🎯 阈值设置: %d", s.config.Threshold)

	// 获取API客户端实例
	config, err := s.db.GetConfig()
	if err != nil {
		utils.Logf("[阈值触发] ❌ 获取配置失败: %v", err)
		return err
	}
	s.apiClient = client.NewClaudeAPIClient(config.Cookie)

	if s.config.ThresholdTimeEnabled {
		utils.Logf("[阈值触发]   📅 时间范围: %s-%s", s.config.ThresholdStartTime, s.config.ThresholdEndTime)
		// 启动时间范围管理任务
		if err := s.startTimeRangeManager(); err != nil {
			return err
		}
	} else {
		utils.Logf("[阈值触发]   📅 时间范围: 全天检查")
		// 全天检查，直接启动阈值检查任务
		if err := s.activateThresholdCheck(); err != nil {
			return err
		}
	}

	// 启动调度器
	s.thresholdScheduler.Start()
	s.thresholdRunning = true

	utils.Logf("[阈值触发] ✅ 阈值检查任务已启动")
	return nil
}

// stopThresholdTask 停止阈值检查任务（采用彻底清理策略）
func (s *AutoResetService) stopThresholdTask() {
	utils.Logf("[阈值触发] 🔴 停止阈值检查任务 (彻底清理所有相关任务)")

	if s.thresholdRunning {
		// 停用阈值检查任务
		if s.thresholdActive {
			s.removeThresholdCheckTask()
		}

		// 完全停止并关闭调度器，确保所有任务都被清理
		utils.Logf("[阈值触发] 🗑️  完全清理阈值调度器")
		s.thresholdScheduler.StopJobs()
		if err := s.thresholdScheduler.Shutdown(); err != nil {
			utils.Logf("[阈值触发] ❌ 关闭阈值调度器失败: %v", err)
		}

		// 重新创建调度器以确保完全清理
		newScheduler, err := gocron.NewScheduler()
		if err != nil {
			utils.Logf("[阈值触发] ❌ 重新创建阈值调度器失败: %v", err)
		} else {
			s.thresholdScheduler = newScheduler
			utils.Logf("[阈值触发] ✅ 阈值调度器已重新创建")
		}

		// 清理所有任务引用
		s.thresholdJob = nil
		s.thresholdTimerJob = nil
		s.thresholdRunning = false
		s.thresholdActive = false

		utils.Logf("[阈值触发] ⏹️  阈值检查任务已完全停止")
	}

	utils.Logf("[阈值触发] ✅ 阈值检查任务已停止")
}

// stopThresholdTaskInternal 停止阈值检查任务（内部方法，无任务协调）
func (s *AutoResetService) stopThresholdTaskInternal() {
	if s.thresholdRunning {
		s.thresholdScheduler.StopJobs()
		s.thresholdRunning = false
		s.thresholdActive = false
		utils.Logf("[阈值触发] ⏹️  阈值检查任务已停止 (内部)")
	}
}

// startTimeRangeManager 启动时间范围管理任务
func (s *AutoResetService) startTimeRangeManager() error {
	utils.Logf("[阈值触发] 🕐 启动时间范围管理器")

	// 创建每分钟检查的定时任务来管理时间范围
	job, err := s.thresholdScheduler.NewJob(
		gocron.DurationJob(1*time.Minute),
		gocron.NewTask(s.manageTimeRange),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		utils.Logf("[阈值触发] ❌ 创建时间范围管理任务失败: %v", err)
		return err
	}

	s.thresholdTimerJob = job
	utils.Logf("[阈值触发] ✅ 时间范围管理任务创建成功, ID: %v", job.ID())

	// 立即检查当前时间是否在范围内
	s.manageTimeRange()

	return nil
}

// manageTimeRange 管理时间范围，动态启动和停止阈值检查
func (s *AutoResetService) manageTimeRange() {
	// 检查阈值任务是否仍在运行，如果已停用则直接返回
	if !s.thresholdRunning {
		utils.Logf("[阈值触发] ⚠️  阈值任务已停用，跳过时间范围检查")
		return
	}

	now := time.Now()
	inRange := s.config.IsInThresholdTimeRange(now)

	utils.Logf("[阈值触发] 🕐 检查时间范围状态")
	utils.Logf("[阈值触发]   ⏰ 当前时间: %s", now.Format("15:04:05"))
	utils.Logf("[阈值触发]   📅 检查范围: %s-%s", s.config.ThresholdStartTime, s.config.ThresholdEndTime)
	utils.Logf("[阈值触发]   🔍 当前活跃: %v", s.thresholdActive)
	utils.Logf("[阈值触发]   🎯 在范围内: %v", inRange)

	if inRange && !s.thresholdActive {
		// 进入时间范围，启动阈值检查
		utils.Logf("[阈值触发] 🟢 进入检查时间范围，启动阈值检查任务")
		if err := s.activateThresholdCheck(); err != nil {
			utils.Logf("[阈值触发] ❌ 启动阈值检查失败: %v", err)
		}
	} else if !inRange && s.thresholdActive {
		// 离开时间范围，停止阈值检查
		utils.Logf("[阈值触发] 🔴 离开检查时间范围，停止阈值检查任务")
		s.deactivateThresholdCheck()
	} else if inRange && s.thresholdActive {
		utils.Logf("[阈值触发] 🟢 仍在检查时间范围内，继续阈值检查")
	} else {
		utils.Logf("[阈值触发] ⏸️  仍在检查时间范围外，保持等待状态")
	}
}

// activateThresholdCheck 激活阈值检查任务
func (s *AutoResetService) activateThresholdCheck() error {
	if s.thresholdActive {
		utils.Logf("[阈值触发] ⚠️  阈值检查已经激活，跳过")
		return nil
	}

	// 检查今日是否已重置，如果已重置则无需创建检查任务
	if s.isAlreadyReset() {
		utils.Logf("[阈值触发] ⚠️  今日已重置过，跳过阈值检查任务创建")
		utils.Logf("[阈值触发] 📋 任务目标已达成，无需继续检查")
		return nil
	}

	utils.Logf("[阈值触发] 🔨 创建30秒阈值检查任务")

	// 创建30秒定时检查任务
	if err := s.createThresholdJob(); err != nil {
		return err
	}

	// 启动阈值检查时暂停SchedulerService积分获取任务（整个检查期间）
	utils.Logf("[阈值触发] ⏸️  暂停SchedulerService积分获取任务 (整个检查期间)")
	s.schedulerSvc.PauseBalanceTask()

	s.thresholdActive = true
	utils.Logf("[阈值触发] ✅ 阈值检查任务已激活")

	return nil
}

// removeThresholdCheckTask 移除阈值检查任务（内部方法）
func (s *AutoResetService) removeThresholdCheckTask() {
	if !s.thresholdActive {
		utils.Logf("[阈值触发] ⚠️  阈值检查已经停用，跳过")
		return
	}

	utils.Logf("[阈值触发] 🗑️  移除阈值检查任务")

	// 移除阈值检查任务
	if s.thresholdJob != nil {
		if err := s.thresholdScheduler.RemoveJob(s.thresholdJob.ID()); err != nil {
			utils.Logf("[阈值触发] ❌ 移除阈值检查任务失败: %v", err)
		} else {
			utils.Logf("[阈值触发] ✅ 阈值检查任务已移除")
		}
		s.thresholdJob = nil
	}

	// 移除时间范围管理任务
	if s.thresholdTimerJob != nil {
		if err := s.thresholdScheduler.RemoveJob(s.thresholdTimerJob.ID()); err != nil {
			utils.Logf("[阈值触发] ❌ 移除时间范围管理任务失败: %v", err)
		} else {
			utils.Logf("[阈值触发] ✅ 时间范围管理任务已移除")
		}
		s.thresholdTimerJob = nil
	}

	// 恢复SchedulerService积分获取任务（采用重建策略）
	utils.Logf("[阈值触发] ▶️  恢复SchedulerService积分获取任务 (阈值检查已结束)")
	s.schedulerSvc.RebuildBalanceTask()

	s.thresholdActive = false
	utils.Logf("[阈值触发] ⏹️  阈值检查任务已停用")
}

// deactivateThresholdCheck 停用阈值检查任务（保持兼容性）
func (s *AutoResetService) deactivateThresholdCheck() {
	s.removeThresholdCheckTask()
}

// executeAutoReset 执行自动重置
func (s *AutoResetService) executeAutoReset(trigger string) {
	utils.Logf("[自动重置] 🚀 开始执行自动重置")
	utils.Logf("[自动重置]   🔖 触发原因: %s", trigger)
	utils.Logf("[自动重置]   ⏰ 执行时间: %s", time.Now().Format("2006-01-02 15:04:05"))

	// 检查是否已重置（每日限制）
	if s.isAlreadyReset() {
		utils.Logf("[自动重置]   ⚠️  今日已重置过，跳过执行")
		utils.Logf("[自动重置]   📋 每日重置限制: 最多执行一次")
		return
	}

	utils.Logf("[自动重置]   ✅ 今日未重置，继续执行重置操作")

	// 调用现有的重置积分API
	success := s.callExistingResetAPI()
	if success {
		utils.Logf("[自动重置] ✅ 自动重置执行成功")

		// 如果是阈值触发，延迟获取最新积分确认重置效果
		if trigger == "threshold_trigger" {
			go func() {
				time.Sleep(10 * time.Second)
				utils.Logf("[阈值触发] 🔄 重置后验证积分余额...")
				if balance, err := s.apiClient.FetchCreditBalance(); err == nil {
					utils.Logf("[阈值触发] ✅ 重置后积分余额: %d", balance.Remaining)
					utils.Logf("[阈值触发] 📊 阈值对比: %d > %d (阈值)", balance.Remaining, s.config.Threshold)
					s.schedulerSvc.NotifyBalanceUpdate(balance)
				} else {
					utils.Logf("[阈值触发] ❌ 重置后获取积分余额失败: %v", err)
				}
			}()
		}
	} else {
		utils.Logf("[自动重置] ❌ 自动重置执行失败")
	}
}

// callExistingResetAPI 调用现有的重置积分API逻辑
func (s *AutoResetService) callExistingResetAPI() bool {
	// 获取当前配置
	config, err := s.db.GetConfig()
	if err != nil {
		utils.Logf("[自动重置] 获取配置失败: %v", err)
		return false
	}

	// 检查Cookie是否配置
	if config.Cookie == "" {
		utils.Logf("[自动重置] Cookie未配置，跳过重置")
		return false
	}

	// 通过调度器服务的重置功能来执行重置
	// 自动重置功能独立于监控功能，不需要检查监控状态
	// 这会复用现有的重置逻辑，包括API调用、状态更新和SSE通知

	// 调用真实的重置API
	err = s.schedulerSvc.ResetCreditsManually()
	if err != nil {
		utils.Logf("[自动重置] 调用重置API失败: %v", err)
		return false
	}

	return true
}

// rebuildTasks 重建任务（时间配置变化时使用）
func (s *AutoResetService) rebuildTasks(config *models.AutoResetConfig) {
	log.Printf("[自动重置] 🔄 开始重建任务 (时间配置变化)")
	log.Printf("[自动重置]   📋 新配置: %s", config.ResetTime)

	// 删除旧任务
	log.Printf("[自动重置]   🗑️  删除旧任务...")
	s.removeTimeJob()

	// 创建新任务
	log.Printf("[自动重置]   🔨 创建新任务...")
	if err := s.createTimeJob(); err != nil {
		log.Printf("[自动重置]   ❌ 创建新任务失败: %v", err)
		return
	}

	// 根据启用状态决定是否启动
	if config.Enabled {
		log.Printf("[自动重置]   🚀 启动新任务...")
		if err := s.startTasksInternal(); err != nil {
			log.Printf("[自动重置]   ❌ 启动新任务失败: %v", err)
			return
		}
		log.Printf("[自动重置] ✅ 任务重建完成并启动")
	} else {
		log.Printf("[自动重置] ✅ 任务重建完成 (未启动，因为自动重置被禁用)")
	}
}

// startTasks 启动任务（启用状态变化时使用）
func (s *AutoResetService) startTasks(_ *models.AutoResetConfig) {
	log.Printf("[自动重置] 🟢 启动自动重置任务")

	if !s.tasksCreated {
		log.Printf("[自动重置]   🔨 首次启用: 需要创建任务")
		if s.config != nil && s.config.TimeEnabled {
			if err := s.createTimeJob(); err != nil {
				log.Printf("[自动重置]   ❌ 创建任务失败: %v", err)
				return
			}
			log.Printf("[自动重置]   ✅ 任务创建完成")
		} else {
			log.Printf("[自动重置]   ⚠️  时间触发条件未启用，跳过任务创建")
			return
		}
	} else {
		log.Printf("[自动重置]   ♻️  复用现有任务 (任务已创建)")
	}

	// 启动任务
	log.Printf("[自动重置]   🚀 启动任务...")
	if err := s.startTasksInternal(); err != nil {
		log.Printf("[自动重置]   ❌ 启动任务失败: %v", err)
		return
	}

	log.Printf("[自动重置] ✅ 自动重置启动完成")
}

// stopTasks 停止任务（禁用状态变化时使用）
func (s *AutoResetService) stopTasks() {
	log.Printf("[自动重置] 🔴 停止自动重置任务")

	if s.tasksRunning {
		log.Printf("[自动重置]   ⏹️  停止运行中的任务...")
		s.stopTasksInternal()
		log.Printf("[自动重置] ✅ 自动重置停止完成")
	} else {
		log.Printf("[自动重置]   ⚠️  任务未在运行，无需停止")
	}
}
