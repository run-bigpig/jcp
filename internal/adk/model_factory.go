package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/httptransport"
	"github.com/run-bigpig/jcp/internal/adk/openai"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"

	"github.com/run-bigpig/jcp/internal/logger"
	go_openai "github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

var log = logger.New("ModelFactory")

// ModelFactory 模型工厂，根据配置创建对应的 adk model
type ModelFactory struct{}

// NewModelFactory 创建模型工厂
func NewModelFactory() *ModelFactory {
	return &ModelFactory{}
}

func resolveHTTPClientTimeoutSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout < 5*time.Second {
		return 5 * time.Second
	}
	if timeout > time.Hour {
		return time.Hour
	}
	return timeout
}

// CreateModel 根据 AI 配置创建对应的模型
func (f *ModelFactory) CreateModel(ctx context.Context, config *models.AIConfig) (model.LLM, error) {
	switch config.Provider {
	case models.AIProviderGemini:
		return f.createGeminiModel(ctx, config)
	case models.AIProviderVertexAI:
		return f.createVertexAIModel(ctx, config)
	case models.AIProviderOpenAI:
		if config.UseResponses {
			return f.createOpenAIResponsesModel(config)
		}
		return f.createOpenAIModel(config)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}

// createGeminiModel 创建 Gemini 模型
func (f *ModelFactory) createGeminiModel(ctx context.Context, config *models.AIConfig) (model.LLM, error) {
	clientConfig := &genai.ClientConfig{
		APIKey:  config.APIKey,
		Backend: genai.BackendGeminiAPI,
		// 注入代理 Transport
		HTTPClient: &http.Client{
			Transport: proxy.GetManager().GetTransport(),
		},
	}

	return gemini.NewModel(ctx, config.ModelName, clientConfig)
}

// createVertexAIModel 创建 Vertex AI 模型
func (f *ModelFactory) createVertexAIModel(ctx context.Context, config *models.AIConfig) (model.LLM, error) {
	// 获取代理 Transport
	proxyTransport := proxy.GetManager().GetTransport()

	// 获取凭证
	var creds *auth.Credentials
	var err error

	if config.CredentialsJSON != "" {
		// 使用提供的证书 JSON
		creds, err = credentials.DetectDefault(&credentials.DetectOptions{
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
			CredentialsJSON: []byte(config.CredentialsJSON),
			Client: &http.Client{
				Transport: proxyTransport,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create credentials: %w", err)
		}
	} else {
		// 使用默认凭证
		creds, err = credentials.DetectDefault(&credentials.DetectOptions{
			Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			Client: &http.Client{
				Transport: proxyTransport,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to detect default credentials: %w", err)
		}
	}

	// 使用 httptransport.NewClient 创建带认证和代理的 HTTP Client
	// BaseRoundTripper 用于注入代理 Transport，Credentials 用于自动添加认证 header
	httpClient, err := httptransport.NewClient(&httptransport.Options{
		Credentials:      creds,
		BaseRoundTripper: proxyTransport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated HTTP client: %w", err)
	}

	clientConfig := &genai.ClientConfig{
		Backend:     genai.BackendVertexAI,
		Project:     config.Project,
		Location:    config.Location,
		Credentials: creds,
		HTTPClient:  httpClient,
	}

	return gemini.NewModel(ctx, config.ModelName, clientConfig)
}

// normalizeOpenAIBaseURL 规范化 OpenAI BaseURL
// 确保 URL 以 /v1 结尾，兼容用户填写带或不带 /v1 的地址
func normalizeOpenAIBaseURL(baseURL string) string {
	if baseURL == "" {
		return "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}
	return baseURL
}

// createOpenAIModel 创建 OpenAI 兼容模型
func (f *ModelFactory) createOpenAIModel(config *models.AIConfig) (model.LLM, error) {
	openaiCfg := go_openai.DefaultConfig(config.APIKey)
	openaiCfg.BaseURL = normalizeOpenAIBaseURL(config.BaseURL)
	timeout := resolveHTTPClientTimeoutSeconds(config.Timeout)
	// 注入代理 Transport
	openaiCfg.HTTPClient = &http.Client{
		Transport: proxy.GetManager().GetTransport(),
		Timeout:   timeout,
	}
	log.Debug("create openai model: model=%s timeout=%s", config.ModelName, timeout)

	return openai.NewOpenAIModel(config.ModelName, openaiCfg, config.APIKey, openaiCfg.BaseURL, openaiCfg.HTTPClient), nil
}

// createOpenAIResponsesModel 创建使用 Responses API 的 OpenAI 模型
func (f *ModelFactory) createOpenAIResponsesModel(config *models.AIConfig) (model.LLM, error) {
	baseURL := normalizeOpenAIBaseURL(config.BaseURL)
	timeout := resolveHTTPClientTimeoutSeconds(config.Timeout)

	// 使用代理管理器的 HTTP Client
	httpClient := &http.Client{
		Transport: proxy.GetManager().GetTransport(),
		Timeout:   timeout,
	}
	log.Debug("create openai responses model: model=%s timeout=%s", config.ModelName, timeout)

	// 主路径优先使用 Responses API；若网关兼容性不足导致空响应/错误，则自动回退到 Chat Completions。
	primary := openai.NewResponsesModel(config.ModelName, config.APIKey, baseURL, httpClient)

	openaiCfg := go_openai.DefaultConfig(config.APIKey)
	openaiCfg.BaseURL = baseURL
	openaiCfg.HTTPClient = httpClient
	secondary := openai.NewOpenAIModel(config.ModelName, openaiCfg, config.APIKey, baseURL, httpClient)

	return newFallbackModel(primary, secondary, config.ModelName), nil
}

// TestConnection 测试 AI 配置的连通性
// 通过发送一个最小请求来验证 API Key、Base URL、模型名称是否正确
func (f *ModelFactory) TestConnection(ctx context.Context, config *models.AIConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	switch config.Provider {
	case models.AIProviderOpenAI:
		return f.testOpenAIConnection(ctx, config)
	case models.AIProviderGemini:
		return f.testGeminiConnection(ctx, config)
	case models.AIProviderVertexAI:
		return f.testVertexAIConnection(ctx, config)
	default:
		return fmt.Errorf("不支持的 provider: %s", config.Provider)
	}
}

// testOpenAIConnection 测试 OpenAI 兼容接口连通性
func (f *ModelFactory) testOpenAIConnection(ctx context.Context, config *models.AIConfig) error {
	baseURL := normalizeOpenAIBaseURL(config.BaseURL)
	transport := proxy.GetManager().GetTransport()

	// 构造最小的 chat completion 请求，max_completion_tokens=1 减少消耗
	body := map[string]interface{}{
		"model":                 config.ModelName,
		"max_completion_tokens": 1,
		"messages":              []map[string]string{{"role": "user", "content": "hi"}},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("请求构造失败: %w", err)
	}

	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("请求创建失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
}

// testGeminiConnection 测试 Gemini 连通性
func (f *ModelFactory) testGeminiConnection(ctx context.Context, config *models.AIConfig) error {
	llm, err := f.createGeminiModel(ctx, config)
	if err != nil {
		return fmt.Errorf("客户端创建失败: %w", err)
	}

	return f.testViaGenerate(ctx, llm)
}

// testVertexAIConnection 测试 Vertex AI 连通性
func (f *ModelFactory) testVertexAIConnection(ctx context.Context, config *models.AIConfig) error {
	llm, err := f.createVertexAIModel(ctx, config)
	if err != nil {
		return fmt.Errorf("客户端创建失败: %w", err)
	}

	return f.testViaGenerate(ctx, llm)
}

// testViaGenerate 通过 GenerateContent 发送最小请求测试连通性
func (f *ModelFactory) testViaGenerate(ctx context.Context, llm model.LLM) error {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hi"}}},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: 1,
		},
	}

	for _, err := range llm.GenerateContent(ctx, req, false) {
		if err != nil {
			return fmt.Errorf("调用失败: %w", err)
		}
		return nil
	}
	return nil
}
