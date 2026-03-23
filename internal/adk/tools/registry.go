package tools

import (
	"fmt"

	"github.com/run-bigpig/jcp/internal/services"
	"github.com/run-bigpig/jcp/internal/services/hottrend"

	"google.golang.org/adk/tool"
)

// ToolInfo 工具信息
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Registry 工具注册中心
type Registry struct {
	marketService         *services.MarketService
	newsService           *services.NewsService
	configService         *services.ConfigService
	researchReportService *services.ResearchReportService
	f10Service            *services.F10Service
	coreDataService       *services.CoreDataService
	hotTrendService       *hottrend.HotTrendService
	longHuBangService     *services.LongHuBangService
	tools                 map[string]tool.Tool
	toolInfos             map[string]ToolInfo // 工具信息映射
}

// NewRegistry 创建工具注册中心
func NewRegistry(
	marketService *services.MarketService,
	newsService *services.NewsService,
	configService *services.ConfigService,
	researchReportService *services.ResearchReportService,
	f10Service *services.F10Service,
	coreDataService *services.CoreDataService,
	hotTrendService *hottrend.HotTrendService,
	longHuBangService *services.LongHuBangService,
) *Registry {
	r := &Registry{
		marketService:         marketService,
		newsService:           newsService,
		configService:         configService,
		researchReportService: researchReportService,
		f10Service:            f10Service,
		coreDataService:       coreDataService,
		hotTrendService:       hotTrendService,
		longHuBangService:     longHuBangService,
		tools:                 make(map[string]tool.Tool),
		toolInfos:             make(map[string]ToolInfo),
	}
	r.registerAllTools()
	return r
}

