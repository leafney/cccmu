package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/leafney/cccmu/server/client"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/utils"
)

// SchedulerService 定时任务服务
type SchedulerService struct {
	scheduler             gocron.Scheduler
	dailyResetScheduler   gocron.Scheduler     // 单独的每日重置任务调度器
	db                    *database.BadgerDB
	apiClient             *client.ClaudeAPIClient
	config                *models.UserConfig
	isRunning             bool
	mu                    sync.RWMutex
	lastData              []models.UsageData
	listeners             []chan []models.UsageData
	lastBalance           *models.CreditBalance
	balanceListeners      []chan *models.CreditBalance
	errorListeners        []chan string
	resetStatusListeners  []chan bool
	autoScheduler         *AutoSchedulerService
	autoScheduleListeners []chan bool              // 自动调度状态变化监听器
	dailyUsageListeners   []chan []models.DailyUsage // 每日积分统计数据监听器
	balanceJob            gocron.Job               // 积分余额任务引用
	balanceTaskPaused     bool                     // 积分余额任务暂停状态
	autoResetService      *AutoResetService        // 自动重置服务引用
	dailyUsageTracker     *DailyUsageTracker       // 每日积分统计跟踪服务
}

// NewSchedulerService 创建新的调度服务
func NewSchedulerService(db *database.BadgerDB) (*SchedulerService, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("创建调度器失败: %w", err)
	}

	// 创建单独的每日重置任务调度器
	dailyResetScheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("创建每日重置调度器失败: %w", err)
	}

	config, err := db.GetConfig()
	if err != nil {
		log.Printf("获取配置失败，使用默认配置: %v", err)
		config = models.GetDefaultConfig()
	}

	apiClient := client.NewClaudeAPIClient(config.Cookie)

	service := &SchedulerService{
		scheduler:             scheduler,
		dailyResetScheduler:   dailyResetScheduler,
		db:                    db,
		apiClient:             apiClient,
		config:                config,
		isRunning:             false,
		listeners:             make([]chan []models.UsageData, 0),
		balanceListeners:      make([]chan *models.CreditBalance, 0),
		errorListeners:        make([]chan string, 0),
		resetStatusListeners:  make([]chan bool, 0),
		autoScheduleListeners: make([]chan bool, 0),
		dailyUsageListeners:   make([]chan []models.DailyUsage, 0),
	}

	// 创建自动调度服务
	service.autoScheduler = NewAutoSchedulerService(service)

	// 创建每日积分统计服务
	dailyUsageTracker, err := NewDailyUsageTracker(db, apiClient)
	if err != nil {
		utils.Logf("[调度器] ❌ 创建每日积分统计服务失败: %v", err)
	} else {
		service.dailyUsageTracker = dailyUsageTracker
		utils.Logf("[调度器] ✅ 每日积分统计服务创建成功")
		
		// 立即初始化每日积分统计服务（程序启动时就初始化）
		if err := dailyUsageTracker.Initialize(service.scheduler); err != nil {
			utils.Logf("[调度器] ❌ 初始化每日积分统计服务失败: %v", err)
		} else {
			utils.Logf("[调度器] ✅ 每日积分统计服务已初始化")
			
			// 根据配置的初始状态决定是否启动任务
			if config.DailyUsageEnabled {
				if err := dailyUsageTracker.Start(); err != nil {
					utils.Logf("[调度器] ❌ 初始化时启动每日积分统计任务失败: %v", err)
				} else {
					utils.Logf("[调度器] ✅ 每日积分统计任务已在初始化时激活")
				}
			} else {
				utils.Logf("[调度器] ℹ️  每日积分统计功能已禁用，任务未激活")
			}
		}
	}

	// 立即创建每日重置任务（只需创建一次）
	if err := service.createDailyResetTask(); err != nil {
		log.Printf("创建每日重置任务失败: %v", err)
	}

	return service, nil
}

// createDailyResetTask 创建每日重置任务
func (s *SchedulerService) createDailyResetTask() error {
	// 添加每日0点重置标记的定时任务
	dailyResetJob, err := s.dailyResetScheduler.NewJob(
		gocron.CronJob("0 0 * * *", false), // 每日0点执行
		gocron.NewTask(s.resetDailyFlags),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建每日重置标记定时任务失败: %w", err)
	}

	log.Printf("每日重置标记定时任务创建成功，任务ID: %v", dailyResetJob.ID())

	// 启动每日重置调度器
	s.dailyResetScheduler.Start()
	log.Printf("每日重置调度器已启动")

	return nil
}

