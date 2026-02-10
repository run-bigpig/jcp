package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/run-bigpig/jcp/internal/adk/mcp"
	"github.com/run-bigpig/jcp/internal/adk/tools"
	"github.com/run-bigpig/jcp/internal/agent"
	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/meeting"
	"github.com/run-bigpig/jcp/internal/memory"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"
	"github.com/run-bigpig/jcp/internal/services"
	"github.com/run-bigpig/jcp/internal/services/hottrend"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var log = logger.New("app")

// App struct
type App struct {
	ctx                context.Context
	configService      *services.ConfigService
	marketService      *services.MarketService
	newsService        *services.NewsService
	hotTrendService    *hottrend.HotTrendService
	longHuBangService  *services.LongHuBangService
	marketPusher       *services.MarketDataPusher
	meetingService     *meeting.Service
	sessionService     *services.SessionService
	agentConfigService *services.AgentConfigService
	agentContainer     *agent.Container
	toolRegistry       *tools.Registry
	mcpManager         *mcp.Manager
	memoryManager      *memory.Manager
	updateService      *services.UpdateService

	// 会议取消管理
	meetingCancels   map[string]context.CancelFunc
	meetingCancelsMu sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	dataDir := getDataDir()

	// 迁移旧数据目录（如果存在）
	migrateDataDir(dataDir)

	// 初始化文件日志
	if err := logger.InitFileLogger(filepath.Join(dataDir, "logs")); err != nil {
		log.Error("初始化文件日志失败: %v", err)
	}
	logger.SetGlobalLevel(logger.DEBUG)

	// 初始化配置服务
	configService, err := services.NewConfigService(dataDir)
	if err != nil {
		panic(err)
	}

	// 初始化研报服务
	researchReportService := services.NewResearchReportService()

	// 初始化舆情热点服务
	hotTrendSvc, err := hottrend.NewHotTrendService()
	if err != nil {
		log.Warn("HotTrend service error: %v", err)
	}

	marketService := services.NewMarketService()
	newsService := services.NewNewsService()

	// 初始化龙虎榜服务
	longHuBangService := services.NewLongHuBangService()

	// 初始化工具注册中心
	toolRegistry := tools.NewRegistry(marketService, newsService, configService, researchReportService, hotTrendSvc, longHuBangService)

	// 初始化 MCP 管理器
	mcpManager := mcp.NewManager()
	if err := mcpManager.LoadConfigs(configService.GetConfig().MCPServers); err != nil {
		log.Warn("MCP load error: %v", err)
	}

	// 初始化会议室服务
	meetingService := meeting.NewServiceFull(toolRegistry, mcpManager)

	// 初始化记忆管理器
	var memoryManager *memory.Manager
	memConfig := configService.GetConfig().Memory
	if memConfig.Enabled {
		memoryManager = memory.NewManagerWithConfig(dataDir, memory.Config{
			MaxRecentRounds:   memConfig.MaxRecentRounds,
			MaxKeyFacts:       memConfig.MaxKeyFacts,
			MaxSummaryLength:  memConfig.MaxSummaryLength,
			CompressThreshold: memConfig.CompressThreshold,
		})
		meetingService.SetMemoryManager(memoryManager)

		if memConfig.AIConfigID != "" {
			for i := range configService.GetConfig().AIConfigs {
				if configService.GetConfig().AIConfigs[i].ID == memConfig.AIConfigID {
					meetingService.SetMemoryAIConfig(&configService.GetConfig().AIConfigs[i])
					log.Info("Memory LLM: %s", configService.GetConfig().AIConfigs[i].ModelName)
					break
				}
			}
		}
		log.Info("Memory manager enabled")
	}

	// 初始化Session服务
	sessionService := services.NewSessionService(dataDir)

	// 初始化Agent配置服务和容器
	agentConfigService := services.NewAgentConfigService(dataDir)
	agentContainer := agent.NewContainer()
	agentContainer.LoadAgents(agentConfigService.GetAllAgents())

	// 初始化更新服务
	updateService := services.NewUpdateService("run-bigpig", "jcp", Version)

	log.Info("所有服务初始化完成")

	return &App{
		configService:      configService,
		marketService:      marketService,
		newsService:        newsService,
		hotTrendService:    hotTrendSvc,
		longHuBangService:  longHuBangService,
		meetingService:     meetingService,
		sessionService:     sessionService,
		agentConfigService: agentConfigService,
		agentContainer:     agentContainer,
		toolRegistry:       toolRegistry,
		mcpManager:         mcpManager,
		memoryManager:      memoryManager,
		updateService:      updateService,
		meetingCancels:     make(map[string]context.CancelFunc),
	}
}

