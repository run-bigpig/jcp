package main

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/run-bigpig/jcp/internal/adk"
	"github.com/run-bigpig/jcp/internal/adk/mcp"
	"github.com/run-bigpig/jcp/internal/adk/tools"
	"github.com/run-bigpig/jcp/internal/agent"
	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/meeting"
	"github.com/run-bigpig/jcp/internal/memory"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/openclaw"
	"github.com/run-bigpig/jcp/internal/pkg/paths"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"
	"github.com/run-bigpig/jcp/internal/services"
	"github.com/run-bigpig/jcp/internal/services/hottrend"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var log = logger.New("app")

// App struct
type App struct {
	ctx               context.Context
	configService     *services.ConfigService
	marketService     *services.MarketService
	newsService       *services.NewsService
	hotTrendService   *hottrend.HotTrendService
	longHuBangService *services.LongHuBangService
	marketPusher      *services.MarketDataPusher
	meetingService    *meeting.Service
	sessionService    *services.SessionService
	strategyService   *services.StrategyService
	agentContainer    *agent.Container
	toolRegistry      *tools.Registry
	mcpManager        *mcp.Manager
	memoryManager     *memory.Manager
	updateService     *services.UpdateService
	openClawServer    *openclaw.Server

	// 会议取消管理
	meetingCancels   map[string]context.CancelFunc
	meetingCancelsMu sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	dataDir := paths.GetDataDir()

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

	// 设置 Moderator AI 配置
	if configService.GetConfig().ModeratorAIID != "" {
		for i := range configService.GetConfig().AIConfigs {
			if configService.GetConfig().AIConfigs[i].ID == configService.GetConfig().ModeratorAIID {
				meetingService.SetModeratorAIConfig(&configService.GetConfig().AIConfigs[i])
				log.Info("Moderator LLM: %s", configService.GetConfig().AIConfigs[i].ModelName)
				break
			}
		}
	}

	// 初始化Session服务
	sessionService := services.NewSessionService(dataDir)

	// 初始化策略服务
	strategyService := services.NewStrategyService(dataDir)

	// 初始化Agent容器（直接从StrategyService获取数据）
	agentContainer := agent.NewContainer()
	agentContainer.LoadAgents(strategyService.GetAllAgents())

	// 初始化更新服务
	updateService := services.NewUpdateService("run-bigpig", "jcp", Version)

	// 初始化 OpenClaw 服务
	openClawServer := openclaw.NewServer(meetingService, agentContainer, func(aiConfigID string) *models.AIConfig {
		cfg := configService.GetConfig()
		if aiConfigID == "" {
			aiConfigID = cfg.DefaultAIID
		}
		for i := range cfg.AIConfigs {
			if cfg.AIConfigs[i].ID == aiConfigID {
				return &cfg.AIConfigs[i]
			}
		}
		return nil
	}, func(code string) (*models.Stock, error) {
		stocks, err := marketService.GetStockRealTimeData(code)
		if err != nil {
			return nil, err
		}
		if len(stocks) == 0 {
			return nil, nil
		}
		return &stocks[0], nil
	})

	log.Info("所有服务初始化完成")

	return &App{
		configService:     configService,
		marketService:     marketService,
		newsService:       newsService,
		hotTrendService:   hotTrendSvc,
		longHuBangService: longHuBangService,
		meetingService:    meetingService,
		sessionService:    sessionService,
		strategyService:   strategyService,
		agentContainer:    agentContainer,
		toolRegistry:      toolRegistry,
		mcpManager:        mcpManager,
		memoryManager:     memoryManager,
		updateService:     updateService,
		openClawServer:    openClawServer,
		meetingCancels:    make(map[string]context.CancelFunc),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化代理配置
	proxy.GetManager().SetConfig(&a.configService.GetConfig().Proxy)

	// 初始化 MCP 管理器（绑定主 context，预创建 toolset）
	if a.mcpManager != nil {
		if err := a.mcpManager.Initialize(ctx); err != nil {
			log.Warn("MCP 初始化失败: %v", err)
		}
	}

	// 设置 Meeting 服务的 AI 配置解析器
	if a.meetingService != nil {
		a.meetingService.SetAIConfigResolver(a.getAIConfigByID)
	}

	// 初始化更新服务
	if a.updateService != nil {
		a.updateService.Startup(ctx)
	}

	// 初始化并启动市场数据推送服务（需要 context）
	a.marketPusher = services.NewMarketDataPusher(a.marketService, a.configService, a.newsService)
	a.marketPusher.Start(ctx)
	log.Info("市场数据推送服务已启动")

	// 启动 OpenClaw 服务（如果已启用）
	cfg := a.configService.GetConfig()
	if cfg.OpenClaw.Enabled && cfg.OpenClaw.Port > 0 {
		if err := a.openClawServer.Start(cfg.OpenClaw.Port, cfg.OpenClaw.APIKey); err != nil {
			log.Warn("OpenClaw 启动失败: %v", err)
		}
	}
}

// shutdown 应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
	log.Info("应用正在关闭...")
	if a.openClawServer != nil {
		a.openClawServer.Stop()
	}
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
	// 更新 Moderator AI 配置
	if a.meetingService != nil && config.ModeratorAIID != "" {
		for i := range config.AIConfigs {
			if config.AIConfigs[i].ID == config.ModeratorAIID {
				a.meetingService.SetModeratorAIConfig(&config.AIConfigs[i])
				break
			}
		}
	}
	// 更新 OpenClaw 服务配置（热更新）
	a.applyOpenClawConfig(&config.OpenClaw)
	return "success"
}