// Start 启动定时任务
func (s *SchedulerService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已经在运行，先停止再重新启动
	if s.isRunning {
		s.scheduler.StopJobs()
		s.scheduler.Shutdown()
		s.isRunning = false
		log.Println("停止现有任务，准备重新启动")
	}

	// 更新配置
	config, err := s.db.GetConfig()
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}
	s.config = config

	if s.config.Cookie == "" {
		return fmt.Errorf("Cookie未设置")
	}

	// 验证Cookie（通过获取积分余额隐式验证）
	s.apiClient.UpdateCookie(s.config.Cookie)
	if _, cookieErr := s.apiClient.FetchCreditBalance(); cookieErr != nil {
		return fmt.Errorf("cookie验证失败: %w", cookieErr)
	}

	// 添加使用数据定时任务
	usageJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Second),
		gocron.NewTask(s.fetchAndSaveData),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建使用数据定时任务失败: %w", err)
	}

	// 检查是否有阈值任务正在运行
	var shouldCreateBalanceTask = true
	if s.autoResetService != nil && s.autoResetService.IsThresholdTaskRunning() {
		utils.Logf("[任务协调] ⚠️  检测到阈值任务正在运行，跳过积分余额任务创建")
		shouldCreateBalanceTask = false
		s.balanceTaskPaused = true
		s.balanceJob = nil
	}

	if shouldCreateBalanceTask {
		// 添加积分余额定时任务，间隔错开20秒执行
		balanceJob, err := s.scheduler.NewJob(
			gocron.DurationJob(time.Duration(s.config.Interval)*time.Second),
			gocron.NewTask(s.fetchAndSaveBalance),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
			gocron.WithStartAt(
				gocron.WithStartDateTime(time.Now().Add(20*time.Second)),
			),
		)
		if err != nil {
			return fmt.Errorf("创建积分余额定时任务失败: %w", err)
		}

		// 保存积分余额任务引用
		s.balanceJob = balanceJob
		s.balanceTaskPaused = false
		utils.Logf("[任务协调] ✅ 积分余额定时任务创建成功，任务ID: %v，间隔: %d秒", balanceJob.ID(), s.config.Interval)
	}

	log.Printf("使用数据定时任务创建成功，任务ID: %v，间隔: %d秒", usageJob.ID(), s.config.Interval)
	if shouldCreateBalanceTask && s.balanceJob != nil {
		log.Printf("积分余额定时任务创建成功，任务ID: %v，间隔: %d秒", s.balanceJob.ID(), s.config.Interval)
	} else {
		log.Printf("积分余额定时任务已跳过创建（检测到阈值任务冲突）")
	}

	// 启动调度器
	s.scheduler.Start()
	s.isRunning = true

	log.Printf("定时任务已启动，间隔: %d秒", s.config.Interval)

	// 每日积分统计任务已在初始化时根据配置激活，无需重复处理

	// 立即执行一次，确保在所有监听器建立后执行
	go func() {
		time.Sleep(100 * time.Millisecond) // 短暂延迟，确保SSE连接已建立
		s.fetchAndSaveData()
		// 延迟5秒后获取积分余额，避免并发（仅在没有阈值任务冲突时执行）
		if shouldCreateBalanceTask {
			time.Sleep(5 * time.Second)
			s.fetchAndSaveBalance()
		} else {
			utils.Logf("[任务协调] ⚠️  跳过立即执行积分获取（阈值任务冲突）")
		}
	}()

	return nil
}

// Stop 停止定时任务
func (s *SchedulerService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return fmt.Errorf("任务未运行")
	}

	// 设置较短的超时时间，避免长时间等待
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用带超时的停止方法
	if err := s.scheduler.StopJobs(); err != nil {
		log.Printf("停止任务失败: %v", err)
	}

	// 等待所有任务完成或超时
	select {
	case <-ctx.Done():
		log.Println("停止调度器超时，强制关闭")
	case <-time.After(100 * time.Millisecond):
		// 短暂等待确保任务停止
	}

	// 强制关闭调度器
	if err := s.scheduler.Shutdown(); err != nil {
		log.Printf("强制关闭调度器失败: %v", err)
		// 不返回错误，继续执行清理
	}

	// 关闭每日积分统计服务（程序退出时注销）
	if s.dailyUsageTracker != nil && s.dailyUsageTracker.IsInitialized() {
		if err := s.dailyUsageTracker.Shutdown(); err != nil {
			utils.Logf("[调度器] ❌ 关闭每日积分统计服务失败: %v", err)
		} else {
			utils.Logf("[调度器] ✅ 每日积分统计服务已关闭")
		}
	}

	s.isRunning = false
	log.Println("定时任务已停止")

	return nil
}

