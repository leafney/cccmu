package services

import (
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/leafney/cccmu/server/client"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/utils"
)

// DailyUsageTracker 每日积分使用量跟踪服务
type DailyUsageTracker struct {
	db        *database.BadgerDB
	apiClient *client.ClaudeAPIClient
	scheduler gocron.Scheduler
	isRunning bool
	mu        sync.RWMutex
}

// NewDailyUsageTracker 创建每日积分跟踪服务
func NewDailyUsageTracker(db *database.BadgerDB, apiClient *client.ClaudeAPIClient) (*DailyUsageTracker, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &DailyUsageTracker{
		db:        db,
		apiClient: apiClient,
		scheduler: scheduler,
		isRunning: false,
	}, nil
}

// Start 启动每日积分统计任务
func (d *DailyUsageTracker) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isRunning {
		utils.Logf("[每日积分统计] 服务已在运行，跳过启动")
		return nil
	}

	// 创建每小时执行的定时任务
	job, err := d.scheduler.NewJob(
		gocron.DurationJob(time.Hour), // 每1小时执行一次
		gocron.NewTask(d.collectHourlyUsage),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		utils.Logf("[每日积分统计] ❌ 创建定时任务失败: %v", err)
		return err
	}

	// 启动调度器
	d.scheduler.Start()
	d.isRunning = true

	// 计算下次执行时间
	nextRun := time.Now().Add(time.Hour).Truncate(time.Hour)
	utils.Logf("[每日积分统计] ✅ 服务已启动")
	utils.Logf("[每日积分统计] 📋 任务ID: %v", job.ID())
	utils.Logf("[每日积分统计] ⏰ 执行间隔: 每1小时")
	utils.Logf("[每日积分统计] 🕐 下次执行: %s", nextRun.Format("2006-01-02 15:04:05"))

	// 立即执行一次统计任务
	go func() {
		time.Sleep(5 * time.Second) // 延迟5秒，避免与其他服务冲突
		utils.Logf("[每日积分统计] 🚀 执行首次统计任务")
		if err := d.collectHourlyUsage(); err != nil {
			utils.Logf("[每日积分统计] ❌ 首次统计任务执行失败: %v", err)
		}
	}()

	return nil
}

// Stop 停止每日积分统计任务
func (d *DailyUsageTracker) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isRunning {
		utils.Logf("[每日积分统计] 服务未运行，跳过停止")
		return nil
	}

	utils.Logf("[每日积分统计] 🛑 开始停止服务...")

	if err := d.scheduler.StopJobs(); err != nil {
		utils.Logf("[每日积分统计] ❌ 停止任务失败: %v", err)
	} else {
		utils.Logf("[每日积分统计] ✅ 所有任务已停止")
	}

	if err := d.scheduler.Shutdown(); err != nil {
		utils.Logf("[每日积分统计] ❌ 关闭调度器失败: %v", err)
	} else {
		utils.Logf("[每日积分统计] ✅ 调度器已关闭")
	}

	d.isRunning = false
	utils.Logf("[每日积分统计] ✅ 服务已完全停止")

	return nil
}

// IsRunning 检查服务是否运行中
func (d *DailyUsageTracker) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning
}