// applyOpenClawConfig 应用 OpenClaw 配置变更
func (a *App) applyOpenClawConfig(cfg *models.OpenClawConfig) {
	if a.openClawServer == nil {
		return
	}
	if !cfg.Enabled {
		a.openClawServer.Stop()
		return
	}
	if cfg.Port <= 0 {
		return
	}
	// 端口或密钥变更时重启
	if a.openClawServer.IsRunning() {
		if a.openClawServer.GetPort() != cfg.Port {
			a.openClawServer.Restart(cfg.Port, cfg.APIKey)
		}
	} else {
		a.openClawServer.Start(cfg.Port, cfg.APIKey)
	}
}

// GetOpenClawStatus 获取 OpenClaw 服务状态
func (a *App) GetOpenClawStatus() map[string]any {
	if a.openClawServer == nil {
		return map[string]any{"running": false}
	}
	return map[string]any{
		"running": a.openClawServer.IsRunning(),
		"port":    a.openClawServer.GetPort(),
	}
}

// mergeRealtimeStock 合并实时行情字段，保留本地静态字段
func (a *App) mergeRealtimeStock(base models.Stock, rt models.Stock) models.Stock {
	merged := base
	if rt.Name != "" {
		merged.Name = rt.Name
	}
	merged.Price = rt.Price
	merged.Change = rt.Change
	merged.ChangePercent = rt.ChangePercent
	merged.Volume = rt.Volume
	merged.Amount = rt.Amount
	merged.Open = rt.Open
	merged.High = rt.High
	merged.Low = rt.Low
	merged.PreClose = rt.PreClose
	return merged
}