func getDataDir() string {
	userConfigDir, err := os.UserConfigDir()
	if err != nil || userConfigDir == "" {
		return filepath.Join(".", "data")
	}
	return filepath.Join(userConfigDir, "jcp")
}

// migrateDataDir 迁移旧数据目录到新位置
// 仅在新目录为空且旧目录存在时执行迁移
func migrateDataDir(newDataDir string) {
	oldDataDir := filepath.Join(".", "data")

	// 检查旧目录是否存在
	if _, err := os.Stat(oldDataDir); os.IsNotExist(err) {
		return
	}

	// 如果新旧路径相同，无需迁移
	absOld, _ := filepath.Abs(oldDataDir)
	absNew, _ := filepath.Abs(newDataDir)
	if absOld == absNew {
		return
	}

	// 检查新目录是否已有数据（存在 config.json 表示已迁移）
	if _, err := os.Stat(filepath.Join(newDataDir, "config.json")); err == nil {
		return
	}

	log.Info("检测到旧数据目录，开始迁移: %s -> %s", oldDataDir, newDataDir)

	// 确保新目录存在
	if err := os.MkdirAll(newDataDir, 0755); err != nil {
		log.Error("创建新数据目录失败: %v", err)
		return
	}

	// 需要迁移的文件和目录
	items := []string{
		"config.json",
		"agents.json",
		"watchlist.json",
		"stock_basic.json",
		"sessions",
		"memories",
	}

	for _, item := range items {
		src := filepath.Join(oldDataDir, item)
		dst := filepath.Join(newDataDir, item)

		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		if err := copyPath(src, dst); err != nil {
			log.Error("迁移 %s 失败: %v", item, err)
		} else {
			log.Info("迁移成功: %s", item)
		}
	}

	log.Info("数据迁移完成")
}

// copyPath 复制文件或目录
func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

// copyFile 复制单个文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if err := copyPath(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化代理配置
	proxy.GetManager().SetConfig(&a.configService.GetConfig().Proxy)

	// 初始化更新服务
	if a.updateService != nil {
		a.updateService.Startup(ctx)
	}

	// 初始化并启动市场数据推送服务（需要 context）
	a.marketPusher = services.NewMarketDataPusher(a.marketService, a.configService, a.newsService)
	a.marketPusher.Start(ctx)
	log.Info("市场数据推送服务已启动")
}

// shutdown 应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
	log.Info("应用正在关闭...")
	if a.marketPusher != nil {
		a.marketPusher.Stop()
	}
	logger.Close()
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return "Hello " + name + ", It's show time!"
}

// GetConfig 获取配置
func (a *App) GetConfig() *models.AppConfig {
	return a.configService.GetConfig()
}

// UpdateConfig 更新配置
func (a *App) UpdateConfig(config *models.AppConfig) string {
	if err := a.configService.UpdateConfig(config); err != nil {
		return err.Error()
	}
	// 重新加载 MCP 配置
	if a.mcpManager != nil && config.MCPServers != nil {
		if err := a.mcpManager.LoadConfigs(config.MCPServers); err != nil {
			log.Warn("MCP reload error: %v", err)
		}
	}
	// 更新代理配置
	proxy.GetManager().SetConfig(&config.Proxy)
	// 更新记忆管理器的 LLM 配置
	if a.meetingService != nil && config.Memory.AIConfigID != "" {
		for i := range config.AIConfigs {
			if config.AIConfigs[i].ID == config.Memory.AIConfigID {
				a.meetingService.SetMemoryAIConfig(&config.AIConfigs[i])
				break
			}
		}
	}
	return "success"
}

// GetWatchlist 获取自选股列表
func (a *App) GetWatchlist() []models.Stock {
	return a.configService.GetWatchlist()
}

// AddToWatchlist 添加自选股
func (a *App) AddToWatchlist(stock models.Stock) string {
	if err := a.configService.AddToWatchlist(stock); err != nil {
		return err.Error()
	}
	// 同步添加到推送订阅
	a.marketPusher.AddSubscription(stock.Symbol)
	return "success"
}

