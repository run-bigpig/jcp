package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/embed"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

const (
	companySurveyURL      = "https://emweb.securities.eastmoney.com/PC_HSF10/CompanySurvey/PageAjax?code=%s"
	shareholderAjaxURL    = "https://emweb.securities.eastmoney.com/PC_HSF10/ShareholderResearch/PageAjax?code=%s"
	bonusFinancingURL     = "https://emweb.securities.eastmoney.com/PC_HSF10/BonusFinancing/PageAjax?code=%s"
	businessAnalysisURL   = "https://emweb.securities.eastmoney.com/PC_HSF10/BusinessAnalysis/PageAjax?code=%s"
	operationsRequiredURL = "https://emweb.securities.eastmoney.com/PC_HSF10/OperationsRequired/PageAjax?code=%s"
	researchReportListURL = "https://reportapi.eastmoney.com/report/list"
	financeDataCenterURL  = "https://datacenter.eastmoney.com/securities/api/data/get"
	dataCenterWebURL      = "https://datacenter-web.eastmoney.com/api/data/v1/get"
	dataCenterV1URL       = "https://datacenter.eastmoney.com/securities/api/data/v1/get"
	fundFlowURL           = "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"
	quoteDataURL          = "https://push2.eastmoney.com/api/qt/stock/get"

	shareholderNumbersReport = "RPT_HOLDERNUM_DET"
	equityPledgeReport       = "RPT_CSDC_LIST_NEWEST"
	lockupReleaseReport      = "RPT_LIFT_STAGE"
	shareholderChangeReport  = "RPT_SHARE_HOLDER_INCREASE"
	buybackReport            = "RPTA_WEB_GETHGDETAIL"
)

// F10Service F10 数据服务
type F10Service struct {
	client *http.Client

	cacheTTL time.Duration
	cacheMu  sync.RWMutex
	cache    map[string]cacheEntry

	basicMu     sync.Mutex
	basicLoaded bool
	basicData   f10StockBasicData
}

type cacheEntry struct {
	value     any
	timestamp time.Time
}

// f10StockBasicData stock_basic.json 的数据结构
type f10StockBasicData struct {
	Data struct {
		Fields []string        `json:"fields"`
		Items  [][]interface{} `json:"items"`
	} `json:"data"`
}

type normalizedCode struct {
	Raw      string
	Prefix   string
	Lower    string
	Upper    string
	SecID    string
	MarketID string
}

// NewF10Service 创建 F10 服务
func NewF10Service() *F10Service {
	return &F10Service{
		client:   proxy.GetManager().GetClientWithTimeout(20 * time.Second),
		cacheTTL: 3 * time.Minute,
		cache:    make(map[string]cacheEntry),
	}
}

// GetOverview 获取 F10 综合数据
func (s *F10Service) GetOverview(code string) (models.F10Overview, error) {
	normalized := normalizeStockCode(code)
	overview := models.F10Overview{
		Code:      normalized.Lower,
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		Source:    "Eastmoney",
		Errors:    make(map[string]string),
	}

	company, err := s.GetCompanySurvey(normalized)
	if company != nil {
		overview.Company = company
	}
	if err != nil {
		overview.Errors["company"] = err.Error()
	}

	financials, err := s.GetFinancialStatements(normalized)
	if hasFinancialData(financials) {
		overview.Financials = financials
	}
	if err != nil {
		overview.Errors["financials"] = err.Error()
	}

	performance, err := s.GetPerformanceEvents(normalized)
	if hasPerformanceData(performance) {
		overview.Performance = performance
	}
	if err != nil {
		overview.Errors["performance"] = err.Error()
	}

	fundFlow, err := s.GetFundFlow(normalized)
	if len(fundFlow.Lines) > 0 {
		overview.FundFlow = fundFlow
	}
	if err != nil {
		overview.Errors["fundFlow"] = err.Error()
	}

	valuation, err := s.GetValuation(normalized)
	if hasValuation(valuation) {
		overview.Valuation = valuation
	}
	if err != nil {
		overview.Errors["valuation"] = err.Error()
	}

	holdings, err := s.GetInstitutionalHoldings(normalized)
	if hasInstitutionData(holdings) {
		overview.Institutions = holdings
	}
	if err != nil {
		overview.Errors["institutions"] = err.Error()
	}

	industry := s.GetIndustryCompare(normalized.Raw)
	overview.Industry = industry

	bonus, err := s.GetBonusFinancing(normalized)
	if hasBonusFinancing(bonus) {
		overview.Bonus = bonus
	}
	if err != nil {
		overview.Errors["bonus"] = err.Error()
	}

	business, err := s.GetBusinessAnalysis(normalized)
	if hasBusinessAnalysis(business) {
		overview.Business = business
	}
	if err != nil {
		overview.Errors["business"] = err.Error()
	}

	shareholders, err := s.GetShareholderNumbers(normalized)
	if hasShareholderNumbers(shareholders) {
		overview.Shareholders = shareholders
	}
	if err != nil {
		overview.Errors["shareholders"] = err.Error()
	}

	pledge, err := s.GetEquityPledge(normalized)
	if hasEquityPledge(pledge) {
		overview.Pledge = pledge
	}
	if err != nil {
		overview.Errors["pledge"] = err.Error()
	}

	lockup, err := s.GetLockupRelease(normalized)
	if hasLockupRelease(lockup) {
		overview.Lockup = lockup
	}
	if err != nil {
		overview.Errors["lockup"] = err.Error()
	}

	holderChange, err := s.GetShareholderChanges(normalized)
	if hasShareholderChanges(holderChange) {
		overview.HolderChange = holderChange
	}
	if err != nil {
		overview.Errors["holderChange"] = err.Error()
	}

	buyback, err := s.GetStockBuyback(normalized)
	if hasStockBuyback(buyback) {
		overview.Buyback = buyback
	}
	if err != nil {
		overview.Errors["buyback"] = err.Error()
	}

	operations, err := s.GetOperationsRequired(normalized.Raw)
	if hasOperationsRequired(operations) {
		overview.Operations = operations
	}
	if err != nil {
		overview.Errors["operations"] = err.Error()
	}

	coreThemes, err := s.GetCoreThemes(normalized.Raw)
	if hasCoreThemes(coreThemes) {
		overview.CoreThemes = coreThemes
	}
	if err != nil {
		overview.Errors["coreThemes"] = err.Error()
	}

	industryMetrics, err := s.GetIndustryCompareMetrics(normalized.Raw)
	if hasIndustryMetrics(industryMetrics) {
		overview.IndustryMetrics = industryMetrics
	}
	if err != nil {
		overview.Errors["industryMetrics"] = err.Error()
	}

	mainIndicators, err := s.GetMainIndicators(normalized.Raw)
	if hasMainIndicators(mainIndicators) {
		overview.MainIndicators = mainIndicators
	}
	if err != nil {
		overview.Errors["mainIndicators"] = err.Error()
	}

	management, err := s.GetManagement(normalized.Raw)
	if hasManagement(management) {
		overview.Management = management
	}
	if err != nil {
		overview.Errors["management"] = err.Error()
	}

	capitalOperation, err := s.GetCapitalOperation(normalized.Raw)
	if hasCapitalOperation(capitalOperation) {
		overview.CapitalOperation = capitalOperation
	}
	if err != nil {
		overview.Errors["capitalOperation"] = err.Error()
	}

	equityStructure, err := s.GetEquityStructure(normalized.Raw)
	if hasEquityStructure(equityStructure) {
		overview.EquityStructure = equityStructure
	}
	if err != nil {
		overview.Errors["equityStructure"] = err.Error()
	}

	relatedStocks, err := s.GetRelatedStocks(normalized.Raw)
	if hasRelatedStocks(relatedStocks) {
		overview.RelatedStocks = relatedStocks
	}
	if err != nil {
		overview.Errors["relatedStocks"] = err.Error()
	}

	valuationTrend, err := s.GetValuationTrend(normalized.Raw, "5y")
	if hasValuationTrend(valuationTrend) {
		overview.ValuationTrend = valuationTrend
	}
	if err != nil {
		overview.Errors["valuationTrend"] = err.Error()
	}

	if len(overview.Errors) == 0 {
		overview.Errors = nil
	}
	return overview, nil
}

// GetOperationsRequired 获取操盘必读数据
func (s *F10Service) GetOperationsRequired(code string) (models.F10OperationsRequired, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "ops:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10OperationsRequired); ok {
			return data, nil
		}
	}

	urlStr := fmt.Sprintf(operationsRequiredURL, normalized.Upper)
	data, err := s.fetchJSON(urlStr, map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.F10OperationsRequired); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.F10OperationsRequired{}, err
	}

	result := models.F10OperationsRequired{
		LatestIndicators:      toMap(data["zxzb"]),
		LatestIndicatorsExtra: toMap(data["zxzbOther"]),
		LatestIndicatorsQuote: normalizeQuoteIndicators(toMap(data["zxzbhq"])),
		EventReminders:        trimRecords(extractDataListFromAny(data["dstx"]), 12),
		Announcements:         trimRecords(extractDataListFromAny(data["zxgg"]), 10),
		ShareholderAnalysis:   trimRecords(extractDataListFromAny(data["gdrs"]), 8),
		DragonTigerList:       trimRecords(extractDataListFromAny(data["lhbd"]), 10),
		BlockTrades:           trimRecords(extractDataListFromAny(data["dzjy"]), 10),
		MarginTrading:         trimRecords(extractDataListFromAny(data["rzrq"]), 10),
		MainIndicators:        trimRecords(extractDataListFromAny(data["zyzb"]), 8),
		SectorTags:            trimRecords(extractDataListFromAny(data["ssbk"]), 20),
		CoreThemes:            trimRecords(extractDataListFromAny(data["hxtc"]), 20),
		InstitutionForecast:   trimRecords(extractDataListFromAny(data["jgyc"]), 12),
		ForecastChart:         trimRecords(extractDataListFromAny(data["yctj_chart"]), 6),
		ReportSummary:         trimRecords(extractDataListFromAny(data["ybzy"]), 10),
	}
	stockCode := strings.TrimSpace(toString(firstNonEmpty(result.LatestIndicators, "SECURITY_CODE")))
	if stockCode == "" {
		stockCode = strings.TrimSpace(toString(firstNonEmpty(result.LatestIndicatorsQuote, "SECURITY_CODE")))
	}
	stockName := strings.TrimSpace(toString(firstNonEmpty(result.LatestIndicators, "SECURITY_NAME_ABBR")))
	if stockName == "" {
		stockName = strings.TrimSpace(toString(firstNonEmpty(result.LatestIndicatorsQuote, "SECURITY_NAME_ABBR")))
	}
	result.News = filterRelevantStockNews(extractDataListFromAny(data["zxzx"]), stockCode, stockName, 8)

	// 公开研报接口补充：用于展示更完整的机构覆盖与评级/预测信息。
	researchReports, reportErr := s.fetchResearchReports(normalized.Raw, 20)
	if reportErr == nil && len(researchReports) > 0 {
		result.ResearchReports = researchReports
	}
	result.ForecastRevisionTrack = buildForecastRevisionTrack(result.InstitutionForecast, result.ResearchReports, 30)

	s.setCache(cacheKey, result)
	return result, nil
}

