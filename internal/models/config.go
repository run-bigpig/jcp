package models

// AIProvider AI服务提供商类型
type AIProvider string

const (
	AIProviderOpenAI   AIProvider = "openai"
	AIProviderGemini   AIProvider = "gemini"
	AIProviderVertexAI AIProvider = "vertexai"
)

// AgentSelectionStyle 小韭菜选人风格
type AgentSelectionStyle string

const (
	AgentSelectionBalanced     AgentSelectionStyle = "balanced"     // 平衡（默认）
	AgentSelectionConservative AgentSelectionStyle = "conservative" // 稳健优先
	AgentSelectionAggressive   AgentSelectionStyle = "aggressive"   // 激进优先
)

// AIConfig AI服务配置
type AIConfig struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Provider    AIProvider `json:"provider"`
	BaseURL     string     `json:"baseUrl"`
	APIKey      string     `json:"apiKey"`
	ModelName   string     `json:"modelName"`
	MaxTokens   int        `json:"maxTokens"`
	Temperature float64    `json:"temperature"`
	Timeout     int        `json:"timeout"`
	IsDefault   bool       `json:"isDefault"`
	// OpenAI Responses API 开关
	UseResponses bool `json:"useResponses"`
	// Vertex AI 专用字段
	Project         string `json:"project"`
	Location        string `json:"location"`
	CredentialsJSON string `json:"credentialsJson"`
}

// MCPTransportType MCP传输类型
type MCPTransportType string

const (
	MCPTransportHTTP    MCPTransportType = "http"    // StreamableHTTP 传输
	MCPTransportSSE     MCPTransportType = "sse"     // SSE 传输（已废弃）
	MCPTransportCommand MCPTransportType = "command" // 命令行传输
)

// MCPServerConfig MCP服务器配置
type MCPServerConfig struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	TransportType MCPTransportType `json:"transportType"`
	Endpoint      string           `json:"endpoint"`   // HTTP/SSE 端点 URL
	Command       string           `json:"command"`    // 命令行传输的命令
	Args          []string         `json:"args"`       // 命令行参数
	ToolFilter    []string         `json:"toolFilter"` // 工具过滤列表（空则全部）
	Enabled       bool             `json:"enabled"`    // 是否启用
}

// AppConfig 应用配置
type AppConfig struct {
	Theme               string              `json:"theme"`           // 主题色: military, ocean, purple, orange, dark
	CandleColorMode     string              `json:"candleColorMode"` // K线颜色模式: red-up, green-up
	AIConfigs           []AIConfig          `json:"aiConfigs"`
	DefaultAIID         string              `json:"defaultAiId"`
	StrategyAIID        string              `json:"strategyAiId"`        // 策略生成用AI
	ModeratorAIID       string              `json:"moderatorAiId"`       // 意图分析(小韭菜)用AI
	AIRetryCount        int                 `json:"aiRetryCount"`        // AI 请求自动重试次数
	VerboseAgentIO      bool                `json:"verboseAgentIO"`      // 是否输出完整 Agent 请求/响应日志
	AgentSelectionStyle AgentSelectionStyle `json:"agentSelectionStyle"` // 小韭菜选人风格
	EnableSecondReview  bool                `json:"enableSecondReview"`  // 是否启用二轮复议（补充数据校正轮）
	MCPServers          []MCPServerConfig   `json:"mcpServers"`          // MCP服务器配置列表
	Memory              MemoryConfig        `json:"memory"`              // 记忆管理配置
	Proxy               ProxyConfig         `json:"proxy"`               // 代理配置
	Layout              LayoutConfig        `json:"layout"`              // 界面布局配置
}

// ProxyMode 代理模式
type ProxyMode string

const (
	ProxyModeNone   ProxyMode = "none"   // 无代理，直连
	ProxyModeSystem ProxyMode = "system" // 使用系统代理
	ProxyModeCustom ProxyMode = "custom" // 自定义代理
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	Mode      ProxyMode `json:"mode"`
	CustomURL string    `json:"customUrl"` // 自定义代理地址
}

// MemoryConfig 记忆管理配置
type MemoryConfig struct {
	Enabled           bool   `json:"enabled"`           // 是否启用记忆管理
	AIConfigID        string `json:"aiConfigId"`        // 使用的 LLM 配置 ID（空则使用默认）
	MaxRecentRounds   int    `json:"maxRecentRounds"`   // 保留最近几轮讨论
	MaxKeyFacts       int    `json:"maxKeyFacts"`       // 最大关键事实数
	MaxSummaryLength  int    `json:"maxSummaryLength"`  // 摘要最大字数
	CompressThreshold int    `json:"compressThreshold"` // 触发压缩的轮次数
}

// LayoutConfig 界面布局配置
type LayoutConfig struct {
	LeftPanelWidth    int `json:"leftPanelWidth"`    // 左侧面板宽度(px)
	RightPanelWidth   int `json:"rightPanelWidth"`   // 右侧面板宽度(px)
	BottomPanelHeight int `json:"bottomPanelHeight"` // 底部面板高度(px)
	WindowWidth       int `json:"windowWidth"`       // 窗口宽度(px)
	WindowHeight      int `json:"windowHeight"`      // 窗口高度(px)
}
