package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

const (
	coreDataPackSuccessTTL = 5 * time.Second
	coreDataPackErrorTTL   = 1 * time.Second
)

type coreDataPackCacheEntry struct {
	pack      models.CoreDataPack
	expiresAt time.Time
}

type coreDataPackFetchResult struct {
	pack models.CoreDataPack
	err  error
}

// CoreDataService 核心数据包服务
type CoreDataService struct {
	marketService *MarketService
	f10Service    *F10Service

	mu       sync.Mutex
	cache    map[string]coreDataPackCacheEntry
	inflight map[string][]chan coreDataPackFetchResult

	successTTL time.Duration
	errorTTL   time.Duration

	// 测试注入：避免单测依赖真实网络服务。
	stockFetcher           func(code string) (models.Stock, error, bool)
	valuationFetcher       func(code string) (models.StockValuation, error)
	fundFlowFetcher        func(code string) (models.FundFlowSeries, error)
	performanceFetcher     func(code string) (models.PerformanceEvents, error)
	mainIndicatorsFetcher  func(code string) (models.F10MainIndicators, error)
	industryMetricsFetcher func(code string) (models.F10IndustryCompareMetrics, error)
}

// NewCoreDataService 创建核心数据包服务
func NewCoreDataService(marketService *MarketService, f10Service *F10Service) *CoreDataService {
	return &CoreDataService{
		marketService: marketService,
		f10Service:    f10Service,
		cache:         make(map[string]coreDataPackCacheEntry),
		inflight:      make(map[string][]chan coreDataPackFetchResult),
		successTTL:    coreDataPackSuccessTTL,
		errorTTL:      coreDataPackErrorTTL,
	}
}

// GetCoreDataPack 获取核心数据包
func (s *CoreDataService) GetCoreDataPack(code string) (models.CoreDataPack, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return models.CoreDataPack{
			Code:      code,
			UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, fmt.Errorf("股票代码不能为空")
	}
	cacheKey := strings.ToUpper(code)

	if pack, ok := s.tryGetCached(cacheKey); ok {
		return pack, nil
	}

	if result, waiting := s.waitInflightOrRegister(cacheKey); waiting {
		return result.pack, result.err
	}

	result := s.fetchCoreDataPack(code)
	s.finishInflight(cacheKey, result)
	return result.pack, result.err
}

func (s *CoreDataService) fetchCoreDataPack(code string) coreDataPackFetchResult {
	pack := models.CoreDataPack{
		Code:      code,
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	var mu sync.Mutex
	setError := func(key string, err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if pack.Errors == nil {
			pack.Errors = make(map[string]string)
		}
		pack.Errors[key] = err.Error()
		mu.Unlock()
	}

	if stock, err, ok := s.fetchStock(code); err != nil {
		setError("stock", err)
	} else if ok {
		pack.Stock = stock
	}

	tasks := make([]func(), 0, 5)
	tasks = append(tasks, func() {
		valuation, err, enabled := s.fetchValuation(code)
		if !enabled {
			return
		}
		if err != nil {
			setError("valuation", err)
			return
		}
		mu.Lock()
		pack.Valuation = valuation
		mu.Unlock()
	})
	tasks = append(tasks, func() {
		flow, err, enabled := s.fetchFundFlow(code)
		if !enabled {
			return
		}
		if err != nil {
			setError("fundFlow", err)
			return
		}
		mu.Lock()
		pack.FundFlow = flow
		mu.Unlock()
	})
	tasks = append(tasks, func() {
		events, err, enabled := s.fetchPerformance(code)
		if !enabled {
			return
		}
		if err != nil {
			setError("performance", err)
			return
		}
		mu.Lock()
		pack.Performance = events
		mu.Unlock()
	})
	tasks = append(tasks, func() {
		indicators, err, enabled := s.fetchMainIndicators(code)
		if !enabled {
			return
		}
		if err != nil {
			setError("mainIndicators", err)
			return
		}
		mu.Lock()
		pack.MainIndicators = indicators
		mu.Unlock()
	})
	tasks = append(tasks, func() {
		metrics, err, enabled := s.fetchIndustryMetrics(code)
		if !enabled {
			return
		}
		if err != nil {
			setError("industryMetrics", err)
			return
		}
		mu.Lock()
		pack.IndustryMetrics = metrics
		mu.Unlock()
	})

	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go func(fn func()) {
			defer wg.Done()
			fn()
		}(task)
	}
	wg.Wait()

	return coreDataPackFetchResult{pack: pack}
}

func (s *CoreDataService) tryGetCached(cacheKey string) (models.CoreDataPack, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[cacheKey]
	if !ok {
		return models.CoreDataPack{}, false
	}
	if now.After(entry.expiresAt) {
		delete(s.cache, cacheKey)
		return models.CoreDataPack{}, false
	}
	return entry.pack, true
}

func (s *CoreDataService) waitInflightOrRegister(cacheKey string) (coreDataPackFetchResult, bool) {
	s.mu.Lock()
	if waiters, ok := s.inflight[cacheKey]; ok {
		ch := make(chan coreDataPackFetchResult, 1)
		s.inflight[cacheKey] = append(waiters, ch)
		s.mu.Unlock()
		return <-ch, true
	}
	s.inflight[cacheKey] = make([]chan coreDataPackFetchResult, 0)
	s.mu.Unlock()
	return coreDataPackFetchResult{}, false
}