// RemoveFromWatchlist 移除自选股
func (a *App) RemoveFromWatchlist(symbol string) string {
	if err := a.configService.RemoveFromWatchlist(symbol); err != nil {
		return err.Error()
	}
	// 同步移除推送订阅
	a.marketPusher.RemoveSubscription(symbol)
	// 清空该股票的聊天记录
	a.sessionService.ClearMessages(symbol)
	// 同步清除该股票的记忆
	if a.memoryManager != nil {
		if err := a.memoryManager.DeleteMemory(symbol); err != nil {
			log.Error("delete memory error: %v", err)
		}
	}
	return "success"
}

// GetStockRealTimeData 获取股票实时数据
func (a *App) GetStockRealTimeData(codes []string) []models.Stock {
	stocks, _ := a.marketService.GetStockRealTimeData(codes...)
	return stocks
}

// GetKLineData 获取K线数据
func (a *App) GetKLineData(code string, period string, days int) []models.KLineData {
	data, _ := a.marketService.GetKLineData(code, period, days)
	return data
}

// GetOrderBook 获取盘口数据（真实五档）
func (a *App) GetOrderBook(code string) models.OrderBook {
	orderBook, _ := a.marketService.GetRealOrderBook(code)
	return orderBook
}

// SearchStocks 搜索股票
func (a *App) SearchStocks(keyword string) []services.StockSearchResult {
	return a.configService.SearchStocks(keyword, 20)
}

// getDefaultAIConfig 获取默认AI配置
func (a *App) getDefaultAIConfig(config *models.AppConfig) *models.AIConfig {
	for i := range config.AIConfigs {
		if config.AIConfigs[i].ID == config.DefaultAIID {
			return &config.AIConfigs[i]
		}
		if config.AIConfigs[i].IsDefault {
			return &config.AIConfigs[i]
		}
	}
	if len(config.AIConfigs) > 0 {
		return &config.AIConfigs[0]
	}
	return nil
}

// ========== Session API ==========

// GetOrCreateSession 获取或创建Session
func (a *App) GetOrCreateSession(stockCode, stockName string) *models.StockSession {
	if a.sessionService == nil {
		return nil
	}
	session, _ := a.sessionService.GetOrCreateSession(stockCode, stockName)
	return session
}

// GetSessionMessages 获取Session消息
func (a *App) GetSessionMessages(stockCode string) []models.ChatMessage {
	if a.sessionService == nil {
		return nil
	}
	return a.sessionService.GetMessages(stockCode)
}

// ClearSessionMessages 清空Session消息
func (a *App) ClearSessionMessages(stockCode string) string {
	if a.sessionService == nil {
		return "service not ready"
	}
	if err := a.sessionService.ClearMessages(stockCode); err != nil {
		return err.Error()
	}
	// 同步清除该股票的记忆
	if a.memoryManager != nil {
		if err := a.memoryManager.DeleteMemory(stockCode); err != nil {
			log.Error("delete memory error: %v", err)
		}
	}
	return "success"
}

// UpdateStockPosition 更新股票持仓信息
func (a *App) UpdateStockPosition(stockCode string, shares int64, costPrice float64) string {
	if a.sessionService == nil {
		return "service not ready"
	}
	if err := a.sessionService.UpdatePosition(stockCode, shares, costPrice); err != nil {
		return err.Error()
	}
	return "success"
}

// ========== Agent Config API ==========

// GetAgentConfigs 获取所有Agent配置
func (a *App) GetAgentConfigs() []models.AgentConfig {
	return a.agentConfigService.GetAllAgents()
}

// AddAgentConfig 添加Agent配置
func (a *App) AddAgentConfig(config models.AgentConfig) string {
	if err := a.agentConfigService.AddAgent(config); err != nil {
		return err.Error()
	}
	// 重新加载容器
	a.agentContainer.LoadAgents(a.agentConfigService.GetAllAgents())
	return "success"
}

// UpdateAgentConfig 更新Agent配置
func (a *App) UpdateAgentConfig(config models.AgentConfig) string {
	if err := a.agentConfigService.UpdateAgent(config); err != nil {
		return err.Error()
	}
	a.agentContainer.LoadAgents(a.agentConfigService.GetAllAgents())
	return "success"
}

// DeleteAgentConfig 删除Agent配置
func (a *App) DeleteAgentConfig(id string) string {
	if err := a.agentConfigService.DeleteAgent(id); err != nil {
		return err.Error()
	}
	a.agentContainer.LoadAgents(a.agentConfigService.GetAllAgents())
	return "success"
}

