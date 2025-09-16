package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
)

// AutoResetService 自动重置服务
type AutoResetService struct {
	scheduler    gocron.Scheduler        // 时间任务调度器
	resetJob     gocron.Job              // 重置任务
	config       *models.AutoResetConfig // 当前配置
	db           *database.BadgerDB      // 数据库访问
	schedulerSvc *SchedulerService       // 调度器服务（用于通知和重置）
	mu           sync.RWMutex            // 并发保护
	tasksCreated bool                    // 标记任务是否已创建
	tasksRunning bool                    // 标记任务是否正在运行
}

// NewAutoResetService 创建自动重置服务
func NewAutoResetService(db *database.BadgerDB, schedulerSvc *SchedulerService) *AutoResetService {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		log.Printf("[自动重置] 创建调度器失败: %v", err)
		return nil
	}

	return &AutoResetService{
		scheduler:    scheduler,
		db:           db,
		schedulerSvc: schedulerSvc,
		tasksCreated: false,
		tasksRunning: false,
	}
}

// UpdateConfig 更新自动重置配置
func (s *AutoResetService) UpdateConfig(config *models.AutoResetConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldConfig := s.config
	s.config = config

	log.Printf("[自动重置] 配置更新:")
	log.Printf("[自动重置] - 启用状态: %v", config.Enabled)
	log.Printf("[自动重置] - 时间触发条件: %v", config.TimeEnabled)
	if config.Enabled && config.TimeEnabled && config.ResetTime != "" {
		log.Printf("[自动重置] - 重置时间: %s", config.ResetTime)
	}

	// 判断启用状态是否变化
	enabledChanged := (oldConfig == nil && config.Enabled) ||
		(oldConfig != nil && oldConfig.Enabled != config.Enabled)

	// 判断时间配置是否变化
	timeConfigChanged := oldConfig != nil && (oldConfig.TimeEnabled != config.TimeEnabled || oldConfig.ResetTime != config.ResetTime)

	if timeConfigChanged {
		// 时间配置变化：必须重建任务
		log.Printf("[自动重置] 检测到时间配置变化，重建任务")
		s.rebuildTasks(config)
	} else if enabledChanged {
		// 只是启用状态变化：控制任务启停
		if config.Enabled {
			log.Printf("[自动重置] 启用自动重置")
			s.startTasks(config)
		} else {
			log.Printf("[自动重置] 禁用自动重置")
			s.stopTasks()
		}
	} else {
		log.Printf("[自动重置] 配置无实质性变化，保持当前状态")
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

	// 关闭调度器
	if s.scheduler != nil {
		log.Printf("[自动重置] 关闭调度器")
		s.scheduler.Shutdown()
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

	s.executeAutoReset("time_trigger")
}

// executeAutoReset 执行自动重置
func (s *AutoResetService) executeAutoReset(trigger string) {
	log.Printf("[自动重置] 开始执行自动重置，触发原因: %s", trigger)

	// 检查是否已重置
	if s.isAlreadyReset() {
		log.Printf("[自动重置] 今日已重置过，跳过执行")
		return
	}

	// 调用现有的重置积分API
	success := s.callExistingResetAPI()
	if success {
		log.Printf("[自动重置] ✅ 自动重置执行成功")
	} else {
		log.Printf("[自动重置] ❌ 自动重置执行失败")
	}
}

// callExistingResetAPI 调用现有的重置积分API逻辑
func (s *AutoResetService) callExistingResetAPI() bool {
	// 获取当前配置
	config, err := s.db.GetConfig()
	if err != nil {
		log.Printf("[自动重置] 获取配置失败: %v", err)
		return false
	}

	// 检查Cookie是否配置
	if config.Cookie == "" {
		log.Printf("[自动重置] Cookie未配置，跳过重置")
		return false
	}

	// 通过调度器服务的重置功能来执行重置
	// 自动重置功能独立于监控功能，不需要检查监控状态
	// 这会复用现有的重置逻辑，包括API调用、状态更新和SSE通知

	// 调用真实的重置API
	err = s.schedulerSvc.ResetCreditsManually()
	if err != nil {
		log.Printf("[自动重置] 调用重置API失败: %v", err)
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