// GetCoreThemes 获取核心题材（含历史题材与所属板块）
func (s *F10Service) GetCoreThemes(code string) (models.F10CoreThemes, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "coreThemes:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10CoreThemes); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)
	filter := fmt.Sprintf(`(SECUCODE="%s")(KEY_CLASSIF_CODE<>"001")`, secuCode)
	themes, err := s.fetchDataCenterV1("RPT_F10_CORETHEME_CONTENT", "ALL", filter, 200, "KEY_CLASSIF_CODE,MAINPOINT", "1,1")
	if err != nil && ok {
		if cached, ok := entry.value.(models.F10CoreThemes); ok {
			return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
		}
	}

	boardFilter := fmt.Sprintf(`(SECUCODE="%s")(IS_PRECISE="1")`, secuCode)
	boardTypes, boardErr := s.fetchDataCenterV1WithQuoteColumns(
		"RPT_F10_CORETHEME_BOARDTYPE",
		"ALL",
		boardFilter,
		50,
		"BOARD_RANK",
		"1",
		"f3~05~NEW_BOARD_CODE~BOARD_YIELD",
	)
	if boardErr != nil && err == nil && ok {
		if cached, ok := entry.value.(models.F10CoreThemes); ok {
			return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), boardErr)
		}
	}

	selectedBoardReasons := make([]map[string]any, 0, len(boardTypes))
	for _, item := range boardTypes {
		if strings.TrimSpace(toString(item["SELECTED_BOARD_REASON"])) != "" {
			selectedBoardReasons = append(selectedBoardReasons, item)
		}
	}

	var popularLeaders []map[string]any
	var leaderErr error
	if len(boardTypes) > 0 {
		leaderBoardCode := ""
		for _, item := range boardTypes {
			code := strings.TrimSpace(toString(item["DERIVE_BOARD_CODE"]))
			if code != "" {
				leaderBoardCode = code
				break
			}
		}
		if leaderBoardCode != "" {
			popularLeaders, leaderErr = s.fetchPopularLeaders(leaderBoardCode, 12)
		}
	}

	var current []map[string]any
	var history []map[string]any
	for _, item := range themes {
		if toFloat(item["IS_HISTORY"]) > 0 {
			history = append(history, item)
		} else {
			current = append(current, item)
		}
	}

	result := models.F10CoreThemes{
		BoardTypes:           boardTypes,
		Themes:               current,
		History:              history,
		SelectedBoardReasons: selectedBoardReasons,
		PopularLeaders:       popularLeaders,
	}

	if len(result.BoardTypes) == 0 && len(result.Themes) == 0 && len(result.History) == 0 &&
		len(result.SelectedBoardReasons) == 0 && len(result.PopularLeaders) == 0 {
		if err != nil {
			return result, err
		}
		if boardErr != nil {
			return result, boardErr
		}
		if leaderErr != nil {
			return result, leaderErr
		}
	}

	s.setCache(cacheKey, result)
	return result, nil
}

// GetIndustryCompareMetrics 获取行业对比指标（估值与成长性）
func (s *F10Service) GetIndustryCompareMetrics(code string) (models.F10IndustryCompareMetrics, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "industryMetrics:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10IndustryCompareMetrics); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)
	filter := fmt.Sprintf(`(SECUCODE="%s")`, secuCode)
	valuation, valErr := s.fetchDataCenterV1("RPT_PCF10_INDUSTRY_CVALUE", "ALL", filter, 60, "REPORT_DATE", "-1")
	performance, perfErr := s.fetchDataCenterV1("RPT_PCF10_INDUSTRY_DBFX", "ALL", filter, 60, "REPORT_DATE", "-1")
	growth, growthErr := s.fetchDataCenterV1("RPT_PCF10_INDUSTRY_GROWTH", "ALL", filter, 60, "REPORT_DATE", "-1")

	result := models.F10IndustryCompareMetrics{
		Valuation:   valuation,
		Performance: performance,
		Growth:      growth,
	}

	if len(result.Valuation) == 0 && len(result.Performance) == 0 && len(result.Growth) == 0 {
		if valErr != nil {
			return result, valErr
		}
		if perfErr != nil {
			return result, perfErr
		}
		if growthErr != nil {
			return result, growthErr
		}
	}

	s.setCache(cacheKey, result)
	return result, nil
}

// GetMainIndicators 获取主要指标（年度/季度）
func (s *F10Service) GetMainIndicators(code string) (models.F10MainIndicators, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "mainIndicators:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10MainIndicators); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)
	filter := fmt.Sprintf(`(SECUCODE="%s")`, secuCode)
	var primaryLatest []map[string]any
	var primaryYearly []map[string]any
	var primaryQuarterly []map[string]any
	records, dataErr := s.fetchDataCenterV1("RPT_PCF10_FINANCEMAINFINADATA", "ALL", filter, 24, "REPORT_DATE", "-1")
	if dataErr == nil && len(records) > 0 {
		sorted := sortRecordsByDateDesc(records, "REPORT_DATE")
		primaryLatest = trimRecords(sorted, 1)
		primaryYearly = filterRecordsByReportType(sorted, []string{"年报", "年度"})
		primaryQuarterly = filterRecordsByReportType(sorted, []string{"一季报", "二季报", "三季报", "季报", "中报", "半年报", "半年度"})
	}

	latestFallback, latestErr := s.fetchFinanceData("RPT_F10_FINANCE_MAINFINADATA", "APP_F10_MAINFINADATA", normalized.Raw, 9)
	latestFallback = trimRecords(sortRecordsByDateDesc(latestFallback, "REPORT_DATE"), 1)

	yearFilter := fmt.Sprintf(`(SECURITY_CODE="%s")(REPORT_TYPE="年报")`, normalized.Raw)
	yearlyFallback, yearlyErr := s.fetchFinanceDataWithFilter("RPT_F10_FINANCE_MAINFINADATA", "APP_F10_MAINFINADATA", yearFilter, 9, "REPORT_DATE", "-1")
	yearlyFallback = sortRecordsByDateDesc(yearlyFallback, "REPORT_DATE")

	quarterFilter := fmt.Sprintf(`(SECUCODE="%s")`, secuCode)
	quarterlyFallback, quarterlyErr := s.fetchDataCenterV1("RPT_F10_QTR_MAINFINADATA", "ALL", quarterFilter, 24, "REPORT_DATE", "-1")
	quarterlyFallback = sortRecordsByDateDesc(quarterlyFallback, "REPORT_DATE")

	result := models.F10MainIndicators{
		Latest:    primaryLatest,
		Yearly:    primaryYearly,
		Quarterly: primaryQuarterly,
	}
	if len(result.Latest) == 0 && len(latestFallback) > 0 {
		result.Latest = latestFallback
	}
	if len(yearlyFallback) > len(result.Yearly) {
		result.Yearly = yearlyFallback
	}
	// 主数据源常出现 REPORT_TYPE 缺失导致季度被过滤，优先选择期数更多的一组季度数据。
	if len(quarterlyFallback) > len(result.Quarterly) {
		result.Quarterly = quarterlyFallback
	}

	if len(result.Latest) == 0 && len(result.Yearly) == 0 && len(result.Quarterly) == 0 {
		if dataErr != nil {
			return result, dataErr
		}
		if latestErr != nil {
			return result, latestErr
		}
		if yearlyErr != nil {
			return result, yearlyErr
		}
		if quarterlyErr != nil {
			return result, quarterlyErr
		}
	}

	s.setCache(cacheKey, result)
	return result, nil
}

// GetManagement 获取公司高管信息
func (s *F10Service) GetManagement(code string) (models.F10Management, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "management:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10Management); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)
	filter := fmt.Sprintf(`(SECUCODE="%s")`, secuCode)
	managementList, listErr := s.fetchDataCenterV1("RPT_F10_ORGINFO_MANAINTRO", "ALL", filter, 80, "REPORT_DATE", "-1")
	salaryDetails, salaryErr := s.fetchDataCenterV1("RPT_F10_ORGINFO_SALARY", "ALL", filter, 80, "END_DATE", "-1")
	holdingChanges, holdErr := s.fetchDataCenterV1("RPT_F10_TRADE_EXCHANGEHOLD", "ALL", filter, 80, "END_DATE", "-1")

	managementList = sortRecordsByDateDesc(managementList, "REPORT_DATE", "INCUMBENT_DATE")
	salaryDetails = sortRecordsByDateDesc(salaryDetails, "END_DATE")
	holdingChanges = sortRecordsByDateDesc(holdingChanges, "END_DATE")

	result := models.F10Management{
		ManagementList: managementList,
		SalaryDetails:  salaryDetails,
		HoldingChanges: holdingChanges,
	}

	if len(result.ManagementList) == 0 && len(result.SalaryDetails) == 0 && len(result.HoldingChanges) == 0 {
		if listErr != nil {
			return result, listErr
		}
		if salaryErr != nil {
			return result, salaryErr
		}
		if holdErr != nil {
			return result, holdErr
		}
	}

	s.setCache(cacheKey, result)
	return result, nil
}

// GetCapitalOperation 获取资本运作信息
func (s *F10Service) GetCapitalOperation(code string) (models.F10CapitalOperation, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "capitalOperation:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10CapitalOperation); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)
	raiseFilter := fmt.Sprintf(`(SECUCODE="%s")`, secuCode)
	projectFilter := fmt.Sprintf(`(SECURITY_CODE="%s")`, normalized.Raw)

	raiseSources, raiseErr := s.fetchDataCenterV1("RPT_F10_CAPITAL_RAISE", "ALL", raiseFilter, 30, "NOTICE_DATE", "-1")
	projectProgress, projectErr := s.fetchDataCenterV1("RPT_F10_CAPITAL_ITEM", "ALL", projectFilter, 50, "RANK", "1")

	raiseSources = sortRecordsByDateDesc(raiseSources, "NOTICE_DATE", "START_DATE")
	projectProgress = sortRecordsByDateDesc(projectProgress, "NOTICE_DATE")

	result := models.F10CapitalOperation{
		RaiseSources:    raiseSources,
		ProjectProgress: projectProgress,
	}

	if len(result.RaiseSources) == 0 && len(result.ProjectProgress) == 0 {
		if raiseErr != nil {
			return result, raiseErr
		}
		if projectErr != nil {
			return result, projectErr
		}
	}

	s.setCache(cacheKey, result)
	return result, nil
}

// GetEquityStructure 获取股本结构
func (s *F10Service) GetEquityStructure(code string) (models.F10EquityStructure, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "equityStructure:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10EquityStructure); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)
	filter := fmt.Sprintf(`(SECUCODE="%s")`, secuCode)
	records, err := s.fetchDataCenterV1("RPT_F10_EH_EQUITY", "ALL", filter, 30, "END_DATE", "-1")
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.F10EquityStructure); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.F10EquityStructure{}, err
	}

	records = sortRecordsByDateDesc(records, "END_DATE")
	result := models.F10EquityStructure{
		Latest:      trimRecords(records, 1),
		History:     records,
		Composition: trimRecords(records, 7),
	}

	s.setCache(cacheKey, result)
	return result, nil
}

// GetRelatedStocks 获取关联个股
func (s *F10Service) GetRelatedStocks(code string) (models.F10RelatedStocks, error) {
	normalized := normalizeStockCode(code)
	cacheKey := "relatedStocks:" + normalized.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10RelatedStocks); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)

	industryBoardsFilter := fmt.Sprintf(`(SECUCODE="%s")(BOARD_TYPE_NEW="2")`, secuCode)
	industryBoards, industryBoardErr := s.fetchDataCenterV1(
		"RPT_F10_RELATE_GN",
		"SECUCODE,SECURITY_CODE,SECURITY_NAME_ABBR,ORG_CODE,BOARD_CODE,BOARD_NAME,BOARD_TYPE_NEW",
		industryBoardsFilter,
		50,
		"",
		"",
	)

	var industryRankings []map[string]any
	var rankErr error
	if len(industryBoards) > 0 {
		boardCode := strings.TrimSpace(toString(industryBoards[0]["BOARD_CODE"]))
		if boardCode != "" {
			rankFilter := fmt.Sprintf(`(BOARD_CODE="%s")(BOARD_TYPE_NEW="2")`, boardCode)
			industryRankings, rankErr = s.fetchDataCenterV1(
				"RPT_F10_RELATE_RANK",
				"BOARD_CODE,BOARD_NAME,BOARD_TYPE_NEW,SECUCODE,SECURITY_CODE,SECURITY_NAME_ABBR,ORG_CODE,TOTAL_CAP,FREECAP,TOTAL_OPERATEINCOME,PARENT_NETPROFIT,TOTALOPERATEREVETZ,PARENTNETPROFITTZ,REPORT_TYPE,Change3,Change6,Change12",
				rankFilter,
				200,
				"REPORT_TYPE",
				"-1",
			)
			industryRankings = normalizeRelatedRankByLatestReportType(industryRankings)
			sort.SliceStable(industryRankings, func(i, j int) bool {
				return toFloat(industryRankings[i]["TOTAL_CAP"]) > toFloat(industryRankings[j]["TOTAL_CAP"])
			})
		}
	}

	conceptFilter := fmt.Sprintf(`(SECUCODE="%s")(BOARD_TYPE_NEW="3")`, secuCode)
	conceptRelations, conceptErr := s.fetchDataCenterV1(
		"RPT_F10_RELATE_GN",
		"SECUCODE,SECURITY_CODE,SECURITY_NAME_ABBR,ORG_CODE,BOARD_CODE,BOARD_NAME,BOARD_TYPE_NEW",
		conceptFilter,
		120,
		"",
		"",
	)

	result := models.F10RelatedStocks{
		IndustryRankings: industryRankings,
		ConceptRelations: conceptRelations,
	}

	if len(result.IndustryRankings) == 0 && len(result.ConceptRelations) == 0 {
		if industryBoardErr != nil {
			return result, industryBoardErr
		}
		if rankErr != nil {
			return result, rankErr
		}
		if conceptErr != nil {
			return result, conceptErr
		}
	}

	s.setCache(cacheKey, result)
	return result, nil
}

