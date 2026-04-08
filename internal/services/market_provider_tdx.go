package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/paths"
)

var tdxIndexNames = map[string]string{
	"sh000001": "上证指数",
	"sz399001": "深证成指",
	"sz399006": "创业板指",
}

type stockCatalogCache struct {
	UpdatedAt time.Time           `json:"updatedAt"`
	Items     []StockSearchResult `json:"items"`
}

type tdxMarketProvider struct {
	mu           sync.RWMutex
	clientMu     sync.Mutex
	client       tdxClient
	dialClient   func() (tdxClient, error)
	catalog      []StockSearchResult
	nameBySymbol map[string]string
	updatedAt    time.Time
	cacheTTL     time.Duration
	cachePath    string
}

func newTDXMarketProvider() *tdxMarketProvider {
	return &tdxMarketProvider{
		nameBySymbol: make(map[string]string),
		cacheTTL:     12 * time.Hour,
		cachePath:    filepath.Join(paths.EnsureCacheDir("market"), "tdx_stock_catalog.json"),
		dialClient: func() (tdxClient, error) {
			return tdx.DialDefault(tdx.WithRedial())
		},
	}
}

func (p *tdxMarketProvider) FetchStockDataWithOrderBook(codes ...string) ([]StockWithOrderBook, error) {
	cli, err := p.getClient()
	if err != nil {
		return nil, err
	}

	quotes, err := cli.GetQuote(codes...)
	if err != nil {
		return nil, err
	}

	if len(quotes) == 0 {
		return nil, errors.New("tdx returned empty quote data")
	}

	if _, err := p.loadCatalog(); err != nil {
		log.Warn("加载 TDX 股票目录失败，名称将回退为代码: %v", err)
	}

	stocks := make([]StockWithOrderBook, 0, len(quotes))
	for _, quote := range quotes {
		symbol := quote.Exchange.String() + quote.Code
		price := quote.K.Close.Float64()
		preClose := quote.K.Last.Float64()
		change := price - preClose
		changePercent := 0.0
		if preClose > 0 {
			changePercent = change / preClose * 100
		}

		stock := models.Stock{
			Symbol:        symbol,
			Name:          p.lookupName(symbol),
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
			Volume:        int64(quote.TotalHand),
			Amount:        quote.Amount,
			Open:          quote.K.Open.Float64(),
			High:          quote.K.High.Float64(),
			Low:           quote.K.Low.Float64(),
			PreClose:      preClose,
		}

		bids := make([]models.OrderBookItem, 0, 5)
		asks := make([]models.OrderBookItem, 0, 5)
		for _, level := range quote.BuyLevel {
			if level.Price <= 0 {
				continue
			}
			bids = append(bids, models.OrderBookItem{
				Price: level.Price.Float64(),
				Size:  int64(level.Number),
			})
		}
		for _, level := range quote.SellLevel {
			if level.Price <= 0 {
				continue
			}
			asks = append(asks, models.OrderBookItem{
				Price: level.Price.Float64(),
				Size:  int64(level.Number),
			})
		}

		calculateOrderBookTotals(bids)
		calculateOrderBookTotals(asks)

		stocks = append(stocks, StockWithOrderBook{
			Stock:     stock,
			OrderBook: models.OrderBook{Bids: bids, Asks: asks},
		})
	}

	return stocks, nil
}

func (p *tdxMarketProvider) FetchKLineData(code string, period string, days int) ([]models.KLineData, error) {
	cli, err := p.getClient()
	if err != nil {
		return nil, err
	}

	var resp *protocol.KlineResp
	switch period {
	case "1m":
		resp, err = cli.GetKlineMinuteAll(code)
	case "1d":
		resp, err = cli.GetKlineDayAll(code)
	case "1w":
		resp, err = cli.GetKlineWeekAll(code)
	case "1mo":
		resp, err = cli.GetKlineMonthAll(code)
	default:
		resp, err = cli.GetKlineDayAll(code)
	}
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.List) == 0 {
		return nil, errors.New("tdx returned empty kline data")
	}

	sort.Slice(resp.List, func(i, j int) bool {
		return resp.List[i].Time.Before(resp.List[j].Time)
	})

	klines := make([]models.KLineData, 0, len(resp.List))
	for _, item := range resp.List {
		timeValue := item.Time.Format("2006-01-02")
		if period == "1m" {
			timeValue = item.Time.Format("2006-01-02 15:04:05")
		}
		klines = append(klines, models.KLineData{
			Time:   timeValue,
			Open:   item.Open.Float64(),
			High:   item.High.Float64(),
			Low:    item.Low.Float64(),
			Close:  item.Close.Float64(),
			Volume: item.Volume,
			Amount: item.Amount.Float64(),
		})
	}

	if period == "1m" {
		klines = filterTodayKLines(klines)
		klines = trimKLines(klines, days)
		klines = calculateAvgLineByLotVolume(klines)
		return klines, nil
	}

	klines = trimKLines(klines, days)
	applyMovingAverages(klines)
	return klines, nil
}

