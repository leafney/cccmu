package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/leafney/cccmu/server/models"
)

// AutoSchedulerService 自动调度服务
type AutoSchedulerService struct {
	config       *models.AutoScheduleConfig
	schedulerSvc *SchedulerService
	scheduler    gocron.Scheduler // 专用于自动调度的调度器
	startTaskJob gocron.Job       // 开始时间任务
	endTaskJob   gocron.Job       // 结束时间任务
	mu           sync.RWMutex
	tasksCreated bool // 标记任务是否已创建
	tasksRunning bool // 标记任务是否正在运行
	lastState    bool // 记录上一次的监控状态
}

// getLastState 获取最近一次记录的监控状态
func (a *AutoSchedulerService) getLastState() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastState
}

// setLastState 更新最近一次记录的监控状态
func (a *AutoSchedulerService) setLastState(state bool) {
	a.mu.Lock()
	a.lastState = state
	a.mu.Unlock()
}

// NewAutoSchedulerService 创建自动调度服务
func NewAutoSchedulerService(schedulerSvc *SchedulerService) *AutoSchedulerService {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		log.Printf("[自动调度] 创建调度器失败: %v", err)
		return nil
	}

	return &AutoSchedulerService{
		schedulerSvc: schedulerSvc,
		scheduler:    scheduler,
		tasksCreated: false,
		tasksRunning: false,
		lastState:    schedulerSvc.IsRunning(),
	}
}

// UpdateConfig 更新自动调度配置
func (a *AutoSchedulerService) UpdateConfig(config *models.AutoScheduleConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()

	oldConfig := a.config
	a.config = config

	// 判断启用状态是否变化
	enabledChanged := (oldConfig == nil && config.Enabled) ||
		(oldConfig != nil && oldConfig.Enabled != config.Enabled)

	// 判断时间配置是否变化
	timeConfigChanged := oldConfig != nil &&
		(oldConfig.StartTime != config.StartTime ||
			oldConfig.EndTime != config.EndTime ||
			oldConfig.MonitoringOn != config.MonitoringOn)

	if timeConfigChanged {
		// 时间配置变化：必须重建任务
		log.Printf("[自动调度] 检测到时间配置变化，重建任务")
		log.Printf("[自动调度] - 旧配置: %s-%s(%s)",
			func() string {
				if oldConfig != nil {
					return oldConfig.StartTime
				} else {
					return ""
				}
			}(),
			func() string {
				if oldConfig != nil {
					return oldConfig.EndTime
				} else {
					return ""
				}
			}(),
			func() string {
				if oldConfig != nil {
					if oldConfig.MonitoringOn {
						return "开启"
					} else {
						return "关闭"
					}
				} else {
					return ""
				}
			}())
		log.Printf("[自动调度] - 新配置: %s-%s(%s)", config.StartTime, config.EndTime,
			func() string {
				if config.MonitoringOn {
					return "开启"
				} else {
					return "关闭"
				}
			}())
		a.rebuildTasks(config)
	} else if enabledChanged {
		// 只是启用状态变化：控制任务启停
		if config.Enabled {
			log.Printf("[自动调度] 启用自动调度")
			a.startTasks(config)
		} else {
			log.Printf("[自动调度] 禁用自动调度")
			a.stopTasks()
		}
	} else {
		log.Printf("[自动调度] 配置无实质性变化，保持当前状态")
	}
}

// Start 启动自动调度
func (a *AutoSchedulerService) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config != nil && a.config.Enabled {
		a.startTasks(a.config)
	}
}

// Stop 停止自动调度
func (a *AutoSchedulerService) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stopTasks()
}

// IsEnabled 检查是否启用了自动调度
func (a *AutoSchedulerService) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.config != nil && a.config.Enabled
}

// GetConfig 获取当前自动调度配置
func (a *AutoSchedulerService) GetConfig() *models.AutoScheduleConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return &models.AutoScheduleConfig{}
	}
	return a.config
}

