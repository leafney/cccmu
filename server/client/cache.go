package client

import (
	"sync"
	"time"

	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/utils"
)

const (
	// CacheExpireDuration 缓存有效期（25秒）
	CacheExpireDuration = 25 * time.Second
	// CleanupInterval 缓存清理间隔（30秒）
	CleanupInterval = 30 * time.Second
)

// CacheEntry 缓存条目结构
type CacheEntry struct {
	Data      any          // 缓存的响应数据
	Error     error        // 缓存的错误信息
	Timestamp time.Time    // 缓存时间戳
	Mutex     sync.RWMutex // 读写锁保证并发安全
}

// APICache API缓存管理器
type APICache struct {
	usageCache    *CacheEntry  // FetchUsageData 缓存
	balanceCache  *CacheEntry  // FetchCreditBalance 缓存
	cleanupTicker *time.Ticker // 清理定时器
	mu            sync.RWMutex // 缓存管理锁
}

// NewAPICache 创建新的缓存管理器
func NewAPICache() *APICache {
	cache := &APICache{}
	cache.startCleanup()
	return cache
}

// startCleanup 启动缓存清理机制
func (cache *APICache) startCleanup() {
	cache.cleanupTicker = time.NewTicker(CleanupInterval)
	go func() {
		for range cache.cleanupTicker.C {
			cache.cleanup()
		}
	}()
}

// cleanup 清理过期缓存
func (cache *APICache) cleanup() {
	now := time.Now()
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// 清理 usage 缓存
	if cache.usageCache != nil && now.Sub(cache.usageCache.Timestamp) > CacheExpireDuration {
		cache.usageCache = nil
	}

	// 清理 balance 缓存
	if cache.balanceCache != nil && now.Sub(cache.balanceCache.Timestamp) > CacheExpireDuration {
		cache.balanceCache = nil
	}
}

// Stop 停止缓存清理
func (cache *APICache) Stop() {
	if cache.cleanupTicker != nil {
		cache.cleanupTicker.Stop()
	}
}

// GetCachedUsageData 获取使用数据缓存
func (cache *APICache) GetCachedUsageData() ([]models.UsageData, error, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	if cache.usageCache == nil {
		utils.Logf("缓存未命中: FetchUsageData - 缓存为空")
		return nil, nil, false
	}

	cache.usageCache.Mutex.RLock()
	defer cache.usageCache.Mutex.RUnlock()

	// 检查缓存是否过期
	if time.Since(cache.usageCache.Timestamp) > CacheExpireDuration {
		utils.Logf("缓存未命中: FetchUsageData - 缓存过期 (过期时间: %.1f秒)", time.Since(cache.usageCache.Timestamp).Seconds())
		return nil, nil, false
	}

	if cache.usageCache.Error != nil {
		utils.Logf("缓存命中: FetchUsageData - 返回缓存错误: %v", cache.usageCache.Error)
		return nil, cache.usageCache.Error, true
	}

	if data, ok := cache.usageCache.Data.([]models.UsageData); ok {
		utils.Logf("缓存命中: FetchUsageData - 返回 %d 条数据记录 (缓存时间: %.1f秒前)", len(data), time.Since(cache.usageCache.Timestamp).Seconds())
		return data, nil, true
	}

	utils.Logf("缓存未命中: FetchUsageData - 数据类型错误")
	return nil, nil, false
}

// SetCachedUsageData 设置使用数据缓存
func (cache *APICache) SetCachedUsageData(data []models.UsageData, err error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry := &CacheEntry{
		Data:      data,
		Error:     err,
		Timestamp: time.Now(),
	}

	cache.usageCache = entry

	if err != nil {
		utils.Logf("缓存设置: FetchUsageData - 缓存错误结果: %v", err)
	} else {
		utils.Logf("缓存设置: FetchUsageData - 缓存 %d 条成功数据记录", len(data))
	}
}

// GetCachedBalance 获取余额缓存
func (cache *APICache) GetCachedBalance() (*models.CreditBalance, error, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	if cache.balanceCache == nil {
		utils.Logf("缓存未命中: FetchCreditBalance - 缓存为空")
		return nil, nil, false
	}

	cache.balanceCache.Mutex.RLock()
	defer cache.balanceCache.Mutex.RUnlock()

	// 检查缓存是否过期
	if time.Since(cache.balanceCache.Timestamp) > CacheExpireDuration {
		utils.Logf("缓存未命中: FetchCreditBalance - 缓存过期 (过期时间: %.1f秒)", time.Since(cache.balanceCache.Timestamp).Seconds())
		return nil, nil, false
	}

	if cache.balanceCache.Error != nil {
		utils.Logf("缓存命中: FetchCreditBalance - 返回缓存错误: %v", cache.balanceCache.Error)
		return nil, cache.balanceCache.Error, true
	}

	if data, ok := cache.balanceCache.Data.(*models.CreditBalance); ok {
		utils.Logf("缓存命中: FetchCreditBalance - 返回余额 %d (缓存时间: %.1f秒前)", data.Remaining, time.Since(cache.balanceCache.Timestamp).Seconds())
		return data, nil, true
	}

	utils.Logf("缓存未命中: FetchCreditBalance - 数据类型错误")
	return nil, nil, false
}

// SetCachedBalance 设置余额缓存
func (cache *APICache) SetCachedBalance(data *models.CreditBalance, err error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry := &CacheEntry{
		Data:      data,
		Error:     err,
		Timestamp: time.Now(),
	}

	cache.balanceCache = entry

	if err != nil {
		utils.Logf("缓存设置: FetchCreditBalance - 缓存错误结果: %v", err)
	} else {
		utils.Logf("缓存设置: FetchCreditBalance - 缓存余额 %d", data.Remaining)
	}
}
