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
)

// SchedulerService 定时任务服务
type SchedulerService struct {
	scheduler        gocron.Scheduler
	dailyResetScheduler gocron.Scheduler // 单独的每日重置任务调度器
	db               *database.BadgerDB
	apiClient        *client.ClaudeAPIClient
	config           *models.UserConfig
	isRunning        bool
	mu               sync.RWMutex
	lastData         []models.UsageData
	listeners        []chan []models.UsageData
	lastBalance      *models.CreditBalance
	balanceListeners []chan *models.CreditBalance
	errorListeners   []chan string
	resetStatusListeners []chan bool
	autoScheduler    *AutoSchedulerService
	autoScheduleListeners []chan bool // 自动调度状态变化监听器
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
		scheduler:        scheduler,
		dailyResetScheduler: dailyResetScheduler,
		db:               db,
		apiClient:        apiClient,
		config:           config,
		isRunning:        false,
		listeners:        make([]chan []models.UsageData, 0),
		balanceListeners: make([]chan *models.CreditBalance, 0),
		errorListeners:   make([]chan string, 0),
		resetStatusListeners: make([]chan bool, 0),
		autoScheduleListeners: make([]chan bool, 0),
	}
	
	// 创建自动调度服务
	service.autoScheduler = NewAutoSchedulerService(service)

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
	if _, err := s.apiClient.FetchCreditBalance(); err != nil {
		return fmt.Errorf("Cookie验证失败: %w", err)
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

	log.Printf("使用数据定时任务创建成功，任务ID: %v，间隔: %d秒", usageJob.ID(), s.config.Interval)
	log.Printf("积分余额定时任务创建成功，任务ID: %v，间隔: %d秒", balanceJob.ID(), s.config.Interval)

	// 启动调度器
	s.scheduler.Start()
	s.isRunning = true

	log.Printf("定时任务已启动，间隔: %d秒", s.config.Interval)

	// 立即执行一次，确保在所有监听器建立后执行
	go func() {
		time.Sleep(100 * time.Millisecond) // 短暂延迟，确保SSE连接已建立
		s.fetchAndSaveData()
		// 延迟5秒后获取积分余额，避免并发
		time.Sleep(5 * time.Second)
		s.fetchAndSaveBalance()
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

// needsTaskRestart 检查配置更新是否需要重启定时任务
func (s *SchedulerService) needsTaskRestart(oldConfig, newConfig *models.UserConfig) bool {
	if oldConfig == nil {
		return newConfig.Enabled // 首次配置，根据是否启用决定
	}
	
	// 检查影响定时任务的关键配置项
	return oldConfig.Interval != newConfig.Interval || // 监控间隔变化
		   oldConfig.Cookie != newConfig.Cookie ||     // Cookie变化
		   oldConfig.Enabled != newConfig.Enabled     // 启用状态变化
}

// UpdateConfig 更新配置并按需重启任务  
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
			func() int { if oldConfig != nil { return oldConfig.Interval } else { return 0 } }(),
			newConfig.Interval,
			func() bool { if oldConfig != nil { return oldConfig.Enabled } else { return false } }(),
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

// startWithoutLock 无锁启动（内部使用）
func (s *SchedulerService) startWithoutLock() error {
	if s.config.Cookie == "" {
		return fmt.Errorf("Cookie未设置")
	}

	// 验证Cookie（通过获取积分余额隐式验证）
	if _, err := s.apiClient.FetchCreditBalance(); err != nil {
		return fmt.Errorf("Cookie验证失败: %w", err)
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
	_, err = s.scheduler.NewJob(
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

	// 更新最新积分余额并通知监听器
	s.mu.Lock()
	s.lastBalance = balance
	s.mu.Unlock()

	s.notifyBalanceListeners(balance)

	return nil
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
	s.listeners = nil
	s.balanceListeners = nil
	s.errorListeners = nil
	s.resetStatusListeners = nil
	s.autoScheduleListeners = nil
	s.mu.Unlock()
}
