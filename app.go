package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/adk"
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
	ctx               context.Context
	configService     *services.ConfigService
	marketService     *services.MarketService
	newsService       *services.NewsService
	f10Service        *services.F10Service
	hotTrendService   *hottrend.HotTrendService
	longHuBangService *services.LongHuBangService
	coreDataService   *services.CoreDataService
	intentDataService *services.IntentDataService
	decisionService   *services.DecisionSupportService
	marketPusher      *services.MarketDataPusher
	meetingService    *meeting.Service
	sessionService    *services.SessionService
	strategyService   *services.StrategyService
	agentContainer    *agent.Container
	toolRegistry      *tools.Registry
	mcpManager        *mcp.Manager
	memoryManager     *memory.Manager
	updateService     *services.UpdateService

	// 会议取消管理
	meetingCancels   map[string]context.CancelFunc
	meetingCancelsMu sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	dataDir := getDataDir()

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
	config := configService.GetConfig()

	// 初始化研报服务
	researchReportService := services.NewResearchReportService()

	// 初始化 F10 数据服务
	f10Service := services.NewF10Service()

	// 初始化舆情热点服务
	hotTrendSvc, err := hottrend.NewHotTrendService(dataDir)
	if err != nil {
		log.Warn("HotTrend service error: %v", err)
	}

	marketService := services.NewMarketService()
	newsService := services.NewNewsService()

	// 初始化龙虎榜服务
	longHuBangService := services.NewLongHuBangService()

	// 初始化核心数据包服务
	coreDataService := services.NewCoreDataService(marketService, f10Service)

	// 初始化意图数据服务
	intentDataService := services.NewIntentDataService(f10Service, newsService, longHuBangService)
	decisionService := services.NewDecisionSupportService(dataDir, f10Service)

	// 初始化工具注册中心
	toolRegistry := tools.NewRegistry(marketService, newsService, configService, researchReportService, f10Service, coreDataService, hotTrendSvc, longHuBangService)

	// 初始化 MCP 管理器
	mcpManager := mcp.NewManager()
	if err := mcpManager.LoadConfigs(config.MCPServers); err != nil {
		log.Warn("MCP load error: %v", err)
	}

	// 初始化会议室服务
	meetingService := meeting.NewServiceFull(toolRegistry, mcpManager)
	meetingService.SetRetryCount(config.AIRetryCount)
	meetingService.SetVerboseAgentIO(config.VerboseAgentIO)
	meetingService.SetAgentSelectionStyle(config.AgentSelectionStyle)
	meetingService.SetEnableSecondReview(config.EnableSecondReview)

	// 初始化记忆管理器
	var memoryManager *memory.Manager
	memConfig := config.Memory
	if memConfig.Enabled {
		memoryManager = memory.NewManagerWithConfig(dataDir, memory.Config{
			MaxRecentRounds:   memConfig.MaxRecentRounds,
			MaxKeyFacts:       memConfig.MaxKeyFacts,
			MaxSummaryLength:  memConfig.MaxSummaryLength,
			CompressThreshold: memConfig.CompressThreshold,
		})
		meetingService.SetMemoryManager(memoryManager)

		if memConfig.AIConfigID != "" {
			for i := range config.AIConfigs {
				if config.AIConfigs[i].ID == memConfig.AIConfigID {
					meetingService.SetMemoryAIConfig(&config.AIConfigs[i])
					log.Info("Memory LLM: %s", config.AIConfigs[i].ModelName)
					break
				}
			}
		}
		log.Info("Memory manager enabled")
	}

	// 设置 Moderator AI 配置
	if config.ModeratorAIID != "" {
		for i := range config.AIConfigs {
			if config.AIConfigs[i].ID == config.ModeratorAIID {
				meetingService.SetModeratorAIConfig(&config.AIConfigs[i])
				log.Info("Moderator LLM: %s", config.AIConfigs[i].ModelName)
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

	log.Info("所有服务初始化完成")

	// 初始化更新服务
	updateService := services.NewUpdateService("run-bigpig", "jcp", "0.3.3")

	app := &App{
		configService:     configService,
		marketService:     marketService,
		newsService:       newsService,
		f10Service:        f10Service,
		hotTrendService:   hotTrendSvc,
		longHuBangService: longHuBangService,
		coreDataService:   coreDataService,
		intentDataService: intentDataService,
		decisionService:   decisionService,
		meetingService:    meetingService,
		sessionService:    sessionService,
		strategyService:   strategyService,
		agentContainer:    agentContainer,
		toolRegistry:      toolRegistry,
		mcpManager:        mcpManager,
		memoryManager:     memoryManager,
		updateService:     updateService,
		meetingCancels:    make(map[string]context.CancelFunc),
	}

	if app.meetingService != nil {
		app.meetingService.SetSupplementContextBuilder(app.buildSupplementContext)
	}

	return app
}

func getDataDir() string {
	// 允许通过环境变量显式指定数据目录，便于部署与调试。
	if custom := strings.TrimSpace(os.Getenv("JCP_DATA_DIR")); custom != "" {
		if filepath.IsAbs(custom) {
			return custom
		}
		if wd, err := os.Getwd(); err == nil {
			return filepath.Join(wd, custom)
		}
		return custom
	}

	// 便携模式：优先使用可执行文件同目录的 data 子目录。
	if exePath, err := os.Executable(); err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		if !isLikelyTempExecutableDir(exeDir) {
			return filepath.Join(exeDir, "data")
		}
	}

	// 开发态兜底：使用当前工作目录的 data 子目录。
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return filepath.Join(wd, "data")
	}

	return filepath.Join(".", "data")
}

