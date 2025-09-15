package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/cccmu/server/auth"
	"github.com/leafney/cccmu/server/models"
)

// AuthMiddleware 认证中间件
func AuthMiddleware(authManager *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()
		
		// 跳过认证API路径
		if strings.HasPrefix(path, "/api/auth/") {
			return c.Next()
		}
		
		// 跳过静态文件
		if strings.HasPrefix(path, "/static/") || 
		   strings.HasPrefix(path, "/assets/") ||
		   path == "/favicon.ico" ||
		   path == "/" {
			return c.Next()
		}
		
		// 获取session cookie
		sessionID := c.Cookies("cccmu_session")
		if sessionID == "" {
			return c.Status(401).JSON(models.Error(401, "未授权访问", nil))
		}
		
		// 验证session
		session, valid := authManager.ValidateSession(sessionID)
		if !valid {
			return c.Status(401).JSON(models.Error(401, "会话无效或已过期", nil))
		}
		
		// 将session信息存储到context中
		c.Locals("session", session)
		
		return c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件（用于首页等）
func OptionalAuthMiddleware(authManager *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("cccmu_session")
		if sessionID != "" {
			if session, valid := authManager.ValidateSession(sessionID); valid {
				c.Locals("session", session)
				c.Locals("authenticated", true)
			}
		}
		return c.Next()
	}
}