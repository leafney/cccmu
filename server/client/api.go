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
	cache                *APICache // API缓存管理器
}

// NewClaudeAPIClient 创建新的Claude API客户端
func NewClaudeAPIClient(cookie string) *ClaudeAPIClient {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(5 * time.Second).
		SetRetryMaxWaitTime(20 * time.Second).
		SetDebug(false) // 开启调试模式

	// 创建缓存管理器
	cache := NewAPICache()

	return &ClaudeAPIClient{
		client:               client,
		cookie:               cookie,
		cookieUpdateCallback: nil,
		cache:                cache,
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

// usageFilterRule 定义要处理的usage数据匹配规则
type usageFilterRule struct {
	Type     string
	Endpoint string
}

// usageFilterRules 定义要处理的usage数据匹配规则
var usageFilterRules = []usageFilterRule{
	{Type: "USAGE", Endpoint: "v1/messages"},
	{Type: "CODEX_USAGE", Endpoint: "backend-api/codex/responses"},
}

func matchesUsageFilter(data ClaudeUsageData) bool {
	for _, rule := range usageFilterRules {
		if data.Type == rule.Type && data.Endpoint == rule.Endpoint {
			return true
		}
	}

	return false
}

// FetchUsageData 获取积分使用数据
func (c *ClaudeAPIClient) FetchUsageData() ([]models.UsageData, error) {
	// 检查缓存
	if cachedData, cachedErr, found := c.cache.GetCachedUsageData(); found {
		return cachedData, cachedErr
	}

	if c.cookie == "" {
		err := fmt.Errorf("Cookie为空")
		c.cache.SetCachedUsageData(nil, err)
		return nil, err
	}

	utils.Logf("发起API请求: FetchUsageData - 请求使用量数据")

	resp, err := c.client.R().
		SetHeader("Cookie", c.cookie).
		SetHeader("Referer", "https://www.aicodemirror.com/dashboard/usage").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		Get("https://www.aicodemirror.com/api/user/usage")

	if err != nil {
		apiErr := fmt.Errorf("API请求失败: %w", err)
		c.cache.SetCachedUsageData(nil, apiErr)
		return nil, apiErr
	}

	if resp.StatusCode() == 401 {
		// 401错误不缓存，直接返回
		return nil, fmt.Errorf("Cookie无效或已过期")
	}

	if resp.StatusCode() != 200 {
		apiErr := fmt.Errorf("API返回错误: %d %s", resp.StatusCode(), resp.Status())
		c.cache.SetCachedUsageData(nil, apiErr)
		return nil, apiErr
	}

	var apiResp []ClaudeUsageData
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		parseErr := fmt.Errorf("解析响应失败: %w", err)
		c.cache.SetCachedUsageData(nil, parseErr)
		return nil, parseErr
	}

	// 通知成功请求，更新Cookie验证时间戳
	c.notifySuccessfulRequest()

	result := c.convertToUsageData(apiResp)
	utils.Logf("API请求成功: FetchUsageData - 获取到 %d 条数据记录", len(result))

	// 缓存成功结果
	c.cache.SetCachedUsageData(result, nil)

	return result, nil
}

// ClaudeCreditsResponse Claude积分API响应
type ClaudeCreditsResponse struct {
	UserID        int    `json:"userId"`
	Email         string `json:"email"`
	Credits       int    `json:"credits"`
	NormalCredits int    `json:"normalCredits"`
	BonusCredits  int    `json:"bonusCredits"`
	CreditLimit   int    `json:"creditLimit"`
	Plan          string `json:"plan"`
}

// FetchCreditBalance 获取积分余额
func (c *ClaudeAPIClient) FetchCreditBalance() (*models.CreditBalance, error) {
	// 检查缓存
	if cachedData, cachedErr, found := c.cache.GetCachedBalance(); found {
		return cachedData, cachedErr
	}

	if c.cookie == "" {
		err := fmt.Errorf("Cookie为空")
		c.cache.SetCachedBalance(nil, err)
		return nil, err
	}

	utils.Logf("发起API请求: FetchCreditBalance - 请求积分余额")

	resp, err := c.client.R().
		SetHeader("Cookie", c.cookie).
		SetHeader("Referer", "https://www.aicodemirror.com/dashboard/usage").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		Get("https://www.aicodemirror.com/api/user/credits")

	if err != nil {
		apiErr := fmt.Errorf("获取积分余额请求失败: %w", err)
		c.cache.SetCachedBalance(nil, apiErr)
		return nil, apiErr
	}

	if resp.StatusCode() == 401 {
		// 401错误不缓存，直接返回
		return nil, fmt.Errorf("Cookie无效或已过期")
	}

	if resp.StatusCode() != 200 {
		apiErr := fmt.Errorf("获取积分余额失败: %d %s", resp.StatusCode(), resp.Status())
		c.cache.SetCachedBalance(nil, apiErr)
		return nil, apiErr
	}

	// 添加调试日志（可控制）
	// utils.Logf("积分余额API原始响应: %s", string(resp.Body()))

	// 解析API返回的数据格式
	var creditsResp ClaudeCreditsResponse
	if err := json.Unmarshal(resp.Body(), &creditsResp); err != nil {
		parseErr := fmt.Errorf("解析积分数据失败: %w", err)
		c.cache.SetCachedBalance(nil, parseErr)
		return nil, parseErr
	}

	utils.Logf("获取到准确的剩余积分: %d", creditsResp.Credits)

	// 通知成功请求，更新Cookie验证时间戳
	c.notifySuccessfulRequest()

	result := &models.CreditBalance{
		Remaining: creditsResp.Credits,
		Plan:      creditsResp.Plan,
		UpdatedAt: time.Now(),
	}
	utils.Logf("API请求成功: FetchCreditBalance - 获取到余额 %d", creditsResp.Credits)

	// 缓存成功结果
	c.cache.SetCachedBalance(result, nil)

	return result, nil
}

// convertToUsageData 转换API数据为内部数据格式
func (c *ClaudeAPIClient) convertToUsageData(apiData []ClaudeUsageData) []models.UsageData {
	var usageData []models.UsageData

	for _, data := range apiData {
		// 仅处理符合白名单规则的usage数据
		if !matchesUsageFilter(data) {
			continue
		}

		// 解析时间字符串
		createdAt, err := time.Parse(time.RFC3339, data.CreatedAt)
		if err != nil {
			// 如果解析失败，使用当前时间
			createdAt = time.Now()
		}

		usage := models.UsageData{
			ID:          data.ID,
			CreditsUsed: data.CreditsUsed,
			CreatedAt:   createdAt,
			Model:       data.Model,
		}

		usageData = append(usageData, usage)
	}

	return usageData
}

// ClaudeResetCreditsResponse Claude重置积分API响应
type ClaudeResetCreditsResponse struct {
	Success        bool   `json:"success"`
	BalanceBefore  string `json:"balanceBefore"`
	BalanceAfter   string `json:"balanceAfter"`
	ResetAmount    string `json:"resetAmount"`
	UsedCount      int    `json:"usedCount"`
	MaxCount       int    `json:"maxCount"`
	RemainingCount int    `json:"remainingCount"`
}

// ResetCredits 重置积分
func (c *ClaudeAPIClient) ResetCredits() (bool, string, error) {
	if c.cookie == "" {
		return false, "", fmt.Errorf("Cookie为空")
	}

	resp, err := c.client.R().
		SetHeader("Cookie", c.cookie).
		SetHeader("Referer", "https://www.aicodemirror.com/dashboard").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json, text/plain, */*").
		SetHeader("Content-Type", "application/json").
		Post("https://www.aicodemirror.com/api/user/credit-reset")

	if err != nil {
		return false, "", fmt.Errorf("HTTP请求失败: %w", err)
	}

	if resp.StatusCode() == 401 {
		return false, "", fmt.Errorf("Cookie无效或已过期")
	}

	// 通知成功请求，更新Cookie验证时间戳
	c.notifySuccessfulRequest()

	// 处理不同状态码
	switch resp.StatusCode() {
	case 200:
		// 重置成功
		resetInfo := fmt.Sprintf("重置成功，API响应: %s", string(resp.Body()))
		return true, resetInfo, nil

	case 400:
		// 今日已重置过，也视为成功状态
		resetInfo := "今日已重置过积分，重置状态有效"
		return true, resetInfo, nil

	default:
		return false, "", fmt.Errorf("HTTP状态码错误: %d, 响应: %s", resp.StatusCode(), string(resp.Body()))
	}
}