// ========== Meeting Room API ==========

// MeetingMessageRequest 会议室消息请求
type MeetingMessageRequest struct {
	StockCode    string   `json:"stockCode"`
	Content      string   `json:"content"`
	MentionIds   []string `json:"mentionIds"`
	ReplyToId    string   `json:"replyToId"`
	ReplyContent string   `json:"replyContent"`
}

// cancelMeetingInternal 内部取消会议方法
func (a *App) cancelMeetingInternal(stockCode string) {
	a.meetingCancelsMu.Lock()
	if cancel, ok := a.meetingCancels[stockCode]; ok {
		cancel()
		delete(a.meetingCancels, stockCode)
	}
	a.meetingCancelsMu.Unlock()
}

// CancelMeeting 取消指定股票的会议（前端调用）
func (a *App) CancelMeeting(stockCode string) bool {
	a.cancelMeetingInternal(stockCode)
	log.Info("会议已取消: %s", stockCode)
	return true
}

// SendMeetingMessage 发送会议室消息（@指定成员回复）
func (a *App) SendMeetingMessage(req MeetingMessageRequest) []models.ChatMessage {
	// 获取Session
	session := a.sessionService.GetSession(req.StockCode)
	if session == nil {
		log.Warn("session not found: %s", req.StockCode)
		return []models.ChatMessage{}
	}

	// 取消之前该股票的会议（如果有）
	a.cancelMeetingInternal(req.StockCode)

	// 创建可取消的 context
	meetingCtx, cancel := context.WithCancel(a.ctx)
	a.meetingCancelsMu.Lock()
	a.meetingCancels[req.StockCode] = cancel
	a.meetingCancelsMu.Unlock()

	// 会议结束后清理
	defer func() {
		a.meetingCancelsMu.Lock()
		delete(a.meetingCancels, req.StockCode)
		a.meetingCancelsMu.Unlock()
	}()

	// 先保存用户消息
	userMsg := models.ChatMessage{
		AgentID:   "user",
		AgentName: "老韭菜",
		Content:   req.Content,
		ReplyTo:   req.ReplyToId,
		Mentions:  req.MentionIds,
	}
	a.sessionService.AddMessage(req.StockCode, userMsg)

	// 获取股票数据
	stocks, _ := a.marketService.GetStockRealTimeData(req.StockCode)
	var stock models.Stock
	if len(stocks) > 0 {
		stock = stocks[0]
	}

	// 获取默认AI配置
	config := a.configService.GetConfig()
	aiConfig := a.getDefaultAIConfig(config)
	if aiConfig == nil {
		log.Warn("no AI config found")
		return []models.ChatMessage{}
	}

	// 获取持仓信息
	position := a.sessionService.GetPosition(req.StockCode)

	// 判断是否为智能模式（无 @ 任何人）
	if len(req.MentionIds) == 0 {
		return a.runSmartMeeting(meetingCtx, req.StockCode, stock, req.Content, aiConfig, position)
	}

	// 原有逻辑：@ 指定专家
	return a.runDirectMeeting(meetingCtx, req, stock, aiConfig, position)
}

// runSmartMeeting 智能会议模式
func (a *App) runSmartMeeting(ctx context.Context, stockCode string, stock models.Stock, query string, aiConfig *models.AIConfig, position *models.StockPosition) []models.ChatMessage {
	allAgents := a.agentConfigService.GetAllAgents()
	chatReq := meeting.ChatRequest{
		Stock:     stock,
		Query:     query,
		AllAgents: allAgents,
		Position:  position,
	}

	// 响应回调：每次发言完成后推送
	respCallback := func(resp meeting.ChatResponse) {
		msg := models.ChatMessage{
			AgentID:   resp.AgentID,
			AgentName: resp.AgentName,
			Role:      resp.Role,
			Content:   resp.Content,
			Round:     resp.Round,
			MsgType:   resp.MsgType,
		}
		a.sessionService.AddMessage(stockCode, msg)
		runtime.EventsEmit(a.ctx, "meeting:message:"+stockCode, msg)
	}

	// 进度回调：工具调用、流式输出等细粒度事件
	progressCallback := func(event meeting.ProgressEvent) {
		runtime.EventsEmit(a.ctx, "meeting:progress:"+stockCode, event)
	}

	responses, err := a.meetingService.RunSmartMeetingWithCallback(ctx, aiConfig, chatReq, respCallback, progressCallback)
	if err != nil {
		log.Error("runSmartMeeting error: %v", err)
		return []models.ChatMessage{}
	}

	// 返回所有响应（前端可能已通过事件收到，这里作为备份）
	var messages []models.ChatMessage
	for _, resp := range responses {
		messages = append(messages, models.ChatMessage{
			AgentID:   resp.AgentID,
			AgentName: resp.AgentName,
			Role:      resp.Role,
			Content:   resp.Content,
			Round:     resp.Round,
			MsgType:   resp.MsgType,
		})
	}
	return messages
}

