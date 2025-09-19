package services

import (
	"fmt"
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
	db            *database.BadgerDB
	apiClient     *client.ClaudeAPIClient
	scheduler     gocron.Scheduler // 独立调度器
	job           gocron.Job       // 定时任务引用
	isActive      bool             // 任务是否激活状态
	isInitialized bool             // 是否已初始化
	mu            sync.RWMutex
}

// NewDailyUsageTracker 创建每日积分跟踪服务
func NewDailyUsageTracker(db *database.BadgerDB, apiClient *client.ClaudeAPIClient) (*DailyUsageTracker, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("创建每日积分统计调度器失败: %w", err)
	}

	return &DailyUsageTracker{
		db:            db,
		apiClient:     apiClient,
		scheduler:     scheduler,
		job:           nil,
		isActive:      false,
		isInitialized: false,
	}, nil
}

// Initialize 初始化服务
func (d *DailyUsageTracker) Initialize() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isInitialized {
		utils.Logf("[每日积分统计] 服务已初始化，跳过操作")
		return nil
	}

	d.isInitialized = true
	utils.Logf("[每日积分统计] ✅ 服务已初始化")

	// 计算下次执行时间（下一个整点）并显示提示
	now := time.Now()
	nextRun := now.Truncate(time.Hour).Add(time.Hour)
	utils.Logf("[每日积分统计] ⏰ 执行间隔: 每小时整点")
	utils.Logf("[每日积分统计] 🕐 下次执行: %s", nextRun.Format("2006-01-02 15:04:05"))

	return nil
}

// Shutdown 完全关闭服务（程序退出时调用）
func (d *DailyUsageTracker) Shutdown() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isInitialized {
		utils.Logf("[每日积分统计] 服务未初始化，跳过关闭")
		return nil
	}

	utils.Logf("[每日积分统计] 🛑 开始关闭服务...")

	// 停止调度器
	if d.scheduler != nil {
		d.scheduler.StopJobs()
		if err := d.scheduler.Shutdown(); err != nil {
			utils.Logf("[每日积分统计] ⚠️  关闭调度器失败: %v", err)
		} else {
			utils.Logf("[每日积分统计] ✅ 调度器已关闭")
		}
		d.scheduler = nil
	}

	d.isActive = false
	d.isInitialized = false
	d.job = nil
	utils.Logf("[每日积分统计] ✅ 服务已完全关闭")

	return nil
}

// IsInitialized 检查服务是否已初始化
func (d *DailyUsageTracker) IsInitialized() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isInitialized
}

// IsActive 检查定时任务是否激活状态
func (d *DailyUsageTracker) IsActive() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isActive
}

// Start 启动定时任务
func (d *DailyUsageTracker) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isInitialized {
		return fmt.Errorf("服务未初始化，无法启动任务")
	}

	if d.isActive {
		utils.Logf("[每日积分统计] 任务已在运行，跳过操作")
		return nil
	}

	// 如果调度器被关闭了，重新创建
	if d.scheduler == nil {
		utils.Logf("[每日积分统计] 🔄 重新创建调度器...")
		scheduler, err := gocron.NewScheduler()
		if err != nil {
			utils.Logf("[每日积分统计] ❌ 创建调度器失败: %v", err)
			return fmt.Errorf("创建调度器失败: %w", err)
		}
		d.scheduler = scheduler
		utils.Logf("[每日积分统计] ✅ 调度器已重新创建")
	}

	// 创建新的定时任务
	job, err := d.scheduler.NewJob(
		gocron.CronJob("0 * * * *", false), // 每小时整点执行
		gocron.NewTask(d.collectHourlyUsage),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		utils.Logf("[每日积分统计] ❌ 创建定时任务失败: %v", err)
		return err
	}

	d.job = job
	d.isActive = true

	// 启动独立调度器
	utils.Logf("[每日积分统计] 🚀 启动独立调度器...")
	d.scheduler.Start()
	utils.Logf("[每日积分统计] ✅ 独立调度器已启动")

	utils.Logf("[每日积分统计] ✅ 任务已启动")
	utils.Logf("[每日积分统计] 📋 任务ID: %v", job.ID())

	// 计算下次执行时间
	now := time.Now()
	nextRun := now.Truncate(time.Hour).Add(time.Hour)
	utils.Logf("[每日积分统计] 🕐 下次执行: %s", nextRun.Format("2006-01-02 15:04:05"))

	return nil
}

