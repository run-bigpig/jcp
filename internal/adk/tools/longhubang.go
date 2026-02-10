package tools

import (
	"fmt"

	"github.com/run-bigpig/jcp/internal/logger"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

var lhbLog = logger.New("tool:longhubang")

// GetLongHuBangInput 龙虎榜输入参数
type GetLongHuBangInput struct {
	PageSize   int    `json:"page_size,omitzero" jsonschema:"每页条数，默认20条，最大50条"`
	PageNumber int    `json:"page_number,omitzero" jsonschema:"页码，默认1"`
	TradeDate  string `json:"trade_date,omitzero" jsonschema:"交易日期，格式YYYY-MM-DD，为空则获取所有日期"`
}

// GetLongHuBangOutput 龙虎榜输出
type GetLongHuBangOutput struct {
	Data string `json:"data" jsonschema:"龙虎榜数据列表"`
}

// createLongHuBangTool 创建龙虎榜工具
func (r *Registry) createLongHuBangTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetLongHuBangInput) (GetLongHuBangOutput, error) {
		lhbLog.Debug("调用开始, pageSize=%d, pageNumber=%d, tradeDate=%s", input.PageSize, input.PageNumber, input.TradeDate)

		pageSize := input.PageSize
		if pageSize <= 0 {
			pageSize = 20
		}
		if pageSize > 50 {
			pageSize = 50
		}
		pageNumber := input.PageNumber
		if pageNumber <= 0 {
			pageNumber = 1
		}

		listResult, err := r.longHuBangService.GetLongHuBangList(pageSize, pageNumber, input.TradeDate)
		if err != nil {
			lhbLog.Error("获取龙虎榜失败: %v", err)
			return GetLongHuBangOutput{}, err
		}

		var result string
		for i, item := range listResult.Items {
			// 格式化金额为万元
			netBuyWan := item.NetBuyAmt / 10000
			buyWan := item.BuyAmt / 10000
			sellWan := item.SellAmt / 10000

			result += fmt.Sprintf("%d. [%s] %s(%s) 收盘:%.2f 涨跌:%.2f%% 换手:%.2f%%\n",
				i+1, item.TradeDate, item.Name, item.SecuCode,
				item.ClosePrice, item.ChangePercent, item.TurnoverRate)
			result += fmt.Sprintf("   净买:%.0f万 买入:%.0f万 卖出:%.0f万 占比:%.2f%%\n",
				netBuyWan, buyWan, sellWan, item.DealRatio)
			result += fmt.Sprintf("   原因:%s\n", item.Reason)
			if item.D1Change != 0 {
				result += fmt.Sprintf("   后续表现: 次日%.2f%% 5日%.2f%% 10日%.2f%%\n",
					item.D1Change, item.D5Change, item.D10Change)
			}
		}

		lhbLog.Debug("调用完成, 返回%d条数据", len(listResult.Items))
		return GetLongHuBangOutput{Data: result}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_longhubang",
		Description: "获取A股龙虎榜数据，包括上榜股票、净买入金额、买卖金额、上榜原因等信息，数据来源于东方财富",
	}, handler)
}

// GetLongHuBangDetailInput 龙虎榜营业部明细输入
type GetLongHuBangDetailInput struct {
	Code      string `json:"code" jsonschema:"股票代码，如600477"`
	TradeDate string `json:"trade_date" jsonschema:"交易日期，格式YYYY-MM-DD，如2026-02-09"`
}

// GetLongHuBangDetailOutput 龙虎榜营业部明细输出
type GetLongHuBangDetailOutput struct {
	Data string `json:"data" jsonschema:"营业部买卖明细"`
}

// createLongHuBangDetailTool 创建龙虎榜营业部明细工具
func (r *Registry) createLongHuBangDetailTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetLongHuBangDetailInput) (GetLongHuBangDetailOutput, error) {
		lhbLog.Debug("调用开始, code=%s, date=%s", input.Code, input.TradeDate)

		if input.Code == "" || input.TradeDate == "" {
			return GetLongHuBangDetailOutput{}, fmt.Errorf("股票代码和交易日期不能为空")
		}

		details, err := r.longHuBangService.GetStockDetail(input.Code, input.TradeDate)
		if err != nil {
			lhbLog.Error("获取营业部明细失败: %v", err)
			return GetLongHuBangDetailOutput{}, err
		}

		if len(details) == 0 {
			return GetLongHuBangDetailOutput{Data: "未找到该股票的龙虎榜营业部数据"}, nil
		}

		var result string
		result += fmt.Sprintf("=== %s 龙虎榜营业部明细 ===\n\n", input.Code)

		// 分别输出买入和卖出
		result += "【买入前五营业部】\n"
		buyCount := 0
		for _, d := range details {
			if d.Direction == "buy" && buyCount < 5 {
				buyCount++
				result += fmt.Sprintf("%d. %s\n", buyCount, d.OperName)
				result += fmt.Sprintf("   买入:%.0f万 占比:%.2f%%\n", d.BuyAmt/10000, d.BuyPercent)
			}
		}

		result += "\n【卖出前五营业部】\n"
		sellCount := 0
		for _, d := range details {
			if d.Direction == "sell" && sellCount < 5 {
				sellCount++
				result += fmt.Sprintf("%d. %s\n", sellCount, d.OperName)
				result += fmt.Sprintf("   卖出:%.0f万 占比:%.2f%%\n", d.SellAmt/10000, d.SellPercent)
			}
		}

		lhbLog.Debug("调用完成")
		return GetLongHuBangDetailOutput{Data: result}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_longhubang_detail",
		Description: "获取个股龙虎榜营业部买卖明细，需要提供股票代码和交易日期",
	}, handler)
}
