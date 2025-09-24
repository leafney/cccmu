package vars

import "time"

const (
	// CacheExpireDuration 缓存有效期（25秒）
	CacheExpireDuration = 25 * time.Second
	// CleanupInterval 缓存清理间隔（30秒）
	CleanupInterval = 30 * time.Second
)
