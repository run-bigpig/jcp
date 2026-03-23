package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version 版本号，通过 ldflags 注入
var Version = "1.0.0"

func main() {
	// 捕获 panic 并写入日志文件
	defer func() {
		if r := recover(); r != nil {
			logPanic(r)
		}
	}()

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:            "韭菜盘",
		Width:            1366,
		Height:           768,
		MinWidth:         1366,
		MinHeight:        768,
		WindowStartState: options.Maximised,
		Frameless:        true,
		CSSDragProperty:  "--wails-draggable",
		CSSDragValue:     "drag",
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// logPanic 将 panic 信息写入日志文件
func logPanic(r interface{}) {
	// 获取可执行文件所在目录
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	logFile := filepath.Join(dir, "crash.log")

	// 写入崩溃日志
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	msg := fmt.Sprintf("PANIC: %v\n%s\n", r, debug.Stack())
	f.WriteString(msg)
}
