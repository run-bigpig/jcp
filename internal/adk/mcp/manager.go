// Package mcp 提供 MCP (Model Context Protocol) 集成功能
package mcp

import (
	"context"
	"os/exec"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
)

var log = logger.New("mcp")

// ServerStatus MCP 服务器状态
type ServerStatus struct {
	ID        string `json:"id"`
	Connected bool   `json:"connected"`
	Error     string `json:"error"`
}

// ToolInfo MCP 工具信息
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ServerID    string `json:"serverId"`
	ServerName  string `json:"serverName"`
}

// Manager MCP 服务管理器
type Manager struct {
	mu      sync.RWMutex
	configs map[string]*models.MCPServerConfig
}

// NewManager 创建 MCP 管理器
func NewManager() *Manager {
	return &Manager{
		configs: make(map[string]*models.MCPServerConfig),
	}
}

// LoadConfigs 加载 MCP 服务器配置（延迟初始化，不预先创建连接）
func (m *Manager) LoadConfigs(configs []models.MCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configs = make(map[string]*models.MCPServerConfig)

	for i := range configs {
		cfg := &configs[i]
		if !cfg.Enabled {
			continue
		}
		m.configs[cfg.ID] = cfg
		log.Info("MCP 服务器配置已加载: %s (延迟初始化)", cfg.Name)
	}
	return nil
}

// createTransport 根据配置创建 MCP 传输层
func createTransport(cfg *models.MCPServerConfig) mcp.Transport {
	switch cfg.TransportType {
	case models.MCPTransportSSE:
		log.Warn("创建 SSE 传输 [%s]: %s (已废弃，建议改为 http)", cfg.Name, cfg.Endpoint)
		return &mcp.SSEClientTransport{Endpoint: cfg.Endpoint}
	case models.MCPTransportCommand:
		log.Info("创建 Command 传输 [%s]: %s %v", cfg.Name, cfg.Command, cfg.Args)
		return &mcp.CommandTransport{Command: exec.Command(cfg.Command, cfg.Args...)}
	default:
		log.Info("创建 StreamableHTTP 传输 [%s]: %s", cfg.Name, cfg.Endpoint)
		return &mcp.StreamableClientTransport{
			Endpoint:   cfg.Endpoint,
			MaxRetries: 3,
		}
	}
}

// createToolset 为指定配置创建新的 toolset
func (m *Manager) createToolset(cfg *models.MCPServerConfig) (tool.Toolset, error) {
	ts, err := mcptoolset.New(mcptoolset.Config{
		Transport:  createTransport(cfg),
		ToolFilter: tool.StringPredicate(cfg.ToolFilter),
	})
	if err != nil {
		return nil, err
	}
	log.Info("MCP toolset 已创建: %s", cfg.Name)
	return ts, nil
}

// GetToolset 获取指定 MCP 服务器的 toolset（按需创建新连接）
func (m *Manager) GetToolset(serverID string) (tool.Toolset, bool) {
	m.mu.RLock()
	cfg, ok := m.configs[serverID]
	m.mu.RUnlock()

	if !ok {
		log.Warn("服务器配置不存在: %s", serverID)
		return nil, false
	}

	// 每次调用时创建新的 toolset，避免连接超时问题
	ts, err := m.createToolset(cfg)
	if err != nil {
		log.Error("创建 toolset 失败 [%s]: %v", cfg.Name, err)
		return nil, false
	}

	log.Debug("获取 toolset: %s", serverID)
	return ts, true
}

