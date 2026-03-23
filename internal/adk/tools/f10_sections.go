package tools

import (
	"fmt"

	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// F10SectionOutput 通用输出结构
type F10SectionOutput[T any] struct {
	Data   T                 `json:"data"`
	Errors map[string]string `json:"errors,omitempty"`
}

func buildF10SectionTool[T any](r *Registry, name, description string, fetch func(string) (T, error)) (tool.Tool, error) {
	handler := func(ctx tool.Context, input F10CodeInput) (F10SectionOutput[T], error) {
		var zero T
		code := input.resolveCode(ctx)
		fmt.Printf("[Tool:%s] 调用开始, rawCode=%s, resolvedCode=%s\n", name, input.Code, code)

		if code == "" {
			return F10SectionOutput[T]{Data: zero, Errors: map[string]string{"code": "未提供股票代码"}}, nil
		}

		if r.f10Service == nil {
			return F10SectionOutput[T]{Data: zero, Errors: map[string]string{"service": "F10 服务未初始化"}}, nil
		}

		data, err := fetch(code)
		if err != nil {
			fmt.Printf("[Tool:%s] 错误: %v\n", name, err)
			return F10SectionOutput[T]{Data: data, Errors: map[string]string{"service": err.Error()}}, nil
		}

		fmt.Printf("[Tool:%s] 调用完成, code=%s\n", name, code)
		return F10SectionOutput[T]{Data: data}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        name,
		Description: description,
	}, handler)
}

func (r *Registry) createF10CompanyTool() (tool.Tool, error) {
	return buildF10SectionTool[map[string]any](
		r,
		"get_f10_company",
		"获取公司概况数据（公司简介、上市信息、主营业务、关键人员等）",
		r.f10Service.GetCompanySurveyByCode,
	)
}

func (r *Registry) createF10FinancialsTool() (tool.Tool, error) {
	return buildF10SectionTool[models.FinancialStatements](
		r,
		"get_f10_financials",
		"获取财务报表（利润表、资产负债表、现金流量表）",
		r.f10Service.GetFinancialStatementsByCode,
	)
}

func (r *Registry) createF10PerformanceTool() (tool.Tool, error) {
	return buildF10SectionTool[models.PerformanceEvents](
		r,
		"get_f10_performance",
		"获取业绩事件（业绩预告、快报、预约披露日）",
		r.f10Service.GetPerformanceEventsByCode,
	)
}

func (r *Registry) createF10FundFlowTool() (tool.Tool, error) {
	return buildF10SectionTool[models.FundFlowSeries](
		r,
		"get_f10_fund_flow",
		"获取资金流数据（日度主力净流入/流出等序列与最新值）",
		r.f10Service.GetFundFlowByCode,
	)
}

func (r *Registry) createF10ValuationTool() (tool.Tool, error) {
	return buildF10SectionTool[models.StockValuation](
		r,
		"get_f10_valuation",
		"获取估值指标（PE/PB/换手率/总市值/流通市值等）",
		r.f10Service.GetValuationByCode,
	)
}

func (r *Registry) createF10InstitutionsTool() (tool.Tool, error) {
	return buildF10SectionTool[models.InstitutionalHoldings](
		r,
		"get_f10_institutions",
		"获取机构持股与实控人信息",
		r.f10Service.GetInstitutionalHoldingsByCode,
	)
}

func (r *Registry) createF10IndustryTool() (tool.Tool, error) {
	return buildF10SectionTool[models.IndustryCompare](
		r,
		"get_f10_industry",
		"获取行业分类与可比公司列表",
		func(code string) (models.IndustryCompare, error) {
			return r.f10Service.GetIndustryCompare(code), nil
		},
	)
}

func (r *Registry) createF10BusinessTool() (tool.Tool, error) {
	return buildF10SectionTool[models.BusinessAnalysis](
		r,
		"get_f10_business",
		"获取经营分析（业务构成、经营评述等）",
		r.f10Service.GetBusinessAnalysisByCode,
	)
}

func (r *Registry) createF10BonusTool() (tool.Tool, error) {
	return buildF10SectionTool[models.BonusFinancing](
		r,
		"get_f10_bonus_financing",
		"获取分红与融资信息（分红、配股、增发等）",
		r.f10Service.GetBonusFinancingByCode,
	)
}

func (r *Registry) createF10ShareholderNumbersTool() (tool.Tool, error) {
	return buildF10SectionTool[models.ShareholderNumbers](
		r,
		"get_f10_shareholder_numbers",
		"获取股东户数及户均持股数据",
		r.f10Service.GetShareholderNumbersByCode,
	)
}

func (r *Registry) createF10ShareholderChangesTool() (tool.Tool, error) {
	return buildF10SectionTool[models.ShareholderChanges](
		r,
		"get_f10_shareholder_changes",
		"获取股东增减持记录",
		r.f10Service.GetShareholderChangesByCode,
	)
}

func (r *Registry) createF10PledgeTool() (tool.Tool, error) {
	return buildF10SectionTool[models.EquityPledge](
		r,
		"get_f10_pledge",
		"获取股权质押概况",
		r.f10Service.GetEquityPledgeByCode,
	)
}

func (r *Registry) createF10LockupTool() (tool.Tool, error) {
	return buildF10SectionTool[models.LockupRelease](
		r,
		"get_f10_lockup",
		"获取限售解禁计划",
		r.f10Service.GetLockupReleaseByCode,
	)
}

func (r *Registry) createF10BuybackTool() (tool.Tool, error) {
	return buildF10SectionTool[models.StockBuyback](
		r,
		"get_f10_buyback",
		"获取股票回购进度与计划",
		r.f10Service.GetStockBuybackByCode,
	)
}
