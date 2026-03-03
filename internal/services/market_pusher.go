package services

import (
	"context"
	"fmt"
	"slices"
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
	EventMarketIndicesUpdate = "market:indices:update"
	EventMarketSubscribe     = "market:subscribe"
	EventOrderBookSubscribe  = "market:orderbook:subscribe"
	EventKLineUpdate         = "market:kline:update"
	EventKLineSubscribe      = "market:kline:subscribe"
)

// 推送频率常量
const (
	tickerFast     = 1 * time.Second  // 盘口（交易时段）
	tickerNormal   = 3 * time.Second  // 股票、指数、分时K线
	tickerSlow     = 30 * time.Second // 快讯、非交易时段降频
	tickerKLineDay = 5 * time.Minute  // 日/周/月K线
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
	klineSub      KLineSubscription
	klineSubMu    sync.RWMutex
	lastKLineTime int64 // 最后一根K线的时间戳，用于增量推送

	// 快讯缓存（用于检测新快讯）
	lastTelegraphContent string

	// 盘口缓存（用于diff检测）
	lastOrderBookHash string

	// 控制
	stopChan  chan struct{}
	stopped   bool
	ctrlMu    sync.Mutex
	ready     bool          // 前端是否已准备好
	readyChan chan struct{} // 前端准备好信号

	// 防止 runParallel 重入堆积
	pushMu sync.Mutex
}

// NewMarketDataPusher 创建市场数据推送服务
func NewMarketDataPusher(marketService *MarketService, configService *ConfigService, newsService *NewsService) *MarketDataPusher {
	return &MarketDataPusher{
		marketService:   marketService,
		configService:   configService,
		newsService:     newsService,
		subscribedCodes: make([]string, 0),
		stopChan:        make(chan struct{}),
		readyChan:       make(chan struct{}),
	}
}

// Start 启动推送服务
func (p *MarketDataPusher) Start(ctx context.Context) {
	p.ctrlMu.Lock()
	if p.stopped {
		p.ctrlMu.Unlock()
		return
	}
	p.ctx = ctx
	p.ctrlMu.Unlock()

	p.setupEventListeners()
	p.initSubscriptions()
	go p.pushLoop()
}

// SetReady 设置前端已准备好，开始推送数据
func (p *MarketDataPusher) SetReady() {
	p.ctrlMu.Lock()
	defer p.ctrlMu.Unlock()
	if p.ready {
		return
	}
	p.ready = true
	close(p.readyChan)
	pusherLog.Info("前端已就绪，开始推送数据")
}

