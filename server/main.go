package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/spf13/pflag"

	"github.com/leafney/cccmu/server/database"
	"github.com/leafney/cccmu/server/handlers"
	"github.com/leafney/cccmu/server/services"
	"github.com/leafney/cccmu/server/utils"
	"github.com/leafney/cccmu/server/web"
)

// 版本信息变量，通过编译时注入
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	GoVersion = runtime.Version()
)

func main() {
	// 解析命令行参数
	var port string
	var enableLog bool
	var showVersion bool
	
	pflag.StringVarP(&port, "port", "p", "", "服务器端口号（例如: 8080 或 :8080）")
	pflag.BoolVarP(&enableLog, "log", "l", false, "启用详细日志输出")
	pflag.BoolVarP(&showVersion, "version", "v", false, "显示版本信息")
	pflag.Parse()

	// 如果请求版本信息，显示并退出
	if showVersion {
		fmt.Printf("Version:   %s\n", Version)
		fmt.Printf("GitCommit: %s\n", GitCommit)
		fmt.Printf("BuildTime: %s\n", BuildTime)
		fmt.Printf("GoVersion: %s\n", GoVersion)
		return
	}

	// 初始化日志系统
	utils.InitLogger(enableLog)

	// 初始化数据库
	db, err := database.NewBadgerDB("./.b")
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
	controlHandler := handlers.NewControlHandler(scheduler, db)
	sseHandler := handlers.NewSSEHandler(db, scheduler)

	// API路由
	api := app.Group("/api")
	{
		// 配置相关
		api.Get("/config", configHandler.GetConfig)
		api.Put("/config", configHandler.UpdateConfig)
		api.Delete("/config/cookie", configHandler.ClearCookie)

		// 控制相关
		api.Post("/control/start", controlHandler.StartTask)
		api.Post("/control/stop", controlHandler.StopTask)
		api.Get("/control/status", controlHandler.GetTaskStatus)
		api.Post("/refresh", controlHandler.RefreshAll)

		// 积分余额相关
		api.Get("/balance", controlHandler.GetCreditBalance)
		api.Post("/balance/reset", controlHandler.ResetCredits)

		// 数据相关
		api.Get("/usage/stream", sseHandler.StreamUsageData)
		api.Get("/usage/data", sseHandler.GetUsageData)
	}

	// 静态文件服务 - 使用embed嵌入的静态文件
	log.Println("使用embed嵌入的静态文件")

	// 获取embed文件系统的子目录
	staticFS, err := fs.Sub(web.StaticFiles, "dist")
	if err != nil {
		log.Fatalf("获取embed静态文件系统失败: %v", err)
	}

	// 使用filesystem中间件服务静态文件
	app.Use("/", filesystem.New(filesystem.Config{
		Root:   http.FS(staticFS),
		Browse: false,
		Index:  "index.html",
	}))

	// SPA路由处理 - 对于所有未匹配的路由，返回index.html
	app.Use(func(c *fiber.Ctx) error {
		// 如果是API路由，直接返回404
		if len(c.Path()) >= 4 && c.Path()[:4] == "/api" {
			return c.Status(404).JSON(fiber.Map{
				"code":    404,
				"message": "API endpoint not found",
			})
		}

		// 尝试读取index.html
		indexFile, err := staticFS.Open("index.html")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"code":    500,
				"message": "Failed to read index.html",
			})
		}
		defer indexFile.Close()

		// 设置正确的Content-Type
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendStream(indexFile)
	})

	// 启动服务器
	serverPort := getPort(port)
	log.Printf("服务器启动在端口 %s", serverPort)
	log.Println("访问地址: http://localhost" + serverPort)

	// 优雅关闭
	go func() {
		if err := app.Listen(serverPort); err != nil {
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

// getPort 获取端口，优先级：命令行参数 > 环境变量 > 默认端口
func getPort(flagPort string) string {
	var port string
	
	// 优先使用命令行参数
	if flagPort != "" {
		port = flagPort
	} else {
		// 其次使用环境变量
		port = os.Getenv("PORT")
		if port == "" {
			// 最后使用默认端口
			port = ":8080"
		}
	}
	
	// 确保端口格式正确（以冒号开头）
	if port[0] != ':' {
		port = ":" + port
	}
	return port
}
