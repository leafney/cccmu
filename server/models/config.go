package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// AutoScheduleConfig 自动调度配置
type AutoScheduleConfig struct {
	Enabled      bool   `json:"enabled"`      // 是否启用自动调度
	StartTime    string `json:"startTime"`    // 开启时间 "HH:MM"
	EndTime      string `json:"endTime"`      // 关闭时间 "HH:MM"
	MonitoringOn bool   `json:"monitoringOn"` // 时间范围内是开启还是关闭监控
}

// ValidateTime 验证时间格式是否正确 (HH:MM)
func (a *AutoScheduleConfig) ValidateTime() error {
	if a.StartTime != "" {
		if err := validateTimeFormat(a.StartTime); err != nil {
			return fmt.Errorf("开始时间格式错误: %v", err)
		}
	}
	if a.EndTime != "" {
		if err := validateTimeFormat(a.EndTime); err != nil {
			return fmt.Errorf("结束时间格式错误: %v", err)
		}
	}
	return nil
}

// IsInTimeRange 检查当前时间是否在设置的时间范围内
func (a *AutoScheduleConfig) IsInTimeRange(now time.Time) bool {
	if !a.Enabled || a.StartTime == "" || a.EndTime == "" {
		return false
	}

	currentTime := now.Format("15:04")
	
	// 如果开始时间和结束时间相同，不在范围内
	if a.StartTime == a.EndTime {
		return false
	}

	// 同日范围 (如 09:00-18:00)
	if a.StartTime <= a.EndTime {
		return currentTime >= a.StartTime && currentTime <= a.EndTime
	}

	// 跨日范围 (如 22:00-06:00)
	return currentTime >= a.StartTime || currentTime <= a.EndTime
}

// ShouldMonitoringBeOn 根据当前时间和配置，判断监控是否应该开启
func (a *AutoScheduleConfig) ShouldMonitoringBeOn(now time.Time) bool {
	if !a.Enabled {
		return false // 自动调度未启用，返回false表示不做改变
	}

	inRange := a.IsInTimeRange(now)
	return inRange == a.MonitoringOn
}

// validateTimeFormat 验证时间格式 HH:MM
func validateTimeFormat(timeStr string) error {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("时间格式必须为 HH:MM")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return fmt.Errorf("小时必须为 00-23")
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return fmt.Errorf("分钟必须为 00-59")
	}

	return nil
}

// UserConfig 用户配置
type UserConfig struct {
	Cookie                  string             `json:"-"`                       // Claude API Cookie (内部存储，不直接序列化)
	Interval                int                `json:"interval"`                // 数据获取间隔(秒)
	TimeRange               int                `json:"timeRange"`               // 显示时间范围(分钟)
	Enabled                 bool               `json:"enabled"`                 // 任务是否启用
	LastCookieValidTime     time.Time          `json:"lastCookieValidTime"`     // 最后一次Cookie验证成功时间
	CookieValidationInterval int               `json:"cookieValidationInterval"` // Cookie验证间隔(分钟)
	DailyResetUsed          bool               `json:"dailyResetUsed"`          // 当日重置是否已使用
	AutoSchedule            AutoScheduleConfig `json:"autoSchedule"`            // 自动调度配置
}

// UserConfigResponse API响应用的用户配置结构
type UserConfigResponse struct {
	Cookie                  bool               `json:"cookie"`                  // Cookie配置状态
	Interval                int                `json:"interval"`                // 数据获取间隔(秒)
	TimeRange               int                `json:"timeRange"`               // 显示时间范围(分钟)
	Enabled                 bool               `json:"enabled"`                 // 任务是否启用
	LastCookieValidTime     time.Time          `json:"lastCookieValidTime"`     // 最后一次Cookie验证成功时间
	CookieValidationInterval int               `json:"cookieValidationInterval"` // Cookie验证间隔(分钟)
	DailyResetUsed          bool               `json:"dailyResetUsed"`          // 当日重置是否已使用
	AutoSchedule            AutoScheduleConfig `json:"autoSchedule"`            // 自动调度配置
}

// UserConfigRequest API请求用的用户配置结构
type UserConfigRequest struct {
	Cookie       *string             `json:"cookie,omitempty"`    // Cookie内容（设置时使用，使用指针类型区分未设置和空字符串）
	Interval     int                 `json:"interval"`            // 数据获取间隔(秒)
	TimeRange    int                 `json:"timeRange"`           // 显示时间范围(分钟)
	Enabled      bool                `json:"enabled"`             // 任务是否启用
	AutoSchedule *AutoScheduleConfig `json:"autoSchedule,omitempty"` // 自动调度配置（可选）
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *UserConfig {
	return &UserConfig{
		Cookie:                   "",
		Interval:                 60,     // 60秒(1分钟)
		TimeRange:                60,     // 60分钟(1小时)
		Enabled:                  false,  // 默认关闭
		LastCookieValidTime:      time.Time{}, // 零值时间
		CookieValidationInterval: 10,     // 10分钟
		DailyResetUsed:          false,   // 默认当日未使用
		AutoSchedule: AutoScheduleConfig{
			Enabled:      false,
			StartTime:    "",
			EndTime:      "",
			MonitoringOn: true, // 默认时间范围内开启监控
		},
	}
}

// ToResponse 转换为API响应格式
func (c *UserConfig) ToResponse() *UserConfigResponse {
	return &UserConfigResponse{
		Cookie:                  c.Cookie != "", // 布尔值表示是否已配置
		Interval:                c.Interval,
		TimeRange:               c.TimeRange,
		Enabled:                 c.Enabled,
		LastCookieValidTime:     c.LastCookieValidTime,
		CookieValidationInterval: c.CookieValidationInterval,
		DailyResetUsed:          c.DailyResetUsed,
		AutoSchedule:            c.AutoSchedule, // 包含自动调度配置
	}
}

// Validate 验证配置有效性
func (c *UserConfig) Validate() error {
	if c.Interval < 30 {
		c.Interval = 60 // 最少30秒，默认60秒
	}
	if c.TimeRange < 30 {
		c.TimeRange = 60
	}
	if c.CookieValidationInterval < 5 {
		c.CookieValidationInterval = 10 // 最少5分钟，默认10分钟
	}
	
	// 验证自动调度配置
	if err := c.AutoSchedule.ValidateTime(); err != nil {
		return fmt.Errorf("自动调度配置无效: %v", err)
	}
	
	return nil
}