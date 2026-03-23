package tools

import (
	"fmt"

	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetCoreDataPackInput 核心数据包输入
type GetCoreDataPackInput struct {
	Code         string `json:"code,omitempty" jsonschema:"股票代码，如 sh600000 或 600000"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：股票代码"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：股票代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：股票代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：股票代码"`
}

func (in GetCoreDataPackInput) resolveCode(ctx tool.Context) string {
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

// GetCoreDataPackOutput 核心数据包输出
type GetCoreDataPackOutput struct {
	Pack models.CoreDataPack `json:"pack"`
}

func (r *Registry) createCoreDataPackTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetCoreDataPackInput) (GetCoreDataPackOutput, error) {
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:get_core_data_pack] 调用开始, rawCode=%s, resolvedCode=%s\n", input.Code, code)

		if code == "" {
			return GetCoreDataPackOutput{}, nil
		}

		if r.coreDataService == nil {
			return GetCoreDataPackOutput{
				Pack: models.CoreDataPack{
					Code:   code,
					Errors: map[string]string{"service": "核心数据包服务未初始化"},
				},
			}, nil
		}

		pack, err := r.coreDataService.GetCoreDataPack(code)
		if err != nil {
			fmt.Printf("[Tool:get_core_data_pack] 错误: %v\n", err)
			return GetCoreDataPackOutput{}, err
		}

		fmt.Printf("[Tool:get_core_data_pack] 调用完成, code=%s\n", pack.Code)
		return GetCoreDataPackOutput{Pack: pack}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_core_data_pack",
		Description: "获取核心数据包（行情+估值+资金流+业绩事件+主要指标+行业对比），用于快速分析",
	}, handler)
}
