package services

import (
	"context"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var pusherLog = logger.New("pusher")

// 事件名称常量
const (
	EventStockUpdate         = "market:stock:update"
	EventOrderBookUpdate     = "market:orderbook:update"
	EventTelegraphUpdate     = "market:telegraph:update"
	EventMarketStatusUpdate  = "market:status:update"
	EventMarketIndicesUpdate = "market:indices:update"
	EventMarketSubscribe     = "market:subscribe"
	EventOrderBookSubscribe  = "market:orderbook:subscribe"
	EventKLineUpdate         = "market:kline:update"
	EventKLineSubscribe      = "market:kline:subscribe"
)

// safeCall 安全调用，捕获 panic 避免崩溃
func safeCall(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			pusherLog.Error("panic recovered: %v", r)
		}
	}()
	fn()
}

// KLineSubscription K线订阅信息
type KLineSubscription struct {
	Code   string // 股票代码
	Period string // K线周期: 1m, 1d, 1w, 1mo
}

// MarketDataPusher 市场数据推送服务
type MarketDataPusher struct {
	ctx           context.Context
	marketService *MarketService
	configService *ConfigService
	newsService   *NewsService

	// 订阅管理
	subscribedCodes  []string
	currentOrderBook string // 当前订阅盘口的股票代码
	mu               sync.RWMutex

	// K线订阅管理
	klineSub   KLineSubscription
	klineSubMu sync.RWMutex

	// 快讯缓存（用于检测新快讯）
	lastTelegraphContent string

	// 控制
	stopChan chan struct{}
	running  bool
}

// NewMarketDataPusher 创建市场数据推送服务
func NewMarketDataPusher(marketService *MarketService, configService *ConfigService, newsService *NewsService) *MarketDataPusher {
	return &MarketDataPusher{
		marketService:   marketService,
		configService:   configService,
		newsService:     newsService,
		subscribedCodes: make([]string, 0),
		stopChan:        make(chan struct{}),
	}
}

// Start 启动推送服务
func (p *MarketDataPusher) Start(ctx context.Context) {
	p.ctx = ctx
	p.running = true

	// 监听前端订阅请求
	p.setupEventListeners()

	// 初始化订阅列表（从自选股加载）
	p.initSubscriptions()

	// 启动数据推送 goroutine
	go p.pushLoop()
}

// Stop 停止推送服务
func (p *MarketDataPusher) Stop() {
	if p.running {
		close(p.stopChan)
		p.running = false
	}
}

// setupEventListeners 设置事件监听
func (p *MarketDataPusher) setupEventListeners() {
	// 监听订阅请求
	runtime.EventsOn(p.ctx, EventMarketSubscribe, func(data ...any) {
		if len(data) > 0 {
			if codes, ok := data[0].([]any); ok {
				p.updateSubscriptions(codes)
			}
		}
	})

	// 监听盘口订阅请求
	runtime.EventsOn(p.ctx, EventOrderBookSubscribe, func(data ...any) {
		if len(data) > 0 {
			if code, ok := data[0].(string); ok {
				p.mu.Lock()
				p.currentOrderBook = code
				p.mu.Unlock()
			}
		}
	})

	// 监听K线订阅请求
	runtime.EventsOn(p.ctx, EventKLineSubscribe, func(data ...any) {
		if len(data) >= 2 {
			code, _ := data[0].(string)
			period, _ := data[1].(string)
			if code != "" && period != "" {
				p.klineSubMu.Lock()
				p.klineSub = KLineSubscription{Code: code, Period: period}
				p.klineSubMu.Unlock()
				// 切换订阅后立即推送一次
				go safeCall(p.pushKLineData)
			}
		}
	})
}

// initSubscriptions 从自选股初始化订阅
func (p *MarketDataPusher) initSubscriptions() {
	watchlist := p.configService.GetWatchlist()
	codes := make([]string, len(watchlist))
	for i, stock := range watchlist {
		codes[i] = stock.Symbol
	}

	p.mu.Lock()
	p.subscribedCodes = codes
	// 默认订阅第一个股票的盘口
	if len(codes) > 0 {
		p.currentOrderBook = codes[0]
	}
	p.mu.Unlock()
}

