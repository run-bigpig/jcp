package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var levelColors = map[Level]string{
	DEBUG: "\033[36m", // cyan
	INFO:  "\033[32m", // green
	WARN:  "\033[33m", // yellow
	ERROR: "\033[31m", // red
}

const resetColor = "\033[0m"

// 全局配置
var (
	globalLevel   = INFO
	globalFile    *os.File
	globalMu      sync.Mutex
	enableConsole = true  // 是否输出到控制台
	enableFile    = false // 是否输出到文件
)

// Logger 日志记录器
type Logger struct {
	module string
	level  Level
}

// SetGlobalLevel 设置全局日志级别
func SetGlobalLevel(level Level) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLevel = level
}

// InitFileLogger 初始化文件日志
func InitFileLogger(logDir string) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 按日期命名日志文件
	logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	globalFile = f
	enableFile = true
	return nil
}

// SetConsoleOutput 设置是否输出到控制台
func SetConsoleOutput(enable bool) {
	globalMu.Lock()
	defer globalMu.Unlock()
	enableConsole = enable
}

// Close 关闭日志文件
func Close() {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalFile != nil {
		globalFile.Close()
		globalFile = nil
	}
	enableFile = false
}

// New 创建新的日志记录器
func New(module string) *Logger {
	return &Logger{
		module: module,
		level:  globalLevel,
	}
}

// log 内部日志方法
func (l *Logger) log(level Level, format string, args ...any) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	levelName := levelNames[level]

	globalMu.Lock()
	defer globalMu.Unlock()

	// 输出到控制台（带颜色）
	if enableConsole {
		color := levelColors[level]
		fmt.Fprintf(os.Stderr, "%s%s%s [%s] %s: %s\n",
			color, levelName, resetColor,
			timestamp, l.module, msg)
	}

	// 输出到文件（无颜色）
	if enableFile && globalFile != nil {
		fmt.Fprintf(globalFile, "%s [%s] %s: %s\n",
			levelName, timestamp, l.module, msg)
	}
}

// Debug 调试日志
func (l *Logger) Debug(format string, args ...any) {
	l.log(DEBUG, format, args...)
}

// Info 信息日志
func (l *Logger) Info(format string, args ...any) {
	l.log(INFO, format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(format string, args ...any) {
	l.log(WARN, format, args...)
}

// Error 错误日志
func (l *Logger) Error(format string, args ...any) {
	l.log(ERROR, format, args...)
}

// WithError 带错误的日志
func (l *Logger) WithError(err error) *Logger {
	if err != nil {
		l.Error("error: %v", err)
	}
	return l
}
