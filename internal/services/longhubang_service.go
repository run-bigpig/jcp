package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// 东方财富龙虎榜API
const (
	// 龙虎榜列表（按日期降序，再按净买入降序）
	// 基础URL，日期筛选通过filter参数动态添加
	lhbListBaseURL = "https://datacenter-web.eastmoney.com/api/data/v1/get?sortColumns=TRADE_DATE,BILLBOARD_NET_AMT&sortTypes=-1,-1&pageSize=%d&pageNumber=%d&reportName=RPT_DAILYBILLBOARD_DETAILSNEW&columns=SECURITY_CODE,SECUCODE,SECURITY_NAME_ABBR,TRADE_DATE,EXPLAIN,CLOSE_PRICE,CHANGE_RATE,BILLBOARD_NET_AMT,BILLBOARD_BUY_AMT,BILLBOARD_SELL_AMT,BILLBOARD_DEAL_AMT,ACCUM_AMOUNT,DEAL_NET_RATIO,DEAL_AMOUNT_RATIO,TURNOVERRATE,FREE_MARKET_CAP,EXPLANATION,D1_CLOSE_ADJCHRATE,D2_CLOSE_ADJCHRATE,D5_CLOSE_ADJCHRATE,D10_CLOSE_ADJCHRATE,SECURITY_TYPE_CODE&source=WEB&client=WEB"
	// 营业部买入明细
	lhbBuyDetailURL = "https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_BILLBOARD_DAILYDETAILSBUY&columns=ALL&filter=(TRADE_DATE%%3D%%27%s%%27)(SECURITY_CODE%%3D%%22%s%%22)&pageNumber=1&pageSize=50&sortTypes=-1&sortColumns=BUY&source=WEB&client=WEB"
	// 营业部卖出明细
	lhbSellDetailURL = "https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_BILLBOARD_DAILYDETAILSSELL&columns=ALL&filter=(TRADE_DATE%%3D%%27%s%%27)(SECURITY_CODE%%3D%%22%s%%22)&pageNumber=1&pageSize=50&sortTypes=-1&sortColumns=SELL&source=WEB&client=WEB"
)

// lhbCache 龙虎榜缓存
type lhbCache struct {
	key       string
	data      []models.LongHuBangItem
	total     int
	timestamp time.Time
}

// LongHuBangListResult 龙虎榜列表结果
type LongHuBangListResult struct {
	Items []models.LongHuBangItem `json:"items"`
	Total int                     `json:"total"` // 总记录数
}

// LongHuBangService 龙虎榜服务
type LongHuBangService struct {
	client   *http.Client
	cache    *lhbCache
	cacheMu  sync.RWMutex
	cacheTTL time.Duration
}

// NewLongHuBangService 创建龙虎榜服务
func NewLongHuBangService() *LongHuBangService {
	return &LongHuBangService{
		client:   proxy.GetManager().GetClientWithTimeout(15 * time.Second),
		cacheTTL: 5 * time.Minute, // 缓存5分钟
	}
}

// GetLongHuBangList 获取龙虎榜列表
// tradeDate: 交易日期，格式 YYYY-MM-DD，为空则获取所有日期
func (s *LongHuBangService) GetLongHuBangList(pageSize, pageNumber int, tradeDate string) (*LongHuBangListResult, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if pageNumber <= 0 {
		pageNumber = 1
	}

	// 生成缓存key
	cacheKey := fmt.Sprintf("%d_%d_%s", pageSize, pageNumber, tradeDate)

	// 检查缓存
	s.cacheMu.RLock()
	if s.cache != nil && s.cache.key == cacheKey && time.Since(s.cache.timestamp) < s.cacheTTL {
		result := &LongHuBangListResult{
			Items: s.cache.data,
			Total: s.cache.total,
		}
		s.cacheMu.RUnlock()
		return result, nil
	}
	s.cacheMu.RUnlock()

	// 从API获取数据
	result, err := s.fetchLongHuBangList(pageSize, pageNumber, tradeDate)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	s.cacheMu.Lock()
	s.cache = &lhbCache{
		key:       cacheKey,
		data:      result.Items,
		total:     result.Total,
		timestamp: time.Now(),
	}
	s.cacheMu.Unlock()

	return result, nil
}

// fetchLongHuBangList 从东方财富API获取龙虎榜数据
func (s *LongHuBangService) fetchLongHuBangList(pageSize, pageNumber int, tradeDate string) (*LongHuBangListResult, error) {
	url := fmt.Sprintf(lhbListBaseURL, pageSize, pageNumber)

	// 添加日期筛选
	if tradeDate != "" {
		url += fmt.Sprintf("&filter=(TRADE_DATE%%3D%%27%s%%27)", tradeDate)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return s.parseLongHuBangResponse(body)
}

// 东方财富API响应结构
type lhbAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
	Result  struct {
		Data  []lhbAPIItem `json:"data"`
		Count int          `json:"count"` // 总记录数
		Pages int          `json:"pages"` // 总页数
	} `json:"result"`
}