// Stop 停止推送服务
func (p *MarketDataPusher) Stop() {
	p.ctrlMu.Lock()
	defer p.ctrlMu.Unlock()
	if p.stopped {
		return
	}
	p.stopped = true
	close(p.stopChan)
	// 清理事件监听
	runtime.EventsOff(p.ctx, EventMarketSubscribe)
	runtime.EventsOff(p.ctx, EventOrderBookSubscribe)
	runtime.EventsOff(p.ctx, EventKLineSubscribe)
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
				p.lastKLineTime = 0 // 重置增量时间戳
				p.klineSubMu.Unlock()
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

// pushLoop 数据推送循环（并行推送 + 超时控制 + 时段感知）
func (p *MarketDataPusher) pushLoop() {
	// 等待前端准备好
	select {
	case <-p.readyChan:
		pusherLog.Info("收到前端就绪信号，启动推送循环")
	case <-p.stopChan:
		return
	}

	fastTicker := time.NewTicker(tickerFast)
	normalTicker := time.NewTicker(tickerNormal)
	slowTicker := time.NewTicker(tickerSlow)
	klineDayTicker := time.NewTicker(tickerKLineDay)

	defer fastTicker.Stop()
	defer normalTicker.Stop()
	defer slowTicker.Stop()
	defer klineDayTicker.Stop()

	// 立即并行推送一次（启动时5个并发请求，冷启动给足时间）
	p.runParallel(15*time.Second, p.pushStockData, p.pushOrderBookData,
		p.pushTelegraphData, p.pushMarketIndices, p.pushKLineData)

	var normalCount int

	for {
		select {
		case <-p.stopChan:
			return
		case <-fastTicker.C:
			status := p.getMarketPhase()
			// 仅交易时段高频推送盘口
			if status == "trading" {
				p.runParallel(2*time.Second, p.pushOrderBookData)
			}
		case <-normalTicker.C:
			normalCount++
			status := p.getMarketPhase()

			switch status {
			case "trading":
				// 交易时段：正常频率
				p.runParallel(8*time.Second, p.pushStockData, p.pushMarketIndices, p.pushKLineMinute)
			case "pre_market":
				// 集合竞价：推送盘口（虚拟撮合价）和股票，降频
				if normalCount%3 == 0 {
					p.runParallel(8*time.Second, p.pushStockData, p.pushOrderBookData, p.pushMarketIndices)
				}
			case "lunch_break":
				// 午休：低频推送
				if normalCount%5 == 0 {
					p.runParallel(8*time.Second, p.pushStockData, p.pushMarketIndices)
				}
			default:
				// 收盘：30秒一次
				if normalCount%10 == 0 {
					p.runParallel(8*time.Second, p.pushStockData, p.pushMarketIndices,
						p.pushOrderBookData, p.pushKLineData)
				}
			}
		case <-slowTicker.C:
			p.runParallel(8*time.Second, p.pushTelegraphData)
		case <-klineDayTicker.C:
			if p.getMarketPhase() == "trading" {
				p.runParallel(8*time.Second, p.pushKLineDay)
			}
		}
	}
}

// runParallel 带超时的并行执行，防止协程堆积
// 使用 TryLock 防止重入：上一轮未完成则跳过本轮
func (p *MarketDataPusher) runParallel(timeout time.Duration, fns ...func()) {
	if !p.pushMu.TryLock() {
		// 上一轮推送还未完成，跳过本轮避免 goroutine 堆积
		return
	}
	var unlockOnce sync.Once
	unlock := func() {
		unlockOnce.Do(func() {
			p.pushMu.Unlock()
		})
	}

	var wg sync.WaitGroup
	wg.Add(len(fns))
	for _, fn := range fns {
		go func(f func()) {
			defer wg.Done()
			safeCall(f)
		}(fn)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		unlock()
	case <-time.After(timeout):
		pusherLog.Warn("推送超时，后台等待当前轮次结束后再释放锁")
		// 超时后不阻塞调用方，但保持锁直到本轮任务结束，避免重入
		go func() {
			<-done
			unlock()
		}()
	}
}

// getMarketPhase 获取市场时段
func (p *MarketDataPusher) getMarketPhase() string {
	return p.marketService.GetMarketStatus().Status
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

// pushOrderBookData 推送盘口数据（带diff检测）
func (p *MarketDataPusher) pushOrderBookData() {
	p.mu.RLock()
	code := p.currentOrderBook
	lastHash := p.lastOrderBookHash
	p.mu.RUnlock()

	if code == "" {
		return
	}

	orderBook, err := p.marketService.GetRealOrderBook(code)
	if err != nil {
		return
	}

	// 简单hash：买一卖一价格+数量
	hash := orderBookHash(orderBook)
	if hash == lastHash {
		return // 无变化，跳过推送
	}

	p.mu.Lock()
	p.lastOrderBookHash = hash
	p.mu.Unlock()

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

// pushKLineMinute 推送分时K线（增量模式，仅推送最新1根）
func (p *MarketDataPusher) pushKLineMinute() {
	p.klineSubMu.RLock()
	sub := p.klineSub
	lastTime := p.lastKLineTime
	p.klineSubMu.RUnlock()

	if sub.Code == "" || sub.Period != "1m" {
		return
	}

	// 只获取最新几根用于增量判断
	klines, err := p.marketService.GetKLineData(sub.Code, "1m", 5)
	if err != nil || len(klines) == 0 {
		return
	}

	latest := klines[len(klines)-1]
	latestTime := parseKLineTime(latest.Time)

	// 推送最新一根（增量）
	p.klineSubMu.Lock()
	p.lastKLineTime = latestTime
	p.klineSubMu.Unlock()

	// 首次或时间变化才推送
	if lastTime == 0 || latestTime != lastTime {
		runtime.EventsEmit(p.ctx, EventKLineUpdate, map[string]any{
			"code":        sub.Code,
			"period":      "1m",
			"data":        []models.KLineData{latest},
			"incremental": true,
		})
	}
}

// parseKLineTime 解析K线时间为时间戳
func parseKLineTime(t string) int64 {
	if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
		return parsed.Unix()
	}
	return 0
}

// orderBookHash 生成盘口简单hash（买一卖一）
func orderBookHash(ob models.OrderBook) string {
	var b1Price, b1Size, a1Price, a1Size float64
	if len(ob.Bids) > 0 {
		b1Price, b1Size = ob.Bids[0].Price, float64(ob.Bids[0].Size)
	}
	if len(ob.Asks) > 0 {
		a1Price, a1Size = ob.Asks[0].Price, float64(ob.Asks[0].Size)
	}
	return fmt.Sprintf("%.2f:%.0f:%.2f:%.0f", b1Price, b1Size, a1Price, a1Size)
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

	if !slices.Contains(p.subscribedCodes, code) {
		p.subscribedCodes = append(p.subscribedCodes, code)
	}
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