// collectHourlyUsage 收集最近一小时的积分使用量
func (d *DailyUsageTracker) collectHourlyUsage() error {
	startTime := time.Now()
	utils.Logf("[每日积分统计] 📊 开始执行积分统计任务 (%s)", startTime.Format("15:04:05"))

	// 获取最近1小时的积分使用数据
	utils.Logf("[每日积分统计] 🔄 正在获取积分使用数据...")
	usageData, err := d.apiClient.FetchUsageData()
	if err != nil {
		utils.Logf("[每日积分统计] ❌ 获取积分使用数据失败: %v", err)
		return err
	}

	totalRecords := len(usageData)
	utils.Logf("[每日积分统计] 📈 获取到 %d 条积分使用记录", totalRecords)

	if totalRecords == 0 {
		utils.Logf("[每日积分统计] ℹ️  无积分使用数据，跳过统计")
		return nil
	}

	// 过滤最近1小时内的数据
	oneHourAgo := time.Now().UTC().Add(-time.Hour)
	var hourlyCredits int
	var recordCount int
	var oldestRecord, newestRecord time.Time

	utils.Logf("[每日积分统计] 🔍 分析时间范围: %s 至 %s (UTC)", 
		oneHourAgo.Format("15:04:05"), time.Now().UTC().Format("15:04:05"))

	for _, data := range usageData {
		if recordCount == 0 {
			oldestRecord = data.CreatedAt
			newestRecord = data.CreatedAt
		} else {
			if data.CreatedAt.Before(oldestRecord) {
				oldestRecord = data.CreatedAt
			}
			if data.CreatedAt.After(newestRecord) {
				newestRecord = data.CreatedAt
			}
		}

		// 将UTC时间与UTC时间比较
		if data.CreatedAt.After(oneHourAgo) {
			hourlyCredits += data.CreditsUsed
			recordCount++
		}
	}

	if totalRecords > 0 {
		utils.Logf("[每日积分统计] 📅 数据时间范围: %s ~ %s (UTC)", 
			oldestRecord.Format("15:04:05"), newestRecord.Format("15:04:05"))
	}

	utils.Logf("[每日积分统计] 📊 过滤结果: %d/%d 条记录在统计时间范围内", recordCount, totalRecords)

	if hourlyCredits == 0 {
		utils.Logf("[每日积分统计] ℹ️  最近1小时积分使用量为0，无需保存")
		return nil
	}

	// 获取当前本地日期
	localDate := models.GetLocalDate(time.Now())
	utils.Logf("[每日积分统计] 📅 目标日期: %s (本地时间)", localDate)

	// 获取保存前的当日统计（用于计算累加）
	beforeUsage, _ := d.db.GetDailyUsage(localDate)
	var beforeCredits int
	if beforeUsage != nil {
		beforeCredits = beforeUsage.TotalCredits
	}

	// 累加到当日总积分使用量
	if err := d.db.SaveDailyUsage(localDate, hourlyCredits); err != nil {
		utils.Logf("[每日积分统计] ❌ 保存每日积分统计失败: %v", err)
		return err
	}

	// 计算保存后的总积分
	afterCredits := beforeCredits + hourlyCredits
	elapsedTime := time.Since(startTime)

	utils.Logf("[每日积分统计] ✅ 统计完成")
	utils.Logf("[每日积分统计] 📋 日期: %s", localDate)
	utils.Logf("[每日积分统计] 🆕 本次积分: +%d", hourlyCredits)
	utils.Logf("[每日积分统计] 📊 记录数: %d", recordCount)
	utils.Logf("[每日积分统计] 📈 累计积分: %d → %d", beforeCredits, afterCredits)
	utils.Logf("[每日积分统计] ⏱️  执行耗时: %v", elapsedTime)

	// 执行数据清理任务（保留7天数据）
	utils.Logf("[每日积分统计] 🧹 开始清理过期数据...")
	if err := d.db.CleanupOldDailyUsage(7); err != nil {
		utils.Logf("[每日积分统计] ⚠️  清理过期数据失败: %v", err)
		// 清理失败不影响主要功能，继续运行
	} else {
		utils.Logf("[每日积分统计] ✅ 过期数据清理完成")
	}

	// 计算下次执行时间
	nextRun := time.Now().Add(time.Hour).Truncate(time.Hour)
	utils.Logf("[每日积分统计] 🕐 下次执行时间: %s", nextRun.Format("2006-01-02 15:04:05"))

	return nil
}

// GetWeeklyUsage 获取最近一周的积分使用统计
func (d *DailyUsageTracker) GetWeeklyUsage() (models.DailyUsageList, error) {
	utils.Logf("[每日积分统计] 📊 获取最近一周积分统计")

	usageList, err := d.db.GetWeeklyUsage()
	if err != nil {
		utils.Logf("[每日积分统计] ❌ 获取周统计数据失败: %v", err)
		return nil, err
	}

	rawCount := len(usageList)
	utils.Logf("[每日积分统计] 📈 数据库中找到 %d 天的统计数据", rawCount)

	// 确保返回完整的7天数据（包括缺失的日期）
	completeList := usageList.FillMissingDates()
	
	// 计算统计信息
	var totalCredits int
	var activeDays int
	for _, usage := range completeList {
		totalCredits += usage.TotalCredits
		if usage.TotalCredits > 0 {
			activeDays++
		}
	}

	utils.Logf("[每日积分统计] ✅ 返回一周统计数据")
	utils.Logf("[每日积分统计] 📅 数据天数: %d天", len(completeList))
	utils.Logf("[每日积分统计] 📊 活跃天数: %d天", activeDays)
	utils.Logf("[每日积分统计] 🔢 总积分: %d", totalCredits)
	
	if activeDays > 0 {
		avgCredits := totalCredits / activeDays
		utils.Logf("[每日积分统计] 📊 平均每活跃日: %d积分", avgCredits)
	}

	return completeList, nil
}


// GetTodayUsage 获取今日积分使用统计
func (d *DailyUsageTracker) GetTodayUsage() (*models.DailyUsage, error) {
	today := models.GetLocalDate(time.Now())
	return d.db.GetDailyUsage(today)
}