func (s *CoreDataService) finishInflight(cacheKey string, result coreDataPackFetchResult) {
	ttl := s.successTTL
	if result.err != nil || len(result.pack.Errors) > 0 {
		ttl = s.errorTTL
	}
	if ttl < 0 {
		ttl = 0
	}

	s.mu.Lock()
	waiters := s.inflight[cacheKey]
	delete(s.inflight, cacheKey)
	if ttl > 0 {
		s.cache[cacheKey] = coreDataPackCacheEntry{
			pack:      result.pack,
			expiresAt: time.Now().Add(ttl),
		}
	}
	s.mu.Unlock()

	for _, waiter := range waiters {
		waiter <- result
		close(waiter)
	}
}

func (s *CoreDataService) fetchStock(code string) (models.Stock, error, bool) {
	if s.stockFetcher != nil {
		return s.stockFetcher(code)
	}
	if s.marketService == nil {
		return models.Stock{}, nil, false
	}
	stocks, err := s.marketService.GetStockRealTimeData(code)
	if err != nil {
		return models.Stock{}, err, true
	}
	if len(stocks) == 0 {
		return models.Stock{}, nil, false
	}
	return stocks[0], nil, true
}

func (s *CoreDataService) fetchValuation(code string) (models.StockValuation, error, bool) {
	if s.valuationFetcher != nil {
		valuation, err := s.valuationFetcher(code)
		return valuation, err, true
	}
	if s.f10Service == nil {
		return models.StockValuation{}, nil, false
	}
	valuation, err := s.f10Service.GetValuationByCode(code)
	return valuation, err, true
}

func (s *CoreDataService) fetchFundFlow(code string) (models.FundFlowSeries, error, bool) {
	if s.fundFlowFetcher != nil {
		flow, err := s.fundFlowFetcher(code)
		return flow, err, true
	}
	if s.f10Service == nil {
		return models.FundFlowSeries{}, nil, false
	}
	flow, err := s.f10Service.GetFundFlowByCode(code)
	return flow, err, true
}

func (s *CoreDataService) fetchPerformance(code string) (models.PerformanceEvents, error, bool) {
	if s.performanceFetcher != nil {
		events, err := s.performanceFetcher(code)
		return events, err, true
	}
	if s.f10Service == nil {
		return models.PerformanceEvents{}, nil, false
	}
	events, err := s.f10Service.GetPerformanceEventsByCode(code)
	return events, err, true
}

func (s *CoreDataService) fetchMainIndicators(code string) (models.F10MainIndicators, error, bool) {
	if s.mainIndicatorsFetcher != nil {
		indicators, err := s.mainIndicatorsFetcher(code)
		return indicators, err, true
	}
	if s.f10Service == nil {
		return models.F10MainIndicators{}, nil, false
	}
	indicators, err := s.f10Service.GetMainIndicators(code)
	return indicators, err, true
}

func (s *CoreDataService) fetchIndustryMetrics(code string) (models.F10IndustryCompareMetrics, error, bool) {
	if s.industryMetricsFetcher != nil {
		metrics, err := s.industryMetricsFetcher(code)
		return metrics, err, true
	}
	if s.f10Service == nil {
		return models.F10IndustryCompareMetrics{}, nil, false
	}
	metrics, err := s.f10Service.GetIndustryCompareMetrics(code)
	return metrics, err, true
}

// BuildCoreContext 将核心数据包压缩为可注入到专家的上下文
func (s *CoreDataService) BuildCoreContext(pack models.CoreDataPack) string {
	quote := map[string]any{
		"symbol":        pack.Stock.Symbol,
		"name":          pack.Stock.Name,
		"price":         pack.Stock.Price,
		"change":        pack.Stock.Change,
		"changePercent": pack.Stock.ChangePercent,
		"open":          pack.Stock.Open,
		"high":          pack.Stock.High,
		"low":           pack.Stock.Low,
		"preClose":      pack.Stock.PreClose,
		"volume":        pack.Stock.Volume,
		"amount":        pack.Stock.Amount,
		"marketCap":     pack.Stock.MarketCap,
	}

	summary := map[string]any{
		"updatedAt": pack.UpdatedAt,
		"quote":     quote,
		"valuation": pack.Valuation,
	}

	if len(pack.FundFlow.Latest) > 0 {
		summary["fundFlowLatest"] = pack.FundFlow.Latest
	}

	if latest := firstMap(pack.Performance.Forecast); len(latest) > 0 {
		summary["performanceForecast"] = latest
	}
	if latest := firstMap(pack.Performance.Express); len(latest) > 0 {
		summary["performanceExpress"] = latest
	}
	if latest := firstMap(pack.Performance.Schedule); len(latest) > 0 {
		summary["performanceSchedule"] = latest
	}

	if latest := firstMap(pack.MainIndicators.Latest); len(latest) > 0 {
		summary["mainIndicatorsLatest"] = latest
	}

	if latest := firstMap(pack.IndustryMetrics.Valuation); len(latest) > 0 {
		summary["industryValuationLatest"] = latest
	}
	if latest := firstMap(pack.IndustryMetrics.Performance); len(latest) > 0 {
		summary["industryPerformanceLatest"] = latest
	}
	if latest := firstMap(pack.IndustryMetrics.Growth); len(latest) > 0 {
		summary["industryGrowthLatest"] = latest
	}

	if len(pack.Errors) > 0 {
		summary["errors"] = pack.Errors
	}

	data, err := json.Marshal(summary)
	if err != nil {
		return ""
	}
	return string(data)
}

func firstMap(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}