// IsRunning 检查任务是否运行中
func (s *SchedulerService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// needsTaskRestart 检查配置更新是否需要重启定时任务（内部方法）
func (s *SchedulerService) needsTaskRestart(oldConfig, newConfig *models.UserConfig) bool {
	if oldConfig == nil {
		return newConfig.Enabled // 首次配置，根据是否启用决定
	}

	// 检查影响定时任务的关键配置项
	return oldConfig.Interval != newConfig.Interval || // 监控间隔变化
		oldConfig.Cookie != newConfig.Cookie || // Cookie变化
		oldConfig.Enabled != newConfig.Enabled // 启用状态变化
}

// NeedsTaskRestart 检查配置更新是否需要重启定时任务（公共方法）
func (s *SchedulerService) NeedsTaskRestart(oldConfig, newConfig *models.UserConfig) bool {
	return s.needsTaskRestart(oldConfig, newConfig)
}

// UpdateConfig 更新配置并按需重启任务（同步版本，已弃用，保留兼容性）
func (s *SchedulerService) UpdateConfig(newConfig *models.UserConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 保存配置到数据库
	if err := s.db.SaveConfig(newConfig); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	oldConfig := s.config
	wasRunning := s.isRunning
	needsRestart := s.needsTaskRestart(oldConfig, newConfig)

	// 记录配置更新情况
	if needsRestart {
		log.Printf("配置更新：检测到关键参数变化，需要重启定时任务")
		log.Printf("配置差异：间隔 %d->%d秒, 启用 %v->%v",
			func() int {
				if oldConfig != nil {
					return oldConfig.Interval
				} else {
					return 0
				}
			}(),
			newConfig.Interval,
			func() bool {
				if oldConfig != nil {
					return oldConfig.Enabled
				} else {
					return false
				}
			}(),
			newConfig.Enabled)
	} else {
		log.Printf("配置更新：仅更新非关键参数，无需重启任务")
	}

	// 更新配置引用
	s.config = newConfig
	s.apiClient.UpdateCookie(newConfig.Cookie)

	// 更新自动调度配置（不直接触发任务启停）
	if s.autoScheduler != nil {
		s.autoScheduler.UpdateConfig(&newConfig.AutoSchedule)
	}

	// 处理每日积分统计配置变更
	s.handleDailyUsageConfigChange(oldConfig, newConfig)

	// 只在必要时重启任务
	if needsRestart {
		// 如果任务正在运行，先停止
		if wasRunning {
			s.scheduler.StopJobs()
			s.scheduler.Shutdown()
			s.isRunning = false
			log.Printf("已停止旧定时任务")
		}

		// 如果新配置启用且之前在运行，重新启动
		if newConfig.Enabled && wasRunning {
			return s.startWithoutLock()
		}
	}

	return nil
}

// UpdateConfigAsync 异步更新配置（仅处理重型操作，数据库保存已在同步阶段完成）
func (s *SchedulerService) UpdateConfigAsync(oldConfig, newConfig *models.UserConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasRunning := s.isRunning
	needsRestart := s.needsTaskRestart(oldConfig, newConfig)

	// 记录配置更新情况
	if needsRestart {
		log.Printf("[异步配置] 检测到关键参数变化，需要重启定时任务")
		log.Printf("[异步配置] 配置差异：间隔 %d->%d秒, 启用 %v->%v",
			func() int {
				if oldConfig != nil {
					return oldConfig.Interval
				} else {
					return 0
				}
			}(),
			newConfig.Interval,
			func() bool {
				if oldConfig != nil {
					return oldConfig.Enabled
				} else {
					return false
				}
			}(),
			newConfig.Enabled)
	} else {
		log.Printf("[异步配置] 仅更新非关键参数，无需重启任务")
	}

	// 更新配置引用
	s.config = newConfig
	s.apiClient.UpdateCookie(newConfig.Cookie)

	// 更新自动调度配置（不直接触发任务启停）
	if s.autoScheduler != nil {
		s.autoScheduler.UpdateConfig(&newConfig.AutoSchedule)
	}

	// 只在必要时重启任务
	if needsRestart {
		// 如果任务正在运行，先停止
		if wasRunning {
			log.Printf("[异步配置] 停止旧定时任务...")
			s.scheduler.StopJobs()
			s.scheduler.Shutdown()
			s.isRunning = false
			log.Printf("[异步配置] 旧定时任务已停止")
		}

		// 如果新配置启用且之前在运行，重新启动
		if newConfig.Enabled && wasRunning {
			log.Printf("[异步配置] 重新启动定时任务...")
			return s.startWithoutLock()
		}
	}

	return nil
}

// UpdateConfigSync 同步更新配置（仅保存到数据库和更新内存配置，不进行重型操作）
func (s *SchedulerService) UpdateConfigSync(newConfig *models.UserConfig) error {
	// 获取当前配置的副本用于比较
	s.mu.Lock()
	var oldConfig *models.UserConfig
	if s.config != nil {
		// 创建旧配置的副本
		oldConfig = &models.UserConfig{
			DailyUsageEnabled: s.config.DailyUsageEnabled,
			// 只需要复制用于比较的字段
		}
	}
	s.mu.Unlock()

	// 仅保存配置到数据库
	if err := s.db.SaveConfig(newConfig); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 更新内存中的非重型配置
	s.mu.Lock()
	// 对于不需要重启任务的配置直接更新
	if s.config != nil {
		// 更新时间范围等不影响任务运行的配置
		s.config.TimeRange = newConfig.TimeRange
		// 更新每日积分统计配置
		s.config.DailyUsageEnabled = newConfig.DailyUsageEnabled
	}
	s.mu.Unlock()

	log.Printf("[同步配置] 配置已同步保存到数据库")
	
	// 处理每日积分统计配置变更
	s.handleDailyUsageConfigChange(oldConfig, newConfig)
	
	return nil
}

// startWithoutLock 无锁启动（内部使用）
func (s *SchedulerService) startWithoutLock() error {
	if s.config.Cookie == "" {
		return fmt.Errorf("Cookie未设置")
	}

	// 验证Cookie（通过获取积分余额隐式验证）
	if _, err := s.apiClient.FetchCreditBalance(); err != nil {
		return fmt.Errorf("cookie验证失败: %w", err)
	}

	// 创建新的调度器，确保任务配置是最新的
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("创建调度器失败: %w", err)
	}
	s.scheduler = scheduler
	log.Printf("已创建新的调度器实例")

	// 添加使用数据定时任务
	_, err = s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Second),
		gocron.NewTask(s.fetchAndSaveData),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建使用数据定时任务失败: %w", err)
	}

	// 添加积分余额定时任务，间隔错开30秒执行
	balanceJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Second),
		gocron.NewTask(s.fetchAndSaveBalance),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(
			gocron.WithStartDateTime(time.Now().Add(30*time.Second)),
		),
	)
	if err != nil {
		return fmt.Errorf("创建积分余额定时任务失败: %w", err)
	}

	// 保存积分余额任务引用
	s.balanceJob = balanceJob
	s.balanceTaskPaused = false

	log.Printf("使用数据定时任务已创建，间隔: %d秒", s.config.Interval)
	log.Printf("积分余额定时任务已创建，间隔: %d秒", s.config.Interval)

	s.scheduler.Start()
	s.isRunning = true

	log.Printf("定时任务已启动，间隔: %d秒", s.config.Interval)

	// 重启时不立即执行，等待定时任务自然触发

	return nil
}

