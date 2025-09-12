package models

import "time"

// UserConfig 用户配置
type UserConfig struct {
	Cookie                  string    `json:"-"`                       // Claude API Cookie (内部存储，不直接序列化)
	Interval                int       `json:"interval"`                // 数据获取间隔(秒)
	TimeRange               int       `json:"timeRange"`               // 显示时间范围(分钟)
	Enabled                 bool      `json:"enabled"`                 // 任务是否启用
	LastCookieValidTime     time.Time `json:"lastCookieValidTime"`     // 最后一次Cookie验证成功时间
	CookieValidationInterval int       `json:"cookieValidationInterval"` // Cookie验证间隔(分钟)
	DailyResetUsed          bool      `json:"dailyResetUsed"`          // 当日重置是否已使用
}

// UserConfigResponse API响应用的用户配置结构
type UserConfigResponse struct {
	Cookie                  bool      `json:"cookie"`                  // Cookie配置状态
	Interval                int       `json:"interval"`                // 数据获取间隔(秒)
	TimeRange               int       `json:"timeRange"`               // 显示时间范围(分钟)
	Enabled                 bool      `json:"enabled"`                 // 任务是否启用
	LastCookieValidTime     time.Time `json:"lastCookieValidTime"`     // 最后一次Cookie验证成功时间
	CookieValidationInterval int       `json:"cookieValidationInterval"` // Cookie验证间隔(分钟)
	DailyResetUsed          bool      `json:"dailyResetUsed"`          // 当日重置是否已使用
}

// UserConfigRequest API请求用的用户配置结构
type UserConfigRequest struct {
	Cookie    *string `json:"cookie,omitempty"`    // Cookie内容（设置时使用，使用指针类型区分未设置和空字符串）
	Interval  int     `json:"interval"`            // 数据获取间隔(秒)
	TimeRange int     `json:"timeRange"`           // 显示时间范围(分钟)
	Enabled   bool    `json:"enabled"`             // 任务是否启用
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
	return nil
}