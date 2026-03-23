package tools

import (
	"fmt"

	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// F10CodeInput F10 数据通用输入参数
type F10CodeInput struct {
	Code         string `json:"code,omitempty" jsonschema:"股票代码，如 sh600000 或 600000"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：股票代码"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：股票代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：股票代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：股票代码"`
}

func (in F10CodeInput) resolveCode(ctx tool.Context) string {
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

// GetF10OperationsOutput 操盘必读数据输出
type GetF10OperationsOutput struct {
	Data   models.F10OperationsRequired `json:"data"`
	Errors map[string]string            `json:"errors,omitempty"`
}

// GetF10CoreThemesOutput 核心题材数据输出
type GetF10CoreThemesOutput struct {
	Data   models.F10CoreThemes `json:"data"`
	Errors map[string]string    `json:"errors,omitempty"`
}

// GetF10IndustryCompareOutput 行业对比数据输出
type GetF10IndustryCompareOutput struct {
	Data   models.F10IndustryCompareMetrics `json:"data"`
	Errors map[string]string                `json:"errors,omitempty"`
}

// GetF10MainIndicatorsOutput 主要指标数据输出
type GetF10MainIndicatorsOutput struct {
	Data   models.F10MainIndicators `json:"data"`
	Errors map[string]string        `json:"errors,omitempty"`
}

// F10ValuationTrendInput 估值趋势输入参数
type F10ValuationTrendInput struct {
	Code         string `json:"code,omitempty" jsonschema:"股票代码，如 sh600000 或 600000"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：股票代码"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：股票代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：股票代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Range        string `json:"range,omitempty" jsonschema:"估值区间，可选 1y/3y/5y/10y，默认5y"`
}

func (in F10ValuationTrendInput) resolveCode(ctx tool.Context) string {
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

// GetF10ValuationTrendOutput 估值趋势输出
type GetF10ValuationTrendOutput struct {
	Data   models.F10ValuationTrend `json:"data"`
	Errors map[string]string        `json:"errors,omitempty"`
}

func (r *Registry) createF10OperationsTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input F10CodeInput) (GetF10OperationsOutput, error) {
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:get_f10_operations] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)

		if code == "" {
			return GetF10OperationsOutput{
				Errors: map[string]string{"code": "未提供股票代码"},
			}, nil
		}

		if r.f10Service == nil {
			return GetF10OperationsOutput{
				Errors: map[string]string{"service": "F10 服务未初始化"},
			}, nil
		}

		data, err := r.f10Service.GetOperationsRequired(code)
		if err != nil {
			fmt.Printf("[Tool:get_f10_operations] 错误: %v\n", err)
			return GetF10OperationsOutput{
				Data:   data,
				Errors: map[string]string{"service": err.Error()},
			}, nil
		}

		fmt.Printf("[Tool:get_f10_operations] 调用完成, code=%s\n", code)
		return GetF10OperationsOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_f10_operations",
		Description: "获取操盘必读F10数据，包括最新指标、大事提醒、资讯公告、核心题材、机构预测、研报摘要、估值分析、主要指标、股东分析、龙虎榜、大宗交易、融资融券等",
	}, handler)
}

func (r *Registry) createF10CoreThemesTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input F10CodeInput) (GetF10CoreThemesOutput, error) {
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:get_f10_core_themes] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)

		if code == "" {
			return GetF10CoreThemesOutput{
				Errors: map[string]string{"code": "未提供股票代码"},
			}, nil
		}

		if r.f10Service == nil {
			return GetF10CoreThemesOutput{
				Errors: map[string]string{"service": "F10 服务未初始化"},
			}, nil
		}

		data, err := r.f10Service.GetCoreThemes(code)
		if err != nil {
			fmt.Printf("[Tool:get_f10_core_themes] 错误: %v\n", err)
			return GetF10CoreThemesOutput{
				Data:   data,
				Errors: map[string]string{"service": err.Error()},
			}, nil
		}

		fmt.Printf("[Tool:get_f10_core_themes] 调用完成, code=%s\n", code)
		return GetF10CoreThemesOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_f10_core_themes",
		Description: "获取核心题材与所属板块数据，包含当前与历史题材",
	}, handler)
}

func (r *Registry) createF10IndustryCompareTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input F10CodeInput) (GetF10IndustryCompareOutput, error) {
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:get_f10_industry_compare] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)

		if code == "" {
			return GetF10IndustryCompareOutput{
				Errors: map[string]string{"code": "未提供股票代码"},
			}, nil
		}

		if r.f10Service == nil {
			return GetF10IndustryCompareOutput{
				Errors: map[string]string{"service": "F10 服务未初始化"},
			}, nil
		}

		data, err := r.f10Service.GetIndustryCompareMetrics(code)
		if err != nil {
			fmt.Printf("[Tool:get_f10_industry_compare] 错误: %v\n", err)
			return GetF10IndustryCompareOutput{
				Data:   data,
				Errors: map[string]string{"service": err.Error()},
			}, nil
		}

		fmt.Printf("[Tool:get_f10_industry_compare] 调用完成, code=%s\n", code)
		return GetF10IndustryCompareOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_f10_industry_compare",
		Description: "获取同行业估值与经营指标对比数据（PE/PB/PS/PCF/PEG与ROE、毛利率等）",
	}, handler)
}

func (r *Registry) createF10MainIndicatorsTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input F10CodeInput) (GetF10MainIndicatorsOutput, error) {
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:get_f10_main_indicators] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)

		if code == "" {
			return GetF10MainIndicatorsOutput{
				Errors: map[string]string{"code": "未提供股票代码"},
			}, nil
		}

		if r.f10Service == nil {
			return GetF10MainIndicatorsOutput{
				Errors: map[string]string{"service": "F10 服务未初始化"},
			}, nil
		}

		data, err := r.f10Service.GetMainIndicators(code)
		if err != nil {
			fmt.Printf("[Tool:get_f10_main_indicators] 错误: %v\n", err)
			return GetF10MainIndicatorsOutput{
				Data:   data,
				Errors: map[string]string{"service": err.Error()},
			}, nil
		}

		fmt.Printf("[Tool:get_f10_main_indicators] 调用完成, code=%s\n", code)
		return GetF10MainIndicatorsOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_f10_main_indicators",
		Description: "获取主要财务指标的年度与季度数据（核心指标、同比与环比）",
	}, handler)
}

func (r *Registry) createF10ValuationTrendTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input F10ValuationTrendInput) (GetF10ValuationTrendOutput, error) {
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:get_f10_valuation_trend] 调用开始, rawCode=%s, resolvedCode=%s, range=%s\n", input.Code, code, input.Range)

		if code == "" {
			return GetF10ValuationTrendOutput{
				Errors: map[string]string{"code": "未提供股票代码"},
			}, nil
		}

		if r.f10Service == nil {
			return GetF10ValuationTrendOutput{
				Errors: map[string]string{"service": "F10 服务未初始化"},
			}, nil
		}

		data, err := r.f10Service.GetValuationTrend(code, input.Range)
		if err != nil {
			fmt.Printf("[Tool:get_f10_valuation_trend] 错误: %v\n", err)
			return GetF10ValuationTrendOutput{
				Data:   data,
				Errors: map[string]string{"service": err.Error()},
			}, nil
		}

		fmt.Printf("[Tool:get_f10_valuation_trend] 调用完成, code=%s\n", code)
		return GetF10ValuationTrendOutput{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_f10_valuation_trend",
		Description: "获取估值趋势数据（市盈率/市净率/市销率/市现率），支持1年/3年/5年/10年区间，缺失时自动回退到报告期估值",
	}, handler)
}
