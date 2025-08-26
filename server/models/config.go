package models

// UserConfig 用户配置
type UserConfig struct {
	Cookie    string `json:"cookie"`    // Claude API Cookie
	Interval  int    `json:"interval"`  // 数据获取间隔(分钟)
	TimeRange int    `json:"timeRange"` // 显示时间范围(小时)
	Enabled   bool   `json:"enabled"`   // 任务是否启用
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *UserConfig {
	return &UserConfig{
		Cookie:    "",
		Interval:  1,     // 1分钟
		TimeRange: 1,     // 1小时
		Enabled:   false, // 默认关闭
	}
}

// Validate 验证配置有效性
func (c *UserConfig) Validate() error {
	if c.Interval < 1 {
		c.Interval = 1
	}
	if c.TimeRange < 1 {
		c.TimeRange = 1
	}
	return nil
}