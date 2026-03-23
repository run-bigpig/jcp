package tools

import (
	"fmt"
	"strings"

	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// BoardFundFlowInput 板块资金流输入
type BoardFundFlowInput struct {
	Category string `json:"category,omitempty" jsonschema:"板块类型: industry/concept/region (行业/概念/地域)"`
	Page     int    `json:"page,omitempty" jsonschema:"页码，默认1"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认20"`
}

// StockMovesInput 盘口异动输入
type StockMovesInput struct {
	MoveType string `json:"moveType,omitempty" jsonschema:"异动类型: surge(涨速)/drop(跌速)/change_up/change_down/mainflow/turnover"`
	Page     int    `json:"page,omitempty" jsonschema:"页码，默认1"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认30"`
}

// IndexFundFlowInput 指数资金流输入
type IndexFundFlowInput struct {
	Code         string `json:"code,omitempty" jsonschema:"指数代码或secid，如 sh000001 或 1.000001"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：指数代码或secid"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：代码"`
	IndexCode    string `json:"indexCode,omitempty" jsonschema:"兼容字段：指数代码"`
	SecID        string `json:"secid,omitempty" jsonschema:"兼容字段：secid，例如 1.000001"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：代码"`
	Interval     string `json:"interval,omitempty" jsonschema:"周期: 1/5/15/30/60/101(分钟/日)，默认1"`
	Limit        int    `json:"limit,omitempty" jsonschema:"返回条数，0表示全部"`
}

// StockAnnouncementsInput 公告输入
type StockAnnouncementsInput struct {
	Code         string `json:"code,omitempty" jsonschema:"股票代码，如 sh600000 或 600000"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：股票代码"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：股票代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：股票代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Page         int    `json:"page,omitempty" jsonschema:"页码，默认1"`
	PageSize     int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认10"`
}

// BoardLeadersInput 板块龙头推荐输入
type BoardLeadersInput struct {
	BoardCode string `json:"boardCode,omitempty" jsonschema:"板块代码，如 BK0448；可先用 get_board_fund_flow 获取"`
	Code      string `json:"code,omitempty" jsonschema:"兼容字段：板块代码"`
	Limit     int    `json:"limit,omitempty" jsonschema:"返回数量，默认6，最大30"`
}

// GetBoardFundFlowOutput 板块资金流输出
type GetBoardFundFlowOutput struct {
	Data   models.BoardFundFlowList `json:"data"`
	Errors map[string]string        `json:"errors,omitempty"`
}

// GetStockMovesOutput 盘口异动输出
type GetStockMovesOutput struct {
	Data   models.StockMoveList `json:"data"`
	Errors map[string]string    `json:"errors,omitempty"`
}

// GetIndexFundFlowOutput 指数资金流输出
type GetIndexFundFlowOutput struct {
	Data   models.FundFlowKLineSeries `json:"data"`
	Errors map[string]string          `json:"errors,omitempty"`
}

// GetStockAnnouncementsOutput 公告输出
type GetStockAnnouncementsOutput struct {
	Data   models.StockAnnouncements `json:"data"`
	Errors map[string]string         `json:"errors,omitempty"`
}

// GetBoardLeadersOutput 板块龙头推荐输出
type GetBoardLeadersOutput struct {
	Data   models.BoardLeaderList `json:"data"`
	Errors map[string]string      `json:"errors,omitempty"`
}

func (r *Registry) createBoardFundFlowTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input BoardFundFlowInput) (GetBoardFundFlowOutput, error) {
		fmt.Printf("[Tool:get_board_fund_flow] 调用开始, category=%s\n", input.Category)
		if r.marketService == nil {
			return GetBoardFundFlowOutput{Errors: map[string]string{"service": "Market 服务未初始化"}}, nil
		}
		data, err := r.marketService.GetBoardFundFlowList(input.Category, input.Page, input.PageSize)
		if err != nil {
			fmt.Printf("[Tool:get_board_fund_flow] 错误: %v\n", err)
			return GetBoardFundFlowOutput{Data: data, Errors: map[string]string{"service": err.Error()}}, nil
		}
		fmt.Printf("[Tool:get_board_fund_flow] 调用完成, category=%s\n", input.Category)
		return GetBoardFundFlowOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_board_fund_flow",
		Description: "获取板块资金流排行（行业/概念/地域），包含主力/超大/大/中/小单净流入与占比",
	}, handler)
}

