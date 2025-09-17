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

// AutoResetConfig 自动重置配置
type AutoResetConfig struct {
	Enabled          bool   `json:"enabled"`          // 是否启用自动重置
	TimeEnabled      bool   `json:"timeEnabled"`      // 时间触发条件是否启用
	ResetTime        string `json:"resetTime"`        // 重置时间 "HH:MM" 格式
	ThresholdEnabled bool   `json:"thresholdEnabled"` // 积分阈值触发是否启用
	Threshold        int    `json:"threshold"`        // 积分阈值
	ThresholdTimeEnabled bool   `json:"thresholdTimeEnabled"` // 阈值时间范围是否启用
	ThresholdStartTime   string `json:"thresholdStartTime"`   // 阈值检查开始时间 "HH:MM"
	ThresholdEndTime     string `json:"thresholdEndTime"`     // 阈值检查结束时间 "HH:MM"
}

// ValidateTime 验证自动重置时间格式
func (a *AutoResetConfig) ValidateTime() error {
	if a.Enabled && a.TimeEnabled && a.ResetTime != "" {
		if err := validateTimeFormat(a.ResetTime); err != nil {
			return fmt.Errorf("重置时间格式错误: %v", err)
		}
	}
	
	// 验证阈值时间范围格式
	if a.Enabled && a.ThresholdEnabled && a.ThresholdTimeEnabled {
		if a.ThresholdStartTime != "" {
			if err := validateTimeFormat(a.ThresholdStartTime); err != nil {
				return fmt.Errorf("阈值检查开始时间格式错误: %v", err)
			}
		}
		if a.ThresholdEndTime != "" {
			if err := validateTimeFormat(a.ThresholdEndTime); err != nil {
				return fmt.Errorf("阈值检查结束时间格式错误: %v", err)
			}
		}
	}
	
	return nil
}

// IsInThresholdTimeRange 检查当前时间是否在阈值检查时间范围内
func (a *AutoResetConfig) IsInThresholdTimeRange(now time.Time) bool {
	if !a.ThresholdEnabled || !a.ThresholdTimeEnabled {
		return true // 未启用时间限制，始终允许检查
	}
	
	if a.ThresholdStartTime == "" || a.ThresholdEndTime == "" {
		return true // 时间未设置，始终允许检查
	}
	
	currentTime := now.Format("15:04")
	
	// 如果开始时间和结束时间相同，不在范围内
	if a.ThresholdStartTime == a.ThresholdEndTime {
		return false
	}
	
	// 同日范围 (如 09:00-18:00)
	if a.ThresholdStartTime <= a.ThresholdEndTime {
		return currentTime >= a.ThresholdStartTime && currentTime <= a.ThresholdEndTime
	}
	
	// 跨日范围 (如 22:00-06:00)
	return currentTime >= a.ThresholdStartTime || currentTime <= a.ThresholdEndTime
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
	AutoReset               AutoResetConfig    `json:"autoReset"`               // 自动重置配置
}

// VersionInfo 版本信息结构
type VersionInfo struct {
	Version   string `json:"version"`   // 版本号
	GitCommit string `json:"gitCommit"` // Git提交短哈希
	BuildTime string `json:"buildTime"` // 构建时间
	GoVersion string `json:"goVersion"` // Go版本
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
	AutoReset               AutoResetConfig    `json:"autoReset"`               // 自动重置配置
	Version                 VersionInfo        `json:"version"`                 // 版本信息
}

// UserConfigRequest API请求用的用户配置结构
type UserConfigRequest struct {
	Cookie       *string             `json:"cookie,omitempty"`    // Cookie内容（设置时使用，使用指针类型区分未设置和空字符串）
	Interval     int                 `json:"interval"`            // 数据获取间隔(秒)
	TimeRange    int                 `json:"timeRange"`           // 显示时间范围(分钟)
	Enabled      bool                `json:"enabled"`             // 任务是否启用
	AutoSchedule *AutoScheduleConfig `json:"autoSchedule,omitempty"` // 自动调度配置（可选）
	AutoReset    *AutoResetConfig    `json:"autoReset,omitempty"`    // 自动重置配置（可选）
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
		AutoReset: AutoResetConfig{
			Enabled:              false,
			TimeEnabled:          false,
			ResetTime:            "",
			ThresholdEnabled:     false,
			Threshold:            0,
			ThresholdTimeEnabled: false,
			ThresholdStartTime:   "",
			ThresholdEndTime:     "",
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
		AutoReset:               c.AutoReset,    // 包含自动重置配置
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
	
	// 验证自动重置配置
	if err := c.AutoReset.ValidateTime(); err != nil {
		return fmt.Errorf("自动重置配置无效: %v", err)
	}
	
	return nil
}