func normalizeRelatedRankByLatestReportType(items []map[string]any) []map[string]any {
	if len(items) == 0 {
		return items
	}

	type rowWithOrder struct {
		item  map[string]any
		order int
	}

	rows := make([]rowWithOrder, 0, len(items))
	for _, item := range items {
		rows = append(rows, rowWithOrder{
			item:  item,
			order: parseReportTypeOrder(toString(item["REPORT_TYPE"])),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].order > rows[j].order
	})

	latestOrder := rows[0].order
	normalized := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		record := row.item
		if row.order != latestOrder {
			record["TOTAL_OPERATEINCOME"] = nil
			record["PARENT_NETPROFIT"] = nil
			record["TOTALOPERATEREVETZ"] = nil
			record["PARENTNETPROFITTZ"] = nil
		}
		normalized = append(normalized, record)
	}
	return normalized
}

func parseReportTypeOrder(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	year := 0
	for i := 0; i+3 < len(value); i++ {
		segment := value[i : i+4]
		if parsed, err := strconv.Atoi(segment); err == nil {
			year = parsed
			break
		}
	}

	monthDay := 0
	switch {
	case strings.Contains(value, "一季"):
		monthDay = 331
	case strings.Contains(value, "中报"), strings.Contains(value, "二季"), strings.Contains(value, "半年"):
		monthDay = 630
	case strings.Contains(value, "三季"):
		monthDay = 930
	case strings.Contains(value, "年报"), strings.Contains(value, "四季"):
		monthDay = 1231
	default:
		monthDay = 101
	}

	return year*10000 + monthDay
}

// GetBonusFinancing 获取分红融资信息
func (s *F10Service) GetBonusFinancing(code normalizedCode) (models.BonusFinancing, error) {
	cacheKey := "bonus:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.BonusFinancing); ok {
			return data, nil
		}
	}

	urlStr := fmt.Sprintf(bonusFinancingURL, code.Upper)
	data, err := s.fetchJSON(urlStr, map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.BonusFinancing); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.BonusFinancing{}, err
	}

	result := models.BonusFinancing{
		Dividend:  extractDataListByKey(data, "fhyx"),
		Annual:    extractDataListByKey(data, "lnfhrz"),
		Financing: extractDataListByKey(data, "zfmx"),
		Allotment: extractDataListByKey(data, "pgmx"),
	}

	normalizeBonusDividend(result.Dividend)
	normalizeBonusAnnual(result.Annual)
	normalizeBonusFinancing(result.Financing)
	normalizeBonusAllotment(result.Allotment)

	s.setCache(cacheKey, result)
	return result, nil
}

// GetBusinessAnalysis 获取经营分析信息
func (s *F10Service) GetBusinessAnalysis(code normalizedCode) (models.BusinessAnalysis, error) {
	cacheKey := "business:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.BusinessAnalysis); ok {
			return data, nil
		}
	}

	urlStr := fmt.Sprintf(businessAnalysisURL, code.Upper)
	data, err := s.fetchJSON(urlStr, map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.BusinessAnalysis); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.BusinessAnalysis{}, err
	}

	result := models.BusinessAnalysis{
		Scope:       extractDataListByKey(data, "zyfw"),
		Composition: extractDataListByKey(data, "zygcfx"),
		Review:      extractDataListByKey(data, "jyps"),
	}

	normalizeBusinessScope(result.Scope)
	normalizeBusinessComposition(result.Composition)
	normalizeBusinessReview(result.Review)

	s.setCache(cacheKey, result)
	return result, nil
}

// GetShareholderNumbers 获取股东户数
func (s *F10Service) GetShareholderNumbers(code normalizedCode) (models.ShareholderNumbers, error) {
	cacheKey := "shareholders:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.ShareholderNumbers); ok {
			return data, nil
		}
	}

	records := []map[string]any{}
	ajaxPayload, ajaxErr := s.fetchJSON(fmt.Sprintf(shareholderAjaxURL, code.Upper), map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
	if ajaxErr == nil {
		records = extractDataListByKey(ajaxPayload, "gdrs")
	}

	if len(records) == 0 {
		legacyRecords, legacyErr := s.fetchDataCenterWeb(shareholderNumbersReport, code.Raw, 12)
		if legacyErr != nil {
			rootErr := legacyErr
			if ajaxErr != nil {
				rootErr = fmt.Errorf("股东研究接口错误: %v；回退接口错误: %v", ajaxErr, legacyErr)
			}
			if ok {
				if cached, ok := entry.value.(models.ShareholderNumbers); ok {
					return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), rootErr)
				}
			}
			return models.ShareholderNumbers{}, rootErr
		}
		records = legacyRecords
	}

	normalizeShareholderNumbers(records)
	result := models.ShareholderNumbers{
		Records: records,
		Latest:  buildLatestRecord(records),
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetEquityPledge 获取股权质押概况
func (s *F10Service) GetEquityPledge(code normalizedCode) (models.EquityPledge, error) {
	cacheKey := "pledge:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.EquityPledge); ok {
			return data, nil
		}
	}

	records, err := s.fetchFinanceData(equityPledgeReport, "ALL", code.Raw, 3)
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.EquityPledge); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.EquityPledge{}, err
	}

	normalizeEquityPledge(records)
	result := models.EquityPledge{
		Records: records,
		Latest:  buildLatestRecord(records),
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetLockupRelease 获取限售解禁信息
func (s *F10Service) GetLockupRelease(code normalizedCode) (models.LockupRelease, error) {
	cacheKey := "lockup:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.LockupRelease); ok {
			return data, nil
		}
	}

	records, err := s.fetchFinanceData(lockupReleaseReport, "ALL", code.Raw, 12)
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.LockupRelease); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.LockupRelease{}, err
	}

	normalizeLockupRelease(records)
	result := models.LockupRelease{
		Records: records,
		Latest:  buildLatestRecord(records),
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetShareholderChanges 获取股东增减持
func (s *F10Service) GetShareholderChanges(code normalizedCode) (models.ShareholderChanges, error) {
	cacheKey := "holderChange:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.ShareholderChanges); ok {
			return data, nil
		}
	}

	filter := fmt.Sprintf(`(SECURITY_CODE="%s")`, code.Raw)
	records, err := s.fetchDataCenterWebWithFilter(shareholderChangeReport, filter, 12, "END_DATE", "-1")
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.ShareholderChanges); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.ShareholderChanges{}, err
	}

	normalizeShareholderChanges(records)
	result := models.ShareholderChanges{
		Records: records,
		Latest:  buildLatestRecord(records),
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetStockBuyback 获取股票回购
func (s *F10Service) GetStockBuyback(code normalizedCode) (models.StockBuyback, error) {
	cacheKey := "buyback:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.StockBuyback); ok {
			return data, nil
		}
	}

	filter := fmt.Sprintf(`(DIM_SCODE="%s")`, code.Raw)
	records, err := s.fetchDataCenterWebWithFilter(buybackReport, filter, 8, "DIM_DATE", "-1")
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.StockBuyback); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.StockBuyback{}, err
	}

	normalizeStockBuyback(records)
	result := models.StockBuyback{
		Records: records,
		Latest:  buildLatestRecord(records),
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetCompanySurvey 获取公司概况
func (s *F10Service) GetCompanySurvey(code normalizedCode) (map[string]any, error) {
	cacheKey := "company:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(map[string]any); ok {
			return data, nil
		}
	}

	urlStr := fmt.Sprintf(companySurveyURL, code.Upper)
	data, err := s.fetchJSON(urlStr, map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
	if err != nil {
		if ok {
			if cached, ok := entry.value.(map[string]any); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return nil, err
	}
	result := extractResult(data)
	if result == nil {
		result = data
	}

	s.setCache(cacheKey, result)
	return result, nil
}

// GetFinancialStatements 获取财务报表
func (s *F10Service) GetFinancialStatements(code normalizedCode) (models.FinancialStatements, error) {
	cacheKey := "financials:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.FinancialStatements); ok {
			return data, nil
		}
	}

	income, err := s.fetchFinanceData("RPT_F10_FINANCE_GINCOME", "APP_F10_GINCOME", code.Raw, 12)
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.FinancialStatements); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.FinancialStatements{}, err
	}
	secuCode := formatSecuCode(code)
	balanceFilter := fmt.Sprintf(`(SECUCODE="%s")`, secuCode)
	balance, err := s.fetchDataCenterV1("RPT_F10_FINANCE_GBALANCE", "ALL", balanceFilter, 12, "REPORT_DATE", "-1")
	if (err != nil || len(balance) == 0) && code.Raw != "" {
		// 兜底：部分场景下 v1 返回异常时，尝试旧入口的通用样式。
		legacyBalance, legacyErr := s.fetchFinanceData("RPT_F10_FINANCE_GBALANCE", "ALL", code.Raw, 12)
		if legacyErr == nil && len(legacyBalance) > 0 {
			balance = legacyBalance
			err = nil
		}
	}
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.FinancialStatements); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.FinancialStatements{}, err
	}
	cashflow, err := s.fetchFinanceData("RPT_F10_FINANCE_GCASHFLOW", "APP_F10_GCASHFLOW", code.Raw, 12)
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.FinancialStatements); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.FinancialStatements{}, err
	}

	result := models.FinancialStatements{
		Income:   income,
		Balance:  balance,
		Cashflow: cashflow,
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetPerformanceEvents 获取业绩事件
func (s *F10Service) GetPerformanceEvents(code normalizedCode) (models.PerformanceEvents, error) {
	cacheKey := "performance:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.PerformanceEvents); ok {
			return data, nil
		}
	}

	forecast, err := s.fetchDataCenterWebSorted("RPT_PUBLIC_OP_NEWPREDICT", code.Raw, 30, "NOTICE_DATE", "-1")
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.PerformanceEvents); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.PerformanceEvents{}, err
	}
	express, err := s.fetchDataCenterWebSorted("RPT_FCI_PERFORMANCEE", code.Raw, 30, "NOTICE_DATE", "-1")
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.PerformanceEvents); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.PerformanceEvents{}, err
	}
	schedule, err := s.fetchDataCenterWebSorted("RPT_PUBLIC_BS_APPOIN", code.Raw, 30, "APPOINT_PUBLISH_DATE", "-1")
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.PerformanceEvents); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.PerformanceEvents{}, err
	}

	normalizePerformanceForecast(forecast)
	normalizePerformanceExpress(express)
	normalizePerformanceSchedule(schedule)

	forecast = sortRecordsByDateDesc(forecast, "NOTICE_DATE", "REPORT_DATE", "END_DATE", "APPOINT_PUBLISH_DATE", "FIRST_APPOINT_DATE", "QDATE")
	express = sortRecordsByDateDesc(express, "NOTICE_DATE", "REPORT_DATE", "END_DATE", "QDATE")
	schedule = sortRecordsByDateDesc(schedule, "APPOINT_PUBLISH_DATE", "FIRST_APPOINT_DATE", "NOTICE_DATE", "REPORT_DATE", "END_DATE")

	forecast = trimRecords(forecast, 6)
	express = trimRecords(express, 6)
	schedule = trimRecords(schedule, 6)

	result := models.PerformanceEvents{
		Forecast: forecast,
		Express:  express,
		Schedule: schedule,
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetFundFlow 获取资金流
func (s *F10Service) GetFundFlow(code normalizedCode) (models.FundFlowSeries, error) {
	cacheKey := "fundflow:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.FundFlowSeries); ok {
			return data, nil
		}
	}

	query := url.Values{}
	query.Set("lmt", "30")
	query.Set("klt", "101")
	query.Set("secid", code.SecID)
	query.Set("fields1", "f1,f2,f3")
	query.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65")
	urlStr := fundFlowURL + "?" + query.Encode()

	raw, err := s.fetchJSON(urlStr, map[string]string{
		"Referer": "https://quote.eastmoney.com/",
	})
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.FundFlowSeries); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.FundFlowSeries{}, err
	}

	var lines [][]string
	if data, ok := raw["data"].(map[string]any); ok {
		if klines, ok := data["klines"].([]any); ok {
			for _, item := range klines {
				if line, ok := item.(string); ok {
					lines = append(lines, strings.Split(line, ","))
				}
			}
		}
	}
	// 接口返回顺序并不稳定，统一按日期倒序，保证 lines[0] 为最近交易日。
	sort.SliceStable(lines, func(i, j int) bool {
		if len(lines[i]) == 0 || len(lines[j]) == 0 {
			return len(lines[i]) > len(lines[j])
		}
		ti, okI := parseDateValue(lines[i][0])
		tj, okJ := parseDateValue(lines[j][0])
		if okI && okJ {
			return ti.After(tj)
		}
		// 兜底：日期字符串按 YYYY-MM-DD 比较也可用
		return lines[i][0] > lines[j][0]
	})

	result := models.FundFlowSeries{
		Fields: []string{
			"f51", "f52", "f53", "f54", "f55", "f56", "f57", "f58", "f59", "f60", "f61", "f62", "f63", "f64", "f65",
		},
		Lines: lines,
		Labels: map[string]string{
			"f51": "date",
			"f52": "mainNet",
			"f53": "superNet",
			"f54": "largeNet",
			"f55": "mediumNet",
			"f56": "smallNet",
			"f57": "mainRatio",
			"f58": "superRatio",
			"f59": "largeRatio",
			"f60": "mediumRatio",
			"f61": "smallRatio",
			"f62": "close",
			"f63": "changePercent",
			"f64": "reserved1",
			"f65": "reserved2",
		},
	}
	result.Latest = buildFundFlowLatest(result.Fields, result.Lines, result.Labels)
	s.setCache(cacheKey, result)
	return result, nil
}

