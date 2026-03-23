package models

// StockPeer 同行业可比公司
type StockPeer struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Market string `json:"market,omitempty"`
}

// FinancialStatements 财务报表集合
type FinancialStatements struct {
	Income   []map[string]any `json:"income,omitempty"`
	Balance  []map[string]any `json:"balance,omitempty"`
	Cashflow []map[string]any `json:"cashflow,omitempty"`
}

// PerformanceEvents 业绩事件集合
type PerformanceEvents struct {
	Forecast []map[string]any `json:"forecast,omitempty"`
	Express  []map[string]any `json:"express,omitempty"`
	Schedule []map[string]any `json:"schedule,omitempty"`
}

// FundFlowSeries 资金流序列
type FundFlowSeries struct {
	Fields []string          `json:"fields,omitempty"`
	Lines  [][]string        `json:"lines,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Latest map[string]any    `json:"latest,omitempty"`
}

// BonusFinancing 分红融资信息
type BonusFinancing struct {
	Dividend  []map[string]any `json:"dividend,omitempty"`
	Annual    []map[string]any `json:"annual,omitempty"`
	Financing []map[string]any `json:"financing,omitempty"`
	Allotment []map[string]any `json:"allotment,omitempty"`
}

// BusinessAnalysis 经营分析信息
type BusinessAnalysis struct {
	Scope       []map[string]any `json:"scope,omitempty"`
	Composition []map[string]any `json:"composition,omitempty"`
	Review      []map[string]any `json:"review,omitempty"`
}

// ShareholderNumbers 股东户数信息
type ShareholderNumbers struct {
	Records []map[string]any `json:"records,omitempty"`
	Latest  map[string]any   `json:"latest,omitempty"`
}

// EquityPledge 股权质押概况
type EquityPledge struct {
	Records []map[string]any `json:"records,omitempty"`
	Latest  map[string]any   `json:"latest,omitempty"`
}

// LockupRelease 限售解禁信息
type LockupRelease struct {
	Records []map[string]any `json:"records,omitempty"`
	Latest  map[string]any   `json:"latest,omitempty"`
}

// ShareholderChanges 股东增减持
type ShareholderChanges struct {
	Records []map[string]any `json:"records,omitempty"`
	Latest  map[string]any   `json:"latest,omitempty"`
}

// StockBuyback 股票回购
type StockBuyback struct {
	Records []map[string]any `json:"records,omitempty"`
	Latest  map[string]any   `json:"latest,omitempty"`
}

// StockValuation 估值指标
type StockValuation struct {
	Price          float64 `json:"price,omitempty"`
	PETTM          float64 `json:"peTtm,omitempty"`
	PB             float64 `json:"pb,omitempty"`
	TotalMarketCap float64 `json:"totalMarketCap,omitempty"`
	FloatMarketCap float64 `json:"floatMarketCap,omitempty"`
	TurnoverRate   float64 `json:"turnoverRate,omitempty"`
	Amplitude      float64 `json:"amplitude,omitempty"`
	TotalShares    float64 `json:"totalShares,omitempty"`
	FloatShares    float64 `json:"floatShares,omitempty"`
}

// F10ValuationTrend 估值趋势
type F10ValuationTrend struct {
	Source         string            `json:"source,omitempty"`
	Range          string            `json:"range,omitempty"`
	RequestedRange string            `json:"requestedRange,omitempty"`
	Fallback       bool              `json:"fallback,omitempty"`
	DateType       int               `json:"dateType,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	PE             []map[string]any  `json:"pe,omitempty"`
	PB             []map[string]any  `json:"pb,omitempty"`
	PS             []map[string]any  `json:"ps,omitempty"`
	PCF            []map[string]any  `json:"pcf,omitempty"`
}

// F10OperationsRequired 操盘必读数据
type F10OperationsRequired struct {
	LatestIndicators      map[string]any   `json:"latestIndicators,omitempty"`
	LatestIndicatorsExtra map[string]any   `json:"latestIndicatorsExtra,omitempty"`
	LatestIndicatorsQuote map[string]any   `json:"latestIndicatorsQuote,omitempty"`
	EventReminders        []map[string]any `json:"eventReminders,omitempty"`
	News                  []map[string]any `json:"news,omitempty"`
	Announcements         []map[string]any `json:"announcements,omitempty"`
	ShareholderAnalysis   []map[string]any `json:"shareholderAnalysis,omitempty"`
	DragonTigerList       []map[string]any `json:"dragonTigerList,omitempty"`
	BlockTrades           []map[string]any `json:"blockTrades,omitempty"`
	MarginTrading         []map[string]any `json:"marginTrading,omitempty"`
	MainIndicators        []map[string]any `json:"mainIndicators,omitempty"`
	SectorTags            []map[string]any `json:"sectorTags,omitempty"`
	CoreThemes            []map[string]any `json:"coreThemes,omitempty"`
	InstitutionForecast   []map[string]any `json:"institutionForecast,omitempty"`
	ForecastChart         []map[string]any `json:"forecastChart,omitempty"`
	ReportSummary         []map[string]any `json:"reportSummary,omitempty"`
	ResearchReports       []map[string]any `json:"researchReports,omitempty"`
	ForecastRevisionTrack []map[string]any `json:"forecastRevisionTrack,omitempty"`
}