// FetchDataManually 手动获取数据
func (s *SchedulerService) FetchDataManually() error {
	// 更新配置
	config, err := s.db.GetConfig()
	if err == nil {
		s.config = config
		s.apiClient.UpdateCookie(config.Cookie)
	}

	return s.fetchAndSaveData()
}

// FetchBalanceManually 手动获取积分余额
func (s *SchedulerService) FetchBalanceManually() error {
	// 更新配置
	config, err := s.db.GetConfig()
	if err == nil {
		s.config = config
		s.apiClient.UpdateCookie(config.Cookie)
	}

	return s.fetchAndSaveBalance()
}

// FetchAllDataManually 手动获取所有数据（使用数据 + 积分余额）
func (s *SchedulerService) FetchAllDataManually() error {
	// 更新配置（只需要更新一次）
	config, err := s.db.GetConfig()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	// 验证cookie是否已配置
	if config.Cookie == "" {
		return fmt.Errorf("Cookie未配置，请先设置Cookie")
	}

	s.config = config
	s.apiClient.UpdateCookie(config.Cookie)

	// 同时获取使用数据和积分余额
	// 使用goroutine并发获取，提高性能
	errChan := make(chan error, 2)

	go func() {
		errChan <- s.fetchAndSaveData()
	}()

	go func() {
		errChan <- s.fetchAndSaveBalance()
	}()

	// 等待两个任务完成
	var errors []error
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	// 如果有错误，返回第一个错误
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// ResetCreditsManually 手动重置积分（供自动重置服务调用）
func (s *SchedulerService) ResetCreditsManually() error {
	// 获取当前配置
	config, err := s.db.GetConfig()
	if err != nil {
		log.Printf("[手动重置] 获取配置失败: %v", err)
		return fmt.Errorf("获取配置失败: %w", err)
	}

	// 检查Cookie是否配置
	if config.Cookie == "" {
		log.Printf("[手动重置] Cookie未配置")
		return fmt.Errorf("Cookie未配置")
	}

	// 调用积分重置API
	apiClient := client.NewClaudeAPIClient(config.Cookie)
	resetSuccess, resetInfo, err := apiClient.ResetCredits()
	if err != nil {
		log.Printf("[手动重置] 调用重置积分API失败: %v", err)
		return fmt.Errorf("调用重置积分API失败: %w", err)
	}

	if !resetSuccess {
		log.Printf("[手动重置] 重置积分API返回失败")
		return fmt.Errorf("重置积分API返回失败")
	}

	// API调用成功后，标记今日已使用重置
	config.DailyResetUsed = true

	// 保存配置
	if err := s.db.SaveConfig(config); err != nil {
		log.Printf("[手动重置] 保存配置失败: %v", err)
		return fmt.Errorf("保存配置失败: %w", err)
	}

	log.Printf("[手动重置] 积分重置成功，已标记今日已使用重置。重置信息: %s", resetInfo)

	// 通知重置状态变化（SSE推送给前端）
	s.NotifyResetStatusChange(true)

	// 触发数据刷新，获取最新的积分余额
	// 延迟10秒后查询，确保服务端处理完重置操作
	go func() {
		time.Sleep(10 * time.Second)
		if err := s.FetchBalanceManually(); err != nil {
			log.Printf("[手动重置] 重置后刷新积分余额失败: %v", err)
		}
	}()

	return nil
}

// resetDailyFlags 重置每日标记（每天0点执行）
func (s *SchedulerService) resetDailyFlags() error {
	// 获取当前配置
	config, err := s.db.GetConfig()
	if err != nil {
		log.Printf("重置每日标记时获取配置失败: %v", err)
		return err
	}

	// 简单重置每日标记为false
	config.DailyResetUsed = false

	// 保存配置
	if err := s.db.SaveConfig(config); err != nil {
		log.Printf("重置每日标记时保存配置失败: %v", err)
		return err
	}

	log.Println("每日重置标记已重置为false")

	// 通过SSE推送重置状态变化到前端
	s.notifyResetStatusListeners(false)

	return nil
}

// fetchAndSaveData 获取并保存数据
func (s *SchedulerService) fetchAndSaveData() error {
	data, err := s.apiClient.FetchUsageData()
	if err != nil {
		log.Printf("获取数据失败: %v", err)
		// 通过SSE推送错误信息
		s.notifyErrorListeners(fmt.Sprintf("获取使用数据失败: %s", err.Error()))
		return err
	}

	// 更新最新数据并通知监听器
	s.mu.Lock()
	s.lastData = data
	s.mu.Unlock()

	s.notifyListeners(data)

	return nil
}

// fetchAndSaveBalance 获取并保存积分余额
func (s *SchedulerService) fetchAndSaveBalance() error {
	balance, err := s.apiClient.FetchCreditBalance()
	if err != nil {
		log.Printf("获取积分余额失败: %v", err)
		// 通过SSE推送错误信息
		s.notifyErrorListeners(fmt.Sprintf("获取积分余额失败: %s", err.Error()))
		return err
	}

	// 保存到BadgerDB（持久化存储）
	if err := s.db.SaveCreditBalance(balance); err != nil {
		log.Printf("保存积分余额到数据库失败: %v", err)
		// 注意：这里不返回错误，继续执行内存更新和通知
	}

	// 更新最新积分余额并通知监听器
	s.mu.Lock()
	s.lastBalance = balance
	s.mu.Unlock()

	s.notifyBalanceListeners(balance)

	return nil
}

// NotifyConfigUpdateError 通知配置更新错误
func (s *SchedulerService) NotifyConfigUpdateError(jobType, jobID, errorMsg string) {
	message := fmt.Sprintf("配置更新失败 [%s:%s]: %s", jobType, jobID, errorMsg)
	s.notifyErrorListeners(message)
}

// NotifyConfigUpdateSuccess 通知配置更新成功
func (s *SchedulerService) NotifyConfigUpdateSuccess(jobType, jobID string) {
	message := fmt.Sprintf("配置更新成功 [%s:%s]", jobType, jobID)
	log.Printf("[SSE通知] %s", message)
	// 成功消息可以通过其他机制通知，例如配置变更通知
	s.NotifyConfigChange()
}

// GetLatestData 获取最新数据
func (s *SchedulerService) GetLatestData() []models.UsageData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastData
}