func isLikelyTempExecutableDir(dir string) bool {
	if dir == "" {
		return true
	}
	lower := strings.ToLower(filepath.ToSlash(dir))
	if strings.Contains(lower, "/go-build/") || strings.Contains(lower, "/tmp/") || strings.Contains(lower, "/temp/") {
		return true
	}

	tempDir := os.TempDir()
	if tempDir == "" {
		return false
	}

	rel, err := filepath.Rel(tempDir, dir)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
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

	// 初始化并启动市场数据推送服务（需要 context）
	a.marketPusher = services.NewMarketDataPusher(a.marketService, a.configService, a.newsService)
	a.marketPusher.Start(ctx)
	log.Info("市场数据推送服务已启动")

	// 初始化更新服务
	if a.updateService != nil {
		a.updateService.Startup(ctx)
	}
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
	// 更新 Moderator AI 配置
	if a.meetingService != nil && config.ModeratorAIID != "" {
		for i := range config.AIConfigs {
			if config.AIConfigs[i].ID == config.ModeratorAIID {
				a.meetingService.SetModeratorAIConfig(&config.AIConfigs[i])
				break
			}
		}
	}
	// 更新 AI 重试次数
	if a.meetingService != nil {
		a.meetingService.SetRetryCount(config.AIRetryCount)
		a.meetingService.SetVerboseAgentIO(config.VerboseAgentIO)
		a.meetingService.SetAgentSelectionStyle(config.AgentSelectionStyle)
		a.meetingService.SetEnableSecondReview(config.EnableSecondReview)
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

// GetMarketStatus 获取市场交易状态
func (a *App) GetMarketStatus() services.MarketStatus {
	return a.marketService.GetMarketStatus()
}

// GetMarketIndices 获取大盘指数数据
func (a *App) GetMarketIndices() []models.MarketIndex {
	indices, _ := a.marketService.GetMarketIndices()
	return indices
}

// GetF10Overview 获取F10综合数据
func (a *App) GetF10Overview(code string) models.F10Overview {
	if a.f10Service == nil {
		return models.F10Overview{
			Code:   code,
			Errors: map[string]string{"service": "F10 服务未初始化"},
		}
	}
	result, err := a.f10Service.GetOverview(code)
	if err != nil {
		return models.F10Overview{
			Code:   code,
			Errors: map[string]string{"request": err.Error()},
		}
	}
	return result
}

// GetF10Valuation 获取估值快照（用于市值/估值展示）
func (a *App) GetF10Valuation(code string) models.StockValuation {
	if a.f10Service == nil {
		return models.StockValuation{}
	}
	valuation, err := a.f10Service.GetValuationByCode(code)
	if err != nil {
		log.Error("GetF10Valuation error: %v", err)
	}
	return valuation
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

func normalizeStockCode(stockCode string) string {
	return strings.ToLower(strings.TrimSpace(stockCode))
}

// ========== Session API ==========

// GetOrCreateSession 获取或创建Session
func (a *App) GetOrCreateSession(stockCode, stockName string) *models.StockSession {
	if a.sessionService == nil {
		return nil
	}
	stockCode = normalizeStockCode(stockCode)
	if stockCode == "" {
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
	stockCode = normalizeStockCode(stockCode)
	if stockCode == "" {
		return []models.ChatMessage{}
	}
	return a.sessionService.GetMessages(stockCode)
}

// ClearSessionMessages 清空Session消息
func (a *App) ClearSessionMessages(stockCode string) string {
	if a.sessionService == nil {
		return "service not ready"
	}
	stockCode = normalizeStockCode(stockCode)
	if stockCode == "" {
		return "stock code is empty"
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
	stockCode = normalizeStockCode(stockCode)
	if stockCode == "" {
		return "stock code is empty"
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
		AIConfigID:  config.AIConfigID,
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
		AIConfigID:  config.AIConfigID,
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
	stockCode = normalizeStockCode(stockCode)
	if stockCode == "" {
		return
	}
	a.meetingCancelsMu.Lock()
	if cancel, ok := a.meetingCancels[stockCode]; ok {
		cancel()
		delete(a.meetingCancels, stockCode)
	}
	a.meetingCancelsMu.Unlock()
}

// CancelMeeting 取消指定股票的会议（前端调用）
func (a *App) CancelMeeting(stockCode string) bool {
	normalized := normalizeStockCode(stockCode)
	a.cancelMeetingInternal(normalized)
	log.Info("会议已取消: %s", normalized)
	return true
}

// SendMeetingMessage 发送会议室消息（@指定成员回复）
func (a *App) SendMeetingMessage(req MeetingMessageRequest) []models.ChatMessage {
	req.StockCode = normalizeStockCode(req.StockCode)
	if req.StockCode == "" {
		log.Warn("empty stock code in SendMeetingMessage")
		return []models.ChatMessage{}
	}

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

	// 先构建基础股票信息（行情失败时也可用）
	stock := models.Stock{
		Symbol: req.StockCode,
		Name:   session.StockName,
	}
	if stock.Name == "" {
		stock.Name = req.StockCode
	}

	// 构建核心数据包（含行情、估值、资金等）
	corePack, coreContext := a.buildCoreContextPack(req.StockCode)
	if corePack != nil {
		stock = mergeStockSnapshot(stock, corePack.Stock)
	}

	// 核心数据包未拿到有效行情时，降级单独拉取一次实时行情
	if stock.Price == 0 && a.marketService != nil {
		stocks, _ := a.marketService.GetStockRealTimeData(req.StockCode)
		if len(stocks) > 0 {
			stock = mergeStockSnapshot(stock, stocks[0])
		}
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
	if position != nil && position.Shares > 0 {
		log.Info("Meeting: loaded position code=%s shares=%d cost=%.2f", req.StockCode, position.Shares, position.CostPrice)
	} else {
		log.Info("Meeting: loaded position code=%s none", req.StockCode)
	}

	intentContext := a.buildIntentContext(req.StockCode, req.Content)
	decisionCtx, decisionText := a.buildDecisionSupportContext(req.StockCode, stock, req.Content, position, corePack)
	intentContext = mergeContextText(intentContext, decisionText)

	// 判断是否为智能模式（无 @ 任何人）
	if len(req.MentionIds) == 0 {
		return a.runSmartMeeting(meetingCtx, req.StockCode, stock, req.Content, coreContext, intentContext, aiConfig, position, decisionCtx)
	}

	// 原有逻辑：@ 指定专家
	return a.runDirectMeeting(meetingCtx, req, stock, coreContext, intentContext, aiConfig, position, decisionCtx)
}

// runSmartMeeting 智能会议模式
func (a *App) runSmartMeeting(ctx context.Context, stockCode string, stock models.Stock, query string, coreContext string, intentContext string, aiConfig *models.AIConfig, position *models.StockPosition, decisionCtx *services.DecisionSupportContext) []models.ChatMessage {
	allAgents := a.strategyService.GetEnabledAgents()
	chatReq := meeting.ChatRequest{
		Stock:         stock,
		Query:         query,
		ReplyContent:  a.buildRecentDiscussionContext(stockCode, query),
		AllAgents:     allAgents,
		CoreContext:   coreContext,
		IntentContext: intentContext,
		Position:      position,
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

	summary := extractSummaryFromMeeting(responses)
	if a.decisionService != nil && decisionCtx != nil && strings.TrimSpace(summary) != "" {
		a.decisionService.RecordDecision(stockCode, stock, query, summary, *decisionCtx)
	}
	return messages
}

// runDirectMeeting 直接 @ 指定专家模式（带事件推送）
func (a *App) runDirectMeeting(ctx context.Context, req MeetingMessageRequest, stock models.Stock, coreContext string, intentContext string, aiConfig *models.AIConfig, position *models.StockPosition, decisionCtx *services.DecisionSupportContext) []models.ChatMessage {
	agentConfigs := a.strategyService.GetAgentsByIDs(req.MentionIds)
	if len(agentConfigs) == 0 {
		return []models.ChatMessage{}
	}

	chatReq := meeting.ChatRequest{
		Stock:         stock,
		Agents:        agentConfigs,
		Query:         req.Content,
		ReplyContent:  req.ReplyContent,
		CoreContext:   coreContext,
		IntentContext: intentContext,
		Position:      position,
	}

	responses, err := a.meetingService.SendMessage(ctx, aiConfig, chatReq)
	if err != nil {
		log.Error("runDirectMeeting error: %v", err)
		return []models.ChatMessage{}
	}

	// 转换并保存响应，同时推送事件
	messages := a.convertSaveAndEmitResponses(req.StockCode, responses, req.ReplyToId)

	// 直连模式也生成小韭菜总结
	if a.meetingService != nil {
		summary, err := a.meetingService.SummarizeDirect(ctx, aiConfig, &stock, req.Content, responses, coreContext, intentContext, position)
		if err != nil {
			log.Warn("direct summary error: %v", err)
		} else if summary != "" {
			msg := models.ChatMessage{
				AgentID:   "moderator",
				AgentName: "小韭菜",
				Role:      "会议主持",
				Content:   summary,
				Round:     1,
				MsgType:   "summary",
			}
			a.sessionService.AddMessage(req.StockCode, msg)
			runtime.EventsEmit(a.ctx, "meeting:message:"+req.StockCode, msg)
			messages = append(messages, msg)
		}
		if a.decisionService != nil && decisionCtx != nil && strings.TrimSpace(summary) != "" {
			a.decisionService.RecordDecision(req.StockCode, stock, req.Content, summary, *decisionCtx)
		}
	}

	return messages
}

func (a *App) buildCoreContext(stockCode string) string {
	_, contextText := a.buildCoreContextPack(stockCode)
	return contextText
}

func (a *App) buildCoreContextPack(stockCode string) (*models.CoreDataPack, string) {
	if a.coreDataService == nil {
		return nil, ""
	}
	pack, err := a.coreDataService.GetCoreDataPack(stockCode)
	if err != nil {
		log.Warn("核心数据包获取失败: %v", err)
		return nil, ""
	}
	contextText := a.coreDataService.BuildCoreContext(pack)
	return &pack, contextText
}

func (a *App) buildIntentContext(stockCode string, query string) string {
	if a.intentDataService == nil {
		return ""
	}
	return a.intentDataService.BuildIntentContext(stockCode, query)
}

func (a *App) buildDecisionSupportContext(stockCode string, stock models.Stock, query string, position *models.StockPosition, corePack *models.CoreDataPack) (*services.DecisionSupportContext, string) {
	if a.decisionService == nil {
		return nil, ""
	}
	ctx, text := a.decisionService.BuildContextWithCorePack(stockCode, stock, query, position, corePack)
	return &ctx, text
}

func mergeStockSnapshot(base models.Stock, latest models.Stock) models.Stock {
	if strings.TrimSpace(latest.Symbol) != "" {
		base.Symbol = latest.Symbol
	}
	if strings.TrimSpace(latest.Name) != "" {
		base.Name = latest.Name
	}
	if latest.Price != 0 {
		base.Price = latest.Price
	}
	if latest.Change != 0 {
		base.Change = latest.Change
	}
	if latest.ChangePercent != 0 {
		base.ChangePercent = latest.ChangePercent
	}
	if latest.Volume != 0 {
		base.Volume = latest.Volume
	}
	if latest.Amount != 0 {
		base.Amount = latest.Amount
	}
	if strings.TrimSpace(latest.MarketCap) != "" {
		base.MarketCap = latest.MarketCap
	}
	if strings.TrimSpace(latest.Sector) != "" {
		base.Sector = latest.Sector
	}
	if latest.Open != 0 {
		base.Open = latest.Open
	}
	if latest.High != 0 {
		base.High = latest.High
	}
	if latest.Low != 0 {
		base.Low = latest.Low
	}
	if latest.PreClose != 0 {
		base.PreClose = latest.PreClose
	}
	return base
}

func mergeContextText(base string, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if base == "" {
		return extra
	}
	if extra == "" {
		return base
	}
	return base + "\n" + extra
}

func extractSummaryFromMeeting(responses []meeting.ChatResponse) string {
	if len(responses) == 0 {
		return ""
	}

	for i := len(responses) - 1; i >= 0; i-- {
		resp := responses[i]
		if strings.TrimSpace(resp.Content) == "" {
			continue
		}
		if resp.MsgType == "summary" {
			return resp.Content
		}
	}

	for i := len(responses) - 1; i >= 0; i-- {
		resp := responses[i]
		if strings.TrimSpace(resp.Content) == "" {
			continue
		}
		if resp.AgentID == "moderator" {
			return resp.Content
		}
	}

	return responses[len(responses)-1].Content
}

func (a *App) buildRecentDiscussionContext(stockCode string, currentQuery string) string {
	if a.sessionService == nil {
		return ""
	}
	messages := a.sessionService.GetMessages(stockCode)
	if len(messages) == 0 {
		return ""
	}

	trimmedQuery := strings.TrimSpace(currentQuery)
	lines := make([]string, 0, 10)
	for index := len(messages) - 1; index >= 0 && len(lines) < 10; index-- {
		msg := messages[index]
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if msg.AgentID == "user" && trimmedQuery != "" && content == trimmedQuery {
			continue
		}

		name := strings.TrimSpace(msg.AgentName)
		if name == "" {
			name = strings.TrimSpace(msg.Role)
		}
		if name == "" {
			name = msg.AgentID
		}
		lines = append(lines, fmt.Sprintf("%s：%s", name, content))
	}

	if len(lines) == 0 {
		return ""
	}

	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}

	return "【上一轮讨论记录（最近消息）】\n" + strings.Join(lines, "\n")
}

func (a *App) buildSupplementContext(stock models.Stock, query string, history []meeting.DiscussionEntry) string {
	if a.intentDataService == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(query)
	for _, entry := range history {
		if entry.Content == "" {
			continue
		}
		sb.WriteString(" ")
		sb.WriteString(entry.Content)
	}
	text := sb.String()
	if len(text) > 2000 {
		text = text[:2000]
	}
	return a.intentDataService.BuildIntentContext(stock.Symbol, text)
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

// TestAIConnection 测试 AI 配置连通性
func (a *App) TestAIConnection(config models.AIConfig) string {
	factory := adk.NewModelFactory()
	if err := factory.TestConnection(context.Background(), &config); err != nil {
		log.Error("AI 连接测试失败 [%s]: %v", config.Name, err)
		return err.Error()
	}
	log.Info("AI 连接测试成功 [%s]", config.Name)
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

// GetBoardFundFlow 获取板块资金流（行业/概念/地域）
func (a *App) GetBoardFundFlow(category string, page, pageSize int) models.BoardFundFlowList {
	if a.marketService == nil {
		return models.BoardFundFlowList{}
	}
	data, err := a.marketService.GetBoardFundFlowList(category, page, pageSize)
	if err != nil {
		log.Error("获取板块资金流失败: %v", err)
		return models.BoardFundFlowList{Category: category}
	}
	return data
}

// GetStockMoves 获取盘口异动列表（涨速/涨跌幅/资金/换手）
func (a *App) GetStockMoves(moveType string, page, pageSize int) models.StockMoveList {
	if a.marketService == nil {
		return models.StockMoveList{MoveType: moveType}
	}
	data, err := a.marketService.GetStockMovesList(moveType, page, pageSize)
	if err != nil {
		log.Error("获取盘口异动失败: %v", err)
		return models.StockMoveList{MoveType: moveType}
	}
	return data
}

// GetBoardLeaders 获取板块龙头推荐（综合涨幅与主力资金评分）
func (a *App) GetBoardLeaders(boardCode string, limit int) models.BoardLeaderList {
	if a.marketService == nil {
		return models.BoardLeaderList{BoardCode: boardCode}
	}
	data, err := a.marketService.GetBoardLeaders(boardCode, limit)
	if err != nil {
		log.Error("获取板块龙头失败: %v", err)
		return models.BoardLeaderList{BoardCode: boardCode}
	}
	return data
}

// CheckForUpdate 检查是否有可用更新
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
	return ""
}

// RestartApp 重启应用
func (a *App) RestartApp() string {
	if a.updateService == nil {
		return "更新服务未初始化"
	}
	if err := a.updateService.RestartApplication(); err != nil {
		return err.Error()
	}
	return ""
}

// GetCurrentVersion 获取当前版本
func (a *App) GetCurrentVersion() string {
	if a.updateService == nil {
		return "0.0.0"
	}
	return a.updateService.GetCurrentVersion()
}

// TradingPeriod 交易时段
type TradingPeriod struct {
	Status    string `json:"status"`
	Text      string `json:"text"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// TradingSchedule 交易时间表
type TradingSchedule struct {
	IsTradeDay  bool            `json:"isTradeDay"`
	HolidayName string          `json:"holidayName"`
	Periods     []TradingPeriod `json:"periods"`
}

// GetTradingSchedule 获取交易时间表
func (a *App) GetTradingSchedule() TradingSchedule {
	now := time.Now()
	weekday := now.Weekday()

	// 周末休市
	if weekday == time.Saturday || weekday == time.Sunday {
		return TradingSchedule{
			IsTradeDay:  false,
			HolidayName: "",
			Periods:     []TradingPeriod{},
		}
	}

	// 交易日的时间段
	return TradingSchedule{
		IsTradeDay:  true,
		HolidayName: "",
		Periods: []TradingPeriod{
			{Status: "pre_market", Text: "集合竞价", StartTime: "09:15", EndTime: "09:25"},
			{Status: "trading", Text: "早盘交易", StartTime: "09:30", EndTime: "11:30"},
			{Status: "trading", Text: "午盘交易", StartTime: "13:00", EndTime: "15:00"},
		},
	}
}