type lhbAPIItem struct {
	TradeDate         string  `json:"TRADE_DATE"`
	SecurityCode      string  `json:"SECURITY_CODE"`
	SecuCode          string  `json:"SECUCODE"`
	SecurityNameAbbr  string  `json:"SECURITY_NAME_ABBR"`
	ClosePrice        float64 `json:"CLOSE_PRICE"`
	ChangeRate        float64 `json:"CHANGE_RATE"`
	BillboardBuyAmt   float64 `json:"BILLBOARD_BUY_AMT"`
	BillboardSellAmt  float64 `json:"BILLBOARD_SELL_AMT"`
	BillboardDealAmt  float64 `json:"BILLBOARD_DEAL_AMT"`
	BillboardNetAmt   float64 `json:"BILLBOARD_NET_AMT"`
	TurnoverRate      float64 `json:"TURNOVERRATE"`
	FreeMarketCap     float64 `json:"FREE_MARKET_CAP"`
	Explain           string  `json:"EXPLAIN"`
	Explanation       string  `json:"EXPLANATION"`
	AccumAmount       float64 `json:"ACCUM_AMOUNT"`
	DealAmountRatio   float64 `json:"DEAL_AMOUNT_RATIO"`
	DealNetRatio      float64 `json:"DEAL_NET_RATIO"`
	D1CloseAdjChRate  float64 `json:"D1_CLOSE_ADJCHRATE"`
	D2CloseAdjChRate  float64 `json:"D2_CLOSE_ADJCHRATE"`
	D5CloseAdjChRate  float64 `json:"D5_CLOSE_ADJCHRATE"`
	D10CloseAdjChRate float64 `json:"D10_CLOSE_ADJCHRATE"`
	SecurityTypeCode  string  `json:"SECURITY_TYPE_CODE"`
}

// parseLongHuBangResponse 解析龙虎榜API响应
func (s *LongHuBangService) parseLongHuBangResponse(body []byte) (*LongHuBangListResult, error) {
	var resp lhbAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析龙虎榜数据失败: %w", err)
	}

	if !resp.Success || resp.Result.Data == nil {
		return nil, fmt.Errorf("获取龙虎榜数据失败: %s", resp.Message)
	}

	items := make([]models.LongHuBangItem, 0, len(resp.Result.Data))
	for _, item := range resp.Result.Data {
		// 解析日期，格式 "2026-02-09 00:00:00" -> "2026-02-09"
		tradeDate := item.TradeDate
		if len(tradeDate) > 10 {
			tradeDate = tradeDate[:10]
		}

		items = append(items, models.LongHuBangItem{
			TradeDate:     tradeDate,
			Code:          item.SecurityCode,
			SecuCode:      item.SecuCode,
			Name:          item.SecurityNameAbbr,
			ClosePrice:    item.ClosePrice,
			ChangePercent: item.ChangeRate,
			NetBuyAmt:     item.BillboardNetAmt,
			BuyAmt:        item.BillboardBuyAmt,
			SellAmt:       item.BillboardSellAmt,
			TotalAmt:      item.BillboardDealAmt,
			TurnoverRate:  item.TurnoverRate,
			FreeCap:       item.FreeMarketCap,
			Reason:        item.Explain,
			ReasonDetail:  item.Explanation,
			AccumAmount:   item.AccumAmount,
			DealRatio:     item.DealAmountRatio,
			NetRatio:      item.DealNetRatio,
			D1Change:      item.D1CloseAdjChRate,
			D2Change:      item.D2CloseAdjChRate,
			D5Change:      item.D5CloseAdjChRate,
			D10Change:     item.D10CloseAdjChRate,
			SecurityType:  item.SecurityTypeCode,
		})
	}

	return &LongHuBangListResult{
		Items: items,
		Total: resp.Result.Count,
	}, nil
}

// 营业部明细API响应结构
type lhbDetailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
	Result  struct {
		Data []lhbDetailItem `json:"data"`
	} `json:"result"`
}

type lhbDetailItem struct {
	OperateName string  `json:"OPERATEDEPT_NAME"`
	Buy         float64 `json:"BUY"`
	Sell        float64 `json:"SELL"`
	Net         float64 `json:"NET"`
	BuyRatio    float64 `json:"TOTAL_BUYRIO"`
	SellRatio   float64 `json:"TOTAL_SELLRIO"`
	Rank        int     `json:"RANK"`
}

// GetStockDetail 获取个股龙虎榜营业部明细
func (s *LongHuBangService) GetStockDetail(code, tradeDate string) ([]models.LongHuBangDetail, error) {
	buyDetails, err := s.fetchDetail(code, tradeDate, "buy")
	if err != nil {
		return nil, err
	}

	sellDetails, err := s.fetchDetail(code, tradeDate, "sell")
	if err != nil {
		return nil, err
	}

	// 合并买卖明细
	result := append(buyDetails, sellDetails...)
	return result, nil
}

// fetchDetail 获取营业部明细
func (s *LongHuBangService) fetchDetail(code, tradeDate, direction string) ([]models.LongHuBangDetail, error) {
	var url string
	if direction == "buy" {
		url = fmt.Sprintf(lhbBuyDetailURL, tradeDate, code)
	} else {
		url = fmt.Sprintf(lhbSellDetailURL, tradeDate, code)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return s.parseDetailResponse(body, direction)
}

// parseDetailResponse 解析营业部明细响应
func (s *LongHuBangService) parseDetailResponse(body []byte, direction string) ([]models.LongHuBangDetail, error) {
	var resp lhbDetailResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析营业部明细失败: %w", err)
	}

	// 无数据时返回空列表
	if !resp.Success || resp.Result.Data == nil {
		return []models.LongHuBangDetail{}, nil
	}

	items := make([]models.LongHuBangDetail, 0, len(resp.Result.Data))
	for i, item := range resp.Result.Data {
		items = append(items, models.LongHuBangDetail{
			Rank:        i + 1,
			OperName:    item.OperateName,
			BuyAmt:      item.Buy,
			BuyPercent:  item.BuyRatio,
			SellAmt:     item.Sell,
			SellPercent: item.SellRatio,
			NetAmt:      item.Net,
			Direction:   direction,
		})
	}

	return items, nil
}