// GetCompanySurveyByCode 根据股票代码获取公司概况
func (s *F10Service) GetCompanySurveyByCode(code string) (map[string]any, error) {
	return s.GetCompanySurvey(normalizeStockCode(code))
}

// GetFinancialStatementsByCode 根据股票代码获取财务报表
func (s *F10Service) GetFinancialStatementsByCode(code string) (models.FinancialStatements, error) {
	return s.GetFinancialStatements(normalizeStockCode(code))
}

// GetPerformanceEventsByCode 根据股票代码获取业绩事件
func (s *F10Service) GetPerformanceEventsByCode(code string) (models.PerformanceEvents, error) {
	return s.GetPerformanceEvents(normalizeStockCode(code))
}

// GetFundFlowByCode 根据股票代码获取资金流
func (s *F10Service) GetFundFlowByCode(code string) (models.FundFlowSeries, error) {
	return s.GetFundFlow(normalizeStockCode(code))
}

// GetInstitutionalHoldingsByCode 根据股票代码获取机构持股
func (s *F10Service) GetInstitutionalHoldingsByCode(code string) (models.InstitutionalHoldings, error) {
	return s.GetInstitutionalHoldings(normalizeStockCode(code))
}

// GetBonusFinancingByCode 根据股票代码获取分红融资
func (s *F10Service) GetBonusFinancingByCode(code string) (models.BonusFinancing, error) {
	return s.GetBonusFinancing(normalizeStockCode(code))
}

// GetBusinessAnalysisByCode 根据股票代码获取经营分析
func (s *F10Service) GetBusinessAnalysisByCode(code string) (models.BusinessAnalysis, error) {
	return s.GetBusinessAnalysis(normalizeStockCode(code))
}

// GetShareholderNumbersByCode 根据股票代码获取股东户数
func (s *F10Service) GetShareholderNumbersByCode(code string) (models.ShareholderNumbers, error) {
	return s.GetShareholderNumbers(normalizeStockCode(code))
}

// GetEquityPledgeByCode 根据股票代码获取股权质押
func (s *F10Service) GetEquityPledgeByCode(code string) (models.EquityPledge, error) {
	return s.GetEquityPledge(normalizeStockCode(code))
}

// GetLockupReleaseByCode 根据股票代码获取限售解禁
func (s *F10Service) GetLockupReleaseByCode(code string) (models.LockupRelease, error) {
	return s.GetLockupRelease(normalizeStockCode(code))
}

// GetShareholderChangesByCode 根据股票代码获取股东增减持
func (s *F10Service) GetShareholderChangesByCode(code string) (models.ShareholderChanges, error) {
	return s.GetShareholderChanges(normalizeStockCode(code))
}

// GetStockBuybackByCode 根据股票代码获取股票回购
func (s *F10Service) GetStockBuybackByCode(code string) (models.StockBuyback, error) {
	return s.GetStockBuyback(normalizeStockCode(code))
}

// GetValuationByCode 通过股票代码获取估值指标
func (s *F10Service) GetValuationByCode(code string) (models.StockValuation, error) {
	normalized := normalizeStockCode(code)
	return s.GetValuation(normalized)
}

// GetValuation 获取估值指标
func (s *F10Service) GetValuation(code normalizedCode) (models.StockValuation, error) {
	cacheKey := "valuation:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.StockValuation); ok {
			return data, nil
		}
	}

	query := url.Values{}
	query.Set("secid", code.SecID)
	query.Set("fields", "f43,f44,f45,f47,f60,f84,f85,f116,f117,f162,f163")
	urlStr := quoteDataURL + "?" + query.Encode()
	raw, err := s.fetchJSON(urlStr, map[string]string{
		"Referer": "https://quote.eastmoney.com/",
	})
	if err != nil {
		if ok {
			if cached, ok := entry.value.(models.StockValuation); ok {
				return cached, fmt.Errorf("使用缓存数据（%s），上游错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), err)
			}
		}
		return models.StockValuation{}, err
	}

	data, ok := raw["data"].(map[string]any)
	if !ok || len(data) == 0 {
		return models.StockValuation{}, fmt.Errorf("估值数据为空")
	}

	price := scaleQuotePrice(toFloat(data["f43"]))
	high := scaleQuotePrice(toFloat(data["f44"]))
	low := scaleQuotePrice(toFloat(data["f45"]))
	preClose := scaleQuotePrice(toFloat(data["f60"]))
	volume := toFloat(data["f47"])
	totalShares := toFloat(data["f84"])
	floatShares := toFloat(data["f85"])
	if floatShares == 0 {
		floatShares = totalShares
	}

	var turnoverRate float64
	if floatShares > 0 && volume > 0 {
		turnoverRate = volume * 10000 / floatShares
	}

	var amplitude float64
	if preClose > 0 && high > 0 && low > 0 {
		amplitude = (high - low) / preClose * 100
	}

	valuation := models.StockValuation{
		Price:          price,
		PETTM:          scaleQuoteValue(toFloat(data["f162"]), 0.01),
		PB:             scaleQuoteValue(toFloat(data["f163"]), 0.001),
		TotalMarketCap: toFloat(data["f116"]),
		FloatMarketCap: toFloat(data["f117"]),
		TurnoverRate:   turnoverRate,
		Amplitude:      amplitude,
		TotalShares:    totalShares,
		FloatShares:    floatShares,
	}

	s.setCache(cacheKey, valuation)
	return valuation, nil
}

// GetValuationTrend 获取估值趋势（市盈率/市净率/市销率/市现率）
func (s *F10Service) GetValuationTrend(code string, rangeKey string) (models.F10ValuationTrend, error) {
	normalized := normalizeStockCode(code)
	dateType, rangeLabel := resolveValuationDateType(rangeKey)
	cacheKey := fmt.Sprintf("valuationTrend:%s:%d", normalized.Raw, dateType)
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.F10ValuationTrend); ok {
			return data, nil
		}
	}

	secuCode := formatSecuCode(normalized)
	if secuCode == "" {
		return models.F10ValuationTrend{}, fmt.Errorf("股票代码不能为空")
	}

	labels := map[string]string{
		"pe":  "市盈率",
		"pb":  "市净率",
		"ps":  "市销率",
		"pcf": "市现率",
	}

	requestedRange := rangeLabel
	tryTypes := uniqueDateTypes([]int{dateType, 1, 2, 3, 4})
	var lastErr error

	for _, dt := range tryTypes {
		pe, peErr := s.fetchValuationTrendSeries(secuCode, 1, dt)
		pb, pbErr := s.fetchValuationTrendSeries(secuCode, 2, dt)
		ps, psErr := s.fetchValuationTrendSeries(secuCode, 3, dt)
		pcf, pcfErr := s.fetchValuationTrendSeries(secuCode, 4, dt)

		if peErr != nil {
			lastErr = peErr
		}
		if pbErr != nil {
			lastErr = pbErr
		}
		if psErr != nil {
			lastErr = psErr
		}
		if pcfErr != nil {
			lastErr = pcfErr
		}

		if len(pe) == 0 && len(pb) == 0 && len(ps) == 0 && len(pcf) == 0 {
			continue
		}

		usedRange := valuationRangeLabel(dt)
		result := models.F10ValuationTrend{
			Source:         "trend",
			Range:          usedRange,
			RequestedRange: requestedRange,
			Fallback:       usedRange != requestedRange,
			DateType:       dt,
			Labels:         labels,
			PE:             pe,
			PB:             pb,
			PS:             ps,
			PCF:            pcf,
		}

		s.setCache(cacheKey, result)
		return result, nil
	}

	price := 0.0
	valuation, valErr := s.GetValuation(normalized)
	if valErr == nil && valuation.Price > 0 {
		price = valuation.Price
	}

	reportTrend, reportErr := s.buildReportValuationTrend(normalized, price, labels)
	if reportErr == nil && (len(reportTrend.PE) > 0 || len(reportTrend.PB) > 0 || len(reportTrend.PS) > 0 || len(reportTrend.PCF) > 0) {
		reportTrend.RequestedRange = requestedRange
		reportTrend.Fallback = true
		s.setCache(cacheKey, reportTrend)
		return reportTrend, nil
	}

	if reportErr != nil {
		lastErr = reportErr
	}

	result := models.F10ValuationTrend{
		Range:          rangeLabel,
		RequestedRange: requestedRange,
		DateType:       dateType,
		Labels:         labels,
	}

	if lastErr != nil {
		return result, lastErr
	}
	return result, fmt.Errorf("估值趋势数据为空")
}

