package handlers

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/auth"
	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/models"
	"github.com/leafney/cccmu/server/services"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authManager *auth.Manager
	scheduler   *services.SchedulerService
	db          *database.BadgerDB
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authManager *auth.Manager, scheduler *services.SchedulerService, db *database.BadgerDB) *AuthHandler {
	return &AuthHandler{
		authManager: authManager,
		scheduler:   scheduler,
		db:          db,
	}
}

// LoginResponse 登录响应
type LoginResponse struct {
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Login 用户登录
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	// 从 Authorization 头获取密钥
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		log.Printf("登录失败: 缺少Authorization头")
		return c.Status(401).JSON(models.Error(401, "缺少访问密钥", nil))
	}

	// 检查Bearer格式
	var key string
	if strings.HasPrefix(authHeader, "Bearer ") {
		key = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		// 兼容直接传递密钥的情况
		key = authHeader
	}

	if key == "" {
		log.Printf("登录失败: 空密钥")
		return c.Status(401).JSON(models.Error(401, "访问密钥不能为空", nil))
	}

	// 验证密钥
	if !h.authManager.ValidateKey(key) {
		log.Printf("登录失败: 密钥错误")
		return c.Status(401).JSON(models.Error(401, "访问密钥错误", nil))
	}

	// 创建会话
	session, err := h.authManager.CreateSession()
	if err != nil {
		log.Printf("创建会话失败: %v", err)
		return c.Status(500).JSON(models.Error(500, "创建会话失败", err))
	}

	// 设置cookie
	cookie := &fiber.Cookie{
		Name:     "cccmu_session",
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     "/",
	}
	c.Cookie(cookie)

	log.Printf("用户登录成功，会话: %s", session.ID[:8]+"...")

	// 登录成功后检查配置并恢复监控状态
	go func() {
		time.Sleep(500 * time.Millisecond) // 短暂延迟确保前端连接就绪
		config, err := h.db.GetConfig()
		if err == nil && config.Enabled && config.Cookie != "" {
			if !h.scheduler.IsRunning() {
				if err := h.scheduler.Start(); err != nil {
					log.Printf("登录后自动启动监控失败: %v", err)
				} else {
					log.Printf("登录成功，已自动恢复监控状态")
				}
			}
		}
	}()

	response := LoginResponse{
		Message:   "登录成功",
		ExpiresAt: session.ExpiresAt,
	}

	return c.JSON(models.Success(response))
}

// Logout 用户登出
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// 获取session ID
	sessionID := c.Cookies("cccmu_session")
	if sessionID != "" {
		// 删除会话
		h.authManager.DeleteSession(sessionID)
		log.Printf("会话已删除: %s", sessionID[:8]+"...")

		// 停止定时任务
		if h.scheduler != nil && h.scheduler.IsRunning() {
			if err := h.scheduler.Stop(); err != nil {
				log.Printf("登出时停止定时任务失败: %v", err)
			} else {
				log.Printf("用户登出，已停止定时任务")
			}
		}
	}

	// 清除cookie
	cookie := &fiber.Cookie{
		Name:     "cccmu_session",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour), // 设置为过去时间
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     "/",
	}
	c.Cookie(cookie)

	log.Printf("用户登出成功")

	return c.JSON(models.SuccessMessage("登出成功"))
}

// Status 检查认证状态
func (h *AuthHandler) Status(c *fiber.Ctx) error {
	sessionID := c.Cookies("cccmu_session")
	if sessionID == "" {
		return c.JSON(models.Success(map[string]any{
			"authenticated": false,
		}))
	}

	session, valid := h.authManager.ValidateSession(sessionID)
	if !valid {
		return c.JSON(models.Success(map[string]any{
			"authenticated": false,
		}))
	}

	return c.JSON(models.Success(map[string]any{
		"authenticated": true,
		"expiresAt":     session.ExpiresAt,
	}))
}
