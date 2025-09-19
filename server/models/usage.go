package models

import "time"

// UsageData 积分使用数据
type UsageData struct {
	ID          int       `json:"id"`
	CreditsUsed int       `json:"creditsUsed"`
	CreatedAt   time.Time `json:"createdAt"`
	Model       string    `json:"model"`
}

// UsageDataList 积分使用数据列表
type UsageDataList []UsageData

// CreditBalance 积分余额信息
type CreditBalance struct {
	Remaining int       `json:"remaining"`
	Plan      string    `json:"plan"` // 订阅等级
	UpdatedAt time.Time `json:"updatedAt"`
}

// FilterByTimeRange 根据时间范围过滤数据
func (u UsageDataList) FilterByTimeRange(minutes int) UsageDataList {
	if minutes <= 0 {
		return u
	}

	// 使用UTC时间计算截止时间，确保与API返回的UTC时间一致
	cutoff := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	var filtered UsageDataList

	for _, data := range u {
		// 将数据时间转换为UTC进行比较
		dataTimeUTC := data.CreatedAt.UTC()
		if dataTimeUTC.After(cutoff) {
			filtered = append(filtered, data)
		}
	}

	return filtered
}

// GroupByModel 按模型分组
func (u UsageDataList) GroupByModel() map[string]UsageDataList {
	groups := make(map[string]UsageDataList)

	for _, data := range u {
		if data.CreditsUsed > 0 {
			groups[data.Model] = append(groups[data.Model], data)
		}
	}

	return groups
}
