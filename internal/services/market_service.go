package services

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var log = logger.New("market")

const (
	sinaStockURL       = "http://hq.sinajs.cn/rn=%d&list=%s"
	sinaKLineURL       = "http://quotes.sina.cn/cn/api/json_v2.php/CN_MarketDataService.getKLineData?symbol=%s&scale=%s&ma=5,10,20&datalen=%d"
	holidayAPIURL      = "https://holiday.dreace.top/"
	emKLineURL         = "https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57&klt=%s&fqt=1&beg=0&end=20500101"
	emIndexTrendURL    = "https://push2.eastmoney.com/api/qt/stock/trends2/get?secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58&iscr=0&ndays=1"
	emQuoteURL         = "https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f12,f14,f2,f3,f4,f5,f6,f15,f16,f17,f18"
	emBoardFundFlowURL = "https://push2.eastmoney.com/api/qt/clist/get"
	emFundFlowKLineURL = "https://push2.eastmoney.com/api/qt/stock/fflow/kline/get"
	emAnnouncementURL  = "https://np-anotice-stock.eastmoney.com/api/security/ann"
)

// 默认大盘指数代码
var defaultIndexCodes = []string{
	"s_sh000001", // 上证指数
	"s_sz399001", // 深证成指
	"s_sz399006", // 创业板指
}

// StockWithOrderBook 包含盘口数据的股票信息
type StockWithOrderBook struct {
	models.Stock
	OrderBook models.OrderBook `json:"orderBook"`
}

// stockCache 股票数据缓存
type stockCache struct {
	data      []StockWithOrderBook
	timestamp time.Time
}

// MarketStatus 市场交易状态
type MarketStatus struct {
	Status      string `json:"status"`      // trading, closed, pre_market, lunch_break
	StatusText  string `json:"statusText"`  // 中文状态描述
	IsTradeDay  bool   `json:"isTradeDay"`  // 是否交易日
	HolidayName string `json:"holidayName"` // 节假日名称（如有）
}

// todayHolidayCache 当天节假日缓存
type todayHolidayCache struct {
	isHoliday bool
	note      string
	timestamp time.Time
}

// MarketService 市场数据服务
type MarketService struct {
	client *http.Client

	// 股票数据缓存
	cache    map[string]*stockCache
	cacheMu  sync.RWMutex
	cacheTTL time.Duration

	// 当天节假日缓存
	todayCache   *todayHolidayCache
	todayCacheMu sync.RWMutex
}

// NewMarketService 创建市场数据服务
func NewMarketService() *MarketService {
	return &MarketService{
		client:   proxy.GetManager().GetClientWithTimeout(10 * time.Second),
		cache:    make(map[string]*stockCache),
		cacheTTL: 2 * time.Second, // 缓存2秒，避免频繁请求
	}
}

// GetStockDataWithOrderBook 获取股票实时数据（含真实盘口），带缓存
func (ms *MarketService) GetStockDataWithOrderBook(codes ...string) ([]StockWithOrderBook, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	cacheKey := strings.Join(codes, ",")

	// 检查缓存
	ms.cacheMu.RLock()
	if cached, ok := ms.cache[cacheKey]; ok {
		if time.Since(cached.timestamp) < ms.cacheTTL {
			ms.cacheMu.RUnlock()
			return cached.data, nil
		}
	}
	ms.cacheMu.RUnlock()

	// 从API获取数据
	data, err := ms.fetchStockDataWithOrderBook(codes...)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	ms.cacheMu.Lock()
	ms.cache[cacheKey] = &stockCache{
		data:      data,
		timestamp: time.Now(),
	}
	ms.cacheMu.Unlock()

	return data, nil
}

// fetchStockDataWithOrderBook 从API获取股票数据（含盘口）
func (ms *MarketService) fetchStockDataWithOrderBook(codes ...string) ([]StockWithOrderBook, error) {
	codeList := strings.Join(codes, ",")
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), codeList)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return ms.parseSinaStockDataWithOrderBook(string(body))
}

// parseSinaStockDataWithOrderBook 解析新浪股票数据（含盘口）
func (ms *MarketService) parseSinaStockDataWithOrderBook(data string) ([]StockWithOrderBook, error) {
	var stocks []StockWithOrderBook
	re := regexp.MustCompile(`var hq_str_(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}
		parts := strings.Split(match[2], ",")
		if len(parts) < 32 {
			continue
		}
		stock := ms.parseStockWithOrderBook(match[1], parts)
		stocks = append(stocks, stock)
	}
	return stocks, nil
}

// GetStockRealTimeData 获取股票实时数据
func (ms *MarketService) GetStockRealTimeData(codes ...string) ([]models.Stock, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	codeList := strings.Join(codes, ",")
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), codeList)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	stocks, err := ms.parseSinaStockData(string(body), codes)
	if err == nil && len(stocks) > 0 && !stocksAllZero(stocks) {
		return stocks, nil
	}

	fallback, fbErr := ms.fetchEastmoneyQuotes(codes)
	if fbErr == nil && len(fallback) > 0 {
		return fallback, nil
	}
	return stocks, err
}

// parseSinaStockData 解析新浪股票数据
func (ms *MarketService) parseSinaStockData(data string, codes []string) ([]models.Stock, error) {
	var stocks []models.Stock
	re := regexp.MustCompile(`var hq_str_(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}
		parts := strings.Split(match[2], ",")
		if len(parts) < 32 {
			continue
		}

		stock := ms.parseStockFields(match[1], parts)
		stocks = append(stocks, stock)
	}
	return stocks, nil
}

// parseStockFields 解析股票字段
func (ms *MarketService) parseStockFields(code string, parts []string) models.Stock {
	price, _ := strconv.ParseFloat(parts[3], 64)
	open, _ := strconv.ParseFloat(parts[1], 64)
	high, _ := strconv.ParseFloat(parts[4], 64)
	low, _ := strconv.ParseFloat(parts[5], 64)
	preClose, _ := strconv.ParseFloat(parts[2], 64)
	volume, _ := strconv.ParseInt(parts[8], 10, 64)
	amount, _ := strconv.ParseFloat(parts[9], 64)

	change := price - preClose
	changePercent := 0.0
	if preClose > 0 {
		changePercent = (change / preClose) * 100
	}

	return models.Stock{
		Symbol:        code,
		Name:          parts[0],
		Price:         price,
		Open:          open,
		High:          high,
		Low:           low,
		PreClose:      preClose,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		Amount:        amount,
	}
}