func (s *F10Service) buildReportValuationTrend(code normalizedCode, price float64, labels map[string]string) (models.F10ValuationTrend, error) {
	if price <= 0 {
		return models.F10ValuationTrend{}, fmt.Errorf("缺少可用价格")
	}

	records, err := s.fetchFinanceData("RPT_F10_FINANCE_MAINFINADATA", "APP_F10_MAINFINADATA", code.Raw, 16)
	if err != nil {
		return models.F10ValuationTrend{}, err
	}
	if len(records) == 0 {
		return models.F10ValuationTrend{}, fmt.Errorf("财务指标为空")
	}

	records = sortRecordsByDateDesc(records, "REPORT_DATE")

	var peSeries []map[string]any
	var pbSeries []map[string]any
	var psSeries []map[string]any
	var pcfSeries []map[string]any

	for _, item := range records {
		reportDate := firstNonEmpty(item, "REPORT_DATE", "END_DATE")
		eps := firstNonZero(item, "EPSJB", "EPSXS", "EPSKCJB")
		bps := toFloat(item["BPS"])
		perToi := firstNonZero(item, "PER_TOI", "PER_OI", "PER_SALES")
		cashPerShare := toFloat(item["MGJYXJJE"])

		if value := safeDivide(price, eps); value > 0 {
			peSeries = append(peSeries, buildValuationPoint(code, reportDate, value, price, eps))
		}
		if value := safeDivide(price, bps); value > 0 {
			pbSeries = append(pbSeries, buildValuationPoint(code, reportDate, value, price, bps))
		}
		if value := safeDivide(price, perToi); value > 0 {
			psSeries = append(psSeries, buildValuationPoint(code, reportDate, value, price, perToi))
		}
		if value := safeDivide(price, cashPerShare); value > 0 {
			pcfSeries = append(pcfSeries, buildValuationPoint(code, reportDate, value, price, cashPerShare))
		}
	}

	return models.F10ValuationTrend{
		Source:   "report",
		Range:    "report",
		DateType: 0,
		Labels:   labels,
		PE:       peSeries,
		PB:       pbSeries,
		PS:       psSeries,
		PCF:      pcfSeries,
	}, nil
}

func buildValuationPoint(code normalizedCode, reportDate any, value float64, price float64, base float64) map[string]any {
	sec := formatSecuCode(code)
	return map[string]any{
		"SECUCODE":        sec,
		"TRADE_DATE":      reportDate,
		"REPORT_DATE":     reportDate,
		"INDICATOR_VALUE": value,
		"PRICE":           price,
		"BASE_VALUE":      base,
		"SOURCE":          "report",
	}
}

func safeDivide(price float64, base float64) float64 {
	if price <= 0 || base <= 0 {
		return 0
	}
	return price / base
}

func firstNonZero(item map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value := toFloat(item[key]); value > 0 {
			return value
		}
	}
	return 0
}

func (s *F10Service) fetchValuationTrendSeries(secuCode string, indicatorType int, dateType int) ([]map[string]any, error) {
	filter := fmt.Sprintf(`(SECUCODE="%s")(INDICATORTYPE=%d)(DATETYPE=%d)`, secuCode, indicatorType, dateType)
	pageSize := 400
	return s.fetchDataCenterV1("RPT_CUSTOM_DMSK_TREND", "ALL", filter, pageSize, "TRADE_DATE", "1")
}

