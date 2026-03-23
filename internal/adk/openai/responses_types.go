package openai

// ===== Responses API 请求类型 =====

// CreateResponseRequest OpenAI Responses API 请求体（对齐 go-openai PR #1089 命名）
type CreateResponseRequest struct {
	Model              string              `json:"model"`
	Input              any                 `json:"input"` // string 或 []ResponsesInputItem
	Instructions       string              `json:"instructions,omitempty"`
	Tools              []ResponsesTool     `json:"tools,omitempty"`
	Stream             bool                `json:"stream,omitempty"`
	MaxOutputTokens    int                 `json:"max_output_tokens,omitempty"`
	Temperature        *float32            `json:"temperature,omitempty"`
	TopP               *float32            `json:"top_p,omitempty"`
	Stop               []string            `json:"stop,omitempty"`
	Reasoning          *ResponsesReasoning `json:"reasoning,omitempty"`
	PreviousResponseID string              `json:"previous_response_id,omitempty"` // 多轮对话关联
}

// ResponsesInputItem input 数组中的一条消息
type ResponsesInputItem struct {
	Role    string `json:"role,omitempty"`    // "user", "assistant", "system", "developer"
	Content any    `json:"content,omitempty"` // string 或 []ContentPart
	// function_call_output 类型专用字段
	Type   string `json:"type,omitempty"`    // "function_call_output"
	CallID string `json:"call_id,omitempty"` // 对应的函数调用 ID
	Output string `json:"output,omitempty"`  // 函数调用结果
	// function_call 类型专用字段（assistant 历史消息中的工具调用）
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ResponsesTool Responses API 工具定义（扁平化，name 在顶层）
type ResponsesTool struct {
	Type        string `json:"type"` // "function"
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
	Strict      bool   `json:"strict,omitempty"`
}

// ResponsesReasoning 推理/思考配置
type ResponsesReasoning struct {
	Effort string `json:"effort,omitempty"` // "low", "medium", "high"
}

// ===== Responses API 响应类型 =====

// CreateResponseResponse Responses API 响应（对齐 go-openai PR #1089 命名）
type CreateResponseResponse struct {
	ID         string                `json:"id"`
	Object     string                `json:"object"`
	CreatedAt  int64                 `json:"created_at"`
	Status     string                `json:"status"`
	Error      any                   `json:"error,omitempty"`
	Model      string                `json:"model"`
	Output     []ResponsesOutputItem `json:"output"`
	OutputText string                `json:"output_text"`
	Usage      *ResponsesUsage       `json:"usage,omitempty"`
}

// ResponsesOutputItem output 数组中的一项
type ResponsesOutputItem struct {
	Type   string `json:"type"` // "message", "function_call"
	ID     string `json:"id"`
	Status string `json:"status"`
	// message 类型字段
	Role    string                 `json:"role,omitempty"`
	Content []ResponsesContentPart `json:"content,omitempty"`
	// function_call 类型字段
	Name      string `json:"name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ResponsesContentPart content 中的一个部分
type ResponsesContentPart struct {
	Type    string `json:"type"`              // "output_text", "refusal", "reasoning"
	Text    string `json:"text,omitempty"`    // output_text/reasoning 常见字段
	Refusal string `json:"refusal,omitempty"` // refusal 常见字段
}

// ResponsesUsage 用量信息
type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ===== 流式 SSE 事件类型 =====

// ResponsesTextDelta 文本增量事件 (response.output_text.delta)
type ResponsesTextDelta struct {
	Type         string `json:"type"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

// ResponsesFuncCallArgsDelta 函数调用参数增量 (response.function_call_arguments.delta)
type ResponsesFuncCallArgsDelta struct {
	Type        string `json:"type"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Delta       string `json:"delta"`
}

// ResponsesOutputItemAdded 输出项添加事件 (response.output_item.added)
type ResponsesOutputItemAdded struct {
	Type        string              `json:"type"`
	OutputIndex int                 `json:"output_index"`
	Item        ResponsesOutputItem `json:"item"`
}

// ResponsesOutputItemDone 输出项完成事件 (response.output_item.done)
type ResponsesOutputItemDone struct {
	Type        string              `json:"type"`
	OutputIndex int                 `json:"output_index"`
	Item        ResponsesOutputItem `json:"item"`
}

// ResponsesCompleted 响应完成事件 (response.completed)
type ResponsesCompleted struct {
	Type     string                 `json:"type"`
	Response CreateResponseResponse `json:"response"`
}
