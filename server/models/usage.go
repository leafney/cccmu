package models

import "time"

// UsageData 积分使用数据
type UsageData struct {
	ID          int       `json:"id"`
	Type        string    `json:"type"`
	Endpoint    string    `json:"endpoint"`
	StatusCode  int       `json:"statusCode"`
	CreditsUsed int       `json:"creditsUsed"`
	CreatedAt   time.Time `json:"createdAt"`
	Model       string    `json:"model"`
}

// UsageDataList 积分使用数据列表
type UsageDataList []UsageData

// FilterByTimeRange 根据时间范围过滤数据
func (u UsageDataList) FilterByTimeRange(hours int) UsageDataList {
	if hours <= 0 {
		return u
	}

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	var filtered UsageDataList

	for _, data := range u {
		if data.CreatedAt.After(cutoff) {
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