// GetInstitutionalHoldings 获取机构/股东信息
func (s *F10Service) GetInstitutionalHoldings(code normalizedCode) (models.InstitutionalHoldings, error) {
	cacheKey := "institutions:" + code.Raw
	entry, ok, fresh := s.getCacheEntry(cacheKey)
	if ok && fresh {
		if data, ok := entry.value.(models.InstitutionalHoldings); ok {
			return data, nil
		}
	}

	controller, ajaxErr := s.fetchJSON(fmt.Sprintf(shareholderAjaxURL, code.Upper), map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
	controllerMapRaw := toMap(controller)
	controllerMap := map[string]any{}
	if controllerMapRaw != nil {
		gdrs := extractDataListByKey(controllerMapRaw, "gdrs")
		sdgd := extractDataListByKey(controllerMapRaw, "sdgd")
		sdltgd := extractDataListByKey(controllerMapRaw, "sdltgd")
		sdgdcgbd := extractDataListByKey(controllerMapRaw, "sdgdcgbd")
		sjkzr := extractDataListByKey(controllerMapRaw, "sjkzr")
		jgcc := extractDataListByKey(controllerMapRaw, "jgcc")
		controllerMap["gdrs"] = gdrs
		controllerMap["sdgd"] = sdgd
		controllerMap["sdltgd"] = sdltgd
		controllerMap["sdgdcgbd"] = sdgdcgbd
		controllerMap["sjkzr"] = sjkzr
		controllerMap["jgcc"] = jgcc
	}

	topHolders := extractDataListByKey(controllerMap, "sdgd")
	if len(topHolders) == 0 {
		fallbackHolders, holderErr := s.fetchDataCenterWeb("RPT_DMSK_HOLDERS", code.Raw, 10)
		if holderErr != nil && ajaxErr != nil {
			if ok {
				if cached, ok := entry.value.(models.InstitutionalHoldings); ok {
					return cached, fmt.Errorf("使用缓存数据（%s），上游错误: 股东研究接口错误: %v；机构持仓接口错误: %v", entry.timestamp.Format("2006-01-02 15:04:05"), ajaxErr, holderErr)
				}
			}
			return models.InstitutionalHoldings{}, fmt.Errorf("股东研究接口错误: %v；机构持仓接口错误: %v", ajaxErr, holderErr)
		}
		topHolders = fallbackHolders
	}

	result := models.InstitutionalHoldings{
		TopHolders: topHolders,
		Controller: controllerMap,
	}
	s.setCache(cacheKey, result)
	return result, nil
}

// GetIndustryCompare 获取行业对比数据
func (s *F10Service) GetIndustryCompare(code string) models.IndustryCompare {
	industry, peers := s.getIndustryPeers(code, 12)
	return models.IndustryCompare{
		Industry: industry,
		Peers:    peers,
	}
}

func (s *F10Service) fetchFinanceData(reportName, style, code string, pageSize int) ([]map[string]any, error) {
	filter := fmt.Sprintf(`(SECURITY_CODE="%s")`, code)
	query := url.Values{}
	query.Set("type", reportName)
	query.Set("sty", style)
	query.Set("filter", filter)
	if pageSize > 0 {
		query.Set("p", "1")
		query.Set("ps", fmt.Sprintf("%d", pageSize))
	}
	urlStr := financeDataCenterURL + "?" + query.Encode()
	return s.fetchDataList(urlStr, nil)
}

func (s *F10Service) fetchFinanceDataWithFilter(reportName, style, filter string, pageSize int, sortColumns string, sortTypes string) ([]map[string]any, error) {
	query := url.Values{}
	query.Set("type", reportName)
	query.Set("sty", style)
	query.Set("filter", filter)
	if pageSize > 0 {
		query.Set("p", "1")
		query.Set("ps", fmt.Sprintf("%d", pageSize))
	}
	if sortColumns != "" {
		query.Set("st", sortColumns)
	}
	if sortTypes != "" {
		query.Set("sr", sortTypes)
	}
	urlStr := financeDataCenterURL + "?" + query.Encode()
	return s.fetchDataList(urlStr, nil)
}

func (s *F10Service) fetchPopularLeaders(boardCode string, pageSize int) ([]map[string]any, error) {
	if strings.TrimSpace(boardCode) == "" {
		return nil, nil
	}
	query := url.Values{}
	query.Set("type", "RTP_F10_POPULAR_LEADING")
	query.Set("sty", "ALL")
	query.Set("params", boardCode)
	if pageSize > 0 {
		query.Set("p", "1")
		query.Set("ps", fmt.Sprintf("%d", pageSize))
	}
	query.Set("extraCols", "f2~01~SECURITY_CODE~NEWEST_PRICE,f3~01~SECURITY_CODE~YIELD")
	urlStr := financeDataCenterURL + "?" + query.Encode()
	return s.fetchDataList(urlStr, map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
}

func (s *F10Service) fetchDataCenterWeb(reportName, code string, pageSize int) ([]map[string]any, error) {
	filter := fmt.Sprintf(`(SECURITY_CODE="%s")`, code)
	query := url.Values{}
	query.Set("reportName", reportName)
	query.Set("columns", "ALL")
	query.Set("filter", filter)
	query.Set("pageNumber", "1")
	query.Set("pageSize", fmt.Sprintf("%d", pageSize))
	urlStr := dataCenterWebURL + "?" + query.Encode()
	return s.fetchDataList(urlStr, map[string]string{
		"Referer": "https://data.eastmoney.com/",
	})
}

func (s *F10Service) fetchDataCenterWebSorted(reportName, code string, pageSize int, sortColumns string, sortTypes string) ([]map[string]any, error) {
	filter := fmt.Sprintf(`(SECURITY_CODE="%s")`, code)
	query := url.Values{}
	query.Set("reportName", reportName)
	query.Set("columns", "ALL")
	query.Set("filter", filter)
	query.Set("pageNumber", "1")
	query.Set("pageSize", fmt.Sprintf("%d", pageSize))
	if sortColumns != "" {
		query.Set("sortColumns", sortColumns)
	}
	if sortTypes != "" {
		query.Set("sortTypes", sortTypes)
	}
	query.Set("source", "WEB")
	query.Set("client", "WEB")
	urlStr := dataCenterWebURL + "?" + query.Encode()

	records, err := s.fetchDataList(urlStr, map[string]string{
		"Referer": "https://data.eastmoney.com/",
	})
	if err == nil {
		return records, nil
	}
	return s.fetchDataCenterWeb(reportName, code, pageSize)
}

func (s *F10Service) fetchDataCenterWebWithFilter(reportName, filter string, pageSize int, sortColumns string, sortTypes string) ([]map[string]any, error) {
	query := url.Values{}
	query.Set("reportName", reportName)
	query.Set("columns", "ALL")
	query.Set("filter", filter)
	query.Set("pageNumber", "1")
	query.Set("pageSize", fmt.Sprintf("%d", pageSize))
	if sortColumns != "" {
		query.Set("sortColumns", sortColumns)
	}
	if sortTypes != "" {
		query.Set("sortTypes", sortTypes)
	}
	query.Set("source", "WEB")
	query.Set("client", "WEB")
	urlStr := dataCenterWebURL + "?" + query.Encode()
	return s.fetchDataList(urlStr, map[string]string{
		"Referer": "https://data.eastmoney.com/",
	})
}

func (s *F10Service) fetchDataCenterV1(reportName, columns, filter string, pageSize int, sortColumns string, sortTypes string) ([]map[string]any, error) {
	query := url.Values{}
	query.Set("reportName", reportName)
	if columns == "" {
		columns = "ALL"
	}
	query.Set("columns", columns)
	query.Set("filter", filter)
	query.Set("pageNumber", "1")
	query.Set("pageSize", fmt.Sprintf("%d", pageSize))
	if sortColumns != "" {
		query.Set("sortColumns", sortColumns)
	}
	if sortTypes != "" {
		query.Set("sortTypes", sortTypes)
	}
	query.Set("source", "HSF10")
	query.Set("client", "PC")
	urlStr := dataCenterV1URL + "?" + query.Encode()
	return s.fetchDataList(urlStr, map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
}

func (s *F10Service) fetchDataCenterV1WithQuoteColumns(reportName, columns, filter string, pageSize int, sortColumns string, sortTypes string, quoteColumns string) ([]map[string]any, error) {
	query := url.Values{}
	query.Set("reportName", reportName)
	if columns == "" {
		columns = "ALL"
	}
	query.Set("columns", columns)
	query.Set("filter", filter)
	query.Set("pageNumber", "1")
	query.Set("pageSize", fmt.Sprintf("%d", pageSize))
	if sortColumns != "" {
		query.Set("sortColumns", sortColumns)
	}
	if sortTypes != "" {
		query.Set("sortTypes", sortTypes)
	}
	if quoteColumns != "" {
		query.Set("quoteColumns", quoteColumns)
	}
	query.Set("source", "HSF10")
	query.Set("client", "PC")
	urlStr := dataCenterV1URL + "?" + query.Encode()
	return s.fetchDataList(urlStr, map[string]string{
		"Referer": "https://emweb.securities.eastmoney.com/",
	})
}

func (s *F10Service) fetchResearchReports(code string, pageSize int) ([]map[string]any, error) {
	if strings.TrimSpace(code) == "" {
		return nil, nil
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	query := url.Values{}
	query.Set("industryCode", "*")
	query.Set("pageSize", fmt.Sprintf("%d", pageSize))
	query.Set("industry", "*")
	query.Set("rating", "*")
	query.Set("ratingChange", "*")
	query.Set("beginTime", "2020-01-01")
	query.Set("endTime", fmt.Sprintf("%d-01-01", time.Now().Year()+1))
	query.Set("pageNo", "1")
	query.Set("fields", "")
	query.Set("qType", "0")
	query.Set("orgCode", "")
	query.Set("code", normalizeStockCode(code).Raw)
	query.Set("rcode", "")

	urlStr := researchReportListURL + "?" + query.Encode()
	raw, err := s.fetchJSON(urlStr, map[string]string{
		"Referer": "https://data.eastmoney.com/",
	})
	if err != nil {
		return nil, err
	}

	records := extractDataListByKey(raw, "data")
	if len(records) == 0 {
		return nil, nil
	}
	records = sortRecordsByDateDesc(records, "publishDate", "PUBLISH_DATE")
	records = trimRecords(records, pageSize)
	normalizeResearchReports(records)
	return records, nil
}

func (s *F10Service) fetchDataList(urlStr string, headers map[string]string) ([]map[string]any, error) {
	raw, err := s.fetchJSON(urlStr, headers)
	if err != nil {
		return nil, err
	}
	data := extractDataList(raw)
	return data, nil
}

func (s *F10Service) fetchJSON(urlStr string, headers map[string]string) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.fetchJSONOnce(urlStr, headers)
		if err == nil {
			return result, nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return nil, lastErr
}

func (s *F10Service) fetchJSONOnce(urlStr string, headers map[string]string) (map[string]any, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if isHTMLBody(body) {
		return nil, fmt.Errorf("上游返回HTML响应，可能被拦截或接口变更")
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *F10Service) getIndustryPeers(code string, limit int) (string, []models.StockPeer) {
	if err := s.loadBasicData(); err != nil {
		return "", nil
	}

	var symbolIdx, nameIdx, industryIdx, tsCodeIdx int = -1, -1, -1, -1
	for i, field := range s.basicData.Data.Fields {
		switch field {
		case "symbol":
			symbolIdx = i
		case "name":
			nameIdx = i
		case "industry":
			industryIdx = i
		case "ts_code":
			tsCodeIdx = i
		}
	}
	if symbolIdx < 0 || nameIdx < 0 || industryIdx < 0 {
		return "", nil
	}

	var industry string
	for _, item := range s.basicData.Data.Items {
		if symbolIdx >= len(item) {
			continue
		}
		symbol, _ := item[symbolIdx].(string)
		if symbol == code {
			if industryIdx < len(item) {
				industry, _ = item[industryIdx].(string)
			}
			break
		}
	}

	if industry == "" {
		return "", nil
	}

	var peers []models.StockPeer
	for _, item := range s.basicData.Data.Items {
		if len(peers) >= limit {
			break
		}
		if industryIdx >= len(item) || symbolIdx >= len(item) || nameIdx >= len(item) {
			continue
		}
		itemIndustry, _ := item[industryIdx].(string)
		if itemIndustry != industry {
			continue
		}
		symbol, _ := item[symbolIdx].(string)
		if symbol == code {
			continue
		}
		name, _ := item[nameIdx].(string)
		peer := models.StockPeer{
			Symbol: symbol,
			Name:   name,
		}
		if tsCodeIdx >= 0 && tsCodeIdx < len(item) {
			tsCode, _ := item[tsCodeIdx].(string)
			if strings.HasSuffix(tsCode, ".SH") {
				peer.Market = "上海"
				peer.Symbol = "sh" + symbol
			} else if strings.HasSuffix(tsCode, ".SZ") {
				peer.Market = "深圳"
				peer.Symbol = "sz" + symbol
			}
		}
		peers = append(peers, peer)
	}

	return industry, peers
}

func (s *F10Service) loadBasicData() error {
	s.basicMu.Lock()
	defer s.basicMu.Unlock()
	if s.basicLoaded {
		return nil
	}
	var basic f10StockBasicData
	if err := json.Unmarshal(embed.StockBasicJSON, &basic); err != nil {
		return err
	}
	s.basicData = basic
	s.basicLoaded = true
	return nil
}

func (s *F10Service) getCacheEntry(key string) (cacheEntry, bool, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	entry, ok := s.cache[key]
	if !ok {
		return cacheEntry{}, false, false
	}
	fresh := time.Since(entry.timestamp) <= s.cacheTTL
	return entry, true, fresh
}

func (s *F10Service) setCache(key string, value any) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[key] = cacheEntry{
		value:     value,
		timestamp: time.Now(),
	}
}

func normalizeStockCode(code string) normalizedCode {
	trimmed := strings.TrimSpace(strings.ToLower(code))
	prefix := ""
	raw := trimmed

	if strings.HasPrefix(trimmed, "sh") || strings.HasPrefix(trimmed, "sz") || strings.HasPrefix(trimmed, "bj") {
		prefix = trimmed[:2]
		raw = trimmed[2:]
	}

	if prefix == "" {
		if strings.HasPrefix(raw, "6") {
			prefix = "sh"
		} else {
			prefix = "sz"
		}
	}

	marketID := "0"
	if prefix == "sh" {
		marketID = "1"
	}

	lower := prefix + raw
	upper := strings.ToUpper(lower)
	secID := fmt.Sprintf("%s.%s", marketID, raw)

	return normalizedCode{
		Raw:      raw,
		Prefix:   prefix,
		Lower:    lower,
		Upper:    upper,
		SecID:    secID,
		MarketID: marketID,
	}
}

func formatSecuCode(code normalizedCode) string {
	if code.Raw == "" {
		return ""
	}
	suffix := strings.ToUpper(code.Prefix)
	if suffix == "" {
		if strings.HasPrefix(code.Raw, "6") {
			suffix = "SH"
		} else {
			suffix = "SZ"
		}
	}
	return fmt.Sprintf("%s.%s", code.Raw, suffix)
}

func resolveValuationDateType(rangeKey string) (int, string) {
	key := strings.TrimSpace(strings.ToLower(rangeKey))
	switch key {
	case "1y", "1yr", "1year", "1年", "1":
		return 1, "1y"
	case "3y", "3yr", "3year", "3年", "3":
		return 2, "3y"
	case "5y", "5yr", "5year", "5年", "5":
		return 3, "5y"
	case "10y", "10yr", "10year", "10年", "10":
		return 4, "10y"
	default:
		return 3, "5y"
	}
}

func valuationRangeLabel(dateType int) string {
	switch dateType {
	case 1:
		return "1y"
	case 2:
		return "3y"
	case 4:
		return "10y"
	default:
		return "5y"
	}
}

func uniqueDateTypes(values []int) []int {
	seen := make(map[int]struct{})
	var result []int
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

func extractResult(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	if result, ok := data["result"].(map[string]any); ok {
		return result
	}
	return nil
}

func extractDataList(data map[string]any) []map[string]any {
	if data == nil {
		return nil
	}
	if list, ok := data["result"].([]any); ok {
		return toMapSlice(list)
	}
	if result, ok := data["result"].(map[string]any); ok {
		if list, ok := result["data"].([]any); ok {
			return toMapSlice(list)
		}
	}
	if list, ok := data["data"].([]any); ok {
		return toMapSlice(list)
	}
	return nil
}

func extractDataListByKey(data map[string]any, key string) []map[string]any {
	if data == nil {
		return nil
	}
	if list, ok := data[key].([]any); ok {
		return toMapSlice(list)
	}
	return nil
}

func extractDataListFromAny(value any) []map[string]any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []any:
		return toMapSlice(v)
	case map[string]any:
		if data, ok := v["data"].([]any); ok {
			return toMapSlice(data)
		}
		if dataObj, ok := v["data"].(map[string]any); ok {
			if items, ok := dataObj["items"].([]any); ok {
				return toMapSlice(items)
			}
			if data, ok := dataObj["data"].([]any); ok {
				return toMapSlice(data)
			}
		}
		if items, ok := v["items"].([]any); ok {
			return toMapSlice(items)
		}
	}
	return nil
}

func filterRelevantStockNews(items []map[string]any, stockCode string, stockName string, limit int) []map[string]any {
	if len(items) == 0 {
		return nil
	}

	normalizedCode := strings.TrimSpace(strings.ToLower(stockCode))
	normalizedName := strings.TrimSpace(strings.ToLower(stockName))
	keywords := make([]string, 0, 3)
	if normalizedName != "" {
		keywords = append(keywords, normalizedName)
	}
	if normalizedCode != "" {
		keywords = append(keywords, normalizedCode)
		keywords = append(keywords, strings.TrimLeft(normalizedCode, "0"))
	}

	cleanKeywords := make([]string, 0, len(keywords))
	seen := make(map[string]struct{})
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		cleanKeywords = append(cleanKeywords, keyword)
	}

	if len(cleanKeywords) == 0 {
		return trimRecords(items, limit)
	}

	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		title := strings.ToLower(strings.TrimSpace(toString(firstNonEmpty(item, "title", "TITLE", "NEWS_TITLE"))))
		summary := strings.ToLower(strings.TrimSpace(toString(firstNonEmpty(item, "summary", "SUMMARY", "CONTENT"))))
		content := title + " " + summary
		if content == "" {
			continue
		}
		for _, keyword := range cleanKeywords {
			if strings.Contains(content, keyword) {
				filtered = append(filtered, item)
				break
			}
		}
	}

	if len(filtered) == 0 {
		return nil
	}
	return trimRecords(filtered, limit)
}

func toMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case map[string]any:
		if data, ok := v["data"].([]any); ok && len(data) > 0 {
			if row, ok := data[0].(map[string]any); ok {
				return row
			}
		}
		return v
	case []any:
		if len(v) > 0 {
			if row, ok := v[0].(map[string]any); ok {
				return row
			}
		}
	}
	return nil
}

func normalizeQuoteIndicators(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	result := map[string]any{}
	if v, ok := raw["f57"]; ok {
		result["SECURITY_CODE"] = v
	}
	if v, ok := raw["f58"]; ok {
		result["SECURITY_NAME_ABBR"] = v
	}
	if v, ok := raw["f116"]; ok {
		result["TOTAL_MARKET_CAP"] = v
	}
	if v, ok := raw["f117"]; ok {
		result["FLOAT_MARKET_CAP"] = v
	}
	if v, ok := raw["f162"]; ok {
		result["PE_DYNAMIC"] = v
	}
	if v, ok := raw["f163"]; ok {
		result["PE_STATIC"] = v
	}
	if v, ok := raw["f164"]; ok {
		result["PE_TTM"] = v
	}
	if v, ok := raw["f167"]; ok {
		result["PB"] = v
	}
	return result
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if v, ok := value.(string); ok {
		return v
	}
	return fmt.Sprintf("%v", value)
}

func toMapSlice(items []any) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		switch row := item.(type) {
		case map[string]any:
			result = append(result, row)
		case []any:
			for _, nested := range row {
				if nestedRow, ok := nested.(map[string]any); ok {
					result = append(result, nestedRow)
				}
			}
		}
	}
	return result
}

func hasFinancialData(data models.FinancialStatements) bool {
	return len(data.Income) > 0 || len(data.Balance) > 0 || len(data.Cashflow) > 0
}

func hasPerformanceData(data models.PerformanceEvents) bool {
	return len(data.Forecast) > 0 || len(data.Express) > 0 || len(data.Schedule) > 0
}