// parseStockWithOrderBook 解析股票字段和真实盘口数据
// 新浪API返回数据格式: 名称,今开,昨收,当前价,最高,最低,买一价,卖一价,成交量,成交额,
// 买一量,买一价,买二量,买二价,买三量,买三价,买四量,买四价,买五量,买五价,
// 卖一量,卖一价,卖二量,卖二价,卖三量,卖三价,卖四量,卖四价,卖五量,卖五价,日期,时间
func (ms *MarketService) parseStockWithOrderBook(code string, parts []string) StockWithOrderBook {
	stock := ms.parseStockFields(code, parts)

	// 解析真实五档盘口数据
	var bids, asks []models.OrderBookItem

	// 买盘数据 (索引 10-19: 买一量,买一价,买二量,买二价...)
	if len(parts) >= 20 {
		for i := 0; i < 5; i++ {
			volIdx := 10 + i*2
			priceIdx := 11 + i*2
			if priceIdx < len(parts) {
				bidVol, _ := strconv.ParseInt(parts[volIdx], 10, 64)
				bidPrice, _ := strconv.ParseFloat(parts[priceIdx], 64)
				if bidPrice > 0 {
					bids = append(bids, models.OrderBookItem{
						Price: bidPrice,
						Size:  bidVol / 100, // 转换为手
					})
				}
			}
		}
	}

	// 卖盘数据 (索引 20-29: 卖一量,卖一价,卖二量,卖二价...)
	if len(parts) >= 30 {
		for i := 0; i < 5; i++ {
			volIdx := 20 + i*2
			priceIdx := 21 + i*2
			if priceIdx < len(parts) {
				askVol, _ := strconv.ParseInt(parts[volIdx], 10, 64)
				askPrice, _ := strconv.ParseFloat(parts[priceIdx], 64)
				if askPrice > 0 {
					asks = append(asks, models.OrderBookItem{
						Price: askPrice,
						Size:  askVol / 100, // 转换为手
					})
				}
			}
		}
	}

	// 计算累计量和占比
	ms.calculateOrderBookTotals(bids)
	ms.calculateOrderBookTotals(asks)

	return StockWithOrderBook{
		Stock:     stock,
		OrderBook: models.OrderBook{Bids: bids, Asks: asks},
	}
}

// calculateOrderBookTotals 计算盘口累计量和占比
func (ms *MarketService) calculateOrderBookTotals(items []models.OrderBookItem) {
	if len(items) == 0 {
		return
	}

	var total int64
	var maxSize int64
	for _, item := range items {
		if item.Size > maxSize {
			maxSize = item.Size
		}
	}

	for i := range items {
		total += items[i].Size
		items[i].Total = total
		if maxSize > 0 {
			items[i].Percent = float64(items[i].Size) / float64(maxSize)
		}
	}
}

// GetKLineData 获取K线数据
func (ms *MarketService) GetKLineData(code string, period string, days int) ([]models.KLineData, error) {
	normalizedCode := normalizeKLineCode(code)
	if period == "1m" && isIndexCode(normalizedCode) {
		if klines, err := ms.fetchEastmoneyKLineData(normalizedCode, period, days); err == nil && len(klines) > 0 {
			klines = sortKLines(klines)
			klines = ms.filterTodayKLines(klines)
			if len(klines) > 1 && isFlatKLine(klines) {
				if fallback, err := ms.fetchEastmoneyIndexTrend(normalizedCode); err == nil && len(fallback) > 0 {
					fallback = sortKLines(fallback)
					fallback = ms.filterTodayKLines(fallback)
					return fallback, nil
				}
			}
			return klines, nil
		}
		if fallback, err := ms.fetchEastmoneyIndexTrend(normalizedCode); err == nil && len(fallback) > 0 {
			fallback = sortKLines(fallback)
			fallback = ms.filterTodayKLines(fallback)
			return fallback, nil
		}
	}

	scale := ms.periodToScale(period)
	url := fmt.Sprintf(sinaKLineURL, normalizedCode, scale, days)

	resp, err := ms.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	klines, err := ms.parseKLineData(string(body))
	if err != nil {
		return nil, err
	}
	klines = sortKLines(klines)

	// 分时模式下只返回当天的数据，并计算均价线
	if period == "1m" {
		klines = ms.filterTodayKLines(klines)
		if !isIndexCode(normalizedCode) {
			klines = ms.calculateAvgLine(klines)
		}
	} else if period == "1w" || period == "1mo" {
		klines = ms.aggregateKLines(klines, period)
		klines = ms.calculateMovingAverages(klines)
	}

	return klines, nil
}

func isIndexCode(code string) bool {
	lower := strings.ToLower(strings.TrimSpace(code))
	return strings.HasPrefix(lower, "sh000") || strings.HasPrefix(lower, "sz399")
}

func normalizeKLineCode(code string) string {
	lower := strings.ToLower(strings.TrimSpace(code))
	if strings.HasPrefix(lower, "s_") {
		return strings.TrimPrefix(lower, "s_")
	}
	return lower
}

func indexSecID(code string) string {
	lower := normalizeKLineCode(code)
	if strings.HasPrefix(lower, "sh") && len(lower) > 2 {
		return "1." + lower[2:]
	}
	if strings.HasPrefix(lower, "sz") && len(lower) > 2 {
		return "0." + lower[2:]
	}
	return lower
}

func stockSecID(code string) string {
	lower := normalizeKLineCode(code)
	if strings.HasPrefix(lower, "sh") && len(lower) > 2 {
		return "1." + lower[2:]
	}
	if strings.HasPrefix(lower, "sz") && len(lower) > 2 {
		return "0." + lower[2:]
	}
	if strings.HasPrefix(lower, "bj") && len(lower) > 2 {
		return "0." + lower[2:]
	}
	return lower
}

func (ms *MarketService) periodToKlt(period string) string {
	switch period {
	case "1m":
		return "1"
	case "1d":
		return "101"
	case "1w":
		return "102"
	case "1mo":
		return "103"
	default:
		return "1"
	}
}