func (p *tdxMarketProvider) FetchMarketIndices() ([]models.MarketIndex, error) {
	cli, err := p.getClient()
	if err != nil {
		return nil, err
	}

	indices := make([]models.MarketIndex, 0, len(tdxIndexNames))
	for code, name := range tdxIndexNames {
		resp, err := cli.GetIndexMinute(code, 0, 2)
		if err != nil {
			return nil, err
		}
		if resp == nil || len(resp.List) == 0 {
			return nil, errors.New("tdx returned empty index data")
		}

		dayResp, err := cli.GetIndexDay(code, 0, 2)
		if err != nil {
			return nil, err
		}
		if dayResp == nil || len(dayResp.List) == 0 {
			return nil, errors.New("tdx returned empty index day data")
		}

		sort.Slice(resp.List, func(i, j int) bool {
			return resp.List[i].Time.Before(resp.List[j].Time)
		})
		sort.Slice(dayResp.List, func(i, j int) bool {
			return dayResp.List[i].Time.Before(dayResp.List[j].Time)
		})

		latest := resp.List[len(resp.List)-1]
		daily := dayResp.List[len(dayResp.List)-1]
		preClose := daily.Last.Float64()
		if preClose == 0 && len(dayResp.List) > 1 {
			preClose = dayResp.List[len(dayResp.List)-2].Close.Float64()
		}
		if preClose == 0 {
			preClose = latest.Last.Float64()
		}
		if preClose == 0 && len(resp.List) > 1 {
			preClose = resp.List[0].Last.Float64()
		}
		if preClose == 0 && len(resp.List) > 1 {
			preClose = resp.List[len(resp.List)-2].Close.Float64()
		}
		price := latest.Close.Float64()
		change := price - preClose
		changePercent := 0.0
		if preClose > 0 {
			changePercent = change / preClose * 100
		}
		indices = append(indices, models.MarketIndex{
			Code:          code,
			Name:          name,
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
			Volume:        latest.Volume,
			Amount:        latest.Amount.Float64(),
		})
	}

	sort.Slice(indices, func(i, j int) bool {
		return indices[i].Code < indices[j].Code
	})
	return indices, nil
}

func (p *tdxMarketProvider) SearchStocks(keyword string, limit int) ([]StockSearchResult, error) {
	catalog, err := p.loadCatalog()
	if err != nil {
		return nil, err
	}
	return filterStockCatalog(catalog, keyword, limit), nil
}

func (p *tdxMarketProvider) getClient() (tdxClient, error) {
	p.mu.RLock()
	if p.client != nil {
		defer p.mu.RUnlock()
		return p.client, nil
	}
	p.mu.RUnlock()

	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	p.mu.RLock()
	if p.client != nil {
		defer p.mu.RUnlock()
		return p.client, nil
	}
	p.mu.RUnlock()

	cli, err := p.dialClient()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.client = cli
	p.mu.Unlock()

	return cli, nil
}

type tdxClient interface {
	GetQuote(codes ...string) (protocol.QuotesResp, error)
	GetKlineMinuteAll(code string) (*protocol.KlineResp, error)
	GetKlineDayAll(code string) (*protocol.KlineResp, error)
	GetIndexDay(code string, start, count uint16) (*protocol.KlineResp, error)
	GetKlineWeekAll(code string) (*protocol.KlineResp, error)
	GetKlineMonthAll(code string) (*protocol.KlineResp, error)
	GetCodeAll(exchange protocol.Exchange) (*protocol.CodeResp, error)
	GetIndexMinute(code string, start, count uint16) (*protocol.KlineResp, error)
}