// GetLatestBalance 获取最新积分余额
func (s *SchedulerService) GetLatestBalance() *models.CreditBalance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastBalance
}

// AddDataListener 添加数据监听器
func (s *SchedulerService) AddDataListener() chan []models.UsageData {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener := make(chan []models.UsageData, 10)
	s.listeners = append(s.listeners, listener)
	return listener
}

// AddBalanceListener 添加积分余额监听器
func (s *SchedulerService) AddBalanceListener() chan *models.CreditBalance {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener := make(chan *models.CreditBalance, 10)
	s.balanceListeners = append(s.balanceListeners, listener)
	return listener
}

// AddErrorListener 添加错误监听器
func (s *SchedulerService) AddErrorListener() chan string {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener := make(chan string, 10)
	s.errorListeners = append(s.errorListeners, listener)
	return listener
}

// AddResetStatusListener 添加重置状态监听器
func (s *SchedulerService) AddResetStatusListener() chan bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener := make(chan bool, 10)
	s.resetStatusListeners = append(s.resetStatusListeners, listener)
	return listener
}

// RemoveDataListener 移除数据监听器
func (s *SchedulerService) RemoveDataListener(listener chan []models.UsageData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, l := range s.listeners {
		if l == listener {
			close(l)
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			break
		}
	}
}

// RemoveBalanceListener 移除积分余额监听器
func (s *SchedulerService) RemoveBalanceListener(listener chan *models.CreditBalance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, l := range s.balanceListeners {
		if l == listener {
			close(l)
			s.balanceListeners = append(s.balanceListeners[:i], s.balanceListeners[i+1:]...)
			break
		}
	}
}

// RemoveErrorListener 移除错误监听器
func (s *SchedulerService) RemoveErrorListener(listener chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, l := range s.errorListeners {
		if l == listener {
			close(l)
			s.errorListeners = append(s.errorListeners[:i], s.errorListeners[i+1:]...)
			break
		}
	}
}

// RemoveResetStatusListener 移除重置状态监听器
func (s *SchedulerService) RemoveResetStatusListener(listener chan bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, l := range s.resetStatusListeners {
		if l == listener {
			close(l)
			s.resetStatusListeners = append(s.resetStatusListeners[:i], s.resetStatusListeners[i+1:]...)
			break
		}
	}
}

// notifyListeners 通知所有监听器
func (s *SchedulerService) notifyListeners(data []models.UsageData) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, listener := range s.listeners {
		select {
		case listener <- data:
			// 数据发送成功
		default:
			// 通道已满，跳过通知
		}
	}
}

// notifyBalanceListeners 通知所有积分余额监听器
func (s *SchedulerService) notifyBalanceListeners(balance *models.CreditBalance) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, listener := range s.balanceListeners {
		select {
		case listener <- balance:
			// 数据发送成功
		default:
			// 通道已满，跳过通知
		}
	}
}

// notifyErrorListeners 通知所有错误监听器
func (s *SchedulerService) notifyErrorListeners(errorMsg string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, listener := range s.errorListeners {
		select {
		case listener <- errorMsg:
			// 错误信息发送成功
		default:
			// 通道已满，跳过通知
		}
	}
}

// notifyResetStatusListeners 通知所有重置状态监听器
func (s *SchedulerService) notifyResetStatusListeners(resetStatus bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, listener := range s.resetStatusListeners {
		select {
		case listener <- resetStatus:
			// 重置状态发送成功
		default:
			// 通道已满，跳过通知
		}
	}
}

// NotifyResetStatusChange 通知重置状态变化（供外部调用）
func (s *SchedulerService) NotifyResetStatusChange(resetStatus bool) {
	s.notifyResetStatusListeners(resetStatus)
}

