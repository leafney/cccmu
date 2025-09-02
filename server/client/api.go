package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/utils"
)

// CookieUpdateCallback Cookie更新回调函数类型
type CookieUpdateCallback func()

// ClaudeAPIClient Claude API客户端
type ClaudeAPIClient struct {
	client               *resty.Client
	cookie               string
	cookieUpdateCallback CookieUpdateCallback
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
		client:               client,
		cookie:               cookie,
		cookieUpdateCallback: nil,
	}
}

// SetCookieUpdateCallback 设置Cookie更新回调
func (c *ClaudeAPIClient) SetCookieUpdateCallback(callback CookieUpdateCallback) {
	c.cookieUpdateCallback = callback
}

// UpdateCookie 更新Cookie
func (c *ClaudeAPIClient) UpdateCookie(cookie string) {
	c.cookie = cookie
}

// notifySuccessfulRequest 通知成功请求，更新Cookie验证时间戳
func (c *ClaudeAPIClient) notifySuccessfulRequest() {
	if c.cookieUpdateCallback != nil {
		c.cookieUpdateCallback()
	}
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

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("Cookie无效或已过期")
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API返回错误: %d %s", resp.StatusCode(), resp.Status())
	}

	var apiResp []ClaudeUsageData
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 通知成功请求，更新Cookie验证时间戳
	c.notifySuccessfulRequest()

	return c.convertToUsageData(apiResp), nil
}

// FetchCreditBalance 获取积分余额
func (c *ClaudeAPIClient) FetchCreditBalance() (*models.CreditBalance, error) {
	if c.cookie == "" {
		return nil, fmt.Errorf("Cookie为空")
	}

	resp, err := c.client.R().
		SetHeader("Cookie", c.cookie).
		SetHeader("Referer", "https://www.aicodemirror.com/dashboard/usage").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		Get("https://www.aicodemirror.com/api/user/usage/chart")

	if err != nil {
		return nil, fmt.Errorf("获取积分余额请求失败: %w", err)
	}

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("Cookie无效或已过期")
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("获取积分余额失败: %d %s", resp.StatusCode(), resp.Status())
	}

	// 添加调试日志（可控制）
	utils.Logf("积分余额API原始响应: %s", string(resp.Body()))

	// 解析API返回的数据格式
	var response struct {
		ChartData []models.CreditChartData `json:"chartData"`
	}
	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return nil, fmt.Errorf("解析积分数据失败: %w", err)
	}

	chartData := response.ChartData

	// 添加调试日志（可控制）
	utils.Logf("积分余额API返回数据条数: %d", len(chartData))

	// 计算剩余积分
	totalConsumed := 0
	totalAdded := 0

	for _, data := range chartData {
		totalConsumed += data.Consumed
		totalAdded += data.Added
		utils.Logf("时间段: %s, 消耗: %d, 增加: %d", data.Hour, data.Consumed, data.Added)
	}

	utils.Logf("总消耗: %d, 总增加: %d", totalConsumed, totalAdded)

	// 修正积分余额计算逻辑
	diff := totalAdded - totalConsumed
	var remaining int
	
	if diff >= 0 {
		// 增加的积分 >= 消耗的积分，基础8000 + 差值
		remaining = 8000 + diff
	} else {
		// 消耗的积分 > 增加的积分，基础8000 - 差值的绝对值
		remaining = 8000 + diff // diff是负数，所以相当于8000 - abs(diff)
	}

	if remaining < 0 {
		remaining = 0
	}

	utils.Logf("计算结果 - 差值: %d, 剩余积分: %d", diff, remaining)

	// 通知成功请求，更新Cookie验证时间戳
	c.notifySuccessfulRequest()

	return &models.CreditBalance{
		Remaining: remaining,
		UpdatedAt: time.Now(),
	}, nil
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