func hasInstitutionData(data models.InstitutionalHoldings) bool {
	return len(data.TopHolders) > 0 || (data.Controller != nil && len(data.Controller) > 0)
}

func hasBonusFinancing(data models.BonusFinancing) bool {
	return len(data.Dividend) > 0 || len(data.Annual) > 0 || len(data.Financing) > 0 || len(data.Allotment) > 0
}

func hasBusinessAnalysis(data models.BusinessAnalysis) bool {
	return len(data.Scope) > 0 || len(data.Composition) > 0 || len(data.Review) > 0
}

func hasShareholderNumbers(data models.ShareholderNumbers) bool {
	return len(data.Records) > 0
}

func hasEquityPledge(data models.EquityPledge) bool {
	return len(data.Records) > 0
}

func hasLockupRelease(data models.LockupRelease) bool {
	return len(data.Records) > 0
}

func hasShareholderChanges(data models.ShareholderChanges) bool {
	return len(data.Records) > 0
}

func hasStockBuyback(data models.StockBuyback) bool {
	return len(data.Records) > 0
}

func hasOperationsRequired(data models.F10OperationsRequired) bool {
	return len(data.LatestIndicators) > 0 ||
		len(data.LatestIndicatorsExtra) > 0 ||
		len(data.LatestIndicatorsQuote) > 0 ||
		len(data.EventReminders) > 0 ||
		len(data.News) > 0 ||
		len(data.Announcements) > 0 ||
		len(data.ShareholderAnalysis) > 0 ||
		len(data.DragonTigerList) > 0 ||
		len(data.BlockTrades) > 0 ||
		len(data.MarginTrading) > 0 ||
		len(data.MainIndicators) > 0 ||
		len(data.SectorTags) > 0 ||
		len(data.CoreThemes) > 0 ||
		len(data.InstitutionForecast) > 0 ||
		len(data.ForecastChart) > 0 ||
		len(data.ReportSummary) > 0 ||
		len(data.ResearchReports) > 0 ||
		len(data.ForecastRevisionTrack) > 0
}

func hasCoreThemes(data models.F10CoreThemes) bool {
	return len(data.BoardTypes) > 0 ||
		len(data.Themes) > 0 ||
		len(data.History) > 0 ||
		len(data.SelectedBoardReasons) > 0 ||
		len(data.PopularLeaders) > 0
}

func hasIndustryMetrics(data models.F10IndustryCompareMetrics) bool {
	return len(data.Valuation) > 0 || len(data.Performance) > 0 || len(data.Growth) > 0
}

func hasMainIndicators(data models.F10MainIndicators) bool {
	return len(data.Latest) > 0 || len(data.Yearly) > 0 || len(data.Quarterly) > 0
}

func hasManagement(data models.F10Management) bool {
	return len(data.ManagementList) > 0 || len(data.SalaryDetails) > 0 || len(data.HoldingChanges) > 0
}

func hasCapitalOperation(data models.F10CapitalOperation) bool {
	return len(data.RaiseSources) > 0 || len(data.ProjectProgress) > 0
}

func hasEquityStructure(data models.F10EquityStructure) bool {
	return len(data.Latest) > 0 || len(data.History) > 0 || len(data.Composition) > 0
}

func hasRelatedStocks(data models.F10RelatedStocks) bool {
	return len(data.IndustryRankings) > 0 || len(data.ConceptRelations) > 0
}

func hasValuationTrend(data models.F10ValuationTrend) bool {
	return len(data.PE) > 0 || len(data.PB) > 0 || len(data.PS) > 0 || len(data.PCF) > 0
}

func hasValuation(data models.StockValuation) bool {
	return data.PETTM > 0 ||
		data.PB > 0 ||
		data.TotalMarketCap > 0 ||
		data.FloatMarketCap > 0 ||
		data.TurnoverRate > 0 ||
		data.Amplitude > 0
}

func normalizeBonusDividend(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"noticeDate": item["NOTICE_DATE"],
			"plan":       item["IMPL_PLAN_PROFILE"],
			"progress":   item["ASSIGN_PROGRESS"],
			"recordDate": item["EQUITY_RECORD_DATE"],
			"exDate":     item["EX_DIVIDEND_DATE"],
			"payDate":    item["PAY_CASH_DATE"],
		}
	}
}

func normalizeBonusAnnual(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"year":          item["STATISTICS_YEAR"],
			"totalDividend": toFloat(item["TOTAL_DIVIDEND"]),
			"seoNum":        toFloat(item["SEO_NUM"]),
			"allotmentNum":  toFloat(item["ALLOTMENT_NUM"]),
			"ipoNum":        toFloat(item["IPO_NUM"]),
		}
	}
}

func normalizeBonusFinancing(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"noticeDate": item["NOTICE_DATE"],
			"issueNum":   toFloat(item["ISSUE_NUM"]),
			"raiseFunds": toFloat(item["NET_RAISE_FUNDS"]),
			"issuePrice": toFloat(item["ISSUE_PRICE"]),
			"issueWay":   item["ISSUE_WAY_EXPLAIN"],
			"regDate":    item["REG_DATE"],
			"listDate":   item["LISTING_DATE"],
		}
	}
}

func normalizeBonusAllotment(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"noticeDate": item["NOTICE_DATE"],
			"issueNum":   toFloat(item["ISSUE_NUM"]),
			"raiseFunds": toFloat(item["NET_RAISE_FUNDS"]),
			"issuePrice": toFloat(item["ISSUE_PRICE"]),
			"issueWay":   item["ISSUE_WAY_EXPLAIN"],
			"regDate":    item["REG_DATE"],
			"listDate":   item["LISTING_DATE"],
		}
	}
}

func normalizeBusinessScope(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"reportDate":    item["REPORT_DATE"],
			"businessScope": item["BUSINESS_SCOPE"],
		}
	}
}

func normalizeBusinessComposition(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"reportDate":  item["REPORT_DATE"],
			"type":        item["MAINOP_TYPE"],
			"itemName":    item["ITEM_NAME"],
			"income":      toFloat(item["MAIN_BUSINESS_INCOME"]),
			"incomeRatio": toFloat(item["MBI_RATIO"]),
			"cost":        toFloat(item["MAIN_BUSINESS_COST"]),
			"costRatio":   toFloat(item["MBC_RATIO"]),
			"profit":      toFloat(item["MAIN_BUSINESS_RPOFIT"]),
			"profitRatio": toFloat(item["MBR_RATIO"]),
			"grossMargin": toFloat(item["GROSS_RPOFIT_RATIO"]),
			"rank":        item["RANK"],
		}
	}
}

func normalizeBusinessReview(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"reportDate":    item["REPORT_DATE"],
			"reviewContent": item["BUSINESS_REVIEW"],
		}
	}
}

func normalizeShareholderNumbers(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"endDate":            item["END_DATE"],
			"noticeDate":         item["HOLD_NOTICE_DATE"],
			"holderNum":          toFloat(firstNonEmpty(item, "HOLDER_TOTAL_NUM", "HOLDER_NUM")),
			"holderChange":       toFloat(item["HOLDER_NUM_CHANGE"]),
			"holderChangeRate":   toFloat(firstNonEmpty(item, "TOTAL_NUM_RATIO", "HOLDER_NUM_RATIO")),
			"avgHoldNum":         toFloat(firstNonEmpty(item, "AVG_FREE_SHARES", "AVG_HOLD_NUM")),
			"avgFreeShares":      toFloat(firstNonEmpty(item, "AVG_FREE_SHARES", "AVG_HOLD_NUM")),
			"avgFreeSharesRatio": toFloat(firstNonEmpty(item, "AVG_FREESHARES_RATIO")),
			"focusLevel":         firstNonEmpty(item, "HOLD_FOCUS"),
			"price":              toFloat(firstNonEmpty(item, "PRICE")),
			"avgHoldAmt":         toFloat(firstNonEmpty(item, "AVG_HOLD_AMT")),
			"top10HoldRatio":     toFloat(firstNonEmpty(item, "HOLD_RATIO_TOTAL")),
			"top10FreeHoldRatio": toFloat(firstNonEmpty(item, "FREEHOLD_RATIO_TOTAL")),
			"totalMarketCap":     toFloat(item["TOTAL_MARKET_CAP"]),
			"totalAShares":       toFloat(item["TOTAL_A_SHARES"]),
			"changeReason":       item["CHANGE_REASON"],
		}
	}
}

func normalizeEquityPledge(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"tradeDate":         item["TRADE_DATE"],
			"pledgeRatio":       toFloat(item["PLEDGE_RATIO"]),
			"pledgeMarketCap":   toFloat(item["PLEDGE_MARKET_CAP"]),
			"pledgeDealNum":     toFloat(item["PLEDGE_DEAL_NUM"]),
			"repurchaseBalance": toFloat(item["REPURCHASE_BALANCE"]),
			"industry":          item["INDUSTRY"],
			"yearChangeRate":    toFloat(item["Y1_CLOSE_ADJCHRATE"]),
		}
	}
}

func normalizeLockupRelease(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"freeDate":          item["FREE_DATE"],
			"freeSharesType":    item["FREE_SHARES_TYPE"],
			"freeShares":        toFloat(item["FREE_SHARES"]),
			"currentFreeShares": toFloat(item["CURRENT_FREE_SHARES"]),
			"freeRatio":         toFloat(item["FREE_RATIO"]),
			"totalRatio":        toFloat(item["TOTAL_RATIO"]),
			"liftMarketCap":     toFloat(item["LIFT_MARKET_CAP"]),
			"batchHolderNum":    toFloat(item["BATCH_HOLDER_NUM"]),
		}
	}
}

func normalizeShareholderChanges(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"holderName":         item["HOLDER_NAME"],
			"direction":          item["DIRECTION"],
			"changeShares":       toFloat(item["CHANGE_NUM"]),
			"changeSharesSigned": toFloat(item["CHANGE_NUM_SYMBOL"]),
			"changeRate":         toFloat(item["CHANGE_RATE"]),
			"afterChangeRate":    toFloat(item["AFTER_CHANGE_RATE"]),
			"holdRatio":          toFloat(item["HOLD_RATIO"]),
			"changeFreeRatio":    toFloat(item["CHANGE_FREE_RATIO"]),
			"startDate":          item["START_DATE"],
			"endDate":            item["END_DATE"],
			"noticeDate":         item["NOTICE_DATE"],
			"tradeDate":          item["TRADE_DATE"],
			"avgPrice":           toFloat(item["TRADE_AVERAGE_PRICE"]),
			"closePrice":         toFloat(item["CLOSE_PRICE"]),
			"market":             item["MARKET"],
		}
	}
}

func normalizeStockBuyback(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"noticeDate":           item["DIM_DATE"],
			"progress":             item["REPURPROGRESS"],
			"progressLabel":        buybackProgressLabel(item["REPURPROGRESS"]),
			"objective":            item["REPUROBJECTIVE"],
			"startDate":            item["REPURSTARTDATE"],
			"endDate":              item["REPURENDDATE"],
			"planPriceLower":       toFloat(item["REPURPRICELOWER"]),
			"planPriceUpper":       toFloat(item["REPURPRICECAP"]),
			"planSharesLower":      toFloat(item["REPURNUMLOWER"]),
			"planSharesUpper":      toFloat(item["REPURNUMCAP"]),
			"planAmountLower":      toFloat(item["REPURAMOUNTLOWER"]),
			"planAmountUpper":      toFloat(item["REPURAMOUNTLIMIT"]),
			"repurchasedShares":    toFloat(item["REPURNUM"]),
			"repurchasedAmount":    toFloat(item["REPURAMOUNT"]),
			"repurchasedPriceLow":  toFloat(item["REPURPRICELOWER1"]),
			"repurchasedPriceHigh": toFloat(item["REPURPRICECAP1"]),
			"advancedDate":         item["REPURADVANCEDATE"],
			"updateDate":           firstNonEmpty(item, "UPD", "UPDATEDATE"),
		}
	}
}

