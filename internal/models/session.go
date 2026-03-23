package models

// StockPosition 股票持仓信息
type StockPosition struct {
	Shares    int64   `json:"shares"`    // 持仓数量
	CostPrice float64 `json:"costPrice"` // 成本价
}

// StockSession 股票会话（每个自选股独立）
type StockSession struct {
	ID        string         `json:"id"`
	StockCode string         `json:"stockCode"` // 股票代码
	StockName string         `json:"stockName"` // 股票名称
	Messages  []ChatMessage  `json:"messages"`  // 讨论历史
	Position  *StockPosition `json:"position"`  // 持仓信息
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	ID        string   `json:"id"`
	AgentID   string   `json:"agentId"`
	AgentName string   `json:"agentName"`
	Role      string   `json:"role"`
	Content   string   `json:"content"`
	Timestamp int64    `json:"timestamp"`
	ReplyTo   string   `json:"replyTo,omitempty"`   // 引用的消息ID
	Mentions  []string `json:"mentions,omitempty"`  // @的成员ID列表
	Round     int      `json:"round,omitempty"`     // 讨论轮次
	MsgType   string   `json:"msgType,omitempty"`   // 消息类型: opening/opinion/summary
}
