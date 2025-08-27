package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/leafney/cccmu/server/models"
)

// ClaudeAPIClient Claude API客户端
type ClaudeAPIClient struct {
	client *resty.Client
	cookie string
}

// NewClaudeAPIClient 创建新的Claude API客户端
func NewClaudeAPIClient(cookie string) *ClaudeAPIClient {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(5 * time.Second).
		SetRetryMaxWaitTime(20 * time.Second).
		SetDebug(false) // 开启调试模式

	return &ClaudeAPIClient{
		client: client,
		cookie: cookie,
	}
}

// UpdateCookie 更新Cookie
func (c *ClaudeAPIClient) UpdateCookie(cookie string) {
	c.cookie = cookie
}

// ClaudeUsageResponse Claude使用量API响应
type ClaudeUsageResponse struct {
	Data []ClaudeUsageData `json:"data"`
}

// ClaudeUsageData Claude使用量数据
type ClaudeUsageData struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	Endpoint    string `json:"endpoint"`
	StatusCode  int    `json:"statusCode"`
	CreditsUsed int    `json:"creditsUsed"`
	CreatedAt   string `json:"createdAt"`
	Model       string `json:"model"`
}

// FetchUsageData 获取积分使用数据
func (c *ClaudeAPIClient) FetchUsageData() ([]models.UsageData, error) {
	if c.cookie == "" {
		return nil, fmt.Errorf("Cookie为空")
	}

	resp, err := c.client.R().
		SetHeader("Cookie", c.cookie).
		SetHeader("Referer", "https://www.aicodemirror.com/dashboard/usage").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		Get("https://www.aicodemirror.com/api/user/usage")

	if err != nil {
		return nil, fmt.Errorf("API请求失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API返回错误: %d %s", resp.StatusCode(), resp.Status())
	}

	var apiResp []ClaudeUsageData
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return c.convertToUsageData(apiResp), nil
}

// ValidateCookie 验证Cookie有效性
func (c *ClaudeAPIClient) ValidateCookie() error {
	if c.cookie == "" {
		return fmt.Errorf("Cookie为空")
	}

	resp, err := c.client.R().
		SetHeader("Cookie", c.cookie).
		SetHeader("Referer", "https://www.aicodemirror.com/dashboard/usage").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		Get("https://www.aicodemirror.com/api/user/usage/chart")

	if err != nil {
		return fmt.Errorf("Cookie验证请求失败: %w", err)
	}

	if resp.StatusCode() == 401 {
		return fmt.Errorf("Cookie无效或已过期")
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("Cookie验证失败: %d %s", resp.StatusCode(), resp.Status())
	}
	return nil
}

// convertToUsageData 转换API数据为内部数据格式
func (c *ClaudeAPIClient) convertToUsageData(apiData []ClaudeUsageData) []models.UsageData {
	var usageData []models.UsageData

	for _, data := range apiData {
		// 解析时间字符串
		createdAt, err := time.Parse(time.RFC3339, data.CreatedAt)
		if err != nil {
			// 如果解析失败，使用当前时间
			createdAt = time.Now()
		}

		usage := models.UsageData{
			ID:          data.ID,
			Type:        data.Type,
			Endpoint:    data.Endpoint,
			StatusCode:  data.StatusCode,
			CreditsUsed: data.CreditsUsed,
			CreatedAt:   createdAt,
			Model:       data.Model,
		}

		usageData = append(usageData, usage)
	}

	return usageData
}