func normalizePerformanceForecast(items []map[string]any) {
	for _, item := range items {
		normalized := map[string]any{
			"netProfitLower": toFloat(item["PREDICT_AMT_LOWER"]),
			"netProfitUpper": toFloat(item["PREDICT_AMT_UPPER"]),
			"netProfitBase":  toFloat(item["PREYEAR_SAME_PERIOD"]),
			"ratioLower":     toFloat(item["PREDICT_RATIO_LOWER"]),
			"ratioUpper":     toFloat(item["PREDICT_RATIO_UPPER"]),
			"ratioMean":      toFloat(item["PREDICT_HBMEAN"]),
			"forecastAmount": toFloat(item["FORECAST_JZ"]),
			"forecastState":  item["FORECAST_STATE"],
			"predictType":    item["PREDICT_TYPE"],
			"predictFinance": item["PREDICT_FINANCE"],
			"reportDate":     item["REPORT_DATE"],
			"noticeDate":     item["NOTICE_DATE"],
			"predictContent": item["PREDICT_CONTENT"],
			"changeReason":   item["CHANGE_REASON_EXPLAIN"],
		}
		item["normalized"] = normalized
	}
}

func normalizePerformanceExpress(items []map[string]any) {
	for _, item := range items {
		normalized := map[string]any{
			"reportDate":   item["REPORT_DATE"],
			"noticeDate":   item["NOTICE_DATE"],
			"revenue":      toFloat(item["TOTAL_OPERATE_INCOME"]),
			"revenueYoY":   toFloat(item["YSTZ"]),
			"netProfit":    toFloat(item["PARENT_NETPROFIT"]),
			"netProfitYoY": toFloat(item["JLRTBZCL"]),
			"eps":          toFloat(item["BASIC_EPS"]),
			"roe":          toFloat(item["WEIGHTAVG_ROE"]),
			"bvps":         toFloat(item["PARENT_BVPS"]),
			"qDate":        item["QDATE"],
			"dataType":     item["DATATYPE"],
			"industry":     item["PUBLISHNAME"],
		}
		item["normalized"] = normalized
	}
}

func normalizePerformanceSchedule(items []map[string]any) {
	for _, item := range items {
		normalized := map[string]any{
			"reportDate":       item["REPORT_DATE"],
			"reportType":       item["REPORT_TYPE_NAME"],
			"appointDate":      firstNonEmpty(item, "APPOINT_PUBLISH_DATE", "FIRST_APPOINT_DATE"),
			"actualDate":       item["ACTUAL_PUBLISH_DATE"],
			"residualDays":     toFloat(item["RESIDUAL_DAYS"]),
			"firstChangeDate":  item["FIRST_CHANGE_DATE"],
			"secondChangeDate": item["SECOND_CHANGE_DATE"],
			"thirdChangeDate":  item["THIRD_CHANGE_DATE"],
		}
		item["normalized"] = normalized
	}
}

func normalizeResearchReports(items []map[string]any) {
	for _, item := range items {
		item["normalized"] = map[string]any{
			"title":              firstNonEmpty(item, "title", "TITLE"),
			"publishDate":        firstNonEmpty(item, "publishDate", "PUBLISH_DATE"),
			"orgName":            firstNonEmpty(item, "orgSName", "ORG_S_NAME", "orgName", "ORG_NAME"),
			"rating":             firstNonEmpty(item, "emRatingName", "EM_RATING_NAME", "sRatingName", "S_RATING_NAME"),
			"predictThisYearEps": firstNonEmpty(item, "predictThisYearEps", "PREDICT_THIS_YEAR_EPS"),
			"predictNextYearEps": firstNonEmpty(item, "predictNextYearEps", "PREDICT_NEXT_YEAR_EPS"),
			"predictThisYearPe":  firstNonEmpty(item, "predictThisYearPe", "PREDICT_THIS_YEAR_PE"),
			"predictNextYearPe":  firstNonEmpty(item, "predictNextYearPe", "PREDICT_NEXT_YEAR_PE"),
			"researcher":         firstNonEmpty(item, "researcher", "RESEARCHER"),
			"industry":           firstNonEmpty(item, "indvInduName", "INDV_INDU_NAME"),
			"infoCode":           firstNonEmpty(item, "infoCode", "INFO_CODE"),
		}
	}
}

func buildForecastRevisionTrack(institutionForecast []map[string]any, researchReports []map[string]any, limit int) []map[string]any {
	if limit <= 0 {
		limit = 30
	}

	rows := make([]map[string]any, 0, len(institutionForecast)+len(researchReports))

	appendValue := func(row map[string]any, key string, value any) {
		if value == nil {
			return
		}
		if text, ok := value.(string); ok {
			text = strings.TrimSpace(text)
			if text == "" {
				return
			}
			if num, err := strconv.ParseFloat(text, 64); err == nil {
				row[key] = num
				return
			}
			row[key] = text
			return
		}
		row[key] = value
	}

	appendTrack := func(
		source string,
		item map[string]any,
		orgKeys []string,
		dateKeys []string,
		ratingKeys []string,
		epsThisKeys []string,
		epsNextKeys []string,
		peThisKeys []string,
		peNextKeys []string,
		titleKeys []string,
	) {
		if item == nil {
			return
		}
		org := strings.TrimSpace(toString(firstNonEmpty(item, orgKeys...)))
		publishDate := firstNonEmpty(item, dateKeys...)
		if org == "" && publishDate == nil {
			return
		}

		row := map[string]any{
			"source":      source,
			"orgName":     org,
			"publishDate": publishDate,
		}
		if rating := firstNonEmpty(item, ratingKeys...); rating != nil {
			row["rating"] = rating
		}
		if title := firstNonEmpty(item, titleKeys...); title != nil {
			row["title"] = title
		}
		appendValue(row, "epsThisYear", firstNonEmpty(item, epsThisKeys...))
		appendValue(row, "epsNextYear", firstNonEmpty(item, epsNextKeys...))
		appendValue(row, "peThisYear", firstNonEmpty(item, peThisKeys...))
		appendValue(row, "peNextYear", firstNonEmpty(item, peNextKeys...))
		rows = append(rows, row)
	}

	for _, item := range institutionForecast {
		appendTrack(
			"机构预测",
			item,
			[]string{"ORG_NAME_ABBR", "ORG_NAME", "ORG"},
			[]string{"PUBLISH_DATE", "NOTICE_DATE", "REPORT_DATE", "END_DATE"},
			[]string{"S_RATING_NAME", "EM_RATING_NAME", "RATING"},
			[]string{"EPS1", "PREDICT_THIS_YEAR_EPS"},
			[]string{"EPS2", "PREDICT_NEXT_YEAR_EPS"},
			[]string{"PE1", "PREDICT_THIS_YEAR_PE"},
			[]string{"PE2", "PREDICT_NEXT_YEAR_PE"},
			[]string{"TITLE", "REPORT_TITLE"},
		)
	}

	for _, item := range researchReports {
		appendTrack(
			"机构研报",
			item,
			[]string{"orgSName", "ORG_S_NAME", "ORG_NAME", "orgName"},
			[]string{"publishDate", "PUBLISH_DATE", "NOTICE_DATE"},
			[]string{"emRatingName", "EM_RATING_NAME", "sRatingName", "S_RATING_NAME"},
			[]string{"predictThisYearEps", "PREDICT_THIS_YEAR_EPS"},
			[]string{"predictNextYearEps", "PREDICT_NEXT_YEAR_EPS"},
			[]string{"predictThisYearPe", "PREDICT_THIS_YEAR_PE"},
			[]string{"predictNextYearPe", "PREDICT_NEXT_YEAR_PE"},
			[]string{"title", "TITLE", "REPORT_TITLE"},
		)
	}

	if len(rows) == 0 {
		return nil
	}

	rows = sortRecordsByDateDesc(rows, "publishDate", "PUBLISH_DATE", "NOTICE_DATE", "REPORT_DATE", "END_DATE")
	unique := make(map[string]struct{})
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		key := strings.Join([]string{
			toString(row["source"]),
			toString(row["orgName"]),
			toString(row["publishDate"]),
			toString(row["rating"]),
			toString(row["epsThisYear"]),
			toString(row["epsNextYear"]),
			toString(row["title"]),
		}, "|")
		if _, ok := unique[key]; ok {
			continue
		}
		unique[key] = struct{}{}
		result = append(result, row)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func buildLatestRecord(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func sortRecordsByDateDesc(items []map[string]any, keys ...string) []map[string]any {
	if len(items) == 0 {
		return items
	}
	sort.SliceStable(items, func(i, j int) bool {
		ti := parseRecordTime(items[i], keys...)
		tj := parseRecordTime(items[j], keys...)
		if ti.IsZero() && tj.IsZero() {
			return false
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.After(tj)
	})
	return items
}

func filterRecordsByReportType(items []map[string]any, keywords []string) []map[string]any {
	if len(items) == 0 || len(keywords) == 0 {
		return nil
	}
	var filtered []map[string]any
	for _, item := range items {
		raw := firstNonEmpty(item, "REPORT_TYPE", "REPORT_TYPE_NAME")
		if raw == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if text == "" {
			continue
		}
		for _, keyword := range keywords {
			if keyword != "" && strings.Contains(text, keyword) {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func trimRecords(items []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func parseRecordTime(record map[string]any, keys ...string) time.Time {
	if record == nil {
		return time.Time{}
	}
	for _, key := range keys {
		if value, ok := record[key]; ok {
			if t, ok := parseDateValue(value); ok {
				return t
			}
		}
	}
	if normalized, ok := record["normalized"].(map[string]any); ok {
		for _, key := range keys {
			if value, ok := normalized[key]; ok {
				if t, ok := parseDateValue(value); ok {
					return t
				}
			}
		}
	}
	return time.Time{}
}

func parseDateValue(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			return time.Time{}, false
		}
		return v, true
	case string:
		return parseDateString(v)
	}
	return time.Time{}, false
}

func parseDateString(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "--" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func buildFundFlowLatest(fields []string, lines [][]string, labels map[string]string) map[string]any {
	if len(lines) == 0 || len(fields) == 0 {
		return nil
	}
	line := lines[0]
	latest := make(map[string]any)
	for idx, field := range fields {
		if idx >= len(line) {
			continue
		}
		value := line[idx]
		if field == "f51" {
			latest["date"] = value
			latest[field] = value
		} else if num, err := strconv.ParseFloat(value, 64); err == nil {
			latest[field] = num
		} else {
			latest[field] = value
		}
		if label, ok := labels[field]; ok {
			latest[label] = latest[field]
		}
	}
	return latest
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case string:
		if v == "" {
			return 0
		}
		num, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return num
		}
	}
	return 0
}

func scaleQuotePrice(value float64) float64 {
	if value == 0 {
		return 0
	}
	if value > 1000 {
		return value / 100
	}
	return value
}

func scaleQuoteValue(value float64, factor float64) float64 {
	if value == 0 || factor == 0 {
		return 0
	}
	return value * factor
}

func firstNonEmpty(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if str, ok := value.(string); ok {
				if strings.TrimSpace(str) != "" {
					return value
				}
			} else if value != nil {
				return value
			}
		}
	}
	return nil
}

func buybackProgressLabel(value any) string {
	if value == nil {
		return ""
	}

	code := strings.TrimSpace(fmt.Sprintf("%v", value))
	if code == "" || code == "<nil>" {
		return ""
	}

	labels := map[string]string{
		"001": "董事会预案",
		"002": "股东大会通过",
		"003": "股东大会否决",
		"004": "实施中",
		"005": "停止实施",
		"006": "完成实施",
	}

	if label, ok := labels[code]; ok {
		return label
	}

	if isAllDigits(code) && len(code) < 3 {
		for len(code) < 3 {
			code = "0" + code
		}
		if label, ok := labels[code]; ok {
			return label
		}
	}

	return code
}

func isAllDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func isHTMLBody(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, "<") || strings.HasPrefix(strings.ToLower(trimmed), "<!doctype")
}
