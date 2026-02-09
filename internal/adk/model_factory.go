package adk

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
	// 注入代理 Transport
	openaiCfg.HTTPClient = &http.Client{
		Transport: proxy.GetManager().GetTransport(),
	}

	return openai.NewOpenAIModel(config.ModelName, openaiCfg), nil
}

// createOpenAIResponsesModel 创建使用 Responses API 的 OpenAI 模型
func (f *ModelFactory) createOpenAIResponsesModel(config *models.AIConfig) (model.LLM, error) {
	baseURL := normalizeOpenAIBaseURL(config.BaseURL)

	// 使用代理管理器的 HTTP Client
	httpClient := &http.Client{
		Transport: proxy.GetManager().GetTransport(),
	}
	return openai.NewResponsesModel(config.ModelName, config.APIKey, baseURL, httpClient), nil
}