func (p *tdxMarketProvider) loadCatalog() ([]StockSearchResult, error) {
	p.mu.RLock()
	if len(p.catalog) > 0 && time.Since(p.updatedAt) < p.cacheTTL {
		items := append([]StockSearchResult(nil), p.catalog...)
		p.mu.RUnlock()
		return items, nil
	}
	p.mu.RUnlock()

	if cache, err := p.loadCatalogCacheFromFile(); err == nil && len(cache.Items) > 0 && time.Since(cache.UpdatedAt) < p.cacheTTL {
		p.setCatalog(cache.Items, cache.UpdatedAt)
		return append([]StockSearchResult(nil), cache.Items...), nil
	}

	items, err := p.fetchCatalogFromTDX()
	if err == nil && len(items) > 0 {
		p.setCatalog(items, time.Now())
		p.saveCatalogCacheToFile(items)
		return append([]StockSearchResult(nil), items...), nil
	}

	p.mu.RLock()
	if len(p.catalog) > 0 {
		items = append([]StockSearchResult(nil), p.catalog...)
		p.mu.RUnlock()
		return items, nil
	}
	p.mu.RUnlock()

	if cache, cacheErr := p.loadCatalogCacheFromFile(); cacheErr == nil && len(cache.Items) > 0 {
		p.setCatalog(cache.Items, cache.UpdatedAt)
		return append([]StockSearchResult(nil), cache.Items...), nil
	}

	return nil, err
}

func (p *tdxMarketProvider) fetchCatalogFromTDX() ([]StockSearchResult, error) {
	cli, err := p.getClient()
	if err != nil {
		return nil, err
	}

	results := make([]StockSearchResult, 0, 6000)
	for _, exchange := range []protocol.Exchange{protocol.ExchangeSH, protocol.ExchangeSZ, protocol.ExchangeBJ} {
		resp, err := cli.GetCodeAll(exchange)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.List {
			symbol := exchange.String() + item.Code
			if !protocol.IsStock(symbol) {
				continue
			}
			results = append(results, StockSearchResult{
				Symbol: symbol,
				Name:   strings.TrimSpace(item.Name),
				Market: exchange.Name(),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Symbol < results[j].Symbol
	})
	return results, nil
}

func (p *tdxMarketProvider) loadCatalogCacheFromFile() (*stockCatalogCache, error) {
	data, err := os.ReadFile(p.cachePath)
	if err != nil {
		return nil, err
	}

	var cache stockCatalogCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func (p *tdxMarketProvider) saveCatalogCacheToFile(items []StockSearchResult) {
	cache := stockCatalogCache{
		UpdatedAt: time.Now(),
		Items:     items,
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		log.Warn("保存 TDX 股票目录缓存失败: %v", err)
		return
	}
	if err := os.WriteFile(p.cachePath, data, 0644); err != nil {
		log.Warn("写入 TDX 股票目录缓存失败: %v", err)
	}
}

func (p *tdxMarketProvider) setCatalog(items []StockSearchResult, updatedAt time.Time) {
	nameBySymbol := make(map[string]string, len(items))
	for _, item := range items {
		nameBySymbol[item.Symbol] = item.Name
	}

	p.mu.Lock()
	p.catalog = append([]StockSearchResult(nil), items...)
	p.nameBySymbol = nameBySymbol
	p.updatedAt = updatedAt
	p.mu.Unlock()
}

func (p *tdxMarketProvider) lookupName(symbol string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if name := p.nameBySymbol[symbol]; name != "" {
		return name
	}
	return symbol
}

func trimKLines(klines []models.KLineData, limit int) []models.KLineData {
	if limit <= 0 || len(klines) <= limit {
		return klines
	}
	return klines[len(klines)-limit:]
}

func applyMovingAverages(klines []models.KLineData) {
	for i := range klines {
		klines[i].MA5 = calculateMovingAverage(klines, i, 5)
		klines[i].MA10 = calculateMovingAverage(klines, i, 10)
		klines[i].MA20 = calculateMovingAverage(klines, i, 20)
	}
}

func calculateMovingAverage(klines []models.KLineData, end int, period int) float64 {
	start := end - period + 1
	if start < 0 {
		return 0
	}

	sum := 0.0
	for i := start; i <= end; i++ {
		sum += klines[i].Close
	}
	return sum / float64(period)
}

func calculateAvgLineByLotVolume(klines []models.KLineData) []models.KLineData {
	if len(klines) == 0 {
		return klines
	}

	var totalAmount float64
	var totalVolume int64

	for i := range klines {
		totalAmount += klines[i].Amount
		totalVolume += klines[i].Volume
		if totalVolume > 0 {
			// TDX 分时成交量单位是“手”，换算成股后再计算均价。
			klines[i].Avg = totalAmount / float64(totalVolume*100)
		}
	}

	return klines
}