// GetWatchlist 获取自选股列表（附带实时行情）
func (a *App) GetWatchlist() []models.Stock {
	list := a.configService.GetWatchlist()
	if len(list) == 0 {
		return list
	}

	// 收集所有股票代码，拉一次实时行情
	codes := make([]string, len(list))
	for i, s := range list {
		codes[i] = s.Symbol
	}
	realtime, err := a.marketService.GetStockRealTimeData(codes...)
	if err != nil || len(realtime) == 0 {
		return list
	}

	// 用实时数据填充
	rtMap := make(map[string]models.Stock, len(realtime))
	for _, s := range realtime {
		rtMap[s.Symbol] = s
	}
	result := make([]models.Stock, len(list))
	for i, s := range list {
		if rt, ok := rtMap[s.Symbol]; ok {
			result[i] = a.mergeRealtimeStock(s, rt)
		} else {
			result[i] = s
		}
	}
	return result
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

// getAIConfigByID 根据ID获取AI配置，找不到则返回默认配置
func (a *App) getAIConfigByID(aiConfigID string) *models.AIConfig {
	config := a.configService.GetConfig()
	// 如果指定了ID，尝试查找
	if aiConfigID != "" {
		for i := range config.AIConfigs {
			if config.AIConfigs[i].ID == aiConfigID {
				return &config.AIConfigs[i]
			}
		}
	}
	// 找不到则返回默认配置
	return a.getDefaultAIConfig(config)
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

// GetAgentConfigs 获取所有已启用的Agent配置
func (a *App) GetAgentConfigs() []models.AgentConfig {
	return a.strategyService.GetEnabledAgents()
}

// AddAgentConfig 添加Agent配置到当前策略
func (a *App) AddAgentConfig(config models.AgentConfig) string {
	agent := models.StrategyAgent{
		ID:          config.ID,
		Name:        config.Name,
		Role:        config.Role,
		Avatar:      config.Avatar,
		Color:       config.Color,
		Instruction: config.Instruction,
		Tools:       config.Tools,
		MCPServers:  config.MCPServers,
		Enabled:     config.Enabled,
	}
	if err := a.strategyService.AddAgentToActiveStrategy(agent); err != nil {
		return err.Error()
	}
	a.agentContainer.LoadAgents(a.strategyService.GetAllAgents())
	return "success"
}

// UpdateAgentConfig 更新当前策略中的Agent配置
func (a *App) UpdateAgentConfig(config models.AgentConfig) string {
	agent := models.StrategyAgent{
		ID:          config.ID,
		Name:        config.Name,
		Role:        config.Role,
		Avatar:      config.Avatar,
		Color:       config.Color,
		Instruction: config.Instruction,
		Tools:       config.Tools,
		MCPServers:  config.MCPServers,
		Enabled:     config.Enabled,
	}
	if err := a.strategyService.UpdateAgentInActiveStrategy(agent); err != nil {
		return err.Error()
	}
	a.agentContainer.LoadAgents(a.strategyService.GetAllAgents())
	return "success"
}

// DeleteAgentConfig 从当前策略删除Agent配置
func (a *App) DeleteAgentConfig(id string) string {
	if err := a.strategyService.DeleteAgentFromActiveStrategy(id); err != nil {
		return err.Error()
	}
	a.agentContainer.LoadAgents(a.strategyService.GetAllAgents())
	return "success"
}

// ========== Strategy API ==========

// GetStrategies 获取所有策略
func (a *App) GetStrategies() []models.Strategy {
	return a.strategyService.GetAllStrategies()
}

// GetActiveStrategyID 获取当前激活策略ID
func (a *App) GetActiveStrategyID() string {
	return a.strategyService.GetActiveID()
}

// SetActiveStrategy 设置当前激活策略
func (a *App) SetActiveStrategy(id string) string {
	if err := a.strategyService.SetActiveStrategy(id); err != nil {
		return err.Error()
	}
	// 重新加载Agent容器
	a.agentContainer.LoadAgents(a.strategyService.GetAllAgents())
	// 通知前端策略已切换
	runtime.EventsEmit(a.ctx, "strategy:changed", id)
	return "success"
}

// AddStrategy 添加策略
func (a *App) AddStrategy(strategy models.Strategy) string {
	if err := a.strategyService.AddStrategy(strategy); err != nil {
		return err.Error()
	}
	return "success"
}

// UpdateStrategy 更新策略
func (a *App) UpdateStrategy(strategy models.Strategy) string {
	if err := a.strategyService.UpdateStrategy(strategy); err != nil {
		return err.Error()
	}
	return "success"
}

// DeleteStrategy 删除策略
func (a *App) DeleteStrategy(id string) string {
	if err := a.strategyService.DeleteStrategy(id); err != nil {
		return err.Error()
	}
	return "success"
}

// GenerateStrategyRequest AI生成策略请求
type GenerateStrategyRequest struct {
	Prompt string `json:"prompt"`
}

// GenerateStrategyResponse AI生成策略响应
type GenerateStrategyResponse struct {
	Success   bool            `json:"success"`
	Error     string          `json:"error,omitempty"`
	Strategy  models.Strategy `json:"strategy,omitempty"`
	Reasoning string          `json:"reasoning,omitempty"`
}

// GenerateStrategy AI生成策略
func (a *App) GenerateStrategy(req GenerateStrategyRequest) GenerateStrategyResponse {
	// 获取策略生成AI配置（优先使用 StrategyAIID，否则使用默认）
	config := a.configService.GetConfig()
	var aiConfig *models.AIConfig
	targetAIID := config.StrategyAIID
	if targetAIID == "" {
		targetAIID = config.DefaultAIID
	}
	for i := range config.AIConfigs {
		if config.AIConfigs[i].ID == targetAIID {
			aiConfig = &config.AIConfigs[i]
			break
		}
	}
	if aiConfig == nil && len(config.AIConfigs) > 0 {
		aiConfig = &config.AIConfigs[0]
	}
	if aiConfig == nil {
		return GenerateStrategyResponse{Success: false, Error: "未配置AI服务"}
	}

	// 创建LLM
	ctx := context.Background()
	factory := adk.NewModelFactory()
	llm, err := factory.CreateModel(ctx, aiConfig)
	if err != nil {
		return GenerateStrategyResponse{Success: false, Error: err.Error()}
	}

	// 构建生成输入
	input := services.GenerateInput{
		Prompt: req.Prompt,
	}

	// 获取可用工具列表
	for _, t := range a.toolRegistry.GetAllToolInfos() {
		input.Tools = append(input.Tools, services.ToolInfoForGen{
			Name:        t.Name,
			Description: t.Description,
		})
	}

	// 获取已启用的MCP服务器列表
	for _, m := range config.MCPServers {
		if m.Enabled {
			// 获取该服务器的工具列表
			var toolNames []string
			if tools, err := a.mcpManager.GetServerTools(m.ID); err == nil {
				for _, t := range tools {
					toolNames = append(toolNames, t.Name)
				}
			}
			input.MCPServers = append(input.MCPServers, services.MCPInfoForGen{
				ID:    m.ID,
				Name:  m.Name,
				Tools: toolNames,
			})
		}
	}

	// 设置LLM并生成策略
	a.strategyService.SetLLM(llm)
	result, err := a.strategyService.Generate(ctx, input)
	if err != nil {
		return GenerateStrategyResponse{Success: false, Error: err.Error()}
	}

	// 保存策略
	if err := a.strategyService.AddStrategy(result.Strategy); err != nil {
		return GenerateStrategyResponse{Success: false, Error: err.Error()}
	}

	return GenerateStrategyResponse{
		Success:   true,
		Strategy:  result.Strategy,
		Reasoning: result.Reasoning,
	}
}

// EnhancePromptRequest 提示词增强请求
type EnhancePromptRequest struct {
	OriginalPrompt string `json:"originalPrompt"`
	AgentRole      string `json:"agentRole"`
	AgentName      string `json:"agentName"`
}

// EnhancePromptResponse 提示词增强响应
type EnhancePromptResponse struct {
	Success        bool   `json:"success"`
	EnhancedPrompt string `json:"enhancedPrompt,omitempty"`
	Error          string `json:"error,omitempty"`
}

// EnhancePrompt 增强Agent提示词
func (a *App) EnhancePrompt(req EnhancePromptRequest) EnhancePromptResponse {
	// 获取策略生成AI配置（优先使用 StrategyAIID，否则使用默认）
	config := a.configService.GetConfig()
	var aiConfig *models.AIConfig
	targetAIID := config.StrategyAIID
	if targetAIID == "" {
		targetAIID = config.DefaultAIID
	}
	for i := range config.AIConfigs {
		if config.AIConfigs[i].ID == targetAIID {
			aiConfig = &config.AIConfigs[i]
			break
		}
	}
	if aiConfig == nil && len(config.AIConfigs) > 0 {
		aiConfig = &config.AIConfigs[0]
	}
	if aiConfig == nil {
		return EnhancePromptResponse{Success: false, Error: "未配置AI服务"}
	}

	// 创建LLM
	ctx := context.Background()
	factory := adk.NewModelFactory()
	llm, err := factory.CreateModel(ctx, aiConfig)
	if err != nil {
		return EnhancePromptResponse{Success: false, Error: err.Error()}
	}

	// 设置LLM并增强提示词
	a.strategyService.SetLLM(llm)
	input := services.EnhancePromptInput{
		OriginalPrompt: req.OriginalPrompt,
		AgentRole:      req.AgentRole,
		AgentName:      req.AgentName,
	}
	result, err := a.strategyService.EnhancePrompt(ctx, input)
	if err != nil {
		return EnhancePromptResponse{Success: false, Error: err.Error()}
	}

	return EnhancePromptResponse{
		Success:        true,
		EnhancedPrompt: result.EnhancedPrompt,
	}
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
	allAgents := a.strategyService.GetEnabledAgents()
	chatReq := meeting.ChatRequest{
		StockCode: stockCode,
		Stock:     stock,
		Query:     query,
		AllAgents: allAgents,
		Position:  position,
	}

	// 响应回调：每次发言完成后推送
	respCallback := func(resp meeting.ChatResponse) {
		msg := models.ChatMessage{
			AgentID:     resp.AgentID,
			AgentName:   resp.AgentName,
			Role:        resp.Role,
			Content:     resp.Content,
			Round:       resp.Round,
			MsgType:     resp.MsgType,
			Error:       resp.Error,
			MeetingMode: resp.MeetingMode,
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
			AgentID:     resp.AgentID,
			AgentName:   resp.AgentName,
			Role:        resp.Role,
			Content:     resp.Content,
			Round:       resp.Round,
			MsgType:     resp.MsgType,
			Error:       resp.Error,
			MeetingMode: resp.MeetingMode,
		})
	}
	return messages
}

// runDirectMeeting 直接 @ 指定专家模式（带事件推送）
func (a *App) runDirectMeeting(ctx context.Context, req MeetingMessageRequest, stock models.Stock, aiConfig *models.AIConfig, position *models.StockPosition) []models.ChatMessage {
	agentConfigs := a.strategyService.GetAgentsByIDs(req.MentionIds)
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
			AgentID:     resp.AgentID,
			AgentName:   resp.AgentName,
			Role:        resp.Role,
			Content:     resp.Content,
			ReplyTo:     replyTo,
			Round:       resp.Round,
			MsgType:     resp.MsgType,
			Error:       resp.Error,
			MeetingMode: resp.MeetingMode,
		}
		// 保存单条消息
		a.sessionService.AddMessage(stockCode, msg)
		// 推送事件（与智能模式一致）
		runtime.EventsEmit(a.ctx, "meeting:message:"+stockCode, msg)
		messages = append(messages, msg)
	}
	return messages
}