// F10Management 公司高管信息
type F10Management struct {
	ManagementList []map[string]any `json:"managementList,omitempty"`
	SalaryDetails  []map[string]any `json:"salaryDetails,omitempty"`
	HoldingChanges []map[string]any `json:"holdingChanges,omitempty"`
}

// F10CapitalOperation 资本运作信息
type F10CapitalOperation struct {
	RaiseSources    []map[string]any `json:"raiseSources,omitempty"`
	ProjectProgress []map[string]any `json:"projectProgress,omitempty"`
}

// F10EquityStructure 股本结构
type F10EquityStructure struct {
	Latest      []map[string]any `json:"latest,omitempty"`
	History     []map[string]any `json:"history,omitempty"`
	Composition []map[string]any `json:"composition,omitempty"`
}

// F10RelatedStocks 关联个股
type F10RelatedStocks struct {
	IndustryRankings []map[string]any `json:"industryRankings,omitempty"`
	ConceptRelations []map[string]any `json:"conceptRelations,omitempty"`
}

// F10CoreThemes 核心题材
type F10CoreThemes struct {
	BoardTypes           []map[string]any `json:"boardTypes,omitempty"`
	Themes               []map[string]any `json:"themes,omitempty"`
	History              []map[string]any `json:"history,omitempty"`
	SelectedBoardReasons []map[string]any `json:"selectedBoardReasons,omitempty"`
	PopularLeaders       []map[string]any `json:"popularLeaders,omitempty"`
}

// F10IndustryCompareMetrics 行业对比指标
type F10IndustryCompareMetrics struct {
	Valuation   []map[string]any `json:"valuation,omitempty"`
	Performance []map[string]any `json:"performance,omitempty"`
	Growth      []map[string]any `json:"growth,omitempty"`
}

// F10MainIndicators 主要指标
type F10MainIndicators struct {
	Latest    []map[string]any `json:"latest,omitempty"`
	Yearly    []map[string]any `json:"yearly,omitempty"`
	Quarterly []map[string]any `json:"quarterly,omitempty"`
}

// InstitutionalHoldings 机构/股东数据
type InstitutionalHoldings struct {
	TopHolders []map[string]any `json:"topHolders,omitempty"`
	Controller map[string]any   `json:"controller,omitempty"`
}

// IndustryCompare 行业对比信息
type IndustryCompare struct {
	Industry string      `json:"industry,omitempty"`
	Peers    []StockPeer `json:"peers,omitempty"`
}

// F10Overview F10 综合数据
type F10Overview struct {
	Code             string                    `json:"code"`
	UpdatedAt        string                    `json:"updatedAt,omitempty"`
	Source           string                    `json:"source,omitempty"`
	Company          map[string]any            `json:"company,omitempty"`
	Financials       FinancialStatements       `json:"financials,omitempty"`
	Performance      PerformanceEvents         `json:"performance,omitempty"`
	FundFlow         FundFlowSeries            `json:"fundFlow,omitempty"`
	Institutions     InstitutionalHoldings     `json:"institutions,omitempty"`
	Industry         IndustryCompare           `json:"industry,omitempty"`
	Bonus            BonusFinancing            `json:"bonus,omitempty"`
	Business         BusinessAnalysis          `json:"business,omitempty"`
	Shareholders     ShareholderNumbers        `json:"shareholders,omitempty"`
	Pledge           EquityPledge              `json:"pledge,omitempty"`
	Lockup           LockupRelease             `json:"lockup,omitempty"`
	HolderChange     ShareholderChanges        `json:"holderChange,omitempty"`
	Buyback          StockBuyback              `json:"buyback,omitempty"`
	Valuation        StockValuation            `json:"valuation,omitempty"`
	Operations       F10OperationsRequired     `json:"operations,omitempty"`
	CoreThemes       F10CoreThemes             `json:"coreThemes,omitempty"`
	IndustryMetrics  F10IndustryCompareMetrics `json:"industryMetrics,omitempty"`
	MainIndicators   F10MainIndicators         `json:"mainIndicators,omitempty"`
	Management       F10Management             `json:"management,omitempty"`
	CapitalOperation F10CapitalOperation       `json:"capitalOperation,omitempty"`
	EquityStructure  F10EquityStructure        `json:"equityStructure,omitempty"`
	RelatedStocks    F10RelatedStocks          `json:"relatedStocks,omitempty"`
	ValuationTrend   F10ValuationTrend         `json:"valuationTrend,omitempty"`
	Errors           map[string]string         `json:"errors,omitempty"`
}
