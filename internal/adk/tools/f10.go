package tools

import (
	"fmt"

	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetF10OverviewInput F10综合数据输入参数
type GetF10OverviewInput struct {
	Code         string `json:"code,omitempty" jsonschema:"股票代码，如 sh600000 或 600000"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：股票代码"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：股票代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：股票代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：股票代码"`
}

func (in GetF10OverviewInput) resolveCode(ctx tool.Context) string {
	return resolveStockCodeFromCandidates(
		ctx,
		in.Code,
		in.Symbol,
		in.StockCode,
		in.Ticker,
		in.SecurityCode,
		in.SecuCode,
	)
}

// GetF10OverviewOutput F10综合数据输出
type GetF10OverviewOutput struct {
	Overview models.F10Overview `json:"overview"`
}

// createF10OverviewTool 创建 F10 综合数据工具
func (r *Registry) createF10OverviewTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetF10OverviewInput) (GetF10OverviewOutput, error) {
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:get_f10_overview] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)

		if code == "" {
			fmt.Println("[Tool:get_f10_overview] 错误: 未提供股票代码")
			return GetF10OverviewOutput{}, nil
		}

		if r.f10Service == nil {
			return GetF10OverviewOutput{
				Overview: models.F10Overview{
					Code:   code,
					Errors: map[string]string{"service": "F10 服务未初始化"},
				},
			}, nil
		}

		overview, err := r.f10Service.GetOverview(code)
		if err != nil {
			fmt.Printf("[Tool:get_f10_overview] 错误: %v\n", err)
			return GetF10OverviewOutput{}, err
		}

		fmt.Printf("[Tool:get_f10_overview] 调用完成, code=%s\n", overview.Code)
		return GetF10OverviewOutput{Overview: overview}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_f10_overview",
		Description: "获取A股F10综合数据，覆盖公司概况、财务报表、业绩事件、资金流、机构/股东与行业对比",
	}, handler)
}