func (r *Registry) createStockMovesTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input StockMovesInput) (GetStockMovesOutput, error) {
		fmt.Printf("[Tool:get_stock_moves] 调用开始, moveType=%s, page=%d, pageSize=%d\n", input.MoveType, input.Page, input.PageSize)
		if r.marketService == nil {
			return GetStockMovesOutput{Errors: map[string]string{"service": "Market 服务未初始化"}}, nil
		}
		data, err := r.marketService.GetStockMovesList(input.MoveType, input.Page, input.PageSize)
		if err != nil {
			fmt.Printf("[Tool:get_stock_moves] 错误: %v\n", err)
			return GetStockMovesOutput{Data: data, Errors: map[string]string{"service": err.Error()}}, nil
		}
		fmt.Printf("[Tool:get_stock_moves] 调用完成, moveType=%s, count=%d\n", data.MoveType, len(data.Items))
		return GetStockMovesOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_stock_moves",
		Description: "获取盘口异动榜单（涨速/跌速/涨跌幅/主力资金/换手率），用于快速筛选关注股票",
	}, handler)
}

func (r *Registry) createIndexFundFlowTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input IndexFundFlowInput) (GetIndexFundFlowOutput, error) {
		code := resolveIndexFundFlowCode(ctx, input)
		fmt.Printf("[Tool:get_index_fund_flow] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)
		if r.marketService == nil {
			return GetIndexFundFlowOutput{Errors: map[string]string{"service": "Market 服务未初始化"}}, nil
		}
		data, err := r.marketService.GetIndexFundFlowSeries(code, input.Interval, input.Limit)
		if err != nil {
			fmt.Printf("[Tool:get_index_fund_flow] 错误: %v\n", err)
			return GetIndexFundFlowOutput{Data: data, Errors: map[string]string{"service": err.Error()}}, nil
		}
		fmt.Printf("[Tool:get_index_fund_flow] 调用完成, code=%s\n", code)
		return GetIndexFundFlowOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_index_fund_flow",
		Description: "获取指数/板块资金流曲线（分钟/日级），用于判断主力净流入趋势",
	}, handler)
}

func (r *Registry) createStockAnnouncementsTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input StockAnnouncementsInput) (GetStockAnnouncementsOutput, error) {
		code := resolveStockAnnouncementCode(ctx, input)
		fmt.Printf("[Tool:get_stock_announcements] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)
		if code == "" {
			return GetStockAnnouncementsOutput{Errors: map[string]string{"code": "未提供股票代码"}}, nil
		}
		if r.marketService == nil {
			return GetStockAnnouncementsOutput{Errors: map[string]string{"service": "Market 服务未初始化"}}, nil
		}
		data, err := r.marketService.GetStockAnnouncements(code, input.Page, input.PageSize)
		if err != nil {
			fmt.Printf("[Tool:get_stock_announcements] 错误: %v\n", err)
			return GetStockAnnouncementsOutput{Data: data, Errors: map[string]string{"service": err.Error()}}, nil
		}
		fmt.Printf("[Tool:get_stock_announcements] 调用完成, code=%s\n", code)
		return GetStockAnnouncementsOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_stock_announcements",
		Description: "获取个股公告摘要列表（公告标题、日期、类型）",
	}, handler)
}

