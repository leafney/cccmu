package utils

import (
	"io"
	"log"
	"os"
)

var (
	// 日志开关
	logEnabled = false
	// 默认logger
	Logger *log.Logger
)

// InitLogger 初始化日志系统
func InitLogger(enabled bool) {
	logEnabled = enabled
	
	if enabled {
		Logger = log.New(os.Stdout, "", log.LstdFlags)
		log.SetOutput(os.Stdout)
	} else {
		Logger = log.New(io.Discard, "", 0)
		log.SetOutput(io.Discard)
	}
}

// IsLogEnabled 检查日志是否启用
func IsLogEnabled() bool {
	return logEnabled
}

// Logf 条件日志输出
func Logf(format string, v ...interface{}) {
	if logEnabled {
		Logger.Printf(format, v...)
	}
}

// Log 条件日志输出
func Log(v ...interface{}) {
	if logEnabled {
		Logger.Print(v...)
	}
}

// LogEnabled 启用时才执行的函数
func LogEnabled(fn func()) {
	if logEnabled {
		fn()
	}
}