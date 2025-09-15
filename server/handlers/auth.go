package handlers

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/auth"
	"github.com/leafney/cccmu/server/models"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authManager *auth.Manager
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authManager *auth.Manager) *AuthHandler {
	return &AuthHandler{
		authManager: authManager,
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