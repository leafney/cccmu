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
	scheduler       gocron.Scheduler
	db              *database.BadgerDB
	apiClient       *client.ClaudeAPIClient
	config          *models.UserConfig
	isRunning       bool
	mu              sync.RWMutex
	lastData        []models.UsageData
	listeners       []chan []models.UsageData
	lastBalance     *models.CreditBalance
	balanceListeners []chan *models.CreditBalance
}

// NewSchedulerService 创建新的调度服务
func NewSchedulerService(db *database.BadgerDB) (*SchedulerService, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("创建调度器失败: %w", err)
	}

	config, err := db.GetConfig()
	if err != nil {
		log.Printf("获取配置失败，使用默认配置: %v", err)
		config = models.GetDefaultConfig()
	}

	apiClient := client.NewClaudeAPIClient(config.Cookie)
	
	service := &SchedulerService{
		scheduler:        scheduler,
		db:               db,
		apiClient:        apiClient,
		config:           config,
		isRunning:        false,
		listeners:        make([]chan []models.UsageData, 0),
		balanceListeners: make([]chan *models.CreditBalance, 0),
	}

	// 设置API客户端的Cookie更新回调
	apiClient.SetCookieUpdateCallback(func() {
		if err := db.UpdateCookieValidTime(); err != nil {
			log.Printf("更新Cookie验证时间失败: %v", err)
		}
	})

	return service, nil
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

	// 验证Cookie
	s.apiClient.UpdateCookie(s.config.Cookie)
	if err := s.apiClient.ValidateCookie(); err != nil {
		return fmt.Errorf("Cookie验证失败: %w", err)
	}

	// 添加使用数据定时任务
	usageJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Minute),
		gocron.NewTask(s.fetchAndSaveData),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建使用数据定时任务失败: %w", err)
	}
	
	// 添加积分余额定时任务，间隔错开30秒执行
	balanceJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Minute),
		gocron.NewTask(s.fetchAndSaveBalance),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(
			gocron.WithStartDateTime(time.Now().Add(30*time.Second)),
		),
	)
	if err != nil {
		return fmt.Errorf("创建积分余额定时任务失败: %w", err)
	}
	
	log.Printf("使用数据定时任务创建成功，任务ID: %v，间隔: %d分钟", usageJob.ID(), s.config.Interval)
	log.Printf("积分余额定时任务创建成功，任务ID: %v，间隔: %d分钟", balanceJob.ID(), s.config.Interval)

	// 添加Cookie验证定时任务
	cookieValidationJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.CookieValidationInterval)*time.Minute),
		gocron.NewTask(s.validateCookieIfNeeded),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(
			gocron.WithStartDateTime(time.Now().Add(1*time.Minute)), // 1分钟后开始执行
		),
	)
	if err != nil {
		return fmt.Errorf("创建Cookie验证定时任务失败: %w", err)
	}

	log.Printf("Cookie验证定时任务创建成功，任务ID: %v，间隔: %d分钟", cookieValidationJob.ID(), s.config.CookieValidationInterval)

	// 启动调度器
	s.scheduler.Start()
	s.isRunning = true

	log.Printf("定时任务已启动，间隔: %d分钟", s.config.Interval)

	// 立即执行一次，确保在所有监听器建立后执行
	go func() {
		time.Sleep(100 * time.Millisecond) // 短暂延迟，确保SSE连接已建立
		s.fetchAndSaveData()
		// 延迟1秒后获取积分余额，避免并发
		time.Sleep(1 * time.Second)
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

// UpdateConfig 更新配置并重启任务
func (s *SchedulerService) UpdateConfig(newConfig *models.UserConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 保存配置到数据库
	if err := s.db.SaveConfig(newConfig); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	wasRunning := s.isRunning

	// 如果任务正在运行，先停止
	if s.isRunning {
		s.scheduler.StopJobs()
		s.scheduler.Shutdown()
		s.isRunning = false
	}

	s.config = newConfig
	s.apiClient.UpdateCookie(newConfig.Cookie)
	
	// 重新设置Cookie更新回调
	s.apiClient.SetCookieUpdateCallback(func() {
		if err := s.db.UpdateCookieValidTime(); err != nil {
			log.Printf("更新Cookie验证时间失败: %v", err)
		}
	})

	// 如果新配置启用且之前在运行，重新启动
	if newConfig.Enabled && wasRunning {
		return s.startWithoutLock()
	}

	return nil
}

// startWithoutLock 无锁启动（内部使用）
func (s *SchedulerService) startWithoutLock() error {
	if s.config.Cookie == "" {
		return fmt.Errorf("Cookie未设置")
	}

	// 验证Cookie
	if err := s.apiClient.ValidateCookie(); err != nil {
		return fmt.Errorf("Cookie验证失败: %w", err)
	}

	// 创建新的调度器
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("创建调度器失败: %w", err)
	}
	s.scheduler = scheduler

	// 添加使用数据定时任务
	usageJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Minute),
		gocron.NewTask(s.fetchAndSaveData),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建使用数据定时任务失败: %w", err)
	}
	
	// 添加积分余额定时任务，间隔错开30秒执行
	balanceJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Minute),
		gocron.NewTask(s.fetchAndSaveBalance),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(
			gocron.WithStartDateTime(time.Now().Add(30*time.Second)),
		),
	)
	if err != nil {
		return fmt.Errorf("创建积分余额定时任务失败: %w", err)
	}
	
	log.Printf("使用数据定时任务重建成功，任务ID: %v，间隔: %d分钟", usageJob.ID(), s.config.Interval)
	log.Printf("积分余额定时任务重建成功，任务ID: %v，间隔: %d分钟", balanceJob.ID(), s.config.Interval)

	// 添加Cookie验证定时任务
	cookieValidationJob, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.CookieValidationInterval)*time.Minute),
		gocron.NewTask(s.validateCookieIfNeeded),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(
			gocron.WithStartDateTime(time.Now().Add(1*time.Minute)), // 1分钟后开始执行
		),
	)
	if err != nil {
		return fmt.Errorf("重建Cookie验证定时任务失败: %w", err)
	}

	log.Printf("Cookie验证定时任务重建成功，任务ID: %v，间隔: %d分钟", cookieValidationJob.ID(), s.config.CookieValidationInterval)

	s.scheduler.Start()
	s.isRunning = true

	log.Printf("定时任务已重启，间隔: %d分钟", s.config.Interval)
	
	// 立即执行一次
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.fetchAndSaveData()
		// 延迟1秒后获取积分余额，避免并发
		time.Sleep(1 * time.Second)
		s.fetchAndSaveBalance()
	}()
	
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

