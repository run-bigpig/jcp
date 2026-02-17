package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/run-bigpig/jcp/internal/adk/mcp"
	"github.com/run-bigpig/jcp/internal/adk/tools"
	"github.com/run-bigpig/jcp/internal/agent"
	"github.com/run-bigpig/jcp/internal/meeting"
	"github.com/run-bigpig/jcp/internal/memory"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/services"
	"github.com/run-bigpig/jcp/internal/services/hottrend"
)

var (
	appService *Service
)

// Service 简化的 API 服务
type Service struct {
	configService   *services.ConfigService
	meetingService  *meeting.Service
	toolRegistry    *tools.Registry
	mcpManager      *mcp.Manager
	agentContainer  *agent.Container
	dataDir         string
	defaultAIConfig *models.AIConfig
}

// NewService 创建服务
func NewService() (*Service, error) {
	dataDir := getDataDir()

	// 初始化配置服务
	configService, err := services.NewConfigService(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create config service: %w", err)
	}

	// 初始化各种服务
	researchReportService := services.NewResearchReportService()
	hotTrendSvc, _ := hottrend.NewHotTrendService()
	marketService := services.NewMarketService()
	newsService := services.NewNewsService()
	longHuBangService := services.NewLongHuBangService()

	// 初始化工具注册中心
	toolRegistry := tools.NewRegistry(marketService, newsService, configService, researchReportService, hotTrendSvc, longHuBangService)

	// 初始化 MCP 管理器
	mcpManager := mcp.NewManager()
	mcpManager.LoadConfigs(configService.GetConfig().MCPServers)

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
		meetingService.SetMemoryAIConfig(nil)
	}

	// 初始化 Agent 容器
	agentContainer := agent.NewContainer()

	// 设置 AI 配置解析器
	meetingService.SetAIConfigResolver(func(aiConfigID string) *models.AIConfig {
		config := configService.GetConfig()
		for _, cfg := range config.AIConfigs {
			if cfg.ID == aiConfigID {
				return &cfg
			}
		}
		return nil
	})

	// 获取默认 AI 配置
	var defaultAIConfig *models.AIConfig
	config := configService.GetConfig()
	if config.DefaultAIID != "" {
		for _, cfg := range config.AIConfigs {
			if cfg.ID == config.DefaultAIID {
				defaultAIConfig = &cfg
				break
			}
		}
	}

	return &Service{
		configService:   configService,
		meetingService:  meetingService,
		toolRegistry:    toolRegistry,
		mcpManager:      mcpManager,
		agentContainer:  agentContainer,
		dataDir:         dataDir,
		defaultAIConfig: defaultAIConfig,
	}, nil
}

// AnalyzeRequest 分析请求
type AnalyzeRequest struct {
	StockCode string `json:"stockCode"`
	Query     string `json:"query"`
}

// AnalyzeResponse 分析响应
type AnalyzeResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Results []meeting.ChatResponse `json:"results,omitempty"`
}

// Analyze 分析
func (s *Service) Analyze(c echo.Context) error {
	var req AnalyzeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, map[string]interface{}{
			"success": false,
			"message": "invalid request",
		})
	}

	if s.defaultAIConfig == nil {
		return c.JSON(400, AnalyzeResponse{
			Success: false,
			Message: "AI not configured",
		})
	}

	// 使用默认的三个专家
	agents := []models.AgentConfig{
		{ID: "1", Name: "技术分析师", Role: "技术分析", Instruction: "从技术角度分析"},
		{ID: "2", Name: "基本面分析师", Role: "基本面分析", Instruction: "从基本面角度分析"},
		{ID: "3", Name: "风控专家", Role: "风险管理", Instruction: "从风险角度分析"},
	}

	meetingReq := meeting.ChatRequest{
		Stock: models.Stock{
			Symbol: req.StockCode,
			Name:   req.StockCode,
		},
		Query:     req.Query,
		Agents:    agents,
		AllAgents: agents,
	}

	ctx := context.Background()
	responses, err := s.meetingService.SendMessage(ctx, s.defaultAIConfig, meetingReq)
	if err != nil {
		return c.JSON(500, AnalyzeResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	return c.JSON(200, AnalyzeResponse{
		Success: true,
		Results: responses,
	})
}

// ConfigRequest 配置请求
type ConfigRequest struct {
	Provider  string `json:"provider"`
	BaseURL   string `json:"baseUrl"`
	APIKey    string `json:"apiKey"`
	ModelName string `json:"modelName"`
}

// Configure 配置
func (s *Service) Configure(c echo.Context) error {
	var req ConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(400, map[string]interface{}{
			"success": false,
		})
	}

	aiConfig := models.AIConfig{
		ID:          "default",
		Name:        "Default AI",
		Provider:    models.AIProvider(req.Provider),
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		ModelName:   req.ModelName,
		MaxTokens:   4000,
		Temperature: 0.7,
		Timeout:     60,
		IsDefault:   true,
	}

	config := s.configService.GetConfig()
	config.AIConfigs = []models.AIConfig{aiConfig}
	config.DefaultAIID = "default"

	s.configService.UpdateConfig(config)
	s.defaultAIConfig = &aiConfig

	return c.JSON(200, map[string]interface{}{
		"success": true,
	})
}

// GetStatus 状态
func (s *Service) GetStatus(c echo.Context) error {
	isConfigured := s.defaultAIConfig != nil
	provider := ""
	modelName := ""

	if isConfigured {
		provider = string(s.defaultAIConfig.Provider)
		modelName = s.defaultAIConfig.ModelName
	}

	return c.JSON(200, map[string]interface{}{
		"success":    true,
		"configured": isConfigured,
		"provider":   provider,
		"modelName":  modelName,
	})
}

func getDataDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".jcp-api")
}

func main() {
	// 初始化服务
	var err error
	appService, err = NewService()
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// 创建 Echo 服务器
	e := echo.New()
	e.Use(middleware.CORS())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// 路由
	e.POST("/analyze", appService.Analyze)
	e.POST("/configure", appService.Configure)
	e.GET("/status", appService.GetStatus)
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// 启动
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting JCP API on port %s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
}
