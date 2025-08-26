package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/handlers"
	"github.com/leafney/cccmu/server/services"
)

// 注释掉embed，在开发阶段先不使用
// //go:embed all:../web/dist
// var embedDirStatic embed.FS

func main() {
	// 初始化数据库
	db, err := database.NewBadgerDB("./badger")
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 初始化调度服务
	scheduler, err := services.NewSchedulerService(db)
	if err != nil {
		log.Fatalf("初始化调度服务失败: %v", err)
	}
	defer scheduler.Shutdown()

	// 初始化Fiber应用
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			log.Printf("请求错误: %v", err)
			return c.Status(code).JSON(fiber.Map{
				"code":    code,
				"message": err.Error(),
			})
		},
	})

	// 中间件
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// 初始化处理器
	configHandler := handlers.NewConfigHandler(db, scheduler)
	controlHandler := handlers.NewControlHandler(scheduler)
	sseHandler := handlers.NewSSEHandler(db, scheduler)

	// API路由
	api := app.Group("/api")
	{
		// 配置相关
		api.Get("/config", configHandler.GetConfig)
		api.Put("/config", configHandler.UpdateConfig)

		// 控制相关
		api.Post("/control/start", controlHandler.StartTask)
		api.Post("/control/stop", controlHandler.StopTask)
		api.Get("/control/status", controlHandler.GetTaskStatus)
		api.Post("/refresh", controlHandler.RefreshData)

		// 数据相关
		api.Get("/usage/stream", sseHandler.StreamUsageData)
		api.Get("/usage/data", sseHandler.GetUsageData)
	}

	// 静态文件服务（开发环境）
	if isDevelopment() {
		log.Println("开发模式：使用代理到前端开发服务器")
		app.All("/*", func(c *fiber.Ctx) error {
			return c.Redirect("http://localhost:3000" + c.OriginalURL())
		})
	} else {
		// 生产环境：直接使用文件系统静态文件
		app.Static("/", "./web/dist", fiber.Static{
			Index:  "index.html",
			Browse: false,
		})

		// SPA路由处理
		app.Use(func(c *fiber.Ctx) error {
			return c.SendFile("./web/dist/index.html")
		})
	}

	// 启动服务器
	port := getPort()
	log.Printf("服务器启动在端口 %s", port)
	log.Println("访问地址: http://localhost" + port)

	// 优雅关闭
	go func() {
		if err := app.Listen(port); err != nil {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")
	if err := app.Shutdown(); err != nil {
		log.Printf("服务器关闭失败: %v", err)
	}
	log.Println("服务器已关闭")
}

// isDevelopment 检查是否为开发环境
func isDevelopment() bool {
	return os.Getenv("ENV") == "development"
}

// getPort 获取端口
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}
	return port
}