// registerAllTools 注册所有工具
func (r *Registry) registerAllTools() {
	// 注册核心数据包工具
	r.registerTool("get_core_data_pack", "获取核心数据包（行情+估值+资金流+业绩事件+主要指标+行业对比）", r.createCoreDataPackTool)

	// 注册股票实时数据工具
	r.registerTool("get_stock_realtime", "获取股票实时行情数据，包括当前价格、涨跌幅、开盘价、最高价、最低价、成交量等", r.createStockRealtimeTool)

	// 注册市场状态与指数工具
	r.registerTool("get_market_status", "获取A股交易状态（交易中/休市/盘前等）", r.createMarketStatusTool)
	r.registerTool("get_market_indices", "获取大盘指数实时数据（上证/深证/创业板）", r.createMarketIndicesTool)
	r.registerTool("get_index_fund_flow", "获取指数/板块资金流曲线（分钟/日级）", r.createIndexFundFlowTool)
	r.registerTool("get_stock_moves", "获取盘口异动榜单（涨速/跌速/涨跌幅/资金/换手）", r.createStockMovesTool)
	r.registerTool("get_board_fund_flow", "获取板块资金流排行（行业/概念/地域）", r.createBoardFundFlowTool)
	r.registerTool("get_board_leaders", "获取板块龙头候选（综合涨跌幅与主力资金评分）", r.createBoardLeadersTool)

	// 注册K线数据工具
	r.registerTool("get_kline_data", "获取股票K线数据，支持5分钟线、日线、周线、月线", r.createKLineTool)

	// 注册盘口数据工具
	r.registerTool("get_orderbook", "获取股票五档盘口数据，包括买卖五档价格和数量", r.createOrderBookTool)

	// 注册快讯工具
	r.registerTool("get_news", "获取最新财经快讯，来源于财联社", r.createNewsTool)

	// 注册股票搜索工具
	r.registerTool("search_stocks", "搜索股票，根据关键词搜索股票代码和名称", r.createSearchStocksTool)

	// 注册研报查询工具
	r.registerTool("get_research_report", "获取个股研报列表，包括券商评级、研究员、预测EPS/PE等信息", r.createResearchReportTool)

	// 注册研报内容查询工具
	r.registerTool("get_report_content", "获取研报正文内容，需要先通过 get_research_report 获取 infoCode", r.createReportContentTool)

	// 注册公告摘要工具
	r.registerTool("get_stock_announcements", "获取个股公告摘要列表（公告标题、日期、类型）", r.createStockAnnouncementsTool)

	// 注册F10分项工具
	r.registerTool("get_f10_overview", "获取F10综合概览数据（公司、财务、业绩、估值、行业、机构、经营等汇总）", r.createF10OverviewTool)
	r.registerTool("get_f10_company", "获取公司概况数据（公司简介、上市信息、主营业务、关键人员等）", r.createF10CompanyTool)
	r.registerTool("get_f10_financials", "获取财务报表（利润表、资产负债表、现金流量表）", r.createF10FinancialsTool)
	r.registerTool("get_f10_performance", "获取业绩事件（业绩预告、快报、预约披露日）", r.createF10PerformanceTool)
	r.registerTool("get_f10_operations", "获取操盘必读F10数据，包括事件提醒、公告/资讯、机构预测、研报摘要与估值指标等", r.createF10OperationsTool)
	r.registerTool("get_f10_fund_flow", "获取资金流数据（日度主力净流入/流出等序列与最新值）", r.createF10FundFlowTool)
	r.registerTool("get_f10_valuation", "获取估值指标（PE/PB/换手率/总市值/流通市值等）", r.createF10ValuationTool)
	r.registerTool("get_f10_valuation_trend", "获取估值趋势数据（市盈率/市净率/市销率/市现率），支持1年/3年/5年/10年区间", r.createF10ValuationTrendTool)
	r.registerTool("get_f10_institutions", "获取机构持股与实控人信息", r.createF10InstitutionsTool)
	r.registerTool("get_f10_industry", "获取行业分类与可比公司列表", r.createF10IndustryTool)
	r.registerTool("get_f10_business", "获取经营分析（业务构成、经营评述等）", r.createF10BusinessTool)
	r.registerTool("get_f10_bonus_financing", "获取分红与融资信息（分红、配股、增发等）", r.createF10BonusTool)
	r.registerTool("get_f10_shareholder_numbers", "获取股东户数及户均持股数据", r.createF10ShareholderNumbersTool)
	r.registerTool("get_f10_shareholder_changes", "获取股东增减持记录", r.createF10ShareholderChangesTool)
	r.registerTool("get_f10_pledge", "获取股权质押概况", r.createF10PledgeTool)
	r.registerTool("get_f10_lockup", "获取限售解禁计划", r.createF10LockupTool)
	r.registerTool("get_f10_buyback", "获取股票回购进度与计划", r.createF10BuybackTool)

	// 注册F10核心题材工具
	r.registerTool("get_f10_core_themes", "获取核心题材与所属板块数据，包含当前与历史题材", r.createF10CoreThemesTool)

	// 注册F10行业对比工具
	r.registerTool("get_f10_industry_compare", "获取同行业估值与经营指标对比数据（PE/PB/PS/PCF/PEG与ROE、毛利率等）", r.createF10IndustryCompareTool)

	// 注册F10主要指标工具
	r.registerTool("get_f10_main_indicators", "获取主要财务指标的年度与季度数据（核心指标、同比与环比）", r.createF10MainIndicatorsTool)

	// 注册舆情热点工具
	r.registerTool("get_hottrend", "获取全网舆情热点，支持微博、知乎、B站、百度、抖音、头条等平台的实时热搜榜单", r.createHotTrendTool)

	// 注册龙虎榜工具
	r.registerTool("get_longhubang", "获取A股龙虎榜数据，包括上榜股票、净买入金额、买卖金额、上榜原因等信息", r.createLongHuBangTool)

	// 注册龙虎榜营业部明细工具
	r.registerTool("get_longhubang_detail", "获取个股龙虎榜营业部买卖明细，需要提供股票代码和交易日期", r.createLongHuBangDetailTool)
}

// registerTool 注册单个工具并保存信息
func (r *Registry) registerTool(name, description string, creator func() (tool.Tool, error)) {
	t, err := creator()
	if err == nil {
		r.tools[name] = t
		r.toolInfos[name] = ToolInfo{Name: name, Description: description}
		return
	}
	fmt.Printf("[ToolRegistry] 注册失败: %s, err=%v\n", name, err)
}

// GetTool 获取指定工具
func (r *Registry) GetTool(name string) (tool.Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// GetTools 根据名称列表获取工具
func (r *Registry) GetTools(names []string) []tool.Tool {
	var result []tool.Tool
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// GetAllTools 获取所有工具
func (r *Registry) GetAllTools() []tool.Tool {
	var result []tool.Tool
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// GetAllToolNames 获取所有工具名称
func (r *Registry) GetAllToolNames() []string {
	var names []string
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// GetAllToolInfos 获取所有工具信息
func (r *Registry) GetAllToolInfos() []ToolInfo {
	var infos []ToolInfo
	for _, info := range r.toolInfos {
		infos = append(infos, info)
	}
	return infos
}

// GetToolInfosByNames 根据名称列表获取工具信息
func (r *Registry) GetToolInfosByNames(names []string) []ToolInfo {
	var infos []ToolInfo
	for _, name := range names {
		if info, ok := r.toolInfos[name]; ok {
			infos = append(infos, info)
		}
	}
	return infos
}