// RetryAgent 重试单个失败的专家（前端手动触发）
func (a *App) RetryAgent(stockCode string, agentId string, query string) models.ChatMessage {
	// 获取股票数据
	stocks, _ := a.marketService.GetStockRealTimeData(stockCode)
	var stock models.Stock
	if len(stocks) > 0 {
		stock = stocks[0]
	}

	// 获取 AI 配置
	config := a.configService.GetConfig()
	aiConfig := a.getDefaultAIConfig(config)
	if aiConfig == nil {
		log.Warn("RetryAgent: no AI config")
		return models.ChatMessage{AgentID: agentId, Error: "未配置 AI 服务"}
	}

	// 获取专家配置
	agents := a.strategyService.GetAgentsByIDs([]string{agentId})
	if len(agents) == 0 {
		log.Warn("RetryAgent: agent not found: %s", agentId)
		return models.ChatMessage{AgentID: agentId, Error: "专家不存在"}
	}
	agentCfg := agents[0]

	position := a.sessionService.GetPosition(stockCode)

	// 进度回调
	progressCallback := func(event meeting.ProgressEvent) {
		runtime.EventsEmit(a.ctx, "meeting:progress:"+stockCode, event)
	}

	resp, err := a.meetingService.RetrySingleAgent(a.ctx, aiConfig, &agentCfg, &stock, query, progressCallback, position)

	msg := models.ChatMessage{
		AgentID:     resp.AgentID,
		AgentName:   resp.AgentName,
		Role:        resp.Role,
		Content:     resp.Content,
		Round:       resp.Round,
		MsgType:     resp.MsgType,
		Error:       resp.Error,
		MeetingMode: resp.MeetingMode,
	}

	if err != nil {
		log.Error("RetryAgent failed: %v", err)
		runtime.EventsEmit(a.ctx, "meeting:message:"+stockCode, msg)
		return msg
	}

	// 成功：保存并推送
	a.sessionService.AddMessage(stockCode, msg)
	runtime.EventsEmit(a.ctx, "meeting:message:"+stockCode, msg)
	return msg
}