// updateSubscriptions 更新订阅列表
func (p *MarketDataPusher) updateSubscriptions(codes []any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.subscribedCodes = make([]string, 0, len(codes))
	for _, code := range codes {
		if s, ok := code.(string); ok {
			p.subscribedCodes = append(p.subscribedCodes, s)
		}
	}
}

// pushLoop 数据推送循环
func (p *MarketDataPusher) pushLoop() {
	// 股票数据推送间隔：3秒
	stockTicker := time.NewTicker(3 * time.Second)
	// 盘口数据推送间隔：1秒
	orderBookTicker := time.NewTicker(1 * time.Second)
	// 快讯数据推送间隔：30秒
	telegraphTicker := time.NewTicker(30 * time.Second)
	// 市场状态推送间隔：60秒
	marketStatusTicker := time.NewTicker(5 * time.Second)
	// 大盘指数推送间隔：3秒
	marketIndicesTicker := time.NewTicker(3 * time.Second)
	// 分时K线推送间隔：3秒
	klineMinuteTicker := time.NewTicker(3 * time.Second)
	// 日/周/月K线推送间隔：5分钟
	klineDayTicker := time.NewTicker(5 * time.Minute)

	defer stockTicker.Stop()
	defer orderBookTicker.Stop()
	defer telegraphTicker.Stop()
	defer marketStatusTicker.Stop()
	defer marketIndicesTicker.Stop()
	defer klineMinuteTicker.Stop()
	defer klineDayTicker.Stop()

	// 立即推送一次
	safeCall(p.pushStockData)
	safeCall(p.pushOrderBookData)
	safeCall(p.pushTelegraphData)
	safeCall(p.pushMarketStatus)
	safeCall(p.pushMarketIndices)
	safeCall(p.pushKLineData)

	for {
		select {
		case <-p.stopChan:
			return
		case <-stockTicker.C:
			safeCall(p.pushStockData)
		case <-orderBookTicker.C:
			safeCall(p.pushOrderBookData)
		case <-telegraphTicker.C:
			safeCall(p.pushTelegraphData)
		case <-marketStatusTicker.C:
			safeCall(p.pushMarketStatus)
		case <-marketIndicesTicker.C:
			safeCall(p.pushMarketIndices)
		case <-klineMinuteTicker.C:
			safeCall(p.pushKLineMinute)
		case <-klineDayTicker.C:
			safeCall(p.pushKLineDay)
		}
	}
}

// pushStockData 推送股票实时数据
func (p *MarketDataPusher) pushStockData() {
	p.mu.RLock()
	codes := make([]string, len(p.subscribedCodes))
	copy(codes, p.subscribedCodes)
	p.mu.RUnlock()

	if len(codes) == 0 {
		return
	}

	stocks, err := p.marketService.GetStockRealTimeData(codes...)
	if err != nil {
		return
	}

	// 推送到前端
	runtime.EventsEmit(p.ctx, EventStockUpdate, stocks)
}

// pushOrderBookData 推送盘口数据
func (p *MarketDataPusher) pushOrderBookData() {
	p.mu.RLock()
	code := p.currentOrderBook
	p.mu.RUnlock()

	if code == "" {
		return
	}

	// 获取当前选中股票的真实盘口数据
	orderBook, err := p.marketService.GetRealOrderBook(code)
	if err != nil {
		return
	}

	// 推送到前端
	runtime.EventsEmit(p.ctx, EventOrderBookUpdate, orderBook)
}