// NotifyConfigChange 通知配置更新（供外部调用）
func (s *SchedulerService) NotifyConfigChange() {
	// 获取最新数据并通知所有监听器
	data := s.GetLatestData()
	s.notifyListeners(data)
}

// StartAuto 自动调度启动监控（由自动调度服务调用）
func (s *SchedulerService) StartAuto() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		log.Printf("[自动调度] 监控已在运行，无需启动")
		return nil // 已经在运行
	}

	if s.config.Cookie == "" {
		log.Printf("[自动调度] 启动失败: Cookie未设置")
		return fmt.Errorf("Cookie未设置")
	}

	log.Printf("[自动调度] 正在启动监控任务...")
	// 启动监控任务
	err := s.startWithoutLock()
	if err != nil {
		log.Printf("[自动调度] 监控任务启动失败: %v", err)
	} else {
		log.Printf("[自动调度] 监控任务已成功启动")
	}
	return err
}

// StopAuto 自动调度停止监控（由自动调度服务调用）
func (s *SchedulerService) StopAuto() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		log.Printf("[自动调度] 监控已停止，无需操作")
		return nil // 已经停止
	}

	log.Printf("[自动调度] 正在停止监控任务...")
	// 停止监控任务
	s.scheduler.StopJobs()
	s.scheduler.Shutdown()
	s.isRunning = false

	log.Printf("[自动调度] 监控任务已成功停止")
	return nil
}

// IsAutoScheduleEnabled 检查是否启用了自动调度
func (s *SchedulerService) IsAutoScheduleEnabled() bool {
	if s.autoScheduler == nil {
		return false
	}
	return s.autoScheduler.IsEnabled()
}

// GetAutoScheduler 获取自动调度服务实例
func (s *SchedulerService) GetAutoScheduler() *AutoSchedulerService {
	return s.autoScheduler
}

// IsInAutoScheduleTimeRange 检查当前是否在自动调度时间范围内
func (s *SchedulerService) IsInAutoScheduleTimeRange() bool {
	if s.autoScheduler == nil {
		return false
	}
	return s.autoScheduler.IsInTimeRange()
}

// GetAutoScheduleConfig 获取自动调度配置
func (s *SchedulerService) GetAutoScheduleConfig() *models.AutoScheduleConfig {
	if s.autoScheduler == nil {
		return &models.AutoScheduleConfig{}
	}
	return s.autoScheduler.GetConfig()
}

// AddAutoScheduleListener 添加自动调度状态监听器
func (s *SchedulerService) AddAutoScheduleListener() chan bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener := make(chan bool, 10)
	s.autoScheduleListeners = append(s.autoScheduleListeners, listener)
	return listener
}

// RemoveAutoScheduleListener 移除自动调度状态监听器
func (s *SchedulerService) RemoveAutoScheduleListener(listener chan bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, l := range s.autoScheduleListeners {
		if l == listener {
			close(l)
			s.autoScheduleListeners = append(s.autoScheduleListeners[:i], s.autoScheduleListeners[i+1:]...)
			break
		}
	}
}

// NotifyAutoScheduleChange 通知自动调度状态变化（供自动调度服务调用）
func (s *SchedulerService) NotifyAutoScheduleChange() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	isEnabled := s.IsAutoScheduleEnabled()
	for _, listener := range s.autoScheduleListeners {
		select {
		case listener <- isEnabled:
			// 状态发送成功
		default:
			// 通道已满，跳过通知
		}
	}
}

// PauseBalanceTask 暂停积分余额获取任务
func (s *SchedulerService) PauseBalanceTask() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已经暂停
	if s.balanceTaskPaused {
		utils.Logf("[任务协调] ⚠️  积分获取任务已暂停，无需重复操作")
		return
	}

	// 检查任务是否存在
	if s.balanceJob == nil {
		utils.Logf("[任务协调] ⚠️  积分获取任务不存在，更新暂停状态")
		s.balanceTaskPaused = true
		return
	}

	// 检查调度器状态
	if s.scheduler == nil || !s.isRunning {
		utils.Logf("[任务协调] ⚠️  调度器未运行，直接更新暂停状态")
		s.balanceTaskPaused = true
		s.balanceJob = nil
		return
	}

	utils.Logf("[任务协调] ⏸️  暂停积分余额获取任务 (ID: %v)", s.balanceJob.ID())
	if err := s.scheduler.RemoveJob(s.balanceJob.ID()); err != nil {
		utils.Logf("[任务协调] ❌ 暂停积分任务失败: %v", err)
		// 即使失败也要清理本地状态
		s.balanceJob = nil
		s.balanceTaskPaused = true
	} else {
		s.balanceTaskPaused = true
		s.balanceJob = nil
		utils.Logf("[任务协调] ✅ 积分余额获取任务已暂停")
	}
}

