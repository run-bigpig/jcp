package models

// Stock 股票基本信息
type Stock struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"changePercent"`
	Volume        int64   `json:"volume"`
	Amount        float64 `json:"amount"`
	MarketCap     string  `json:"marketCap"`
	Sector        string  `json:"sector"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	PreClose      float64 `json:"preClose"`
}

// KLineData K线数据
type KLineData struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
	Amount float64 `json:"amount,omitempty"`
	Avg    float64 `json:"avg,omitempty"` // 分时均价线
	// 均线数据
	MA5  float64 `json:"ma5,omitempty"`
	MA10 float64 `json:"ma10,omitempty"`
	MA20 float64 `json:"ma20,omitempty"`
}

// OrderBookItem 盘口单项
type OrderBookItem struct {
	Price   float64 `json:"price"`
	Size    int64   `json:"size"`
	Total   int64   `json:"total"`
	Percent float64 `json:"percent"`
}

// OrderBook 盘口数据
type OrderBook struct {
	Bids []OrderBookItem `json:"bids"`
	Asks []OrderBookItem `json:"asks"`
}

// MarketIndex 大盘指数数据
type MarketIndex struct {
	Code          string  `json:"code"`          // 指数代码，如 sh000001
	Name          string  `json:"name"`          // 指数名称，如 上证指数
	Price         float64 `json:"price"`         // 当前点位
	Change        float64 `json:"change"`        // 涨跌点数
	ChangePercent float64 `json:"changePercent"` // 涨跌幅(%)
	Volume        int64   `json:"volume"`        // 成交量(手)
	Amount        float64 `json:"amount"`        // 成交额(万元)
}

// LongHuBangItem 龙虎榜单条数据
type LongHuBangItem struct {
	TradeDate     string  `json:"tradeDate"`     // 交易日期
	Code          string  `json:"code"`          // 股票代码
	SecuCode      string  `json:"secuCode"`      // 证券代码(含市场后缀，如000001.SZ)
	Name          string  `json:"name"`          // 股票名称
	ClosePrice    float64 `json:"closePrice"`    // 收盘价
	ChangePercent float64 `json:"changePercent"` // 涨跌幅(%)
	NetBuyAmt     float64 `json:"netBuyAmt"`     // 龙虎榜净买额(元)
	BuyAmt        float64 `json:"buyAmt"`        // 龙虎榜买入额(元)
	SellAmt       float64 `json:"sellAmt"`       // 龙虎榜卖出额(元)
	TotalAmt      float64 `json:"totalAmt"`      // 龙虎榜成交额(元)
	TurnoverRate  float64 `json:"turnoverRate"`  // 换手率(%)
	FreeCap       float64 `json:"freeCap"`       // 流通市值(元)
	Reason        string  `json:"reason"`        // 上榜原因
	ReasonDetail  string  `json:"reasonDetail"`  // 上榜原因详情
	AccumAmount   float64 `json:"accumAmount"`   // 当日总成交额(元)
	DealRatio     float64 `json:"dealRatio"`     // 龙虎榜成交占比(%)
	NetRatio      float64 `json:"netRatio"`      // 龙虎榜净买占比(%)
	D1Change      float64 `json:"d1Change"`      // 次日涨跌幅(%)
	D2Change      float64 `json:"d2Change"`      // 次2日涨跌幅(%)
	D5Change      float64 `json:"d5Change"`      // 5日涨跌幅(%)
	D10Change     float64 `json:"d10Change"`     // 10日涨跌幅(%)
	SecurityType  string  `json:"securityType"`  // 证券类型代码
}

// LongHuBangDetail 龙虎榜营业部明细
type LongHuBangDetail struct {
	Rank        int     `json:"rank"`        // 排名
	OperName    string  `json:"operName"`    // 营业部名称
	BuyAmt      float64 `json:"buyAmt"`      // 买入金额(元)
	BuyPercent  float64 `json:"buyPercent"`  // 买入占总成交比(%)
	SellAmt     float64 `json:"sellAmt"`     // 卖出金额(元)
	SellPercent float64 `json:"sellPercent"` // 卖出占总成交比(%)
	NetAmt      float64 `json:"netAmt"`      // 净买入(元)
	Direction   string  `json:"direction"`   // 方向: buy/sell
}