func (ms *MarketService) fetchEastmoneyKLineData(code string, period string, days int) ([]models.KLineData, error) {
	secID := indexSecID(code)
	klt := ms.periodToKlt(period)
	url := fmt.Sprintf(emKLineURL, secID, klt)

	resp, err := ms.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data struct {
			KLines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	klines := make([]models.KLineData, 0, len(payload.Data.KLines))
	for _, line := range payload.Data.KLines {
		parts := strings.Split(line, ",")
		if len(parts) < 7 {
			continue
		}
		open, _ := strconv.ParseFloat(parts[1], 64)
		closePrice, _ := strconv.ParseFloat(parts[2], 64)
		high, _ := strconv.ParseFloat(parts[3], 64)
		low, _ := strconv.ParseFloat(parts[4], 64)
		volume, _ := strconv.ParseInt(parts[5], 10, 64)
		amount, _ := strconv.ParseFloat(parts[6], 64)
		if amount == 0 && volume > 0 && closePrice > 0 {
			amount = closePrice * float64(volume)
		}
		klines = append(klines, models.KLineData{
			Time:   parts[0],
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: volume,
			Amount: amount,
		})
	}

	if days > 0 && len(klines) > days {
		klines = klines[len(klines)-days:]
	}
	return klines, nil
}

func (ms *MarketService) fetchEastmoneyIndexTrend(code string) ([]models.KLineData, error) {
	secID := indexSecID(code)
	url := fmt.Sprintf(emIndexTrendURL, secID)

	resp, err := ms.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data struct {
			Trends []string `json:"trends"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	klines := make([]models.KLineData, 0, len(payload.Data.Trends))
	for _, line := range payload.Data.Trends {
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		price, _ := strconv.ParseFloat(parts[1], 64)
		var volume int64
		var amount float64
		if len(parts) >= 3 {
			volume, _ = strconv.ParseInt(parts[2], 10, 64)
		}
		if len(parts) >= 5 {
			amount, _ = strconv.ParseFloat(parts[4], 64)
		}
		if amount == 0 && volume > 0 && price > 0 {
			amount = price * float64(volume)
		}
		klines = append(klines, models.KLineData{
			Time:   parts[0],
			Open:   price,
			High:   price,
			Low:    price,
			Close:  price,
			Volume: volume,
			Amount: amount,
		})
	}
	return klines, nil
}

func (ms *MarketService) fetchEastmoneyQuotes(codes []string) ([]models.Stock, error) {
	stocks := make([]models.Stock, 0, len(codes))
	for _, code := range codes {
		secID := stockSecID(code)
		if secID == "" {
			continue
		}
		url := fmt.Sprintf(emQuoteURL, secID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Referer", "https://quote.eastmoney.com/")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		resp, err := ms.client.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		if isHTMLBodyLocal(body) {
			continue
		}

		var payload struct {
			Data map[string]any `json:"data"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			continue
		}
		if payload.Data == nil {
			continue
		}

		price := toFloat64Any(payload.Data["f2"])
		open := toFloat64Any(payload.Data["f17"])
		high := toFloat64Any(payload.Data["f15"])
		low := toFloat64Any(payload.Data["f16"])
		preClose := toFloat64Any(payload.Data["f18"])
		volume := toInt64Any(payload.Data["f5"])
		amount := toFloat64Any(payload.Data["f6"])
		change := toFloat64Any(payload.Data["f4"])
		changePercent := toFloat64Any(payload.Data["f3"])
		if change == 0 {
			change = price - preClose
		}
		if changePercent == 0 && preClose > 0 {
			changePercent = change / preClose * 100
		}
		if price == 0 && preClose > 0 {
			price = preClose
		}

		name, _ := payload.Data["f14"].(string)
		codeValue, _ := payload.Data["f12"].(string)
		normalizedCode := normalizeKLineCode(code)
		if codeValue != "" {
			normalizedCode = normalizeKLineCode(codeValue)
		}

		stocks = append(stocks, models.Stock{
			Symbol:        normalizedCode,
			Name:          name,
			Price:         price,
			Open:          open,
			High:          high,
			Low:           low,
			PreClose:      preClose,
			Change:        change,
			ChangePercent: changePercent,
			Volume:        volume,
			Amount:        amount,
		})
	}
	return stocks, nil
}

func sortKLines(klines []models.KLineData) []models.KLineData {
	if len(klines) < 2 {
		return klines
	}
	sort.Slice(klines, func(i, j int) bool {
		ti, okI := parseKLineTime(klines[i].Time)
		tj, okJ := parseKLineTime(klines[j].Time)
		if !okI && !okJ {
			return klines[i].Time < klines[j].Time
		}
		if !okI {
			return false
		}
		if !okJ {
			return true
		}
		return ti.Before(tj)
	})
	return klines
}

func isFlatKLine(klines []models.KLineData) bool {
	if len(klines) < 2 {
		return false
	}
	min := klines[0].Close
	max := klines[0].Close
	for _, item := range klines[1:] {
		if item.Close < min {
			min = item.Close
		}
		if item.Close > max {
			max = item.Close
		}
	}
	return math.Abs(max-min) < 0.000001
}

func stocksAllZero(stocks []models.Stock) bool {
	if len(stocks) == 0 {
		return true
	}
	for _, stock := range stocks {
		if stock.Price != 0 || stock.Change != 0 || stock.ChangePercent != 0 || stock.Volume != 0 || stock.Amount != 0 {
			return false
		}
	}
	return true
}

func isHTMLBodyLocal(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html")
}