// Stop 停止定时任务
func (d *DailyUsageTracker) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isActive {
		utils.Logf("[每日积分统计] 任务已停止，跳过操作")
		return nil
	}

	// 停止调度器并移除任务
	if d.scheduler != nil {
		utils.Logf("[每日积分统计] 🛑 停止独立调度器...")
		d.scheduler.StopJobs()
		if err := d.scheduler.Shutdown(); err != nil {
			utils.Logf("[每日积分统计] ⚠️  关闭调度器失败: %v", err)
		} else {
			utils.Logf("[每日积分统计] ✅ 独立调度器已停止")
		}
		d.scheduler = nil // 置空调度器，下次启动时重新创建
		d.job = nil
	}

	d.isActive = false
	utils.Logf("[每日积分统计] ✅ 任务已停止")

	return nil
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
	modelCredits := make(map[string]int) // 按模型分组的积分统计

	utils.Logf("[每日积分统计] 🔍 分析时间范围: %s 至 %s",
		oneHourAgo.In(time.Local).Format("15:04:05"), time.Now().Format("15:04:05"))

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
			
			// 按模型统计积分
			if data.Model != "" && data.CreditsUsed > 0 {
				modelCredits[data.Model] += data.CreditsUsed
			}
		}
	}

	if totalRecords > 0 {
		utils.Logf("[每日积分统计] 📅 数据时间范围: %s 至 %s",
			oldestRecord.In(time.Local).Format("15:04:05"), newestRecord.In(time.Local).Format("15:04:05"))
	}

	utils.Logf("[每日积分统计] 📊 过滤结果: %d/%d 条记录在统计时间范围内", recordCount, totalRecords)

	// 输出按模型分组的统计结果
	if len(modelCredits) > 0 {
		utils.Logf("[每日积分统计] 📊 按模型分组统计:")
		for model, credits := range modelCredits {
			utils.Logf("[每日积分统计]   - %s: %d 积分", model, credits)
		}
	}

	if hourlyCredits == 0 {
		utils.Logf("[每日积分统计] ℹ️  最近1小时积分使用量为0，无需保存")
		return nil
	}

	// 获取当前本地日期
	localDate := models.GetLocalDate(time.Now())
	utils.Logf("[每日积分统计] 📅 目标日期: %s", localDate)

	// 获取保存前的当日统计（用于计算累加）
	beforeUsage, _ := d.db.GetDailyUsage(localDate)
	var beforeCredits int
	if beforeUsage != nil {
		beforeCredits = beforeUsage.TotalCredits
	}

	// 累加到当日总积分使用量（包含按模型分组的数据）
	if err := d.db.SaveDailyUsageWithModels(localDate, hourlyCredits, modelCredits); err != nil {
		utils.Logf("[每日积分统计] ❌ 保存每日积分统计失败: %v", err)
		return err
	}

	// 计算保存后的总积分
	afterCredits := beforeCredits + hourlyCredits
	elapsedTime := time.Since(startTime)

	// 计算统计时间范围
	endTime := time.Now()
	utils.Logf("[每日积分统计] ✅ 日期 %s %s ~ %s 共统计 %d 条数据，积分 %d，积分变动 %d → %d，(耗时 %v)",
		localDate,
		oneHourAgo.In(time.Local).Format("15:04:05"),
		endTime.Format("15:04:05"),
		recordCount,
		hourlyCredits,
		beforeCredits,
		afterCredits,
		elapsedTime)

	// 执行数据清理任务（保留7天数据）
	utils.Logf("[每日积分统计] 🧹 开始清理过期数据...")
	if err := d.db.CleanupOldDailyUsage(7); err != nil {
		utils.Logf("[每日积分统计] ⚠️  清理过期数据失败: %v", err)
		// 清理失败不影响主要功能，继续运行
	} else {
		utils.Logf("[每日积分统计] ✅ 过期数据清理完成")
	}

	// 计算下次执行时间（下一个整点）
	now := time.Now()
	nextRun := now.Truncate(time.Hour).Add(time.Hour)
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
