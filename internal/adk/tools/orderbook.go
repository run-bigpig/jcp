package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetOrderBookInput 盘口数据输入参数
type GetOrderBookInput struct {
	Code         string `json:"code,omitempty" jsonschema:"股票代码，如 sh600519"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：股票代码"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：股票代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：股票代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：股票代码"`
}

// GetOrderBookOutput 盘口数据输出
type GetOrderBookOutput struct {
	Data string `json:"data" jsonschema:"五档盘口数据"`
}

// createOrderBookTool 创建盘口数据工具
func (r *Registry) createOrderBookTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetOrderBookInput) (GetOrderBookOutput, error) {
		code := resolveStockCodeFromCandidates(
			ctx,
			input.Code,
			input.Symbol,
			input.StockCode,
			input.Ticker,
			input.SecurityCode,
			input.SecuCode,
		)
		fmt.Printf("[Tool:get_orderbook] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)

		if code == "" {
			fmt.Println("[Tool:get_orderbook] 错误: 未提供股票代码")
			return GetOrderBookOutput{Data: "请提供股票代码"}, nil
		}

		ob, err := r.marketService.GetRealOrderBook(code)
		if err != nil {
			fmt.Printf("[Tool:get_orderbook] 错误: %v\n", err)
			return GetOrderBookOutput{}, err
		}

		// 格式化输出
		result := "【卖盘】\n"
		for i := len(ob.Asks) - 1; i >= 0; i-- {
			a := ob.Asks[i]
			result += fmt.Sprintf("卖%d: %.2f x %d手\n", i+1, a.Price, a.Size)
		}
		result += "【买盘】\n"
		for i, b := range ob.Bids {
			result += fmt.Sprintf("买%d: %.2f x %d手\n", i+1, b.Price, b.Size)
		}

		fmt.Printf("[Tool:get_orderbook] 调用完成, 买盘%d档, 卖盘%d档\n", len(ob.Bids), len(ob.Asks))
		return GetOrderBookOutput{Data: result}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_orderbook",
		Description: "获取股票五档盘口数据，显示买卖五档的价格和挂单量",
	}, handler)
}