func toFloat64Any(value any) float64 {
	switch v := value.(type) {
	case json.Number:
		f, _ := v.Float64()
		return f
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func toInt64Any(value any) int64 {
	switch v := value.(type) {
	case json.Number:
		i, _ := v.Int64()
		return i
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	default:
		return 0
	}
}

// periodToScale 周期转换为新浪API的scale参数
func (ms *MarketService) periodToScale(period string) string {
	switch period {
	case "1m":
		return "1" // 1分钟线（分时图）
	case "1d":
		return "240" // 日线
	case "1w":
		return "1680" // 周线
	case "1mo":
		return "7200" // 月线
	default:
		return "240"
	}
}

// filterTodayKLines 过滤只返回当天的K线数据
func (ms *MarketService) filterTodayKLines(klines []models.KLineData) []models.KLineData {
	if len(klines) == 0 {
		return klines
	}

	today := time.Now().Format("2006-01-02")
	result := make([]models.KLineData, 0)

	for _, k := range klines {
		// 时间格式为 "2006-01-02 15:04:05"，取日期部分比较
		if len(k.Time) >= 10 && k.Time[:10] == today {
			result = append(result, k)
		}
	}

	// 如果当天没有数据（非交易日），返回最后一天的数据
	if len(result) == 0 && len(klines) > 0 {
		lastDay := klines[len(klines)-1].Time[:10]
		for _, k := range klines {
			if len(k.Time) >= 10 && k.Time[:10] == lastDay {
				result = append(result, k)
			}
		}
	}

	return result
}

func (ms *MarketService) aggregateKLines(klines []models.KLineData, period string) []models.KLineData {
	if len(klines) == 0 {
		return klines
	}

	type agg struct {
		key    string
		time   time.Time
		open   float64
		high   float64
		low    float64
		close  float64
		volume int64
		amount float64
	}

	var result []models.KLineData
	var current *agg

	for _, k := range klines {
		t, ok := parseKLineTime(k.Time)
		if !ok {
			continue
		}

		key := buildAggKey(t, period)
		if current == nil || current.key != key {
			if current != nil {
				result = append(result, models.KLineData{
					Time:   current.time.Format("2006-01-02"),
					Open:   current.open,
					High:   current.high,
					Low:    current.low,
					Close:  current.close,
					Volume: current.volume,
					Amount: current.amount,
				})
			}
			current = &agg{
				key:    key,
				time:   t,
				open:   k.Open,
				high:   k.High,
				low:    k.Low,
				close:  k.Close,
				volume: k.Volume,
				amount: k.Amount,
			}
			continue
		}

		if k.High > current.high {
			current.high = k.High
		}
		if k.Low < current.low {
			current.low = k.Low
		}
		current.close = k.Close
		current.time = t
		current.volume += k.Volume
		current.amount += k.Amount
	}

	if current != nil {
		result = append(result, models.KLineData{
			Time:   current.time.Format("2006-01-02"),
			Open:   current.open,
			High:   current.high,
			Low:    current.low,
			Close:  current.close,
			Volume: current.volume,
			Amount: current.amount,
		})
	}

	return result
}

func (ms *MarketService) calculateMovingAverages(klines []models.KLineData) []models.KLineData {
	if len(klines) == 0 {
		return klines
	}

	ma5 := calculateMA(klines, 5)
	ma10 := calculateMA(klines, 10)
	ma20 := calculateMA(klines, 20)

	for i := range klines {
		klines[i].MA5 = ma5[i]
		klines[i].MA10 = ma10[i]
		klines[i].MA20 = ma20[i]
	}

	return klines
}

func calculateMA(klines []models.KLineData, window int) []float64 {
	result := make([]float64, len(klines))
	if window <= 0 {
		return result
	}

	var sum float64
	for i := range klines {
		sum += klines[i].Close
		if i >= window {
			sum -= klines[i-window].Close
		}
		if i >= window-1 {
			result[i] = sum / float64(window)
		}
	}
	return result
}

func parseKLineTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if len(value) >= 19 {
		if t, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
			return t, true
		}
	}
	if len(value) >= 10 {
		if t, err := time.Parse("2006-01-02", value[:10]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func buildAggKey(t time.Time, period string) string {
	if period == "1mo" {
		return fmt.Sprintf("%04d-%02d", t.Year(), t.Month())
	}
	year, week := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

// calculateAvgLine 计算分时均价线 (VWAP = 累计成交额 / 累计成交量)
func (ms *MarketService) calculateAvgLine(klines []models.KLineData) []models.KLineData {
	if len(klines) == 0 {
		return klines
	}

	var totalAmount float64
	var totalVolume int64

	for i := range klines {
		totalAmount += klines[i].Amount
		totalVolume += klines[i].Volume

		if totalVolume > 0 {
			klines[i].Avg = totalAmount / float64(totalVolume)
		}
	}

	return klines
}

// parseKLineData 解析K线数据 - 使用标准JSON解析
func (ms *MarketService) parseKLineData(data string) ([]models.KLineData, error) {
	// 新浪API返回的K线数据结构（含均线和成交额）
	type sinaKLine struct {
		Day       string  `json:"day"`
		Open      string  `json:"open"`
		High      string  `json:"high"`
		Low       string  `json:"low"`
		Close     string  `json:"close"`
		Volume    string  `json:"volume"`
		Amount    string  `json:"amount"`
		MAPrice5  float64 `json:"ma_price5"`
		MAPrice10 float64 `json:"ma_price10"`
		MAPrice20 float64 `json:"ma_price20"`
	}

	var sinaData []sinaKLine
	if err := json.Unmarshal([]byte(data), &sinaData); err != nil {
		return nil, err
	}

	klines := make([]models.KLineData, 0, len(sinaData))
	for _, item := range sinaData {
		open, _ := strconv.ParseFloat(item.Open, 64)
		high, _ := strconv.ParseFloat(item.High, 64)
		low, _ := strconv.ParseFloat(item.Low, 64)
		closePrice, _ := strconv.ParseFloat(item.Close, 64)
		volume, _ := strconv.ParseInt(item.Volume, 10, 64)
		amount, _ := strconv.ParseFloat(item.Amount, 64)
		if amount == 0 && volume > 0 && closePrice > 0 {
			amount = closePrice * float64(volume)
		}

		klines = append(klines, models.KLineData{
			Time:   item.Day,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: volume,
			Amount: amount,
			MA5:    item.MAPrice5,
			MA10:   item.MAPrice10,
			MA20:   item.MAPrice20,
		})
	}
	return klines, nil
}

// GetRealOrderBook 获取真实盘口数据
func (ms *MarketService) GetRealOrderBook(code string) (models.OrderBook, error) {
	data, err := ms.GetStockDataWithOrderBook(code)
	if err != nil || len(data) == 0 {
		return models.OrderBook{}, err
	}
	return data[0].OrderBook, nil
}

// GenerateOrderBook 生成盘口数据（保留兼容，建议使用 GetRealOrderBook）
func (ms *MarketService) GenerateOrderBook(price float64) models.OrderBook {
	var bids, asks []models.OrderBookItem

	for i := 0; i < 5; i++ {
		bidPrice := price - float64(i+1)*0.01
		askPrice := price + float64(i+1)*0.01

		bids = append(bids, models.OrderBookItem{
			Price:   bidPrice,
			Size:    int64(100 + i*50),
			Total:   int64((100 + i*50) * (i + 1)),
			Percent: float64(100-i*15) / 100,
		})
		asks = append(asks, models.OrderBookItem{
			Price:   askPrice,
			Size:    int64(100 + i*50),
			Total:   int64((100 + i*50) * (i + 1)),
			Percent: float64(100-i*15) / 100,
		})
	}

	return models.OrderBook{Bids: bids, Asks: asks}
}

// GetMarketStatus 获取当前市场交易状态
func (ms *MarketService) GetMarketStatus() MarketStatus {
	log.Debug("开始获取市场状态")
	now := time.Now()
	// 使用固定时区 UTC+8，避免 Windows 缺少时区数据库的问题
	loc := time.FixedZone("CST", 8*60*60)
	now = now.In(loc)
	log.Debug("当前时间: %s, 星期: %s", now.Format("2006-01-02 15:04:05"), now.Weekday())

	// 检查是否为交易日
	isTradeDay, holidayName := ms.isTradeDay(now)
	log.Debug("isTradeDay=%v, holidayName=%s", isTradeDay, holidayName)

	if !isTradeDay {
		statusText := "休市"
		if holidayName != "" {
			statusText = holidayName + "休市"
		} else if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
			statusText = "周末休市"
		}
		result := MarketStatus{
			Status:      "closed",
			StatusText:  statusText,
			IsTradeDay:  false,
			HolidayName: holidayName,
		}
		log.Debug("返回结果: %+v", result)
		return result
	}

	// 交易日，判断当前时间段
	hour, minute := now.Hour(), now.Minute()
	currentMinutes := hour*60 + minute
	log.Debug("交易日时间判断: %02d:%02d, currentMinutes=%d", hour, minute, currentMinutes)

	// A股交易时间: 9:30-11:30, 13:00-15:00
	var result MarketStatus
	switch {
	case currentMinutes < 9*60+15:
		result = MarketStatus{Status: "pre_market", StatusText: "盘前", IsTradeDay: true}
	case currentMinutes < 9*60+30:
		result = MarketStatus{Status: "pre_market", StatusText: "集合竞价", IsTradeDay: true}
	case currentMinutes < 11*60+30:
		result = MarketStatus{Status: "trading", StatusText: "交易中", IsTradeDay: true}
	case currentMinutes < 13*60:
		result = MarketStatus{Status: "lunch_break", StatusText: "午间休市", IsTradeDay: true}
	case currentMinutes < 15*60:
		result = MarketStatus{Status: "trading", StatusText: "交易中", IsTradeDay: true}
	default:
		result = MarketStatus{Status: "closed", StatusText: "已收盘", IsTradeDay: true}
	}
	log.Debug("返回结果: %+v", result)
	return result
}

// isTradeDay 判断是否为交易日
func (ms *MarketService) isTradeDay(_ time.Time) (bool, string) {
	log.Debug("开始判断是否为交易日")
	isHoliday, note := ms.getTodayHolidayStatus()
	log.Debug("getTodayHolidayStatus返回: isHoliday=%v, note=%s", isHoliday, note)
	if isHoliday {
		return false, note
	}
	return true, ""
}

// getTodayHolidayStatus 获取当天节假日状态（带缓存）
func (ms *MarketService) getTodayHolidayStatus() (bool, string) {
	log.Debug("检查缓存")
	ms.todayCacheMu.RLock()
	if ms.todayCache != nil && time.Since(ms.todayCache.timestamp) < time.Hour {
		defer ms.todayCacheMu.RUnlock()
		log.Debug("命中缓存: isHoliday=%v, note=%s", ms.todayCache.isHoliday, ms.todayCache.note)
		return ms.todayCache.isHoliday, ms.todayCache.note
	}
	ms.todayCacheMu.RUnlock()

	// 缓存过期或不存在，重新获取
	log.Debug("缓存未命中，调用API")
	isHoliday, note := ms.fetchTodayHolidayStatus()
	log.Debug("API返回: isHoliday=%v, note=%s", isHoliday, note)

	ms.todayCacheMu.Lock()
	ms.todayCache = &todayHolidayCache{
		isHoliday: isHoliday,
		note:      note,
		timestamp: time.Now(),
	}
	ms.todayCacheMu.Unlock()

	return isHoliday, note
}

// fetchTodayHolidayStatus 从 API 获取当天节假日状态
func (ms *MarketService) fetchTodayHolidayStatus() (bool, string) {
	resp, err := ms.client.Get(holidayAPIURL)
	if err != nil {
		fmt.Println("[fetchTodayHolidayStatus] request error:", err)
		return false, ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[fetchTodayHolidayStatus] read body error:", err)
		return false, ""
	}

	// 解析 API 响应: {"date":"2026-02-04","isHoliday":false,"note":"普通工作日","type":"工作日"}
	var apiResp struct {
		Date      string `json:"date"`
		IsHoliday bool   `json:"isHoliday"`
		Note      string `json:"note"`
		Type      string `json:"type"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Println("[fetchTodayHolidayStatus] parse error:", err)
		return false, ""
	}

	return apiResp.IsHoliday, apiResp.Note
}

// GetMarketIndices 获取大盘指数数据
func (ms *MarketService) GetMarketIndices() ([]models.MarketIndex, error) {
	codeList := strings.Join(defaultIndexCodes, ",")
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), codeList)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return ms.parseMarketIndices(string(body))
}

// parseMarketIndices 解析大盘指数数据
// 新浪简化指数数据格式: var hq_str_s_sh000001="上证指数,3094.668,-128.073,-3.97,436653,5458126"
// 字段: 名称,当前点位,涨跌点数,涨跌幅(%),成交量(手),成交额(万元)
func (ms *MarketService) parseMarketIndices(data string) ([]models.MarketIndex, error) {
	var indices []models.MarketIndex
	re := regexp.MustCompile(`var hq_str_s_(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}
		parts := strings.Split(match[2], ",")
		if len(parts) < 6 {
			continue
		}

		price, _ := strconv.ParseFloat(parts[1], 64)
		change, _ := strconv.ParseFloat(parts[2], 64)
		changePercent, _ := strconv.ParseFloat(parts[3], 64)
		volume, _ := strconv.ParseInt(parts[4], 10, 64)
		amount, _ := strconv.ParseFloat(parts[5], 64)

		indices = append(indices, models.MarketIndex{
			Code:          match[1],
			Name:          parts[0],
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
			Volume:        volume,
			Amount:        amount,
		})
	}
	return indices, nil
}

// GetBoardFundFlowList 获取板块资金流列表（行业/概念/地域）
func (ms *MarketService) GetBoardFundFlowList(category string, page int, size int) (models.BoardFundFlowList, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}

	normalizedCategory := normalizeBoardCategory(category)
	fs := boardFundFlowFS(normalizedCategory)

	params := url.Values{}
	params.Set("np", "1")
	params.Set("fltt", "2")
	params.Set("invt", "2")
	params.Set("po", "1")
	params.Set("fid", "f62")
	params.Set("stat", "1")
	params.Set("fields", "f12,f14,f2,f3,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f124")
	params.Set("ut", "8dec03ba335b81bf4ebdf7b29ec27d15")
	params.Set("pn", strconv.Itoa(page))
	params.Set("pz", strconv.Itoa(size))
	params.Set("fs", fs)

	urlStr := emBoardFundFlowURL + "?" + params.Encode()
	raw, err := ms.fetchMarketJSON(urlStr, map[string]string{
		"Referer":    "https://data.eastmoney.com/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	})
	if err != nil {
		return models.BoardFundFlowList{}, err
	}

	data, ok := raw["data"].(map[string]any)
	if !ok || data == nil {
		return models.BoardFundFlowList{}, fmt.Errorf("板块资金流响应缺少data")
	}

	diffRows := toMapSlice(toSliceAny(data["diff"]))
	items := make([]models.BoardFundFlowItem, 0, len(diffRows))
	var updateTime string
	for _, row := range diffRows {
		item := models.BoardFundFlowItem{
			Code:                 strings.TrimSpace(toString(row["f12"])),
			Name:                 strings.TrimSpace(toString(row["f14"])),
			Price:                toFloat64Any(row["f2"]),
			ChangePercent:        toFloat64Any(row["f3"]),
			MainNetInflow:        toFloat64Any(row["f62"]),
			MainNetInflowRatio:   toFloat64Any(row["f184"]),
			SuperNetInflow:       toFloat64Any(row["f66"]),
			SuperNetInflowRatio:  toFloat64Any(row["f69"]),
			LargeNetInflow:       toFloat64Any(row["f72"]),
			LargeNetInflowRatio:  toFloat64Any(row["f75"]),
			MediumNetInflow:      toFloat64Any(row["f78"]),
			MediumNetInflowRatio: toFloat64Any(row["f81"]),
			SmallNetInflow:       toFloat64Any(row["f84"]),
			SmallNetInflowRatio:  toFloat64Any(row["f87"]),
		}
		if ts := toInt64Any(row["f124"]); ts > 0 {
			item.UpdateTime = formatEastmoneyTimestamp(ts)
			if updateTime == "" {
				updateTime = item.UpdateTime
			}
		}
		items = append(items, item)
	}

	return models.BoardFundFlowList{
		Category:   normalizedCategory,
		Items:      items,
		Total:      toInt64Any(data["total"]),
		UpdateTime: updateTime,
	}, nil
}

// GetStockMovesList 获取盘口异动列表（按涨速/涨跌幅/资金等维度排序）
func (ms *MarketService) GetStockMovesList(moveType string, page int, size int) (models.StockMoveList, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 30
	}
	if size > 200 {
		size = 200
	}

	normalizedMoveType := normalizeStockMoveType(moveType)
	fid, po := stockMoveSort(normalizedMoveType)

	params := url.Values{}
	params.Set("np", "1")
	params.Set("fltt", "2")
	params.Set("invt", "2")
	params.Set("fid", fid)
	params.Set("po", po)
	params.Set("pn", strconv.Itoa(page))
	params.Set("pz", strconv.Itoa(size))
	params.Set("fs", "m:0 t:6,m:0 t:80,m:1 t:2,m:1 t:23")
	params.Set("fields", "f12,f14,f2,f3,f22,f8,f5,f6,f62,f184,f15,f16,f17,f18,f124")
	params.Set("ut", "8dec03ba335b81bf4ebdf7b29ec27d15")

	urlStr := emBoardFundFlowURL + "?" + params.Encode()
	raw, err := ms.fetchMarketJSON(urlStr, map[string]string{
		"Referer":         "https://quote.eastmoney.com/",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Accept":          "*/*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Connection":      "keep-alive",
	})
	if err != nil {
		return models.StockMoveList{}, err
	}

	data, ok := raw["data"].(map[string]any)
	if !ok || data == nil {
		return models.StockMoveList{}, fmt.Errorf("盘口异动响应缺少data")
	}

	diffRows := toMapSlice(toSliceAny(data["diff"]))
	items := make([]models.StockMoveItem, 0, len(diffRows))
	var updateTime string
	for idx, row := range diffRows {
		item := models.StockMoveItem{
			Rank:               (page-1)*size + idx + 1,
			Code:               strings.TrimSpace(toString(row["f12"])),
			Name:               strings.TrimSpace(toString(row["f14"])),
			Price:              toFloat64Any(row["f2"]),
			ChangePercent:      toFloat64Any(row["f3"]),
			Speed:              toFloat64Any(row["f22"]),
			TurnoverRate:       toFloat64Any(row["f8"]),
			Volume:             toInt64Any(row["f5"]),
			Amount:             toFloat64Any(row["f6"]),
			MainNetInflow:      toFloat64Any(row["f62"]),
			MainNetInflowRatio: toFloat64Any(row["f184"]),
			High:               toFloat64Any(row["f15"]),
			Low:                toFloat64Any(row["f16"]),
			Open:               toFloat64Any(row["f17"]),
			PreClose:           toFloat64Any(row["f18"]),
		}
		if ts := toInt64Any(row["f124"]); ts > 0 {
			item.UpdateTime = formatEastmoneyTimestamp(ts)
			if updateTime == "" {
				updateTime = item.UpdateTime
			}
		}
		items = append(items, item)
	}

	return models.StockMoveList{
		MoveType:   normalizedMoveType,
		Items:      items,
		Total:      toInt64Any(data["total"]),
		UpdateTime: updateTime,
	}, nil
}

// GetBoardLeaders 获取板块龙头候选（基于涨幅+主力资金综合评分）
func (ms *MarketService) GetBoardLeaders(boardCode string, limit int) (models.BoardLeaderList, error) {
	normalizedBoard := normalizeBoardCode(boardCode)
	if normalizedBoard == "" {
		return models.BoardLeaderList{}, fmt.Errorf("无效板块代码: %s", boardCode)
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 30 {
		limit = 30
	}

	fetchSize := limit * 6
	if fetchSize < 30 {
		fetchSize = 30
	}
	if fetchSize > 200 {
		fetchSize = 200
	}

	params := url.Values{}
	params.Set("np", "1")
	params.Set("fltt", "2")
	params.Set("invt", "2")
	params.Set("po", "1")
	params.Set("fid", "f3")
	params.Set("fields", "f12,f14,f2,f3,f8,f62,f184,f124")
	params.Set("ut", "8dec03ba335b81bf4ebdf7b29ec27d15")
	params.Set("pn", "1")
	params.Set("pz", strconv.Itoa(fetchSize))
	params.Set("fs", "b:"+normalizedBoard)

	urlStr := emBoardFundFlowURL + "?" + params.Encode()
	raw, err := ms.fetchMarketJSON(urlStr, map[string]string{
		"Referer":    "https://data.eastmoney.com/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	})
	if err != nil {
		return models.BoardLeaderList{}, err
	}

	data, ok := raw["data"].(map[string]any)
	if !ok || data == nil {
		return models.BoardLeaderList{}, fmt.Errorf("板块龙头响应缺少data")
	}

	diffRows := toMapSlice(toSliceAny(data["diff"]))
	items := make([]models.BoardLeaderItem, 0, len(diffRows))
	var updateTime string
	for _, row := range diffRows {
		item := models.BoardLeaderItem{
			Code:               strings.TrimSpace(toString(row["f12"])),
			Name:               strings.TrimSpace(toString(row["f14"])),
			Price:              toFloat64Any(row["f2"]),
			ChangePercent:      toFloat64Any(row["f3"]),
			TurnoverRate:       toFloat64Any(row["f8"]),
			MainNetInflow:      toFloat64Any(row["f62"]),
			MainNetInflowRatio: toFloat64Any(row["f184"]),
		}
		item.Score = calculateBoardLeaderScore(item.ChangePercent, item.MainNetInflow, item.MainNetInflowRatio)
		if ts := toInt64Any(row["f124"]); ts > 0 {
			item.UpdateTime = formatEastmoneyTimestamp(ts)
			if updateTime == "" {
				updateTime = item.UpdateTime
			}
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			if items[i].ChangePercent == items[j].ChangePercent {
				return items[i].MainNetInflow > items[j].MainNetInflow
			}
			return items[i].ChangePercent > items[j].ChangePercent
		}
		return items[i].Score > items[j].Score
	})

	if len(items) > limit {
		items = items[:limit]
	}
	for i := range items {
		items[i].Rank = i + 1
	}

	return models.BoardLeaderList{
		BoardCode:  normalizedBoard,
		Items:      items,
		UpdateTime: updateTime,
	}, nil
}

// GetIndexFundFlowSeries 获取指数/板块资金流曲线
func (ms *MarketService) GetIndexFundFlowSeries(code string, interval string, limit int) (models.FundFlowKLineSeries, error) {
	if strings.TrimSpace(code) == "" {
		return models.FundFlowKLineSeries{}, fmt.Errorf("未提供指数代码")
	}
	if limit < 0 {
		limit = 0
	}

	klt := normalizeFundFlowInterval(interval)
	secID := indexSecID(code)

	params := url.Values{}
	params.Set("lmt", "0")
	params.Set("klt", klt)
	params.Set("fields1", "f1,f2,f3,f7")
	params.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65")
	params.Set("ut", "b2884a393a59ad64002292a3e90d46a5")
	params.Set("secid", secID)

	urlStr := emFundFlowKLineURL + "?" + params.Encode()
	raw, err := ms.fetchMarketJSON(urlStr, map[string]string{
		"Referer":    "https://quote.eastmoney.com/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	})
	if err != nil {
		return models.FundFlowKLineSeries{}, err
	}

	data, ok := raw["data"].(map[string]any)
	if !ok || data == nil {
		return models.FundFlowKLineSeries{}, fmt.Errorf("资金流曲线响应缺少data")
	}

	series := models.FundFlowKLineSeries{
		Code:   toString(data["code"]),
		Name:   toString(data["name"]),
		Market: int(toInt64Any(data["market"])),
	}
	series.TradePeriods = parseTradePeriods(data["tradePeriods"])

	lines := toStringSlice(data["klines"])
	points := make([]models.FundFlowKLine, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		points = append(points, models.FundFlowKLine{
			Time:            strings.TrimSpace(parts[0]),
			MainNetInflow:   parseFloat64Safe(parts[1]),
			SuperNetInflow:  parseFloat64Safe(parts[2]),
			LargeNetInflow:  parseFloat64Safe(parts[3]),
			MediumNetInflow: parseFloat64Safe(parts[4]),
			SmallNetInflow:  parseFloat64Safe(parts[5]),
		})
	}

	if limit > 0 && len(points) > limit {
		points = points[len(points)-limit:]
	}
	series.KLines = points
	return series, nil
}

// GetStockAnnouncements 获取个股公告摘要
func (ms *MarketService) GetStockAnnouncements(code string, page int, size int) (models.StockAnnouncements, error) {
	normalized := normalizeStockListCode(code)
	if normalized == "" {
		return models.StockAnnouncements{}, fmt.Errorf("未提供股票代码")
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	params := url.Values{}
	params.Set("ann_type", "A")
	params.Set("client_source", "web")
	params.Set("stock_list", normalized)
	params.Set("page_index", strconv.Itoa(page))
	params.Set("page_size", strconv.Itoa(size))

	urlStr := emAnnouncementURL + "?" + params.Encode()
	raw, err := ms.fetchMarketJSON(urlStr, map[string]string{
		"Referer":    "https://notice.eastmoney.com/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	})
	if err != nil {
		return models.StockAnnouncements{}, err
	}

	data, ok := raw["data"].(map[string]any)
	if !ok || data == nil {
		return models.StockAnnouncements{}, fmt.Errorf("公告响应缺少data")
	}

	rows := toMapSlice(toSliceAny(data["list"]))
	items := make([]models.StockAnnouncement, 0, len(rows))
	for _, row := range rows {
		items = append(items, models.StockAnnouncement{
			Title:      strings.TrimSpace(toString(row["title"])),
			NoticeDate: strings.TrimSpace(toString(row["notice_date"])),
			Type:       strings.TrimSpace(toString(row["ann_type"])),
			Columns:    strings.TrimSpace(toString(row["columns"])),
			ArtCode:    strings.TrimSpace(toString(row["art_code"])),
		})
	}

	return models.StockAnnouncements{
		Code:  normalized,
		Items: items,
		Total: toInt64Any(data["total"]),
	}, nil
}

func normalizeBoardCategory(category string) string {
	lower := strings.ToLower(strings.TrimSpace(category))
	switch lower {
	case "industry", "hy", "行业":
		return "industry"
	case "concept", "gn", "概念", "题材":
		return "concept"
	case "region", "dy", "地区", "地域":
		return "region"
	default:
		return "industry"
	}
}

func normalizeStockMoveType(moveType string) string {
	lower := strings.ToLower(strings.TrimSpace(moveType))
	switch lower {
	case "surge", "speed_up", "speedup", "rapid_up", "up":
		return "surge"
	case "drop", "speed_down", "speeddown", "rapid_down", "down":
		return "drop"
	case "change_up", "rise", "up_change":
		return "change_up"
	case "change_down", "fall", "down_change":
		return "change_down"
	case "mainflow", "fund", "capital":
		return "mainflow"
	case "turnover", "activity", "active":
		return "turnover"
	default:
		return "surge"
	}
}

func stockMoveSort(moveType string) (fid string, po string) {
	switch moveType {
	case "drop":
		return "f22", "0"
	case "change_up":
		return "f3", "1"
	case "change_down":
		return "f3", "0"
	case "mainflow":
		return "f62", "1"
	case "turnover":
		return "f8", "1"
	default:
		return "f22", "1"
	}
}

func normalizeBoardCode(boardCode string) string {
	candidate := strings.ToUpper(strings.TrimSpace(boardCode))
	candidate = strings.TrimPrefix(candidate, "B:")
	if strings.HasPrefix(candidate, "BI") && len(candidate) == 6 {
		candidate = "BK" + candidate[2:]
	}
	if strings.HasPrefix(candidate, "BK") && len(candidate) == 6 {
		return candidate
	}
	if len(candidate) == 4 && isDigits(candidate) {
		return "BK" + candidate
	}
	return ""
}

func isDigits(text string) bool {
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return text != ""
}

func calculateBoardLeaderScore(changePercent, mainNetInflow, mainNetInflowRatio float64) float64 {
	flowScore := 0.0
	if mainNetInflow != 0 {
		flowScore = math.Log10(math.Abs(mainNetInflow)/1e6 + 1)
		if mainNetInflow < 0 {
			flowScore = -flowScore
		}
	}
	score := changePercent*1.8 + mainNetInflowRatio*0.8 + flowScore*3.0
	return math.Round(score*100) / 100
}

func boardFundFlowFS(category string) string {
	switch category {
	case "concept":
		return "m:90 t:3"
	case "region":
		return "m:90 t:1"
	default:
		return "m:90 s:4"
	}
}

func normalizeFundFlowInterval(interval string) string {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "1", "1m", "1min", "min":
		return "1"
	case "5", "5m":
		return "5"
	case "15", "15m":
		return "15"
	case "30", "30m":
		return "30"
	case "60", "60m":
		return "60"
	case "101", "1d", "day":
		return "101"
	default:
		return "1"
	}
}

func normalizeStockListCode(code string) string {
	lower := normalizeKLineCode(code)
	if strings.HasPrefix(lower, "sh") || strings.HasPrefix(lower, "sz") || strings.HasPrefix(lower, "bj") {
		return strings.TrimPrefix(lower, lower[:2])
	}
	return lower
}

func formatEastmoneyTimestamp(ts int64) string {
	if ts <= 0 {
		return ""
	}
	text := strconv.FormatInt(ts, 10)
	cst := time.FixedZone("CST", 8*60*60)
	if len(text) == 10 {
		return time.Unix(ts, 0).In(cst).Format("2006-01-02 15:04:05")
	}
	if len(text) == 13 {
		return time.UnixMilli(ts).In(cst).Format("2006-01-02 15:04:05")
	}
	if len(text) == 12 {
		if t, err := time.Parse("200601021504", text); err == nil {
			return t.Format("2006-01-02 15:04")
		}
	}
	if len(text) == 14 {
		if t, err := time.Parse("20060102150405", text); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	if len(text) == 8 {
		if t, err := time.Parse("20060102", text); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return text
}

func parseTradePeriods(value any) models.TradePeriods {
	raw, ok := value.(map[string]any)
	if !ok || raw == nil {
		return models.TradePeriods{}
	}

	result := models.TradePeriods{
		Pre:   parseTradePeriod(raw["pre"]),
		After: parseTradePeriod(raw["after"]),
	}
	periodRows := toSliceAny(raw["periods"])
	for _, item := range periodRows {
		if period := parseTradePeriod(item); period != nil {
			result.Periods = append(result.Periods, *period)
		}
	}
	return result
}

func parseTradePeriod(value any) *models.TradePeriod {
	raw, ok := value.(map[string]any)
	if !ok || raw == nil {
		return nil
	}
	begin := toInt64Any(raw["b"])
	end := toInt64Any(raw["e"])
	if begin == 0 && end == 0 {
		return nil
	}
	return &models.TradePeriod{Begin: begin, End: end}
}

func toSliceAny(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []string:
		result := make([]any, 0, len(v))
		for _, item := range v {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func toStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

func parseFloat64Safe(value string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return f
}

func (ms *MarketService) fetchMarketJSON(urlStr string, headers map[string]string) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, err := ms.fetchMarketJSONOnce(urlStr, headers)
		if err == nil {
			return result, nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return nil, lastErr
}

func (ms *MarketService) fetchMarketJSONOnce(urlStr string, headers map[string]string) (map[string]any, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if isHTMLBodyLocal(body) {
		return nil, fmt.Errorf("上游返回HTML响应，可能被拦截或接口变更")
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}
