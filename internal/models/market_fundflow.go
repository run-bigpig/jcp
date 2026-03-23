package models

// BoardFundFlowItem 板块资金流条目
type BoardFundFlowItem struct {
	Code                 string  `json:"code"`
	Name                 string  `json:"name"`
	Price                float64 `json:"price"`
	ChangePercent        float64 `json:"changePercent"`
	MainNetInflow        float64 `json:"mainNetInflow"`
	MainNetInflowRatio   float64 `json:"mainNetInflowRatio"`
	SuperNetInflow       float64 `json:"superNetInflow"`
	SuperNetInflowRatio  float64 `json:"superNetInflowRatio"`
	LargeNetInflow       float64 `json:"largeNetInflow"`
	LargeNetInflowRatio  float64 `json:"largeNetInflowRatio"`
	MediumNetInflow      float64 `json:"mediumNetInflow"`
	MediumNetInflowRatio float64 `json:"mediumNetInflowRatio"`
	SmallNetInflow       float64 `json:"smallNetInflow"`
	SmallNetInflowRatio  float64 `json:"smallNetInflowRatio"`
	UpdateTime           string  `json:"updateTime,omitempty"`
}

// BoardFundFlowList 板块资金流列表
type BoardFundFlowList struct {
	Category   string              `json:"category"`
	Items      []BoardFundFlowItem `json:"items"`
	Total      int64               `json:"total,omitempty"`
	UpdateTime string              `json:"updateTime,omitempty"`
}

// BoardLeaderItem 板块龙头候选
type BoardLeaderItem struct {
	Rank               int     `json:"rank"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Price              float64 `json:"price"`
	ChangePercent      float64 `json:"changePercent"`
	TurnoverRate       float64 `json:"turnoverRate,omitempty"`
	MainNetInflow      float64 `json:"mainNetInflow"`
	MainNetInflowRatio float64 `json:"mainNetInflowRatio"`
	Score              float64 `json:"score"`
	UpdateTime         string  `json:"updateTime,omitempty"`
}

// BoardLeaderList 板块龙头结果
type BoardLeaderList struct {
	BoardCode  string            `json:"boardCode"`
	Items      []BoardLeaderItem `json:"items"`
	UpdateTime string            `json:"updateTime,omitempty"`
}

// StockMoveItem 盘口异动候选
type StockMoveItem struct {
	Rank               int     `json:"rank"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Price              float64 `json:"price"`
	ChangePercent      float64 `json:"changePercent"`
	Speed              float64 `json:"speed"`
	TurnoverRate       float64 `json:"turnoverRate"`
	Volume             int64   `json:"volume"`
	Amount             float64 `json:"amount"`
	MainNetInflow      float64 `json:"mainNetInflow"`
	MainNetInflowRatio float64 `json:"mainNetInflowRatio"`
	High               float64 `json:"high"`
	Low                float64 `json:"low"`
	Open               float64 `json:"open"`
	PreClose           float64 `json:"preClose"`
	UpdateTime         string  `json:"updateTime,omitempty"`
}

// StockMoveList 盘口异动结果
type StockMoveList struct {
	MoveType   string          `json:"moveType"`
	Items      []StockMoveItem `json:"items"`
	Total      int64           `json:"total,omitempty"`
	UpdateTime string          `json:"updateTime,omitempty"`
}

// FundFlowKLine 资金流曲线单点
type FundFlowKLine struct {
	Time            string  `json:"time"`
	MainNetInflow   float64 `json:"mainNetInflow"`
	SuperNetInflow  float64 `json:"superNetInflow"`
	LargeNetInflow  float64 `json:"largeNetInflow"`
	MediumNetInflow float64 `json:"mediumNetInflow"`
	SmallNetInflow  float64 `json:"smallNetInflow"`
}

// TradePeriod 单段交易时间
type TradePeriod struct {
	Begin int64 `json:"begin"`
	End   int64 `json:"end"`
}

// TradePeriods 交易时段
type TradePeriods struct {
	Pre     *TradePeriod  `json:"pre,omitempty"`
	After   *TradePeriod  `json:"after,omitempty"`
	Periods []TradePeriod `json:"periods,omitempty"`
}

// FundFlowKLineSeries 资金流曲线
type FundFlowKLineSeries struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	Market       int             `json:"market"`
	TradePeriods TradePeriods    `json:"tradePeriods,omitempty"`
	KLines       []FundFlowKLine `json:"klines"`
}

// StockAnnouncement 公告摘要
type StockAnnouncement struct {
	Title      string `json:"title"`
	NoticeDate string `json:"noticeDate"`
	Type       string `json:"type,omitempty"`
	Columns    string `json:"columns,omitempty"`
	ArtCode    string `json:"artCode,omitempty"`
}

// StockAnnouncements 公告列表
type StockAnnouncements struct {
	Code  string              `json:"code"`
	Items []StockAnnouncement `json:"items"`
	Total int64               `json:"total,omitempty"`
}
