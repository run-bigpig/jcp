package tools

import (
	"fmt"
	"math"
	"strings"

	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetKLineInput K线数据输入参数
type GetKLineInput struct {
	Code         string   `json:"code,omitempty" jsonschema:"股票代码，如 sh600519"`
	Symbol       string   `json:"symbol,omitempty" jsonschema:"股票代码别名，如 sz000001"`
	StockCode    string   `json:"stockCode,omitempty" jsonschema:"股票代码别名，如 sz000001"`
	Ticker       string   `json:"ticker,omitempty" jsonschema:"股票代码别名，如 000001 或 sz000001"`
	SecurityCode string   `json:"securityCode,omitempty" jsonschema:"股票代码别名，如 000001"`
	SecuCode     string   `json:"secuCode,omitempty" jsonschema:"股票代码别名，如 000001"`
	Codes        []string `json:"codes,omitempty" jsonschema:"股票代码列表，兼容字段，取第一项"`
	Period       string   `json:"period,omitempty" jsonschema:"K线周期: 1m(5分钟), 1d(日线), 1w(周线), 1mo(月线)，默认1d"`
	Days         int      `json:"days,omitzero" jsonschema:"获取天数，默认30"`
}

// GetKLineOutput K线数据输出
type GetKLineOutput struct {
	Data string `json:"data" jsonschema:"K线数据"`
}

// createKLineTool 创建K线数据工具
func (r *Registry) createKLineTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetKLineInput) (GetKLineOutput, error) {
		code := resolveKLineCode(ctx, input)
		fmt.Printf("[Tool:get_kline_data] 调用开始, code=%s, period=%s, days=%d\n", code, input.Period, input.Days)

		if code == "" {
			fmt.Println("[Tool:get_kline_data] 错误: 未提供股票代码")
			return GetKLineOutput{Data: "请提供股票代码"}, nil
		}

		period := input.Period
		if period == "" {
			period = "1d"
		}
		days := input.Days
		if days == 0 {
			days = 30
		}

		klines, err := r.marketService.GetKLineData(code, period, days)
		if err != nil {
			fmt.Printf("[Tool:get_kline_data] 错误: %v\n", err)
			return GetKLineOutput{}, err
		}

		// 格式化输出（只取最近10条避免过长）
		var builder strings.Builder
		start := 0
		if len(klines) > 10 {
			start = len(klines) - 10
		}
		for _, k := range klines[start:] {
			builder.WriteString(fmt.Sprintf("%s: 开%.2f 高%.2f 低%.2f 收%.2f 量%d",
				k.Time, k.Open, k.High, k.Low, k.Close, k.Volume))
			if hasAnyMA(k) {
				builder.WriteString(fmt.Sprintf(" MA5%s MA10%s MA20%s",
					formatIndicator(k.MA5, 2),
					formatIndicator(k.MA10, 2),
					formatIndicator(k.MA20, 2),
				))
			}
			builder.WriteString("\n")
		}

		if summary := buildKLineIndicatorSummary(klines); summary != "" {
			builder.WriteString(summary)
		}
		result := strings.TrimSpace(builder.String())

		fmt.Printf("[Tool:get_kline_data] 调用完成, 返回%d条数据\n", len(klines))
		return GetKLineOutput{Data: result}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_kline_data",
		Description: "获取股票K线数据，支持5分钟线、日线、周线、月线",
	}, handler)
}

func resolveKLineCode(ctx tool.Context, input GetKLineInput) string {
	candidates := []string{
		input.Code,
		input.Symbol,
		input.StockCode,
		input.Ticker,
		input.SecurityCode,
		input.SecuCode,
	}
	for _, candidate := range candidates {
		if code := normalizeStockSymbol(candidate); code != "" {
			return code
		}
	}
	for _, code := range normalizeStockSymbolList(input.Codes) {
		return code
	}
	if code := stockCodeFromToolContext(ctx); code != "" {
		fmt.Printf("[Tool:get_kline_data] 兜底命中上下文股票代码: %s\n", code)
		return code
	}
	return ""
}

type macdPoint struct {
	dif  float64
	dea  float64
	hist float64
}