// RebuildBalanceTask 重建积分余额获取任务（移除+重建策略）
func (s *SchedulerService) RebuildBalanceTask() {
	s.mu.Lock()
	defer s.mu.Unlock()

	utils.Logf("[任务协调] 🔄 重建积分余额获取任务")

	// 第一步：移除现有任务
	if s.balanceJob != nil {
		utils.Logf("[任务协调] 🗑️  移除现有积分任务 (ID: %v)", s.balanceJob.ID())
		if s.scheduler != nil {
			if err := s.scheduler.RemoveJob(s.balanceJob.ID()); err != nil {
				utils.Logf("[任务协调] ⚠️  移除积分任务失败: %v", err)
			}
		}
		s.balanceJob = nil
	}

	// 第二步：检查调度器状态，如果异常则重建整个调度器
	if s.scheduler == nil || !s.isRunning {
		utils.Logf("[任务协调] 🔧 检测到调度器异常，尝试重建调度器")
		if err := s.rebuildScheduler(); err != nil {
			utils.Logf("[任务协调] ❌ 重建调度器失败: %v", err)
			s.balanceTaskPaused = false
			return
		}
	}

	// 第三步：创建新的积分任务
	utils.Logf("[任务协调] 🔨 创建新的积分余额获取任务")
	balanceJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Second),
		gocron.NewTask(s.fetchAndSaveBalance),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartDateTime(time.Now().Add(5*time.Second))), // 缩短延迟到5秒
	)
	if err != nil {
		utils.Logf("[任务协调] ❌ 创建积分任务失败: %v", err)
		s.balanceTaskPaused = false
		return
	}

	s.balanceJob = balanceJob
	s.balanceTaskPaused = false
	utils.Logf("[任务协调] ✅ 积分余额获取任务已重建 (ID: %v)", balanceJob.ID())

	// 第四步：立即执行一次获取，避免等待
	go func() {
		time.Sleep(1 * time.Second) // 短暂延迟确保任务已就绪
		utils.Logf("[任务协调] 🚀 立即执行积分余额获取")
		if err := s.fetchAndSaveBalance(); err != nil {
			utils.Logf("[任务协调] ⚠️  立即执行积分获取失败: %v", err)
		}
	}()
}

// ResumeBalanceTask 恢复积分余额获取任务（优化版本）
func (s *SchedulerService) ResumeBalanceTask() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已经在运行
	if !s.balanceTaskPaused {
		utils.Logf("[任务协调] ⚠️  积分获取任务未暂停，无需恢复")
		return
	}

	utils.Logf("[任务协调] ▶️  恢复积分余额获取任务")

	// 检查是否已经存在任务（防止重复创建）
	if s.balanceJob != nil {
		utils.Logf("[任务协调] ⚠️  积分获取任务已存在 (ID: %v)，更新状态", s.balanceJob.ID())
		s.balanceTaskPaused = false
		return
	}

	// 如果调度器不存在或未运行，使用重建策略
	if s.scheduler == nil || !s.isRunning {
		utils.Logf("[任务协调] 🔧 调度器状态异常，采用重建策略")
		s.mu.Unlock() // 临时释放锁
		s.RebuildBalanceTask()
		s.mu.Lock() // 重新获取锁
		return
	}

	// 重新创建积分余额任务
	utils.Logf("[任务协调] 🔨 重新创建积分余额获取任务")
	balanceJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Second),
		gocron.NewTask(s.fetchAndSaveBalance),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartDateTime(time.Now().Add(5*time.Second))), // 缩短延迟到5秒
	)
	if err != nil {
		utils.Logf("[任务协调] ❌ 恢复积分任务失败: %v", err)
		s.balanceTaskPaused = false
		return
	}

	s.balanceJob = balanceJob
	s.balanceTaskPaused = false
	utils.Logf("[任务协调] ✅ 积分余额获取任务已恢复 (ID: %v)", balanceJob.ID())

	// 立即执行一次获取
	go func() {
		time.Sleep(1 * time.Second)
		utils.Logf("[任务协调] 🚀 立即执行积分余额获取")
		if err := s.fetchAndSaveBalance(); err != nil {
			utils.Logf("[任务协调] ⚠️  立即执行积分获取失败: %v", err)
		}
	}()
}

// IsBalanceTaskRunning 检查积分余额获取任务是否正在运行
func (s *SchedulerService) IsBalanceTaskRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 检查基本状态
	if s.balanceTaskPaused || s.balanceJob == nil || !s.isRunning {
		return false
	}

	// 检查调度器状态
	if s.scheduler == nil {
		return false
	}

	return true
}

// NotifyBalanceUpdate 通知积分余额更新（供阈值触发任务调用）
func (s *SchedulerService) NotifyBalanceUpdate(balance *models.CreditBalance) {
	// 保存到BadgerDB（持久化存储）
	if err := s.db.SaveCreditBalance(balance); err != nil {
		log.Printf("保存积分余额到数据库失败: %v", err)
		// 注意：这里不返回错误，继续执行内存更新和通知
	}

	s.mu.Lock()
	s.lastBalance = balance
	s.mu.Unlock()

	s.notifyBalanceListeners(balance)
	utils.Logf("[任务协调] 📡 积分余额已更新并推送: %d", balance.Remaining)
}

