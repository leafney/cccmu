package services

import (
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
	scheduler gocron.Scheduler
	db        *database.BadgerDB
	apiClient *client.ClaudeAPIClient
	config    *models.UserConfig
	isRunning bool
	mu        sync.RWMutex
	lastData  []models.UsageData
	listeners []chan []models.UsageData
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

	return &SchedulerService{
		scheduler: scheduler,
		db:        db,
		apiClient: client.NewClaudeAPIClient(config.Cookie),
		config:    config,
		isRunning: false,
		listeners: make([]chan []models.UsageData, 0),
	}, nil
}

// Start 启动定时任务
func (s *SchedulerService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已经在运行，先停止再重新启动
	if s.isRunning {
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

	// 添加定时任务
	job, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Minute),
		gocron.NewTask(s.fetchAndSaveData),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建定时任务失败: %w", err)
	}
	
	log.Printf("定时任务创建成功，任务ID: %v，间隔: %d分钟", job.ID(), s.config.Interval)

	// 启动调度器
	s.scheduler.Start()
	s.isRunning = true

	log.Printf("定时任务已启动，间隔: %d分钟", s.config.Interval)

	// 立即执行一次，确保在所有监听器建立后执行
	go func() {
		time.Sleep(100 * time.Millisecond) // 短暂延迟，确保SSE连接已建立
		s.fetchAndSaveData()
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

	err := s.scheduler.Shutdown()
	if err != nil {
		return fmt.Errorf("停止调度器失败: %w", err)
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
		s.scheduler.Shutdown()
		s.isRunning = false
	}

	s.config = newConfig
	s.apiClient.UpdateCookie(newConfig.Cookie)

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

	// 添加定时任务
	job, err := s.scheduler.NewJob(
		gocron.DurationJob(time.Duration(s.config.Interval)*time.Minute),
		gocron.NewTask(s.fetchAndSaveData),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("创建定时任务失败: %w", err)
	}
	
	log.Printf("定时任务重建成功，任务ID: %v，间隔: %d分钟", job.ID(), s.config.Interval)

	s.scheduler.Start()
	s.isRunning = true

	log.Printf("定时任务已重启，间隔: %d分钟", s.config.Interval)
	
	// 立即执行一次
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.fetchAndSaveData()
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

// fetchAndSaveData 获取并保存数据
func (s *SchedulerService) fetchAndSaveData() error {
	log.Printf("定时任务触发: 开始获取积分使用数据... (当前监听器数量: %d)", len(s.listeners))

	data, err := s.apiClient.FetchUsageData()
	if err != nil {
		log.Printf("获取数据失败: %v", err)
		return err
	}

	// 直接通知监听器，不保存到数据库
	log.Printf("获取到 %d 条积分数据，直接推送给前端", len(data))
	
	// 打印前3条数据的详细信息用于调试
	for i, item := range data {
		if i < 3 {
			log.Printf("数据[%d]: ID=%d, 积分=%d, 时间=%s, 模型=%s", i, item.ID, item.CreditsUsed, item.CreatedAt, item.Model)
		}
	}

	// 更新最新数据并通知监听器
	s.mu.Lock()
	s.lastData = data
	currentListeners := len(s.listeners)
	s.mu.Unlock()

	log.Printf("开始通知 %d 个监听器", currentListeners)
	s.notifyListeners(data)
	log.Printf("数据推送完成")

	return nil
}

// GetLatestData 获取最新数据
func (s *SchedulerService) GetLatestData() []models.UsageData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastData
}

// AddDataListener 添加数据监听器
func (s *SchedulerService) AddDataListener() chan []models.UsageData {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener := make(chan []models.UsageData, 10)
	s.listeners = append(s.listeners, listener)
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

// notifyListeners 通知所有监听器
func (s *SchedulerService) notifyListeners(data []models.UsageData) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log.Printf("开始向 %d 个监听器发送数据", len(s.listeners))
	
	for i, listener := range s.listeners {
		select {
		case listener <- data:
			log.Printf("成功向监听器 %d 发送数据", i)
		default:
			log.Printf("监听器 %d 通道已满，跳过通知", i)
		}
	}
	
	log.Printf("完成向所有监听器发送数据")
}

// Shutdown 关闭服务
func (s *SchedulerService) Shutdown() {
	s.Stop()
	
	// 关闭所有监听器
	s.mu.Lock()
	for _, listener := range s.listeners {
		close(listener)
	}
	s.listeners = nil
	s.mu.Unlock()
}