// fetchAndSaveData 获取并保存数据
func (s *SchedulerService) fetchAndSaveData() error {
	data, err := s.apiClient.FetchUsageData()
	if err != nil {
		log.Printf("获取数据失败: %v", err)
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

// validateCookieIfNeeded 根据时间戳判断是否需要验证Cookie
func (s *SchedulerService) validateCookieIfNeeded() error {
	shouldValidate, err := s.db.ShouldValidateCookie()
	if err != nil {
		log.Printf("检查Cookie验证需求失败: %v", err)
		return err
	}

	if !shouldValidate {
		log.Println("Cookie验证: 无需验证，跳过")
		return nil
	}

	log.Println("Cookie验证: 开始验证Cookie有效性")
	
	// 更新配置，确保使用最新的Cookie
	config, err := s.db.GetConfig()
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return err
	}
	
	if config.Cookie == "" {
		log.Println("Cookie验证: Cookie为空，跳过验证")
		return nil
	}

	s.apiClient.UpdateCookie(config.Cookie)
	
	// 执行Cookie验证
	if err := s.apiClient.ValidateCookie(); err != nil {
		log.Printf("Cookie验证失败: %v", err)
		// 这里将在下一个任务中实现失败处理逻辑
		return s.handleCookieValidationFailure(err)
	}

	log.Println("Cookie验证: 验证成功")
	return nil
}

// handleCookieValidationFailure 处理Cookie验证失败
func (s *SchedulerService) handleCookieValidationFailure(err error) error {
	log.Printf("Cookie验证失败，开始执行失败处理流程: %v", err)
	
	// 1. 停止当前的定时任务
	s.mu.Lock()
	if s.isRunning {
		log.Println("Cookie失效: 停止所有定时任务")
		if stopErr := s.scheduler.StopJobs(); stopErr != nil {
			log.Printf("停止定时任务失败: %v", stopErr)
		}
		s.isRunning = false
	}
	s.mu.Unlock()

	// 2. 获取当前配置
	config, configErr := s.db.GetConfig()
	if configErr != nil {
		log.Printf("获取配置失败: %v", configErr)
		config = models.GetDefaultConfig()
	}

	// 3. 清除Cookie并禁用任务
	config.Cookie = ""
	config.Enabled = false
	config.LastCookieValidTime = time.Now() // 记录失败时间

	// 4. 保存更新后的配置
	if saveErr := s.db.SaveConfig(config); saveErr != nil {
		log.Printf("保存配置失败: %v", saveErr)
	} else {
		log.Println("Cookie失效: 已清除Cookie并禁用任务")
	}

	// 5. 更新内存中的配置
	s.mu.Lock()
	s.config = config
	s.mu.Unlock()

	log.Println("Cookie失效处理完成，用户需要重新配置Cookie并启用任务")
	return fmt.Errorf("Cookie已失效，任务已停止，请重新配置Cookie: %w", err)
}

// Shutdown 关闭服务
func (s *SchedulerService) Shutdown() {
	s.Stop()
	
	// 关闭所有监听器
	s.mu.Lock()
	for _, listener := range s.listeners {
		close(listener)
	}
	for _, listener := range s.balanceListeners {
		close(listener)
	}
	s.listeners = nil
	s.balanceListeners = nil
	s.mu.Unlock()
}