// Shutdown 关闭服务
func (s *SchedulerService) Shutdown() {
	s.Stop()

	// 关闭每日重置调度器
	if s.dailyResetScheduler != nil {
		s.dailyResetScheduler.StopJobs()
		s.dailyResetScheduler.Shutdown()
		log.Printf("每日重置调度器已关闭")
	}

	// 关闭自动调度服务
	if s.autoScheduler != nil {
		s.autoScheduler.Close()
	}

	// 关闭每日积分统计服务
	if s.dailyUsageTracker != nil {
		s.dailyUsageTracker.Shutdown()
	}

	// 关闭所有监听器
	s.mu.Lock()
	for _, listener := range s.listeners {
		close(listener)
	}
	for _, listener := range s.balanceListeners {
		close(listener)
	}
	for _, listener := range s.errorListeners {
		close(listener)
	}
	for _, listener := range s.resetStatusListeners {
		close(listener)
	}
	for _, listener := range s.autoScheduleListeners {
		close(listener)
	}
	for _, listener := range s.dailyUsageListeners {
		close(listener)
	}
	s.listeners = nil
	s.balanceListeners = nil
	s.errorListeners = nil
	s.resetStatusListeners = nil
	s.autoScheduleListeners = nil
	s.dailyUsageListeners = nil
	s.mu.Unlock()
}

// rebuildScheduler 重建调度器（内部方法）
func (s *SchedulerService) rebuildScheduler() error {
	utils.Logf("[任务协调] 🔄 重建调度器")

	// 停止并关闭现有调度器
	if s.scheduler != nil {
		s.scheduler.StopJobs()
		if err := s.scheduler.Shutdown(); err != nil {
			utils.Logf("[任务协调] ⚠️  关闭旧调度器失败: %v", err)
		}
	}

	// 创建新调度器
	newScheduler, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("创建新调度器失败: %w", err)
	}

	s.scheduler = newScheduler

	// 重新创建使用数据任务
	usageJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Second),
		gocron.NewTask(s.fetchAndSaveData),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建使用数据任务失败: %w", err)
	}

	// 启动调度器
	s.scheduler.Start()
	s.isRunning = true

	utils.Logf("[任务协调] ✅ 调度器重建完成，使用数据任务ID: %v", usageJob.ID())
	return nil
}

// SetAutoResetService 设置自动重置服务引用（用于任务协调）
func (s *SchedulerService) SetAutoResetService(autoResetService *AutoResetService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoResetService = autoResetService
}

// handleDailyUsageConfigChange 处理每日积分统计配置变更
func (s *SchedulerService) handleDailyUsageConfigChange(oldConfig, newConfig *models.UserConfig) {
	if s.dailyUsageTracker == nil {
		utils.Logf("[配置更新] ⚠️  每日积分统计服务为空，跳过配置变更")
		return
	}

	oldEnabled := oldConfig != nil && oldConfig.DailyUsageEnabled
	newEnabled := newConfig.DailyUsageEnabled
	
	utils.Logf("[配置更新] 🔄 检查每日积分统计配置变更: %v -> %v", oldEnabled, newEnabled)

	// 配置没有变化，无需处理
	if oldEnabled == newEnabled {
		utils.Logf("[配置更新] ℹ️  每日积分统计配置无变化，跳过处理")
		return
	}

	if newEnabled {
		// 启用每日积分统计任务
		if !s.dailyUsageTracker.IsActive() {
			if err := s.dailyUsageTracker.Start(); err != nil {
				utils.Logf("[配置更新] ❌ 启用每日积分统计任务失败: %v", err)
			} else {
				utils.Logf("[配置更新] ✅ 每日积分统计任务已启用")
			}
		} else {
			utils.Logf("[配置更新] ℹ️  每日积分统计任务已在运行中")
		}
	} else {
		// 停止每日积分统计任务
		if s.dailyUsageTracker.IsActive() {
			if err := s.dailyUsageTracker.Stop(); err != nil {
				utils.Logf("[配置更新] ❌ 停止每日积分统计任务失败: %v", err)
			} else {
				utils.Logf("[配置更新] ✅ 每日积分统计任务已停止")
			}
		} else {
			utils.Logf("[配置更新] ℹ️  每日积分统计任务已停止")
		}
	}
}

// GetDailyUsageTracker 获取每日积分统计服务引用
func (s *SchedulerService) GetDailyUsageTracker() *DailyUsageTracker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dailyUsageTracker
}

// GetWeeklyUsage 获取最近一周的积分使用统计
func (s *SchedulerService) GetWeeklyUsage() (models.DailyUsageList, error) {
	s.mu.RLock()
	tracker := s.dailyUsageTracker
	s.mu.RUnlock()

	if tracker == nil {
		return nil, fmt.Errorf("每日积分统计服务未初始化")
	}

	return tracker.GetWeeklyUsage()
}

// GetConfig 获取当前配置
func (s *SchedulerService) GetConfig() *models.UserConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// AddDailyUsageListener 添加每日积分统计监听器
func (s *SchedulerService) AddDailyUsageListener() chan []models.DailyUsage {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener := make(chan []models.DailyUsage, 10)
	s.dailyUsageListeners = append(s.dailyUsageListeners, listener)
	return listener
}

// RemoveDailyUsageListener 移除每日积分统计监听器
func (s *SchedulerService) RemoveDailyUsageListener(listener chan []models.DailyUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, l := range s.dailyUsageListeners {
		if l == listener {
			close(l)
			s.dailyUsageListeners = append(s.dailyUsageListeners[:i], s.dailyUsageListeners[i+1:]...)
			break
		}
	}
}

// BroadcastDailyUsage 广播每日积分统计数据
func (s *SchedulerService) BroadcastDailyUsage(data []models.DailyUsage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, listener := range s.dailyUsageListeners {
		select {
		case listener <- data:
			// 数据发送成功
		default:
			// 通道已满，跳过通知
		}
	}
}