// runDirectMeeting 直接 @ 指定专家模式（带事件推送）
func (a *App) runDirectMeeting(ctx context.Context, req MeetingMessageRequest, stock models.Stock, aiConfig *models.AIConfig, position *models.StockPosition) []models.ChatMessage {
	agentConfigs := a.agentConfigService.GetAgentsByIDs(req.MentionIds)
	if len(agentConfigs) == 0 {
		return []models.ChatMessage{}
	}

	chatReq := meeting.ChatRequest{
		Stock:        stock,
		Agents:       agentConfigs,
		Query:        req.Content,
		ReplyContent: req.ReplyContent,
		Position:     position,
	}

	responses, err := a.meetingService.SendMessage(ctx, aiConfig, chatReq)
	if err != nil {
		log.Error("runDirectMeeting error: %v", err)
		return []models.ChatMessage{}
	}

	// 转换并保存响应，同时推送事件
	return a.convertSaveAndEmitResponses(req.StockCode, responses, req.ReplyToId)
}

// convertSaveAndEmitResponses 转换响应、保存并推送事件（统一体验）
func (a *App) convertSaveAndEmitResponses(stockCode string, responses []meeting.ChatResponse, replyTo string) []models.ChatMessage {
	var messages []models.ChatMessage
	for _, resp := range responses {
		msg := models.ChatMessage{
			AgentID:   resp.AgentID,
			AgentName: resp.AgentName,
			Role:      resp.Role,
			Content:   resp.Content,
			ReplyTo:   replyTo,
			Round:     resp.Round,
			MsgType:   resp.MsgType,
		}
		// 保存单条消息
		a.sessionService.AddMessage(stockCode, msg)
		// 推送事件（与智能模式一致）
		runtime.EventsEmit(a.ctx, "meeting:message:"+stockCode, msg)
		messages = append(messages, msg)
	}
	return messages
}

// ========== News API ==========

// GetTelegraphList 获取快讯列表
func (a *App) GetTelegraphList() []services.Telegraph {
	telegraphs, err := a.newsService.GetTelegraphList()
	if err != nil {
		return []services.Telegraph{}
	}
	return telegraphs
}

// OpenURL 在浏览器中打开URL
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// ========== Tools API ==========

// GetAvailableTools 获取可用的内置工具列表
func (a *App) GetAvailableTools() []tools.ToolInfo {
	return a.toolRegistry.GetAllToolInfos()
}

// ========== MCP API ==========

// GetMCPServers 获取 MCP 服务器配置列表
func (a *App) GetMCPServers() []models.MCPServerConfig {
	config := a.configService.GetConfig()
	if config.MCPServers == nil {
		return []models.MCPServerConfig{}
	}
	return config.MCPServers
}

// AddMCPServer 添加 MCP 服务器配置
func (a *App) AddMCPServer(server models.MCPServerConfig) string {
	config := a.configService.GetConfig()
	config.MCPServers = append(config.MCPServers, server)
	if err := a.configService.UpdateConfig(config); err != nil {
		return err.Error()
	}
	// 重新加载 MCP 配置
	if err := a.mcpManager.LoadConfigs(config.MCPServers); err != nil {
		return err.Error()
	}
	return "success"
}

// UpdateMCPServer 更新 MCP 服务器配置
func (a *App) UpdateMCPServer(server models.MCPServerConfig) string {
	config := a.configService.GetConfig()
	for i, s := range config.MCPServers {
		if s.ID == server.ID {
			config.MCPServers[i] = server
			break
		}
	}
	if err := a.configService.UpdateConfig(config); err != nil {
		return err.Error()
	}
	if err := a.mcpManager.LoadConfigs(config.MCPServers); err != nil {
		return err.Error()
	}
	return "success"
}