func (r *Registry) createBoardLeadersTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input BoardLeadersInput) (GetBoardLeadersOutput, error) {
		boardCode := strings.TrimSpace(input.BoardCode)
		if boardCode == "" {
			boardCode = strings.TrimSpace(input.Code)
		}
		fmt.Printf("[Tool:get_board_leaders] 调用开始, boardCode=%s, limit=%d\n", boardCode, input.Limit)
		if boardCode == "" {
			return GetBoardLeadersOutput{Errors: map[string]string{"boardCode": "未提供板块代码"}}, nil
		}
		if r.marketService == nil {
			return GetBoardLeadersOutput{Errors: map[string]string{"service": "Market 服务未初始化"}}, nil
		}
		data, err := r.marketService.GetBoardLeaders(boardCode, input.Limit)
		if err != nil {
			fmt.Printf("[Tool:get_board_leaders] 错误: %v\n", err)
			return GetBoardLeadersOutput{Data: data, Errors: map[string]string{"service": err.Error()}}, nil
		}
		fmt.Printf("[Tool:get_board_leaders] 调用完成, boardCode=%s, count=%d\n", data.BoardCode, len(data.Items))
		return GetBoardLeadersOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_board_leaders",
		Description: "获取板块龙头候选（综合涨跌幅与主力资金评分），需传入板块代码如 BK0448",
	}, handler)
}

func resolveIndexFundFlowCode(ctx tool.Context, input IndexFundFlowInput) string {
	candidates := []string{
		input.Code,
		input.Symbol,
		input.StockCode,
		input.IndexCode,
		input.SecID,
		input.Ticker,
		input.SecurityCode,
		input.SecuCode,
	}
	for _, candidate := range candidates {
		if code := normalizeIndexCandidate(candidate); code != "" {
			if isLikelyIndexCode(code) {
				return code
			}
			if benchmark := benchmarkIndexBySymbol(code); benchmark != "" {
				return benchmark
			}
		}
	}
	if code := stockCodeFromToolContext(ctx); code != "" {
		if benchmark := benchmarkIndexBySymbol(code); benchmark != "" {
			fmt.Printf("[Tool:get_index_fund_flow] 兜底命中上下文股票代码 %s -> 基准指数 %s\n", code, benchmark)
			return benchmark
		}
	}
	// 无法判断时默认上证指数，避免 agent 因缺参失败。
	fmt.Println("[Tool:get_index_fund_flow] 未提供有效代码，使用默认基准指数 sh000001")
	return "sh000001"
}

func resolveStockAnnouncementCode(ctx tool.Context, input StockAnnouncementsInput) string {
	candidates := []string{
		input.Code,
		input.Symbol,
		input.StockCode,
		input.Ticker,
		input.SecurityCode,
		input.SecuCode,
	}
	for _, candidate := range candidates {
		if code := normalizeStockSymbol(candidate); code != "" {
			return code
		}
	}
	if code := stockCodeFromToolContext(ctx); code != "" {
		fmt.Printf("[Tool:get_stock_announcements] 兜底命中上下文股票代码: %s\n", code)
		return code
	}
	return ""
}

func normalizeIndexCandidate(raw string) string {
	value := normalizeStockSymbol(raw)
	if value != "" {
		return value
	}
	normalizedSecID := normalizeSecID(raw)
	if normalizedSecID != "" {
		return normalizedSecID
	}
	return ""
}

func normalizeSecID(raw string) string {
	raw = trimLower(raw)
	if raw == "" {
		return ""
	}
	parts := splitCodeCandidates(raw)
	for _, part := range parts {
		switch part {
		case "1.000001":
			return "sh000001"
		case "0.399001":
			return "sz399001"
		case "0.399006":
			return "sz399006"
		}
	}
	return ""
}

func isLikelyIndexCode(code string) bool {
	return code == "sh000001" || code == "sz399001" || code == "sz399006"
}

func benchmarkIndexBySymbol(symbol string) string {
	symbol = trimLower(symbol)
	switch {
	case symbol == "":
		return ""
	case symbol == "sh000001" || symbol == "sz399001" || symbol == "sz399006":
		return symbol
	case len(symbol) >= 2 && symbol[:2] == "sh":
		return "sh000001"
	case len(symbol) >= 2 && (symbol[:2] == "sz" || symbol[:2] == "bj"):
		return "sz399001"
	default:
		return ""
	}
}

func trimLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