// RetryAgentAndContinue 重试失败专家并继续执行剩余专家（前端手动触发）
func (a *App) RetryAgentAndContinue(stockCode string) []models.ChatMessage {
	if !a.meetingService.HasInterruptedMeeting(stockCode) {
		log.Warn("RetryAgentAndContinue: no interrupted meeting for %s", stockCode)
		return []models.ChatMessage{}
	}

	// 创建可取消的 context
	meetingCtx, cancel := context.WithCancel(a.ctx)
	a.meetingCancelsMu.Lock()
	a.meetingCancels[stockCode] = cancel
	a.meetingCancelsMu.Unlock()

	defer func() {
		a.meetingCancelsMu.Lock()
		delete(a.meetingCancels, stockCode)
		a.meetingCancelsMu.Unlock()
	}()

	// 响应回调
	respCallback := func(resp meeting.ChatResponse) {
		msg := models.ChatMessage{
			AgentID:     resp.AgentID,
			AgentName:   resp.AgentName,
			Role:        resp.Role,
			Content:     resp.Content,
			Round:       resp.Round,
			MsgType:     resp.MsgType,
			Error:       resp.Error,
			MeetingMode: resp.MeetingMode,
		}
		a.sessionService.AddMessage(stockCode, msg)
		runtime.EventsEmit(a.ctx, "meeting:message:"+stockCode, msg)
	}

	// 进度回调
	progressCallback := func(event meeting.ProgressEvent) {
		runtime.EventsEmit(a.ctx, "meeting:progress:"+stockCode, event)
	}

	responses, err := a.meetingService.ContinueMeeting(meetingCtx, stockCode, respCallback, progressCallback)
	if err != nil {
		log.Error("RetryAgentAndContinue error: %v", err)
		return []models.ChatMessage{}
	}

	var messages []models.ChatMessage
	for _, resp := range responses {
		messages = append(messages, models.ChatMessage{
			AgentID:     resp.AgentID,
			AgentName:   resp.AgentName,
			Role:        resp.Role,
			Content:     resp.Content,
			Round:       resp.Round,
			MsgType:     resp.MsgType,
			Error:       resp.Error,
			MeetingMode: resp.MeetingMode,
		})
	}
	return messages
}