func buildKLineIndicatorSummary(klines []models.KLineData) string {
	if len(klines) == 0 {
		return ""
	}

	latest := klines[len(klines)-1]
	var summary strings.Builder
	summary.WriteString("\n技术指标摘要:\n")

	if len(klines) >= 2 {
		prev := klines[len(klines)-2]
		if isMAAvailable(prev.MA5) && isMAAvailable(prev.MA10) &&
			isMAAvailable(latest.MA5) && isMAAvailable(latest.MA10) {
			maSignal := detectCross(prev.MA5, prev.MA10, latest.MA5, latest.MA10)
			summary.WriteString(fmt.Sprintf("MA: MA5=%s MA10=%s MA20=%s; MA5/MA10信号=%s\n",
				formatIndicator(latest.MA5, 2),
				formatIndicator(latest.MA10, 2),
				formatIndicator(latest.MA20, 2),
				maSignal,
			))
		} else if hasAnyMA(latest) {
			summary.WriteString(fmt.Sprintf("MA: MA5=%s MA10=%s MA20=%s\n",
				formatIndicator(latest.MA5, 2),
				formatIndicator(latest.MA10, 2),
				formatIndicator(latest.MA20, 2),
			))
		} else {
			summary.WriteString("MA: 未获取到\n")
		}
	} else if hasAnyMA(latest) {
		summary.WriteString(fmt.Sprintf("MA: MA5=%s MA10=%s MA20=%s\n",
			formatIndicator(latest.MA5, 2),
			formatIndicator(latest.MA10, 2),
			formatIndicator(latest.MA20, 2),
		))
	} else {
		summary.WriteString("MA: 未获取到\n")
	}

	macdSeries := calculateMACDSeries(klines)
	if len(macdSeries) >= 2 {
		last := macdSeries[len(macdSeries)-1]
		prev := macdSeries[len(macdSeries)-2]
		macdSignal := detectCross(prev.dif, prev.dea, last.dif, last.dea)
		summary.WriteString(fmt.Sprintf("MACD: DIF=%s DEA=%s 柱=%s; DIF/DEA信号=%s",
			formatIndicator(last.dif, 4),
			formatIndicator(last.dea, 4),
			formatIndicator(last.hist, 4),
			macdSignal,
		))
	} else {
		summary.WriteString("MACD: 数据不足")
	}

	return summary.String()
}

func calculateMACDSeries(klines []models.KLineData) []macdPoint {
	if len(klines) == 0 {
		return nil
	}

	result := make([]macdPoint, 0, len(klines))
	ema12 := klines[0].Close
	ema26 := klines[0].Close
	dea := 0.0

	for i := range klines {
		closePrice := klines[i].Close
		if i == 0 {
			result = append(result, macdPoint{})
			continue
		}
		ema12 = ema12*(11.0/13.0) + closePrice*(2.0/13.0)
		ema26 = ema26*(25.0/27.0) + closePrice*(2.0/27.0)
		dif := ema12 - ema26
		dea = dea*(8.0/10.0) + dif*(2.0/10.0)
		hist := (dif - dea) * 2
		result = append(result, macdPoint{dif: dif, dea: dea, hist: hist})
	}

	return result
}

func detectCross(prevFast, prevSlow, fast, slow float64) string {
	if !isFiniteNumber(prevFast) || !isFiniteNumber(prevSlow) ||
		!isFiniteNumber(fast) || !isFiniteNumber(slow) {
		return "未获取到"
	}
	if prevFast <= prevSlow && fast > slow {
		return "金叉"
	}
	if prevFast >= prevSlow && fast < slow {
		return "死叉"
	}
	return "无新增交叉"
}

func hasAnyMA(item models.KLineData) bool {
	return isMAAvailable(item.MA5) ||
		isMAAvailable(item.MA10) ||
		isMAAvailable(item.MA20)
}

func isMAAvailable(value float64) bool {
	return isFiniteNumber(value) && value > 0
}

func isFiniteNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func formatIndicator(value float64, precision int) string {
	if !isFiniteNumber(value) {
		return "未获取到"
	}
	return fmt.Sprintf("%.*f", precision, value)
}
