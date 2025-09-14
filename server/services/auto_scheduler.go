package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/leafney/cccmu/server/models"
)

// AutoSchedulerService 自动调度服务
type AutoSchedulerService struct {
	config         *models.AutoScheduleConfig
	schedulerSvc   *SchedulerService
	ctx            context.Context
	cancel         context.CancelFunc
	ticker         *time.Ticker
	mu             sync.RWMutex
	isRunning      bool
	lastState      bool // 记录上一次的监控状态，用于检测状态变化
}

// NewAutoSchedulerService 创建自动调度服务
func NewAutoSchedulerService(schedulerSvc *SchedulerService) *AutoSchedulerService {
	ctx, cancel := context.WithCancel(context.Background())
	return &AutoSchedulerService{
		schedulerSvc: schedulerSvc,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// UpdateConfig 更新自动调度配置
func (a *AutoSchedulerService) UpdateConfig(config *models.AutoScheduleConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()

	oldConfig := a.config
	a.config = config
	
	// 检查启用状态是否发生变化
	enabledChanged := (oldConfig == nil && config.Enabled) || 
					  (oldConfig != nil && oldConfig.Enabled != config.Enabled)
	
	// 检查时间配置是否发生变化（仅在启用状态下才关心）
	timeConfigChanged := config.Enabled && oldConfig != nil && 
						(oldConfig.StartTime != config.StartTime || 
						 oldConfig.EndTime != config.EndTime || 
						 oldConfig.MonitoringOn != config.MonitoringOn)
	
	if enabledChanged {
		if config.Enabled {
			// 启动自动调度服务
			if !a.isRunning {
				a.startInternal()
			}
			log.Printf("[自动调度] 配置已更新并启用:")
			log.Printf("[自动调度] - 时间范围: %s-%s", config.StartTime, config.EndTime)
			log.Printf("[自动调度] - 范围内监控状态: %s", func() string { if config.MonitoringOn { return "开启" } else { return "关闭" } }())
			log.Printf("[自动调度] - 当前时间: %s", time.Now().Format("15:04"))
			log.Printf("[自动调度] - 当前是否在范围内: %v", config.IsInTimeRange(time.Now()))
		} else {
			// 禁用自动调度服务
			a.stopInternal()
			log.Printf("[自动调度] 配置已更新，自动调度已禁用")
		}
	} else if timeConfigChanged {
		// 只是时间配置变化，无需重建任务，只需记录配置变更
		log.Printf("[自动调度] 时间配置已更新（无需重建任务）:")
		log.Printf("[自动调度] - 新时间范围: %s-%s", config.StartTime, config.EndTime)
		log.Printf("[自动调度] - 范围内监控状态: %s", func() string { if config.MonitoringOn { return "开启" } else { return "关闭" } }())
	} else {
		log.Printf("[自动调度] 配置无实质性变化，保持当前状态")
	}
}

// Start 启动自动调度
func (a *AutoSchedulerService) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.config != nil && a.config.Enabled {
		a.startInternal()
	}
}

// Stop 停止自动调度
func (a *AutoSchedulerService) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.stopInternal()
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

// startInternal 内部启动方法（无锁）
func (a *AutoSchedulerService) startInternal() {
	if a.isRunning {
		log.Printf("[自动调度] 服务已在运行中")
		return
	}

	// 创建定时器，每分钟检查一次
	a.ticker = time.NewTicker(time.Minute)
	a.isRunning = true

	log.Printf("[自动调度] 服务已启动，每分钟检查一次")

	go a.checkLoop()
	
	// 在配置更新场景下，不立即执行检查，等待定时器触发
	// 这样可以避免与主调度器的任务启停产生冲突
}

// stopInternal 内部停止方法（无锁）
func (a *AutoSchedulerService) stopInternal() {
	if !a.isRunning {
		log.Printf("[自动调度] 服务已经停止")
		return
	}

	a.isRunning = false
	if a.ticker != nil {
		a.ticker.Stop()
		a.ticker = nil
	}
	log.Printf("[自动调度] 服务已停止")
}

// checkLoop 检查循环
func (a *AutoSchedulerService) checkLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.ticker.C:
			a.checkSchedule()
		}
	}
}

// checkSchedule 检查并执行自动调度逻辑
func (a *AutoSchedulerService) checkSchedule() {
	a.mu.RLock()
	config := a.config
	a.mu.RUnlock()

	if config == nil || !config.Enabled {
		log.Printf("[自动调度] 跳过检查: 配置未启用")
		return
	}

	now := time.Now()
	inTimeRange := config.IsInTimeRange(now)
	shouldMonitoringBeOn := config.ShouldMonitoringBeOn(now)
	
	// 获取当前监控状态
	currentlyOn := a.schedulerSvc.IsRunning()
	
	log.Printf("[自动调度] 执行调度检查:")
	log.Printf("[自动调度] - 当前时间: %s", now.Format("15:04"))
	log.Printf("[自动调度] - 时间范围: %s-%s", config.StartTime, config.EndTime)
	log.Printf("[自动调度] - 当前在时间范围内: %v", inTimeRange)
	log.Printf("[自动调度] - 应该开启监控: %v", shouldMonitoringBeOn)
	log.Printf("[自动调度] - 当前监控状态: %v", currentlyOn)
	
	// 检查是否需要改变状态
	if shouldMonitoringBeOn != currentlyOn {
		a.mu.Lock()
		log.Printf("[自动调度] 需要改变监控状态: %v -> %v", currentlyOn, shouldMonitoringBeOn)
		
		// 记录状态变化
		if currentlyOn != a.lastState {
			a.lastState = currentlyOn
		}
		
		if shouldMonitoringBeOn {
			log.Printf("[自动调度] 启动监控: 当前时间 %s 符合调度条件", now.Format("15:04"))
			if err := a.schedulerSvc.StartAuto(); err != nil {
				log.Printf("[自动调度] 启动监控失败: %v", err)
			} else {
				log.Printf("[自动调度] 监控已成功启动")
			}
		} else {
			log.Printf("[自动调度] 停止监控: 当前时间 %s 不符合调度条件", now.Format("15:04"))
			if err := a.schedulerSvc.StopAuto(); err != nil {
				log.Printf("[自动调度] 停止监控失败: %v", err)
			} else {
				log.Printf("[自动调度] 监控已成功停止")
			}
		}
		
		a.lastState = shouldMonitoringBeOn
		a.mu.Unlock()
		
		// 通知状态变化
		log.Printf("[自动调度] 通知前端状态变化...")
		a.schedulerSvc.NotifyAutoScheduleChange()
	} else {
		log.Printf("[自动调度] 无需改变状态，监控状态保持: %v", currentlyOn)
	}
}

// Close 关闭自动调度服务
func (a *AutoSchedulerService) Close() {
	a.Stop()
	a.cancel()
}