// DeleteMCPServer 删除 MCP 服务器配置
func (a *App) DeleteMCPServer(id string) string {
	config := a.configService.GetConfig()
	var newServers []models.MCPServerConfig
	for _, s := range config.MCPServers {
		if s.ID != id {
			newServers = append(newServers, s)
		}
	}
	config.MCPServers = newServers
	if err := a.configService.UpdateConfig(config); err != nil {
		return err.Error()
	}
	if err := a.mcpManager.LoadConfigs(config.MCPServers); err != nil {
		return err.Error()
	}
	return "success"
}

// GetMCPStatus 获取所有 MCP 服务器连接状态
func (a *App) GetMCPStatus() []mcp.ServerStatus {
	return a.mcpManager.GetAllStatus()
}

// TestMCPConnection 测试指定 MCP 服务器连接
func (a *App) TestMCPConnection(serverID string) *mcp.ServerStatus {
	return a.mcpManager.TestConnection(serverID)
}

// GetMCPServerTools 获取指定 MCP 服务器的工具列表
func (a *App) GetMCPServerTools(serverID string) []mcp.ToolInfo {
	tools, err := a.mcpManager.GetServerTools(serverID)
	if err != nil {
		return []mcp.ToolInfo{}
	}
	return tools
}

// ========== Window Control API ==========

// WindowMinimize 最小化窗口
func (a *App) WindowMinimize() {
	runtime.WindowMinimise(a.ctx)
}

// WindowMaximize 最大化/还原窗口
func (a *App) WindowMaximize() {
	runtime.WindowToggleMaximise(a.ctx)
}

// WindowClose 关闭窗口
func (a *App) WindowClose() {
	runtime.Quit(a.ctx)
}

// ========== HotTrend API ==========

// GetHotTrendPlatforms 获取支持的热点平台列表
func (a *App) GetHotTrendPlatforms() []hottrend.PlatformInfo {
	return hottrend.SupportedPlatforms
}

// GetHotTrend 获取单个平台的热点数据
func (a *App) GetHotTrend(platform string) hottrend.HotTrendResult {
	if a.hotTrendService == nil {
		return hottrend.HotTrendResult{Platform: platform, Error: "服务未初始化"}
	}
	return a.hotTrendService.GetHotTrend(platform)
}

// GetAllHotTrends 获取所有平台的热点数据
func (a *App) GetAllHotTrends() []hottrend.HotTrendResult {
	if a.hotTrendService == nil {
		return []hottrend.HotTrendResult{}
	}
	return a.hotTrendService.GetAllHotTrends()
}

// ========== Update API ==========

// CheckForUpdate 检查更新
func (a *App) CheckForUpdate() services.UpdateInfo {
	if a.updateService == nil {
		return services.UpdateInfo{Error: "更新服务未初始化"}
	}
	return a.updateService.CheckForUpdate()
}

// DoUpdate 执行更新
func (a *App) DoUpdate() string {
	if a.updateService == nil {
		return "更新服务未初始化"
	}
	if err := a.updateService.Update(); err != nil {
		return err.Error()
	}
	return "success"
}

// RestartApp 重启应用
func (a *App) RestartApp() string {
	if a.updateService == nil {
		return "更新服务未初始化"
	}
	if err := a.updateService.RestartApplication(); err != nil {
		return err.Error()
	}
	return "success"
}

// GetCurrentVersion 获取当前版本
func (a *App) GetCurrentVersion() string {
	if a.updateService == nil {
		return "unknown"
	}
	return a.updateService.GetCurrentVersion()
}

// GetLongHuBangList 获取龙虎榜列表
func (a *App) GetLongHuBangList(pageSize, pageNumber int, tradeDate string) *services.LongHuBangListResult {
	if a.longHuBangService == nil {
		return nil
	}
	result, err := a.longHuBangService.GetLongHuBangList(pageSize, pageNumber, tradeDate)
	if err != nil {
		log.Error("获取龙虎榜失败: %v", err)
		return nil
	}
	return result
}

// GetLongHuBangDetail 获取龙虎榜营业部明细
func (a *App) GetLongHuBangDetail(code, tradeDate string) []models.LongHuBangDetail {
	if a.longHuBangService == nil {
		return nil
	}
	details, err := a.longHuBangService.GetStockDetail(code, tradeDate)
	if err != nil {
		log.Error("获取龙虎榜明细失败: %v", err)
		return nil
	}
	return details
}