// IsInTimeRange 检查当前时间是否在自动调度时间范围内
func (a *AutoSchedulerService) IsInTimeRange() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil || !a.config.Enabled {
		return false
	}

	return a.config.IsInTimeRange(time.Now())
}

// generateCronExpression 根据时间字符串生成cron表达式
// timeStr格式: "HH:MM" (如 "18:30")
// 返回格式: "MM HH * * *" (分 时 日 月 星期)
func (a *AutoSchedulerService) generateCronExpression(timeStr string) (string, error) {
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

// calculateInitialState 计算服务启动时应该处于的监控状态
func (a *AutoSchedulerService) calculateInitialState(config *models.AutoScheduleConfig) bool {
	if config == nil || !config.Enabled {
		return false
	}

	now := time.Now()
	inRange := config.IsInTimeRange(now)

	// 根据配置的monitoringOn决定在时间范围内应该是什么状态
	if inRange {
		return config.MonitoringOn // 在范围内：按配置设置
	} else {
		return !config.MonitoringOn // 在范围外：与配置相反
	}
}

// isValidTimeRange 验证时间范围是否有效
func (a *AutoSchedulerService) isValidTimeRange(startTime, endTime string) error {
	if startTime == "" || endTime == "" {
		return fmt.Errorf("开始时间和结束时间不能为空")
	}

	if startTime == endTime {
		return fmt.Errorf("开始时间不能等于结束时间")
	}

	// 验证时间格式
	if _, err := a.generateCronExpression(startTime); err != nil {
		return fmt.Errorf("开始时间格式错误: %w", err)
	}

	if _, err := a.generateCronExpression(endTime); err != nil {
		return fmt.Errorf("结束时间格式错误: %w", err)
	}

	return nil
}

// createTasks 创建定时任务
func (a *AutoSchedulerService) createTasks(config *models.AutoScheduleConfig) error {
	log.Printf("[自动调度] 开始创建定时任务...")

	if config == nil {
		log.Printf("[自动调度] 创建任务失败: 配置为空")
		return fmt.Errorf("配置为空")
	}

	// 验证时间范围
	log.Printf("[自动调度] 验证时间范围: %s-%s", config.StartTime, config.EndTime)
	if err := a.isValidTimeRange(config.StartTime, config.EndTime); err != nil {
		log.Printf("[自动调度] 时间范围验证失败: %v", err)
		return fmt.Errorf("时间范围验证失败: %w", err)
	}

	// 生成开始时间的cron表达式
	log.Printf("[自动调度] 生成开始时间cron表达式...")
	startCron, err := a.generateCronExpression(config.StartTime)
	if err != nil {
		log.Printf("[自动调度] 生成开始时间cron表达式失败: %v", err)
		return fmt.Errorf("生成开始时间cron表达式失败: %w", err)
	}
	log.Printf("[自动调度] 开始时间cron表达式: %s -> %s", config.StartTime, startCron)

	// 生成结束时间的cron表达式
	log.Printf("[自动调度] 生成结束时间cron表达式...")
	endCron, err := a.generateCronExpression(config.EndTime)
	if err != nil {
		log.Printf("[自动调度] 生成结束时间cron表达式失败: %v", err)
		return fmt.Errorf("生成结束时间cron表达式失败: %w", err)
	}
	log.Printf("[自动调度] 结束时间cron表达式: %s -> %s", config.EndTime, endCron)

	// 创建开始时间任务
	log.Printf("[自动调度] 创建开始时间任务...")
	startJob, err := a.scheduler.NewJob(
		gocron.CronJob(startCron, false),
		gocron.NewTask(a.handleStartTimeTask, config),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		log.Printf("[自动调度] 创建开始时间任务失败: %v", err)
		return fmt.Errorf("创建开始时间任务失败: %w", err)
	}
	log.Printf("[自动调度] 开始时间任务创建成功, ID: %v", startJob.ID())

	// 创建结束时间任务
	log.Printf("[自动调度] 创建结束时间任务...")
	endJob, err := a.scheduler.NewJob(
		gocron.CronJob(endCron, false),
		gocron.NewTask(a.handleEndTimeTask, config),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		log.Printf("[自动调度] 创建结束时间任务失败: %v", err)
		return fmt.Errorf("创建结束时间任务失败: %w", err)
	}
	log.Printf("[自动调度] 结束时间任务创建成功, ID: %v", endJob.ID())

	a.startTaskJob = startJob
	a.endTaskJob = endJob
	a.tasksCreated = true

	log.Printf("[自动调度] ✅ 定时任务创建完成:")
	log.Printf("[自动调度]   📅 开始时间: %s (cron: %s)", config.StartTime, startCron)
	log.Printf("[自动调度]   📅 结束时间: %s (cron: %s)", config.EndTime, endCron)
	log.Printf("[自动调度]   🎯 范围内监控状态: %s", func() string {
		if config.MonitoringOn {
			return "开启"
		} else {
			return "关闭"
		}
	}())
	log.Printf("[自动调度]   🆔 开始任务ID: %v", startJob.ID())
	log.Printf("[自动调度]   🆔 结束任务ID: %v", endJob.ID())

	return nil
}

// removeTasks 删除现有任务
func (a *AutoSchedulerService) removeTasks() {
	log.Printf("[自动调度] 开始删除现有任务...")

	if !a.tasksCreated {
		log.Printf("[自动调度] ⚠️  无任务需要删除 (任务未创建)")
		return
	}

	// 记录要删除的任务信息
	startJobID := "未知"
	endJobID := "未知"
	if a.startTaskJob != nil {
		startJobID = fmt.Sprintf("%v", a.startTaskJob.ID())
	}
	if a.endTaskJob != nil {
		endJobID = fmt.Sprintf("%v", a.endTaskJob.ID())
	}
	log.Printf("[自动调度] 准备删除任务: 开始任务ID=%s, 结束任务ID=%s", startJobID, endJobID)

	// 先停止任务
	if a.tasksRunning {
		log.Printf("[自动调度] 停止运行中的任务...")
		a.scheduler.StopJobs()
		a.tasksRunning = false
		log.Printf("[自动调度] ✅ 已停止运行中的任务")
	} else {
		log.Printf("[自动调度] 任务未在运行，无需停止")
	}

	// 删除开始时间任务
	if a.startTaskJob != nil {
		log.Printf("[自动调度] 删除开始时间任务 (ID: %v)...", a.startTaskJob.ID())
		if err := a.scheduler.RemoveJob(a.startTaskJob.ID()); err != nil {
			log.Printf("[自动调度] ❌ 删除开始时间任务失败: %v", err)
		} else {
			log.Printf("[自动调度] ✅ 开始时间任务删除成功")
		}
		a.startTaskJob = nil
	} else {
		log.Printf("[自动调度] 开始时间任务不存在，跳过删除")
	}

	// 删除结束时间任务
	if a.endTaskJob != nil {
		log.Printf("[自动调度] 删除结束时间任务 (ID: %v)...", a.endTaskJob.ID())
		if err := a.scheduler.RemoveJob(a.endTaskJob.ID()); err != nil {
			log.Printf("[自动调度] ❌ 删除结束时间任务失败: %v", err)
		} else {
			log.Printf("[自动调度] ✅ 结束时间任务删除成功")
		}
		a.endTaskJob = nil
	} else {
		log.Printf("[自动调度] 结束时间任务不存在，跳过删除")
	}

	a.tasksCreated = false
	log.Printf("[自动调度] ✅ 任务删除完成，状态已重置")
}

// startTasksInternal 启动任务（内部方法，无锁）
func (a *AutoSchedulerService) startTasksInternal() error {
	log.Printf("[自动调度] 开始启动定时任务...")

	if !a.tasksCreated {
		log.Printf("[自动调度] ❌ 启动失败: 任务未创建")
		return fmt.Errorf("任务未创建")
	}

	if a.tasksRunning {
		log.Printf("[自动调度] ⚠️  任务已在运行中，跳过启动")
		return nil
	}

	// 记录要启动的任务信息
	startJobID := "未知"
	endJobID := "未知"
	if a.startTaskJob != nil {
		startJobID = fmt.Sprintf("%v", a.startTaskJob.ID())
	}
	if a.endTaskJob != nil {
		endJobID = fmt.Sprintf("%v", a.endTaskJob.ID())
	}
	log.Printf("[自动调度] 启动任务: 开始任务ID=%s, 结束任务ID=%s", startJobID, endJobID)

	// 启动调度器
	log.Printf("[自动调度] 启动调度器...")
	a.scheduler.Start()
	a.tasksRunning = true

	log.Printf("[自动调度] ✅ 定时任务启动完成")
	log.Printf("[自动调度]   🟢 调度器状态: 运行中")
	log.Printf("[自动调度]   📊 任务数量: 2个 (开始+结束)")
	return nil
}

// stopTasksInternal 停止任务（内部方法，无锁）
func (a *AutoSchedulerService) stopTasksInternal() {
	log.Printf("[自动调度] 开始停止定时任务...")

	if !a.tasksRunning {
		log.Printf("[自动调度] ⚠️  任务已经停止，跳过操作")
		return
	}

	// 记录要停止的任务信息
	startJobID := "未知"
	endJobID := "未知"
	if a.startTaskJob != nil {
		startJobID = fmt.Sprintf("%v", a.startTaskJob.ID())
	}
	if a.endTaskJob != nil {
		endJobID = fmt.Sprintf("%v", a.endTaskJob.ID())
	}
	log.Printf("[自动调度] 停止任务: 开始任务ID=%s, 结束任务ID=%s", startJobID, endJobID)

	// 停止任务（保留任务实例）
	log.Printf("[自动调度] 停止调度器...")
	a.scheduler.StopJobs()
	a.tasksRunning = false

	log.Printf("[自动调度] ✅ 定时任务停止完成")
	log.Printf("[自动调度]   🔴 调度器状态: 已停止")
	log.Printf("[自动调度]   💾 任务实例: 已保留 (可复用)")
}

// handleStartTimeTask 处理开始时间任务
func (a *AutoSchedulerService) handleStartTimeTask(config *models.AutoScheduleConfig) {
	// 检查服务是否正在关闭
	if !a.tasksRunning {
		log.Printf("[自动调度] ⚠️  开始时间任务触发但服务正在关闭，跳过执行")
		return
	}

	now := time.Now()
	log.Printf("[自动调度] 🚀 开始时间任务触发!")
	log.Printf("[自动调度]   ⏰ 触发时间: %s", now.Format("2006-01-02 15:04:05"))
	log.Printf("[自动调度]   📋 配置时间: %s", config.StartTime)
	log.Printf("[自动调度]   🎯 目标操作: %s监控", func() string {
		if config.MonitoringOn {
			return "开启"
		} else {
			return "关闭"
		}
	}())

	// 计算应该执行的操作
	shouldMonitoringOn := config.MonitoringOn
	currentlyOn := a.schedulerSvc.IsRunning()
	lastRecorded := a.getLastState()

	log.Printf("[自动调度]   📊 当前监控状态: %v", currentlyOn)
	log.Printf("[自动调度]   🎯 目标监控状态: %v", shouldMonitoringOn)

	needsChange := shouldMonitoringOn != currentlyOn || lastRecorded != shouldMonitoringOn

	if needsChange {
		if lastRecorded != shouldMonitoringOn {
			log.Printf("[自动调度]   🔁 记录状态为: %v，需与目标状态同步", lastRecorded)
		}
		log.Printf("[自动调度]   🔄 需要改变监控状态: %v → %v", currentlyOn, shouldMonitoringOn)

		if shouldMonitoringOn {
			log.Printf("[自动调度]   ▶️  执行操作: 启动监控")
			if err := a.schedulerSvc.StartAuto(); err != nil {
				log.Printf("[自动调度]   ❌ 启动监控失败: %v", err)
				log.Printf("[自动调度]   ⏳ 保持上次记录状态: %v", lastRecorded)
			} else {
				log.Printf("[自动调度]   ✅ 监控已成功启动")
				a.setLastState(shouldMonitoringOn)
			}
		} else {
			log.Printf("[自动调度]   ⏹️  执行操作: 停止监控")
			if err := a.schedulerSvc.StopAuto(); err != nil {
				log.Printf("[自动调度]   ❌ 停止监控失败: %v", err)
				log.Printf("[自动调度]   ⏳ 保持上次记录状态: %v", lastRecorded)
			} else {
				log.Printf("[自动调度]   ✅ 监控已成功停止")
				a.setLastState(shouldMonitoringOn)
			}
		}

		log.Printf("[自动调度]   📡 通知前端状态变化...")
		a.schedulerSvc.NotifyAutoScheduleChange()
		log.Printf("[自动调度] 🏁 开始时间任务处理完成")
	} else {
		log.Printf("[自动调度]   ✨ 监控状态无需改变 (已是期望状态)")
		a.setLastState(shouldMonitoringOn)
		log.Printf("[自动调度] 🏁 开始时间任务处理完成")
	}
}

// handleEndTimeTask 处理结束时间任务
func (a *AutoSchedulerService) handleEndTimeTask(config *models.AutoScheduleConfig) {
	// 检查服务是否正在关闭
	if !a.tasksRunning {
		log.Printf("[自动调度] ⚠️  结束时间任务触发但服务正在关闭，跳过执行")
		return
	}

	now := time.Now()
	log.Printf("[自动调度] 🏁 结束时间任务触发!")
	log.Printf("[自动调度]   ⏰ 触发时间: %s", now.Format("2006-01-02 15:04:05"))
	log.Printf("[自动调度]   📋 配置时间: %s", config.EndTime)
	log.Printf("[自动调度]   🎯 目标操作: %s监控 (与范围内相反)", func() string {
		if !config.MonitoringOn {
			return "开启"
		} else {
			return "关闭"
		}
	}())

	// 计算应该执行的操作（结束时间执行相反操作）
	shouldMonitoringOn := !config.MonitoringOn
	currentlyOn := a.schedulerSvc.IsRunning()
	lastRecorded := a.getLastState()

	log.Printf("[自动调度]   📊 当前监控状态: %v", currentlyOn)
	log.Printf("[自动调度]   🎯 目标监控状态: %v", shouldMonitoringOn)

	needsChange := shouldMonitoringOn != currentlyOn || lastRecorded != shouldMonitoringOn

	if needsChange {
		if lastRecorded != shouldMonitoringOn {
			log.Printf("[自动调度]   🔁 记录状态为: %v，需与目标状态同步", lastRecorded)
		}
		log.Printf("[自动调度]   🔄 需要改变监控状态: %v → %v", currentlyOn, shouldMonitoringOn)

		if shouldMonitoringOn {
			log.Printf("[自动调度]   ▶️  执行操作: 启动监控")
			if err := a.schedulerSvc.StartAuto(); err != nil {
				log.Printf("[自动调度]   ❌ 启动监控失败: %v", err)
				log.Printf("[自动调度]   ⏳ 保持上次记录状态: %v", lastRecorded)
			} else {
				log.Printf("[自动调度]   ✅ 监控已成功启动")
				a.setLastState(shouldMonitoringOn)
			}
		} else {
			log.Printf("[自动调度]   ⏹️  执行操作: 停止监控")
			if err := a.schedulerSvc.StopAuto(); err != nil {
				log.Printf("[自动调度]   ❌ 停止监控失败: %v", err)
				log.Printf("[自动调度]   ⏳ 保持上次记录状态: %v", lastRecorded)
			} else {
				log.Printf("[自动调度]   ✅ 监控已成功停止")
				a.setLastState(shouldMonitoringOn)
			}
		}

		log.Printf("[自动调度]   📡 通知前端状态变化...")
		a.schedulerSvc.NotifyAutoScheduleChange()
		log.Printf("[自动调度] 🏁 结束时间任务处理完成")
	} else {
		log.Printf("[自动调度]   ✨ 监控状态无需改变 (已是期望状态)")
		a.setLastState(shouldMonitoringOn)
		log.Printf("[自动调度] 🏁 结束时间任务处理完成")
	}
}

// rebuildTasks 重建任务（时间配置变化时使用）
func (a *AutoSchedulerService) rebuildTasks(config *models.AutoScheduleConfig) {
	log.Printf("[自动调度] 🔄 开始重建任务 (时间配置变化)")
	log.Printf("[自动调度]   📋 新配置: %s-%s (%s监控)",
		config.StartTime, config.EndTime,
		func() string {
			if config.MonitoringOn {
				return "范围内开启"
			} else {
				return "范围内关闭"
			}
		}())

	// 删除旧任务
	log.Printf("[自动调度]   🗑️  删除旧任务...")
	a.removeTasks()

	// 创建新任务
	log.Printf("[自动调度]   🔨 创建新任务...")
	if err := a.createTasks(config); err != nil {
		log.Printf("[自动调度]   ❌ 创建新任务失败: %v", err)
		return
	}

	// 根据启用状态决定是否启动
	if config.Enabled {
		log.Printf("[自动调度]   🚀 启动新任务...")
		if err := a.startTasksInternal(); err != nil {
			log.Printf("[自动调度]   ❌ 启动新任务失败: %v", err)
			return
		}
		// 异步设置初始状态，避免阻塞配置更新请求
		go func() {
			log.Printf("[自动调度]   ⚙️  设置初始状态...")
			a.setInitialState()
		}()
		log.Printf("[自动调度] ✅ 任务重建完成并启动")
	} else {
		log.Printf("[自动调度] ✅ 任务重建完成 (未启动，因为自动调度被禁用)")
	}
}

// startTasks 启动任务（启用状态变化时使用）
func (a *AutoSchedulerService) startTasks(config *models.AutoScheduleConfig) {
	log.Printf("[自动调度] 🟢 启动自动调度任务")

	if !a.tasksCreated {
		log.Printf("[自动调度]   🔨 首次启用: 需要创建任务")
		if err := a.createTasks(config); err != nil {
			log.Printf("[自动调度]   ❌ 创建任务失败: %v", err)
			return
		}
		log.Printf("[自动调度]   ✅ 任务创建完成")
	} else {
		log.Printf("[自动调度]   ♻️  复用现有任务 (任务已创建)")
	}

	// 启动任务
	log.Printf("[自动调度]   🚀 启动任务...")
	if err := a.startTasksInternal(); err != nil {
		log.Printf("[自动调度]   ❌ 启动任务失败: %v", err)
		return
	}

	// 异步设置初始状态，避免阻塞配置更新请求
	go func() {
		log.Printf("[自动调度]   ⚙️  设置初始状态...")
		a.setInitialState()
	}()
	log.Printf("[自动调度] ✅ 自动调度启动完成")
}

// stopTasks 停止任务（禁用状态变化时使用）
func (a *AutoSchedulerService) stopTasks() {
	log.Printf("[自动调度] 🔴 停止自动调度任务")

	if a.tasksRunning {
		log.Printf("[自动调度]   ⏹️  停止运行中的任务...")
		a.stopTasksInternal()
		log.Printf("[自动调度] ✅ 自动调度停止完成")
	} else {
		log.Printf("[自动调度]   ⚠️  任务未在运行，无需停止")
	}
}

// setInitialState 设置初始监控状态
func (a *AutoSchedulerService) setInitialState() {
	log.Printf("[自动调度] ⚙️  开始设置初始状态...")

	if a.config == nil || !a.config.Enabled {
		log.Printf("[自动调度]   ⚠️  配置无效或未启用，跳过初始状态设置")
		return
	}

	now := time.Now()
	shouldMonitoringBeOn := a.calculateInitialState(a.config)
	currentlyOn := a.schedulerSvc.IsRunning()
	inRange := a.config.IsInTimeRange(now)

	log.Printf("[自动调度] 📊 初始状态分析:")
	log.Printf("[自动调度]   ⏰ 当前时间: %s", now.Format("2006-01-02 15:04:05"))
	log.Printf("[自动调度]   📅 时间范围: %s-%s", a.config.StartTime, a.config.EndTime)
	log.Printf("[自动调度]   🎯 范围内监控: %s", func() string {
		if a.config.MonitoringOn {
			return "开启"
		} else {
			return "关闭"
		}
	}())
	log.Printf("[自动调度]   📍 当前在范围内: %v", inRange)
	log.Printf("[自动调度]   📊 当前监控状态: %v", currentlyOn)
	log.Printf("[自动调度]   🎯 应该监控状态: %v", shouldMonitoringBeOn)

	if shouldMonitoringBeOn != currentlyOn {
		log.Printf("[自动调度]   🔄 需要调整监控状态: %v → %v", currentlyOn, shouldMonitoringBeOn)

		if shouldMonitoringBeOn {
			log.Printf("[自动调度]   ▶️  初始化: 启动监控")
			if err := a.schedulerSvc.StartAuto(); err != nil {
				log.Printf("[自动调度]   ❌ 初始化启动监控失败: %v", err)
			} else {
				log.Printf("[自动调度]   ✅ 初始化: 监控已成功启动")
			}
		} else {
			log.Printf("[自动调度]   ⏹️  初始化: 停止监控")
			if err := a.schedulerSvc.StopAuto(); err != nil {
				log.Printf("[自动调度]   ❌ 初始化停止监控失败: %v", err)
			} else {
				log.Printf("[自动调度]   ✅ 初始化: 监控已成功停止")
			}
		}

		log.Printf("[自动调度]   📡 通知前端状态变化...")
		a.schedulerSvc.NotifyAutoScheduleChange()
		log.Printf("[自动调度] ✅ 初始状态设置完成")
	} else {
		log.Printf("[自动调度]   ✨ 监控状态正确，无需调整")
		log.Printf("[自动调度] ✅ 初始状态检查完成")
	}

	a.setLastState(shouldMonitoringBeOn)
}

// Close 关闭自动调度服务
func (a *AutoSchedulerService) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	log.Printf("[自动调度] 🔄 开始关闭自动调度服务...")

	// 停止并关闭调度器
	if a.scheduler != nil {
		log.Printf("[自动调度]   ⏹️  停止调度器任务...")
		// 先设置任务状态，阻止新任务执行
		a.tasksRunning = false

		// 停止所有任务
		a.scheduler.StopJobs()

		log.Printf("[自动调度]   🔐 关闭调度器...")
		// 直接关闭，不等待
		a.scheduler.Shutdown()

		log.Printf("[自动调度]   ✅ 调度器已关闭")
	} else {
		log.Printf("[自动调度]   ⚠️  调度器不存在，无需关闭")
	}

	// 重置状态
	log.Printf("[自动调度]   🔄 重置内部状态...")
	a.tasksCreated = false
	a.tasksRunning = false
	a.startTaskJob = nil
	a.endTaskJob = nil

	log.Printf("[自动调度] ✅ 自动调度服务已完全关闭")
}
