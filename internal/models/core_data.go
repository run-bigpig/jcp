package models

// CoreDataPack 核心数据包（给专家快速获取关键决策信息）
type CoreDataPack struct {
	Code            string                    `json:"code"`
	UpdatedAt       string                    `json:"updatedAt,omitempty"`
	Stock           Stock                     `json:"stock,omitempty"`
	Valuation       StockValuation            `json:"valuation,omitempty"`
	FundFlow        FundFlowSeries            `json:"fundFlow,omitempty"`
	Performance     PerformanceEvents         `json:"performance,omitempty"`
	MainIndicators  F10MainIndicators         `json:"mainIndicators,omitempty"`
	IndustryMetrics F10IndustryCompareMetrics `json:"industryMetrics,omitempty"`
	Errors          map[string]string         `json:"errors,omitempty"`
}
