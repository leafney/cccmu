package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
)

// ConfigUpdateJobType 配置更新任务类型
type ConfigUpdateJobType string

const (
	JobTypeScheduler     ConfigUpdateJobType = "scheduler"
	JobTypeAutoSchedule  ConfigUpdateJobType = "auto_schedule"
	JobTypeAutoReset     ConfigUpdateJobType = "auto_reset"
)

// ConfigUpdateJob 配置更新任务
type ConfigUpdateJob struct {
	ID        string              `json:"id"`
	Type      ConfigUpdateJobType `json:"type"`
	OldConfig interface{}         `json:"-"`
	NewConfig interface{}         `json:"-"`
	CreatedAt time.Time           `json:"created_at"`
}

// AsyncConfigUpdater 异步配置更新服务
type AsyncConfigUpdater struct {
	jobQueue    chan ConfigUpdateJob
	workers     int
	scheduler   *SchedulerService
	autoScheduler *AutoSchedulerService
	autoResetService *AutoResetService
	db          *database.BadgerDB
	
	// 错误通知回调
	onError     func(jobType ConfigUpdateJobType, jobID string, err error)
	onSuccess   func(jobType ConfigUpdateJobType, jobID string)
	
	// 运行状态
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	isRunning   bool
	mu          sync.RWMutex
}

// NewAsyncConfigUpdater 创建异步配置更新服务
func NewAsyncConfigUpdater(
	scheduler *SchedulerService,
	autoScheduler *AutoSchedulerService, 
	autoResetService *AutoResetService,
	db *database.BadgerDB,
) *AsyncConfigUpdater {
	ctx, cancel := context.WithCancel(context.Background())
	
	updater := &AsyncConfigUpdater{
		jobQueue:         make(chan ConfigUpdateJob, 100), // 缓冲100个任务
		workers:          3, // 3个工作协程
		scheduler:        scheduler,
		autoScheduler:    autoScheduler,
		autoResetService: autoResetService,
		db:              db,
		ctx:             ctx,
		cancel:          cancel,
		isRunning:       false,
	}
	
	// 设置SSE错误和成功通知回调
	updater.SetErrorCallback(func(jobType ConfigUpdateJobType, jobID string, err error) {
		if scheduler != nil {
			scheduler.NotifyConfigUpdateError(string(jobType), jobID, err.Error())
		}
	})
	
	updater.SetSuccessCallback(func(jobType ConfigUpdateJobType, jobID string) {
		if scheduler != nil {
			scheduler.NotifyConfigUpdateSuccess(string(jobType), jobID)
		}
	})
	
	return updater
}

// SetErrorCallback 设置错误回调
func (a *AsyncConfigUpdater) SetErrorCallback(callback func(ConfigUpdateJobType, string, error)) {
	a.onError = callback
}

// SetSuccessCallback 设置成功回调  
func (a *AsyncConfigUpdater) SetSuccessCallback(callback func(ConfigUpdateJobType, string)) {
	a.onSuccess = callback
}

// Start 启动异步更新服务
func (a *AsyncConfigUpdater) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.isRunning {
		return fmt.Errorf("异步配置更新服务已在运行中")
	}
	
	// 启动工作协程
	for i := 0; i < a.workers; i++ {
		a.wg.Add(1)
		go a.worker(i)
	}
	
	a.isRunning = true
	log.Printf("[异步配置] 异步配置更新服务已启动，工作协程数: %d", a.workers)
	return nil
}

// Stop 停止异步更新服务
func (a *AsyncConfigUpdater) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if !a.isRunning {
		return nil
	}
	
	log.Printf("[异步配置] 正在停止异步配置更新服务...")
	
	// 取消上下文，通知所有工作协程退出
	a.cancel()
	
	// 关闭任务队列
	close(a.jobQueue)
	
	// 等待所有工作协程完成
	a.wg.Wait()
	
	a.isRunning = false
	log.Printf("[异步配置] 异步配置更新服务已停止")
	return nil
}

// SubmitJob 提交配置更新任务
func (a *AsyncConfigUpdater) SubmitJob(jobType ConfigUpdateJobType, oldConfig, newConfig interface{}) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	if !a.isRunning {
		return "", fmt.Errorf("异步配置更新服务未运行")
	}
	
	jobID := fmt.Sprintf("%s_%d", string(jobType), time.Now().UnixNano())
	job := ConfigUpdateJob{
		ID:        jobID,
		Type:      jobType,
		OldConfig: oldConfig,
		NewConfig: newConfig,
		CreatedAt: time.Now(),
	}
	
	select {
	case a.jobQueue <- job:
		log.Printf("[异步配置] 任务已提交: %s (类型: %s)", jobID, jobType)
		return jobID, nil
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("提交任务超时，任务队列可能已满")
	}
}

