package adk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/httptransport"
	"github.com/run-bigpig/jcp/internal/adk/anthropic"
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
	case models.AIProviderAnthropic:
		return f.createAnthropicModel(config)
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

	return openai.NewOpenAIModel(config.ModelName, openaiCfg, config.NoSystemRole), nil
}

// normalizeAnthropicBaseURL 规范化 Anthropic BaseURL
func normalizeAnthropicBaseURL(baseURL string) string {
	if baseURL == "" {
		return "https://api.anthropic.com"
	}
	baseURL = strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return baseURL
}

// createAnthropicModel 创建 Anthropic 模型
func (f *ModelFactory) createAnthropicModel(config *models.AIConfig) (model.LLM, error) {
	baseURL := normalizeAnthropicBaseURL(config.BaseURL)
	httpClient := &http.Client{
		Transport: proxy.GetManager().GetTransport(),
	}
	return anthropic.NewAnthropicModel(config.ModelName, config.APIKey, baseURL, httpClient), nil
}

// createOpenAIResponsesModel 创建使用 Responses API 的 OpenAI 模型
func (f *ModelFactory) createOpenAIResponsesModel(config *models.AIConfig) (model.LLM, error) {
	baseURL := normalizeOpenAIBaseURL(config.BaseURL)

	// 使用代理管理器的 HTTP Client
	httpClient := &http.Client{
		Transport: proxy.GetManager().GetTransport(),
	}
	return openai.NewResponsesModel(config.ModelName, config.APIKey, baseURL, httpClient, config.NoSystemRole), nil
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
	case models.AIProviderAnthropic:
		return f.testAnthropicConnection(ctx, config)
	default:
		return fmt.Errorf("不支持的 provider: %s", config.Provider)
	}
}

// systemRoleProbeKeyword 探测暗号，不可能在正常对话中自然出现
const systemRoleProbeKeyword = "SYS_PROBE_7X3K"

// DetectSystemRoleSupport 检测 OpenAI 兼容接口是否支持 system role
// 通过系统指令要求模型回复特定暗号，检查响应中是否包含该暗号
// 返回 true 表示不支持（需要降级）
func (f *ModelFactory) DetectSystemRoleSupport(ctx context.Context, config *models.AIConfig) bool {
	if config.Provider != models.AIProviderOpenAI {
		return false // Gemini/VertexAI/Anthropic 原生支持
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	baseURL := normalizeOpenAIBaseURL(config.BaseURL)
	transport := proxy.GetManager().GetTransport()

	systemPrompt := fmt.Sprintf(
		"You must reply with exactly: %s. Do not add anything else.",
		systemRoleProbeKeyword,
	)

	var body map[string]any
	var endpoint string

	if config.UseResponses {
		endpoint = strings.TrimSuffix(baseURL, "/") + "/responses"
		body = map[string]any{
			"model":             config.ModelName,
			"max_output_tokens": 30,
			"input":             "Please follow the system instruction.",
			"instructions":      systemPrompt,
		}
	} else {
		endpoint = strings.TrimSuffix(baseURL, "/") + "/chat/completions"
		body = map[string]any{
			"model":      config.ModelName,
			"max_tokens": 30,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": "Please follow the system instruction."},
			},
		}
	}

	respBody, statusCode, err := f.doProbeRequest(ctx, endpoint, config.APIKey, transport, body)
	if err != nil {
		log.Warn("模型 [%s] system role 探测请求失败: %v", config.ModelName, err)
		return false // 请求失败时保守处理，不降级
	}

	// HTTP 非 200，接口本身不支持 system role
	if statusCode != http.StatusOK {
		log.Warn("模型 [%s] 不支持 system role (HTTP %d): %s",
			config.ModelName, statusCode, string(respBody))
		return true
	}

	// 从响应中提取文本内容，检查是否包含暗号
	replyText := f.extractReplyText(respBody, config.UseResponses)
	if strings.Contains(replyText, systemRoleProbeKeyword) {
		log.Info("模型 [%s] 支持 system role（暗号匹配）", config.ModelName)
		return false
	}

	log.Warn("模型 [%s] 不遵循 system role 指令（回复: %s）",
		config.ModelName, replyText)
	return true
}

// testOpenAIConnection 测试 OpenAI 兼容接口连通性
// 根据 UseResponses 配置决定使用 Responses API 或 Chat Completions API
func (f *ModelFactory) testOpenAIConnection(ctx context.Context, config *models.AIConfig) error {
	baseURL := normalizeOpenAIBaseURL(config.BaseURL)
	transport := proxy.GetManager().GetTransport()

	var body map[string]interface{}
	var endpoint string

	if config.UseResponses {
		// 使用 Responses API 端点测试
		endpoint = strings.TrimSuffix(baseURL, "/") + "/responses"
		body = map[string]interface{}{
			"model":             config.ModelName,
			"max_output_tokens": 1,
			"input":             "hi",
		}
	} else {
		// 使用 Chat Completions API 端点测试
		endpoint = strings.TrimSuffix(baseURL, "/") + "/chat/completions"
		body = map[string]interface{}{
			"model":      config.ModelName,
			"max_tokens": 1,
			"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("请求构造失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(jsonBody)))
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

// testAnthropicConnection 测试 Anthropic 连通性
func (f *ModelFactory) testAnthropicConnection(ctx context.Context, config *models.AIConfig) error {
	baseURL := normalizeAnthropicBaseURL(config.BaseURL)
	transport := proxy.GetManager().GetTransport()

	body := map[string]any{
		"model":      config.ModelName,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("请求构造失败: %w", err)
	}

	endpoint, err := url.JoinPath(baseURL, "v1", "messages")
	if err != nil {
		return fmt.Errorf("无效 BaseURL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("请求创建失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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

// doProbeRequest 发送探测请求，返回响应体、状态码
func (f *ModelFactory) doProbeRequest(ctx context.Context, endpoint, apiKey string, transport http.RoundTripper, body map[string]any) ([]byte, int, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("请求构造失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, 0, fmt.Errorf("请求创建失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return respBody, resp.StatusCode, nil
}

// extractReplyText 从 API 响应 JSON 中提取模型回复文本
func (f *ModelFactory) extractReplyText(respBody []byte, useResponses bool) string {
	if useResponses {
		return f.extractResponsesReplyText(respBody)
	}
	return f.extractChatCompletionReplyText(respBody)
}

// extractChatCompletionReplyText 从 Chat Completions 响应中提取文本
func (f *ModelFactory) extractChatCompletionReplyText(respBody []byte) string {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return ""
	}
	if len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

// extractResponsesReplyText 从 Responses API 响应中提取文本
func (f *ModelFactory) extractResponsesReplyText(respBody []byte) string {
	var resp struct {
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return ""
	}
	return resp.OutputText
}
