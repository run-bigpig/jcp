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
	return resolveSystemProxy(req, runtime.GOOS, http.ProxyFromEnvironment, m.getOSProxy)
}

func resolveSystemProxy(
	req *http.Request,
	goos string,
	envResolver func(*http.Request) (*url.URL, error),
	osResolver func(*http.Request) string,
) (*url.URL, error) {
	switch goos {
	case "windows":
		return parseProxyURL(osResolver(req))
	case "linux":
		return envResolver(req)
	default:
		if proxy, err := envResolver(req); proxy != nil || err != nil {
			return proxy, err
		}
		return parseProxyURL(osResolver(req))
	}
}

func parseProxyURL(proxyStr string) (*url.URL, error) {
	if proxyStr == "" {
		return nil, nil
	}
	return url.Parse(proxyStr)
}

// getOSProxy 根据操作系统获取系统代理设置
func (m *Manager) getOSProxy(req *http.Request) string {
	switch runtime.GOOS {
	case "windows":
		scheme := "http"
		if req != nil && req.URL != nil && req.URL.Scheme != "" {
			scheme = strings.ToLower(req.URL.Scheme)
		}
		return m.getWindowsProxy(scheme)
	case "darwin":
		return m.getMacOSProxy()
	default:
		return "" // Linux 通常依赖环境变量
	}
}

// getWindowsProxy 从 Windows 注册表读取系统代理
func (m *Manager) getWindowsProxy(targetScheme string) string {
	// 检查代理是否启用
	enableCmd := newCommand("reg", "query",
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
	serverCmd := newCommand("reg", "query",
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
				if proxy, ok := parseWindowsProxyValue(fields[len(fields)-1], targetScheme); ok {
					return proxy
				}
			}
		}
	}
	return ""
}

func parseWindowsProxyValue(raw string, targetScheme string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	normalizedScheme := strings.ToLower(strings.TrimSpace(targetScheme))
	if normalizedScheme == "" {
		normalizedScheme = "http"
	}

	if !strings.Contains(raw, "=") && !strings.Contains(raw, ";") {
		return normalizeWindowsProxyURL(raw, "http")
	}

	entries := strings.Split(raw, ";")
	byScheme := make(map[string]string, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		byScheme[key] = value
	}

	for _, candidate := range []string{normalizedScheme, "https", "http", "socks"} {
		if value := byScheme[candidate]; value != "" {
			proxyScheme := "http"
			if candidate == "socks" {
				proxyScheme = "socks5"
			}
			return normalizeWindowsProxyURL(value, proxyScheme)
		}
	}

	return "", false
}

func normalizeWindowsProxyURL(value string, defaultScheme string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}

	lowerValue := strings.ToLower(value)
	if strings.HasPrefix(lowerValue, "http://") || strings.HasPrefix(lowerValue, "https://") || strings.HasPrefix(lowerValue, "socks5://") {
		return value, true
	}

	if defaultScheme == "" {
		defaultScheme = "http"
	}
	return defaultScheme + "://" + value, true
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