// worker 工作协程
func (a *AsyncConfigUpdater) worker(workerID int) {
	defer a.wg.Done()
	
	log.Printf("[异步配置] 工作协程 #%d 已启动", workerID)
	
	for {
		select {
		case <-a.ctx.Done():
			log.Printf("[异步配置] 工作协程 #%d 收到退出信号", workerID)
			return
		case job, ok := <-a.jobQueue:
			if !ok {
				log.Printf("[异步配置] 工作协程 #%d 任务队列已关闭", workerID)
				return
			}
			
			log.Printf("[异步配置] 工作协程 #%d 开始处理任务: %s", workerID, job.ID)
			a.processJob(workerID, job)
		}
	}
}

// processJob 处理配置更新任务
func (a *AsyncConfigUpdater) processJob(workerID int, job ConfigUpdateJob) {
	startTime := time.Now()
	
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("任务处理发生panic: %v", r)
			log.Printf("[异步配置] 工作协程 #%d 任务 %s 发生panic: %v", workerID, job.ID, r)
			if a.onError != nil {
				a.onError(job.Type, job.ID, err)
			}
		}
	}()
	
	var err error
	
	switch job.Type {
	case JobTypeScheduler:
		err = a.processSchedulerJob(job)
	case JobTypeAutoSchedule:
		err = a.processAutoScheduleJob(job)
	case JobTypeAutoReset:
		err = a.processAutoResetJob(job)
	default:
		err = fmt.Errorf("未知的任务类型: %s", job.Type)
	}
	
	duration := time.Since(startTime)
	
	if err != nil {
		log.Printf("[异步配置] 工作协程 #%d 任务 %s 处理失败 (耗时: %v): %v", 
			workerID, job.ID, duration, err)
		if a.onError != nil {
			a.onError(job.Type, job.ID, err)
		}
	} else {
		log.Printf("[异步配置] 工作协程 #%d 任务 %s 处理成功 (耗时: %v)", 
			workerID, job.ID, duration)
		if a.onSuccess != nil {
			a.onSuccess(job.Type, job.ID)
		}
	}
}

// processSchedulerJob 处理调度器配置更新任务
func (a *AsyncConfigUpdater) processSchedulerJob(job ConfigUpdateJob) error {
	oldConfig, okOld := job.OldConfig.(*models.UserConfig)
	newConfig, okNew := job.NewConfig.(*models.UserConfig)
	
	if !okOld || !okNew {
		return fmt.Errorf("调度器任务配置类型错误")
	}
	
	log.Printf("[异步配置] 开始处理调度器配置更新任务")
	
	// 调用异步更新方法，跳过数据库保存（已在同步阶段完成）
	return a.scheduler.UpdateConfigAsync(oldConfig, newConfig)
}

// processAutoScheduleJob 处理自动调度配置更新任务
func (a *AsyncConfigUpdater) processAutoScheduleJob(job ConfigUpdateJob) error {
	newConfig, ok := job.NewConfig.(*models.AutoScheduleConfig)
	if !ok {
		return fmt.Errorf("自动调度任务配置类型错误")
	}
	
	log.Printf("[异步配置] 开始处理自动调度配置更新任务")
	
	if a.autoScheduler != nil {
		a.autoScheduler.UpdateConfig(newConfig)
	}
	return nil
}

// processAutoResetJob 处理自动重置配置更新任务
func (a *AsyncConfigUpdater) processAutoResetJob(job ConfigUpdateJob) error {
	newConfig, ok := job.NewConfig.(*models.AutoResetConfig)
	if !ok {
		return fmt.Errorf("自动重置任务配置类型错误")
	}
	
	log.Printf("[异步配置] 开始处理自动重置配置更新任务")
	
	if a.autoResetService != nil {
		return a.autoResetService.UpdateConfig(newConfig)
	}
	return nil
}

// GetQueueSize 获取当前队列大小
func (a *AsyncConfigUpdater) GetQueueSize() int {
	return len(a.jobQueue)
}

// IsRunning 检查服务是否运行中
func (a *AsyncConfigUpdater) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isRunning
}