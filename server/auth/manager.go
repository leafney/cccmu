package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Session 会话信息
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SessionEventType 会话事件类型
type SessionEventType int

const (
	SessionEventDeleted SessionEventType = iota
	SessionEventExpired
)

// SessionEvent 会话事件
type SessionEvent struct {
	Type      SessionEventType
	SessionID string
	Timestamp time.Time
}

// SessionEventHandler 会话事件处理器
type SessionEventHandler func(event SessionEvent)

// Manager 认证管理器
type Manager struct {
	authKey        string
	sessions       sync.Map
	expireDuration time.Duration
	authFilePath   string
	eventHandlers  []SessionEventHandler
	eventMutex     sync.RWMutex
}

// NewManager 创建认证管理器
func NewManager(expireDuration time.Duration) *Manager {
	manager := &Manager{
		expireDuration: expireDuration,
		authFilePath:   ".auth",
	}

	// 加载或生成认证密钥
	if err := manager.loadOrGenerateAuthKey(); err != nil {
		log.Fatalf("初始化认证密钥失败: %v", err)
	}

	// 启动定时清理器
	go manager.startSessionCleaner()

	return manager
}

// loadOrGenerateAuthKey 加载或生成认证密钥
func (m *Manager) loadOrGenerateAuthKey() error {
	// 检查文件是否存在
	if _, err := os.Stat(m.authFilePath); os.IsNotExist(err) {
		// 生成新密钥
		key, err := m.generateRandomKey(32)
		if err != nil {
			return fmt.Errorf("生成随机密钥失败: %v", err)
		}

		// 保存到文件
		if err := m.saveAuthKey(key); err != nil {
			return fmt.Errorf("保存认证密钥失败: %v", err)
		}

		m.authKey = key
		fmt.Printf("🔑 访问密钥: %s\n", key)
		fmt.Printf("💡 密钥已保存到 %s 文件\n", m.authFilePath)
	} else {
		// 从文件加载
		key, err := m.loadAuthKey()
		if err != nil {
			return fmt.Errorf("加载认证密钥失败: %v", err)
		}

		m.authKey = key
		fmt.Printf("🔑 当前访问密钥: %s\n", key)
	}

	return nil
}

// generateRandomKey 生成随机密钥
func (m *Manager) generateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// saveAuthKey 保存认证密钥到文件
func (m *Manager) saveAuthKey(key string) error {
	file, err := os.OpenFile(m.authFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(key)
	return err
}

// loadAuthKey 从文件加载认证密钥
func (m *Manager) loadAuthKey() (string, error) {
	data, err := os.ReadFile(m.authFilePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ValidateKey 验证密钥
func (m *Manager) ValidateKey(key string) bool {
	return key == m.authKey
}

// CreateSession 创建会话
func (m *Manager) CreateSession() (*Session, error) {
	sessionID, err := m.generateRandomKey(64)
	if err != nil {
		return nil, fmt.Errorf("生成会话ID失败: %v", err)
	}

	session := &Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.expireDuration),
	}

	m.sessions.Store(sessionID, session)
	log.Printf("创建新会话: %s, 过期时间: %s", sessionID[:8]+"...", session.ExpiresAt.Format("2006-01-02 15:04:05"))

	return session, nil
}

// ValidateSession 验证会话
func (m *Manager) ValidateSession(sessionID string) (*Session, bool) {
	if sessionID == "" {
		return nil, false
	}

	value, ok := m.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}

	session, ok := value.(*Session)
	if !ok {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		m.sessions.Delete(sessionID)
		log.Printf("会话已过期并清理: %s", sessionID[:8]+"...")
		m.fireSessionEvent(SessionEvent{
			Type:      SessionEventExpired,
			SessionID: sessionID,
			Timestamp: time.Now(),
		})
		return nil, false
	}

	return session, true
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(sessionID string) {
	m.sessions.Delete(sessionID)
	log.Printf("删除会话: %s", sessionID[:8]+"...")
	m.fireSessionEvent(SessionEvent{
		Type:      SessionEventDeleted,
		SessionID: sessionID,
		Timestamp: time.Now(),
	})
}

// GetExpireDuration 获取过期时间
func (m *Manager) GetExpireDuration() time.Duration {
	return m.expireDuration
}

// AddSessionEventHandler 添加会话事件处理器
func (m *Manager) AddSessionEventHandler(handler SessionEventHandler) {
	m.eventMutex.Lock()
	defer m.eventMutex.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)
}

// fireSessionEvent 触发会话事件
func (m *Manager) fireSessionEvent(event SessionEvent) {
	m.eventMutex.RLock()
	handlers := make([]SessionEventHandler, len(m.eventHandlers))
	copy(handlers, m.eventHandlers)
	m.eventMutex.RUnlock()

	for _, handler := range handlers {
		go handler(event) // 异步调用处理器，避免阻塞
	}
}

// startSessionCleaner 启动会话清理器
func (m *Manager) startSessionCleaner() {
	ticker := time.NewTicker(1 * time.Hour) // 每小时清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanExpiredSessions()
		}
	}
}

// cleanExpiredSessions 清理过期会话
func (m *Manager) cleanExpiredSessions() {
	now := time.Now()
	count := 0

	m.sessions.Range(func(key, value interface{}) bool {
		session, ok := value.(*Session)
		if !ok {
			return true
		}

		if now.After(session.ExpiresAt) {
			m.sessions.Delete(key)
			count++
		}

		return true
	})

	if count > 0 {
		log.Printf("清理了 %d 个过期会话", count)
	}
}
