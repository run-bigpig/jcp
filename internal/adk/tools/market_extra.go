package tools

import (
	"fmt"

	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/services"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetMarketStatusOutput 市场状态输出
type GetMarketStatusOutput struct {
	Status services.MarketStatus `json:"status"`
}

// GetMarketIndicesOutput 大盘指数输出
type GetMarketIndicesOutput struct {
	Indices []models.MarketIndex `json:"indices"`
}

func (r *Registry) createMarketStatusTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, _ struct{}) (GetMarketStatusOutput, error) {
		fmt.Println("[Tool:get_market_status] 调用开始")
		if r.marketService == nil {
			return GetMarketStatusOutput{}, nil
		}
		status := r.marketService.GetMarketStatus()
		fmt.Println("[Tool:get_market_status] 调用完成")
		return GetMarketStatusOutput{Status: status}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_market_status",
		Description: "获取A股当前交易状态（交易中/休市/盘前等）",
	}, handler)
}

func (r *Registry) createMarketIndicesTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, _ struct{}) (GetMarketIndicesOutput, error) {
		fmt.Println("[Tool:get_market_indices] 调用开始")
		if r.marketService == nil {
			return GetMarketIndicesOutput{}, nil
		}
		indices, err := r.marketService.GetMarketIndices()
		if err != nil {
			return GetMarketIndicesOutput{}, nil
		}
		fmt.Println("[Tool:get_market_indices] 调用完成")
		return GetMarketIndicesOutput{Indices: indices}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_market_indices",
		Description: "获取大盘指数实时数据（上证/深证/创业板）",
	}, handler)
}
