package models

import (
	"fmt"
	"time"
)

// DailyUsage 每日积分使用统计
type DailyUsage struct {
	Date         string         `json:"date"`         // 日期 (YYYY-MM-DD)
	TotalCredits int            `json:"totalCredits"` // 当日总积分使用量
	ModelCredits map[string]int `json:"modelCredits"` // 按模型分组的积分使用量
}

// DailyUsageList 每日使用统计数据列表
type DailyUsageList []DailyUsage

// GetDailyUsageKey 生成BadgerDB存储键
func GetDailyUsageKey(date string) string {
	return fmt.Sprintf("daily_usage:%s", date)
}

// GetLocalDate 获取本地时区的日期字符串 (YYYY-MM-DD)
func GetLocalDate(t time.Time) string {
	return t.Local().Format("2006-01-02")
}

// GetLocalDateFromUTC 将UTC时间转换为本地日期字符串
func GetLocalDateFromUTC(utcTime time.Time) string {
	return utcTime.Local().Format("2006-01-02")
}

// IsToday 检查指定日期是否为今天（本地时区）
func IsToday(date string) bool {
	today := time.Now().Local().Format("2006-01-02")
	return date == today
}

// GetWeekDates 获取最近一周的日期列表（包括今天）
func GetWeekDates() []string {
	dates := make([]string, 7)
	now := time.Now().Local()
	
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -6+i)
		dates[i] = date.Format("2006-01-02")
	}
	
	return dates
}

// FilterByDateRange 按日期范围过滤数据
func (d DailyUsageList) FilterByDateRange(days int) DailyUsageList {
	if days <= 0 {
		return d
	}

	// 计算截止日期
	cutoffDate := time.Now().Local().AddDate(0, 0, -days).Format("2006-01-02")
	var filtered DailyUsageList

	for _, usage := range d {
		if usage.Date >= cutoffDate {
			filtered = append(filtered, usage)
		}
	}

	return filtered
}

// GetTotalCredits 计算总积分使用量
func (d DailyUsageList) GetTotalCredits() int {
	total := 0
	for _, usage := range d {
		total += usage.TotalCredits
	}
	return total
}

// SortByDate 按日期排序（升序）
func (d DailyUsageList) SortByDate() DailyUsageList {
	if len(d) <= 1 {
		return d
	}

	// 简单的冒泡排序
	sorted := make(DailyUsageList, len(d))
	copy(sorted, d)
	
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j].Date > sorted[j+1].Date {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	return sorted
}

// ToMap 转换为以日期为键的map
func (d DailyUsageList) ToMap() map[string]DailyUsage {
	result := make(map[string]DailyUsage)
	for _, usage := range d {
		result[usage.Date] = usage
	}
	return result
}

// FillMissingDates 填充缺失的日期数据，确保返回完整的一周数据
func (d DailyUsageList) FillMissingDates() DailyUsageList {
	weekDates := GetWeekDates()
	usageMap := d.ToMap()
	
	result := make(DailyUsageList, len(weekDates))
	for i, date := range weekDates {
		if usage, exists := usageMap[date]; exists {
			result[i] = usage
		} else {
			// 创建空数据
			result[i] = DailyUsage{
				Date:         date,
				TotalCredits: 0,
				ModelCredits: make(map[string]int),
			}
		}
	}
	
	return result
}

// GetModelList 获取某日使用的模型列表
func (d *DailyUsage) GetModelList() []string {
	if d.ModelCredits == nil {
		return []string{}
	}
	
	models := make([]string, 0, len(d.ModelCredits))
	for model := range d.ModelCredits {
		if d.ModelCredits[model] > 0 {
			models = append(models, model)
		}
	}
	return models
}

// GetModelCredits 获取特定模型的积分使用量
func (d *DailyUsage) GetModelCredits(model string) int {
	if d.ModelCredits == nil {
		return 0
	}
	return d.ModelCredits[model]
}

// AddModelCredits 累加指定模型的积分使用量
func (d *DailyUsage) AddModelCredits(model string, credits int) {
	if d.ModelCredits == nil {
		d.ModelCredits = make(map[string]int)
	}
	d.ModelCredits[model] += credits
	d.TotalCredits += credits
}

// GetAllModelList 获取所有天数中使用过的模型列表（用于前端图表）
func (d DailyUsageList) GetAllModelList() []string {
	modelSet := make(map[string]bool)
	
	for _, usage := range d {
		if usage.ModelCredits != nil {
			for model := range usage.ModelCredits {
				if usage.ModelCredits[model] > 0 {
					modelSet[model] = true
				}
			}
		}
	}
	
	models := make([]string, 0, len(modelSet))
	for model := range modelSet {
		models = append(models, model)
	}
	return models
}