// GetAllToolsets 获取所有已启用的 toolsets（按需创建新连接）
func (m *Manager) GetAllToolsets() []tool.Toolset {
	m.mu.RLock()
	configs := make([]*models.MCPServerConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	m.mu.RUnlock()

	result := make([]tool.Toolset, 0, len(configs))
	for _, cfg := range configs {
		ts, err := m.createToolset(cfg)
		if err != nil {
			log.Error("创建 toolset 失败 [%s]: %v", cfg.Name, err)
			continue
		}
		result = append(result, ts)
	}
	log.Debug("获取所有 toolsets, 数量: %d", len(result))
	return result
}

// GetToolsetsByIDs 根据 ID 列表获取 toolsets（按需创建新连接）
func (m *Manager) GetToolsetsByIDs(ids []string) []tool.Toolset {
	m.mu.RLock()
	configs := make(map[string]*models.MCPServerConfig)
	for _, id := range ids {
		if cfg, ok := m.configs[id]; ok {
			configs[id] = cfg
		}
	}
	m.mu.RUnlock()

	log.Info("请求获取 toolsets, IDs: %v", ids)
	var result []tool.Toolset
	for id, cfg := range configs {
		ts, err := m.createToolset(cfg)
		if err != nil {
			log.Error("创建 toolset 失败 [%s]: %v", id, err)
			continue
		}
		result = append(result, ts)
		log.Debug("创建 toolset: %s", id)
	}
	log.Info("返回 toolsets 数量: %d", len(result))
	return result
}

// TestConnection 测试指定 MCP 服务器的连接
func (m *Manager) TestConnection(serverID string) *ServerStatus {
	log.Info("测试连接: %s", serverID)
	m.mu.RLock()
	cfg, ok := m.configs[serverID]
	m.mu.RUnlock()

	if !ok {
		log.Warn("测试连接失败: 服务器未配置 %s", serverID)
		return &ServerStatus{ID: serverID, Connected: false, Error: "服务器未配置"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	impl := &mcp.Implementation{Name: cfg.Name, Version: "1.0.0"}
	client := mcp.NewClient(impl, nil)
	_, err := client.Connect(ctx, createTransport(cfg), nil)

	if err != nil {
		log.Error("测试连接失败 [%s]: %v", cfg.Name, err)
		return &ServerStatus{ID: serverID, Connected: false, Error: err.Error()}
	}
	log.Info("测试连接成功: %s", cfg.Name)
	return &ServerStatus{ID: serverID, Connected: true}
}

// GetAllStatus 获取所有服务器状态
func (m *Manager) GetAllStatus() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ServerStatus, 0, len(m.configs))
	for id := range m.configs {
		result = append(result, ServerStatus{ID: id})
	}
	return result
}

// GetServerTools 获取指定 MCP 服务器的工具列表
func (m *Manager) GetServerTools(serverID string) ([]ToolInfo, error) {
	m.mu.RLock()
	cfg, ok := m.configs[serverID]
	m.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	impl := &mcp.Implementation{Name: cfg.Name, Version: "1.0.0"}
	client := mcp.NewClient(impl, nil)
	session, err := client.Connect(ctx, createTransport(cfg), nil)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// 获取工具列表
	toolsResp, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}

	var tools []ToolInfo
	for _, t := range toolsResp.Tools {
		tools = append(tools, ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			ServerID:    serverID,
			ServerName:  cfg.Name,
		})
	}
	return tools, nil
}

// GetAllServerTools 获取所有已启用 MCP 服务器的工具列表
func (m *Manager) GetAllServerTools() []ToolInfo {
	m.mu.RLock()
	serverIDs := make([]string, 0, len(m.configs))
	for id := range m.configs {
		serverIDs = append(serverIDs, id)
	}
	m.mu.RUnlock()

	var allTools []ToolInfo
	for _, id := range serverIDs {
		tools, err := m.GetServerTools(id)
		if err == nil && tools != nil {
			allTools = append(allTools, tools...)
		}
	}
	return allTools
}

// GetToolInfosByServerIDs 根据服务器 ID 列表获取工具信息
func (m *Manager) GetToolInfosByServerIDs(serverIDs []string) []ToolInfo {
	log.Info("获取工具信息, 服务器IDs: %v", serverIDs)
	var allTools []ToolInfo
	for _, id := range serverIDs {
		tools, err := m.GetServerTools(id)
		if err != nil {
			log.Error("获取服务器工具失败 [%s]: %v", id, err)
			continue
		}
		if tools != nil {
			log.Debug("服务器 %s 返回 %d 个工具", id, len(tools))
			allTools = append(allTools, tools...)
		}
	}
	log.Info("共获取 %d 个工具", len(allTools))
	return allTools
}