// CancelInterruptedMeeting 取消中断的会议（用户放弃重试）
func (a *App) CancelInterruptedMeeting(stockCode string) bool {
	a.meetingService.CancelInterruptedMeeting(stockCode)
	return true
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

// TestAIConnection 测试 AI 配置连通性
// 连接成功后自动检测是否支持 system role，并持久化结果
func (a *App) TestAIConnection(config models.AIConfig) string {
	factory := adk.NewModelFactory()
	ctx := context.Background()
	if err := factory.TestConnection(ctx, &config); err != nil {
		log.Error("AI 连接测试失败 [%s]: %v", config.Name, err)
		return err.Error()
	}
	log.Info("AI 连接测试成功 [%s]", config.Name)

	// 连接成功后，探测是否支持 system role
	noSystemRole := factory.DetectSystemRoleSupport(ctx, &config)
	config.NoSystemRole = noSystemRole

	// 持久化检测结果到配置
	if appConfig := a.configService.GetConfig(); appConfig != nil {
		for i := range appConfig.AIConfigs {
			if appConfig.AIConfigs[i].ID == config.ID {
				appConfig.AIConfigs[i].NoSystemRole = noSystemRole
				if err := a.configService.UpdateConfig(appConfig); err != nil {
					log.Warn("保存 NoSystemRole 检测结果失败: %v", err)
				} else {
					log.Info("模型 [%s] NoSystemRole=%v 已保存", config.Name, noSystemRole)
				}
				break
			}
		}
	}

	return "success"
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

// GetTradeDates 获取交易日列表
func (a *App) GetTradeDates(days int) []string {
	if a.marketService == nil {
		return nil
	}
	dates, err := a.marketService.GetTradeDates(days)
	if err != nil {
		return nil
	}
	return dates
}

// GetTradingSchedule 获取交易时间表
func (a *App) GetTradingSchedule() *services.TradingSchedule {
	if a.marketService == nil {
		return nil
	}
	schedule := a.marketService.GetTradingSchedule()
	return &schedule
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

// NotifyFrontendReady 前端通知已准备好，开始推送数据
func (a *App) NotifyFrontendReady() {
	if a.marketPusher != nil {
		a.marketPusher.SetReady()
	}
}
