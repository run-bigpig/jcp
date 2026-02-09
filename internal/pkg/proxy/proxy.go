// Package proxy 提供应用级别的代理管理
// 支持三种模式：无代理、系统代理、自定义代理
package proxy

import (
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

// Manager 代理管理器（单例）
type Manager struct {
	mu        sync.RWMutex
	config    *models.ProxyConfig
	transport *http.Transport
	client    *http.Client
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager 获取代理管理器单例
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			config: &models.ProxyConfig{Mode: models.ProxyModeNone},
		}
		instance.rebuildTransport()
	})
	return instance
}

// SetConfig 更新代理配置
func (m *Manager) SetConfig(cfg *models.ProxyConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = cfg
	m.rebuildTransport()
}

// GetConfig 获取当前代理配置
func (m *Manager) GetConfig() *models.ProxyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetTransport 获取配置好代理的 Transport（用于自定义 Client）
func (m *Manager) GetTransport() *http.Transport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transport.Clone()
}

// GetClient 获取配置好代理的 HTTP Client
func (m *Manager) GetClient() *http.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// GetClientWithTimeout 获取带自定义超时的 HTTP Client
func (m *Manager) GetClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: m.GetTransport(),
		Timeout:   timeout,
	}
}

// rebuildTransport 根据当前配置重建 Transport
func (m *Manager) rebuildTransport() {
	m.transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true, // 与 http.DefaultTransport 保持一致
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	switch m.config.Mode {
	case models.ProxyModeNone:
		m.transport.Proxy = nil

	case models.ProxyModeSystem:
		m.transport.Proxy = m.systemProxyFunc

	case models.ProxyModeCustom:
		if m.config.CustomURL != "" {
			if proxyURL, err := url.Parse(m.config.CustomURL); err == nil {
				m.transport.Proxy = http.ProxyURL(proxyURL)
			}
		}
	}

	m.client = &http.Client{
		Transport: m.transport,
		Timeout:   30 * time.Second,
	}
}

// systemProxyFunc 获取系统代理（作为 Transport.Proxy 函数）
func (m *Manager) systemProxyFunc(req *http.Request) (*url.URL, error) {
	// 优先使用环境变量
	if proxy, err := http.ProxyFromEnvironment(req); proxy != nil || err != nil {
		return proxy, err
	}

	// 根据操作系统获取系统级代理
	proxyStr := m.getOSProxy()
	if proxyStr == "" {
		return nil, nil
	}
	return url.Parse(proxyStr)
}

// getOSProxy 根据操作系统获取系统代理设置
func (m *Manager) getOSProxy() string {
	switch runtime.GOOS {
	case "windows":
		return m.getWindowsProxy()
	case "darwin":
		return m.getMacOSProxy()
	default:
		return "" // Linux 通常依赖环境变量
	}
}

// getWindowsProxy 从 Windows 注册表读取系统代理
func (m *Manager) getWindowsProxy() string {
	// 检查代理是否启用
	enableCmd := exec.Command("reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyEnable")
	enableOut, err := enableCmd.Output()
	if err != nil {
		return ""
	}
	// ProxyEnable 为 0x1 表示启用
	if !strings.Contains(string(enableOut), "0x1") {
		return ""
	}

	// 获取代理服务器地址
	serverCmd := exec.Command("reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyServer")
	serverOut, err := serverCmd.Output()
	if err != nil {
		return ""
	}

	// 解析输出，格式: "    ProxyServer    REG_SZ    127.0.0.1:7890"
	lines := strings.Split(string(serverOut), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ProxyServer") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				proxy := fields[len(fields)-1]
				if !strings.HasPrefix(proxy, "http") {
					proxy = "http://" + proxy
				}
				return proxy
			}
		}
	}
	return ""
}

// getMacOSProxy 从 macOS 系统偏好设置读取代理
func (m *Manager) getMacOSProxy() string {
	cmd := exec.Command("scutil", "--proxy")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	var httpEnabled, host, port string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HTTPEnable") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				httpEnabled = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "HTTPProxy") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				host = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "HTTPPort") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				port = strings.TrimSpace(parts[1])
			}
		}
	}

	if httpEnabled == "1" && host != "" && port != "" {
		return "http://" + host + ":" + port
	}
	return ""
}