// pushTelegraphData 推送快讯数据
func (p *MarketDataPusher) pushTelegraphData() {
	if p.newsService == nil {
		return
	}

	telegraphs, err := p.newsService.GetTelegraphList()
	if err != nil || len(telegraphs) == 0 {
		return
	}

	// 获取最新一条快讯
	latest := telegraphs[0]

	// 检查是否有新快讯（避免重复推送）
	p.mu.Lock()
	if latest.Content == p.lastTelegraphContent {
		p.mu.Unlock()
		return
	}
	p.lastTelegraphContent = latest.Content
	p.mu.Unlock()

	// 推送到前端
	runtime.EventsEmit(p.ctx, EventTelegraphUpdate, latest)
}

// pushMarketStatus 推送市场状态
func (p *MarketDataPusher) pushMarketStatus() {
	status := p.marketService.GetMarketStatus()
	runtime.EventsEmit(p.ctx, EventMarketStatusUpdate, status)
}

// pushMarketIndices 推送大盘指数
func (p *MarketDataPusher) pushMarketIndices() {
	indices, err := p.marketService.GetMarketIndices()
	if err != nil {
		return
	}
	runtime.EventsEmit(p.ctx, EventMarketIndicesUpdate, indices)
}

// pushKLineData 推送K线数据（初始化时调用）
func (p *MarketDataPusher) pushKLineData() {
	p.klineSubMu.RLock()
	sub := p.klineSub
	p.klineSubMu.RUnlock()

	if sub.Code == "" {
		return
	}

	klines, err := p.marketService.GetKLineData(sub.Code, sub.Period, 240)
	if err != nil {
		return
	}

	runtime.EventsEmit(p.ctx, EventKLineUpdate, map[string]any{
		"code":   sub.Code,
		"period": sub.Period,
		"data":   klines,
	})
}

// pushKLineMinute 推送分时K线（3秒间隔，仅当订阅周期为1m时推送）
func (p *MarketDataPusher) pushKLineMinute() {
	p.klineSubMu.RLock()
	sub := p.klineSub
	p.klineSubMu.RUnlock()

	if sub.Code == "" || sub.Period != "1m" {
		return
	}

	klines, err := p.marketService.GetKLineData(sub.Code, "1m", 240)
	if err != nil {
		return
	}

	runtime.EventsEmit(p.ctx, EventKLineUpdate, map[string]any{
		"code":   sub.Code,
		"period": "1m",
		"data":   klines,
	})
}

// pushKLineDay 推送日/周/月K线（5分钟间隔，仅当订阅周期非1m时推送）
func (p *MarketDataPusher) pushKLineDay() {
	p.klineSubMu.RLock()
	sub := p.klineSub
	p.klineSubMu.RUnlock()

	// 仅推送日K/周K/月K
	if sub.Code == "" || sub.Period == "1m" {
		return
	}

	klines, err := p.marketService.GetKLineData(sub.Code, sub.Period, 120)
	if err != nil {
		return
	}

	runtime.EventsEmit(p.ctx, EventKLineUpdate, map[string]any{
		"code":   sub.Code,
		"period": sub.Period,
		"data":   klines,
	})
}

// AddSubscription 添加订阅
func (p *MarketDataPusher) AddSubscription(code string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查是否已存在
	for _, c := range p.subscribedCodes {
		if c == code {
			return
		}
	}
	p.subscribedCodes = append(p.subscribedCodes, code)
}

// RemoveSubscription 移除订阅
func (p *MarketDataPusher) RemoveSubscription(code string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, c := range p.subscribedCodes {
		if c == code {
			p.subscribedCodes = append(p.subscribedCodes[:i], p.subscribedCodes[i+1:]...)
			return
		}
	}
}

// GetSubscribedStocks 获取当前订阅的股票数据
func (p *MarketDataPusher) GetSubscribedStocks() []models.Stock {
	p.mu.RLock()
	codes := make([]string, len(p.subscribedCodes))
	copy(codes, p.subscribedCodes)
	p.mu.RUnlock()

	if len(codes) == 0 {
		return []models.Stock{}
	}

	stocks, _ := p.marketService.GetStockRealTimeData(codes...)
	return stocks
}
