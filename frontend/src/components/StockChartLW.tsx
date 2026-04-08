import React, { useEffect, useRef, useCallback, useMemo, useState } from 'react';
import { ZoomIn, ZoomOut, MoveHorizontal, Eye, EyeOff } from 'lucide-react';
import {
  createChart,
  IChartApi,
  ISeriesApi,
  CandlestickData,
  HistogramData,
  LineData,
  Time,
  CrosshairMode,
  LineStyle,
  CandlestickSeries,
  LineSeries,
  HistogramSeries,
  SeriesType,
  MouseEventParams,
} from 'lightweight-charts';
import { KLineData, TimePeriod, Stock } from '../types';
import { useTheme } from '../contexts/ThemeContext';
import { useCandleColor } from '../contexts/CandleColorContext';
import { ResizeHandle } from './ResizeHandle';
import { useIndicator } from '../contexts/IndicatorContext';
import {
  parseTime,
  calculateSMA,
  calculateEMA,
  calculateBOLL,
  calculateMACD,
  calculateRSI,
  calculateKDJ,
} from '../utils/indicators';

const VOLUME_MIN = 20;
const VOLUME_MAX = 200;
const VOLUME_DEFAULT = 72;

interface StockChartProps {
  data: KLineData[];
  updateMode: 'full' | 'incremental' | 'refresh';
  period: TimePeriod;
  onPeriodChange: (p: TimePeriod) => void;
  stock?: Stock;
}

// 副图类型
type SubChartType = 'volume' | 'macd' | 'rsi' | 'kdj';

// 指标线颜色常量
const MA_COLORS = ['#facc15', '#a855f7', '#f97316', '#38bdf8', '#f43f5e'];
const EMA_COLORS = ['#06b6d4', '#ec4899'];
const BOLL_COLOR = '#e91e63';
const CHART_FONT_FAMILY = 'Menlo, Monaco, Consolas, monospace';

// 批量移除 series 并清空 ref
function clearSeriesArray(chart: IChartApi, refs: React.MutableRefObject<ISeriesApi<SeriesType, Time>[]>) {
  for (const s of refs.current) {
    try { chart.removeSeries(s); } catch { /* already removed */ }
  }
  refs.current = [];
}

// 将 UTC 秒级时间戳格式化为 YYYY-MM-DD HH:mm
function formatTimestamp(ts: number): string {
  const d = new Date(ts * 1000);
  const Y = d.getUTCFullYear();
  const M = String(d.getUTCMonth() + 1).padStart(2, '0');
  const D = String(d.getUTCDate()).padStart(2, '0');
  const h = String(d.getUTCHours()).padStart(2, '0');
  const m = String(d.getUTCMinutes()).padStart(2, '0');
  return `${Y}-${M}-${D} ${h}:${m}`;
}

// 格式化时间显示，统一为 YYYY-MM-DD HH:MM:SS
function formatTimeDisplay(timeStr: string): string {
  if (timeStr.length > 10) {
    return timeStr.slice(0, 19);
  }
  return timeStr.slice(0, 10) + ' 00:00:00';
}

export const StockChartLW: React.FC<StockChartProps> = ({ data, updateMode, period, onPeriodChange, stock }) => {
  const { colors } = useTheme();
  const cc = useCandleColor();
  const { config: indicatorConfig, updateIndicator } = useIndicator();
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const volumeContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const volumeChartRef = useRef<IChartApi | null>(null);
  const mainSeriesRef = useRef<ISeriesApi<SeriesType, Time> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<SeriesType, Time> | null>(null);
  const maSeriesRefs = useRef<ISeriesApi<SeriesType, Time>[]>([]);
  const emaSeriesRefs = useRef<ISeriesApi<SeriesType, Time>[]>([]);
  const bollSeriesRefs = useRef<ISeriesApi<SeriesType, Time>[]>([]);
  const subSeriesRefs = useRef<ISeriesApi<SeriesType, Time>[]>([]);
  const seriesTypeRef = useRef<'line' | 'candle' | null>(null);
  const hasFittedRef = useRef(false);

  const [volumeHeight, setVolumeHeight] = useState(VOLUME_DEFAULT);

  const handleVolumeResize = useCallback((delta: number) => {
    setVolumeHeight(prev => Math.max(VOLUME_MIN, Math.min(VOLUME_MAX, prev - delta)));
  }, []);

  // series → 指标类型映射（用于点击识别）
  type MainIndicatorType = 'ma' | 'ema' | 'boll';
  const seriesIndicatorMap = useRef<Map<ISeriesApi<SeriesType, Time>, MainIndicatorType>>(new Map());

  // 浮动配置面板状态
  const [indicatorPopup, setIndicatorPopup] = React.useState<{
    type: MainIndicatorType;
    xRatio: number;
    yRatio: number;
  } | null>(null);
  const [chartViewport, setChartViewport] = useState({ width: 0, height: 0 });

  const [subChartType, setSubChartType] = React.useState<SubChartType>('volume');
  const subChartTypeRef = useRef<SubChartType>('volume');

  const safeData = data || [];
  const isIntraday = period === '1m';
  const preClose = stock?.preClose || 0;

  const [hoverData, setHoverData] = React.useState<KLineData | null>(null);
  const lastData = safeData[safeData.length - 1];
  const displayData = hoverData || lastData;

  const chartColors = useMemo(() => ({
    background: colors.isDark ? '#0f172a' : '#ffffff',
    textColor: colors.isDark ? '#94a3b8' : '#64748b',
    gridColor: colors.isDark ? '#1e293b' : '#e2e8f0',
    upColor: cc.upColor,
    downColor: cc.downColor,
    priceLineColor: colors.isDark ? '#64748b' : '#94a3b8',
  }), [colors.isDark, cc.upColor, cc.downColor]);

  const periods: { id: TimePeriod; label: string }[] = [
    { id: '1m', label: '分时' },
    { id: '1d', label: '日K' },
    { id: '1w', label: '周K' },
    { id: '1mo', label: '月K' },
  ];

  const getPriceColor = useCallback((price: number) => {
    if (preClose <= 0) return colors.isDark ? 'text-slate-100' : 'text-slate-700';
    if (price > preClose) return cc.upClass;
    if (price < preClose) return cc.downClass;
    return colors.isDark ? 'text-slate-100' : 'text-slate-700';
  }, [preClose, colors.isDark, cc.upClass, cc.downClass]);

  const formatChangePercent = useCallback((price: number) => {
    if (preClose <= 0) return '0.00%';
    const percent = ((price - preClose) / preClose) * 100;
    const sign = percent >= 0 ? '+' : '';
    return `${sign}${percent.toFixed(2)}%`;
  }, [preClose]);

  const formatChange = useCallback((price: number) => {
    if (preClose <= 0) return '0.00';
    const change = price - preClose;
    const sign = change >= 0 ? '+' : '';
    return `${sign}${change.toFixed(2)}`;
  }, [preClose]);

  // 清除所有 series（不销毁图表实例）
  const clearAllSeries = useCallback(() => {
    const chart = chartRef.current;
    const volumeChart = volumeChartRef.current;
    if (!chart || !volumeChart) return;

    if (mainSeriesRef.current) {
      try { chart.removeSeries(mainSeriesRef.current); } catch { /* already removed */ }
      mainSeriesRef.current = null;
    }
    clearSeriesArray(chart, maSeriesRefs);
    clearSeriesArray(chart, emaSeriesRefs);
    clearSeriesArray(chart, bollSeriesRefs);
    clearSeriesArray(volumeChart, subSeriesRefs);
    if (volumeSeriesRef.current) {
      try { volumeChart.removeSeries(volumeSeriesRef.current); } catch { /* already removed */ }
      volumeSeriesRef.current = null;
    }
    seriesTypeRef.current = null;
    hasFittedRef.current = false;
  }, []);

  // ========== 唯一的图表创建：组件挂载时创建，卸载时销毁 ==========
  useEffect(() => {
    if (!chartContainerRef.current || !volumeContainerRef.current) return;

    const chart = createChart(chartContainerRef.current, {
      layout: {
        background: { color: '#0f172a' },
        textColor: '#94a3b8',
        attributionLogo: false,
        fontFamily: CHART_FONT_FAMILY,
      },
      grid: { vertLines: { color: '#1e293b' }, horzLines: { color: '#1e293b' } },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: '#1e293b', scaleMargins: { top: 0.15, bottom: 0.15 } },
      timeScale: { borderColor: '#1e293b', timeVisible: true, secondsVisible: false },
      localization: { timeFormatter: (time: Time) => typeof time === 'number' ? formatTimestamp(time) : String(time) },
      handleScroll: true,
      handleScale: true,
    });

    const volumeChart = createChart(volumeContainerRef.current, {
      layout: {
        background: { color: '#0f172a' },
        textColor: '#94a3b8',
        attributionLogo: false,
        fontFamily: CHART_FONT_FAMILY,
      },
      grid: { vertLines: { color: '#1e293b' }, horzLines: { color: '#1e293b' } },
      rightPriceScale: { borderColor: '#1e293b', scaleMargins: { top: 0.15, bottom: 0.1 } },
      timeScale: { borderColor: '#1e293b', timeVisible: true, secondsVisible: false },
      localization: { timeFormatter: (time: Time) => typeof time === 'number' ? formatTimestamp(time) : String(time) },
      handleScroll: true,
      handleScale: true,
    });

    chartRef.current = chart;
    volumeChartRef.current = volumeChart;

    // 同步时间轴
    chart.timeScale().subscribeVisibleLogicalRangeChange(range => {
      if (range) volumeChart.timeScale().setVisibleLogicalRange(range);
    });
    volumeChart.timeScale().subscribeVisibleLogicalRangeChange(range => {
      if (range) chart.timeScale().setVisibleLogicalRange(range);
    });

    // resize
    const resizeObserver = new ResizeObserver(() => {
      if (chartContainerRef.current && volumeContainerRef.current) {
        const w = chartContainerRef.current.clientWidth;
        const h = chartContainerRef.current.clientHeight;
        const vh = volumeContainerRef.current.clientHeight;
        setChartViewport(prev => (prev.width === w && prev.height === h ? prev : { width: w, height: h }));
        if (w > 0 && h > 0) chart.applyOptions({ width: w, height: h });
        if (w > 0 && vh > 0) volumeChart.applyOptions({ width: w, height: vh });
      }
    });
    resizeObserver.observe(chartContainerRef.current);
    resizeObserver.observe(volumeContainerRef.current);

    // 仅在组件卸载时销毁
    return () => {
      resizeObserver.disconnect();
      chart.remove();
      volumeChart.remove();
      chartRef.current = null;
      volumeChartRef.current = null;
      mainSeriesRef.current = null;
      volumeSeriesRef.current = null;
      maSeriesRefs.current = [];
      emaSeriesRefs.current = [];
      bollSeriesRefs.current = [];
      subSeriesRefs.current = [];
      seriesTypeRef.current = null;
    };
  }, []); // 空依赖 —— 只执行一次

  // ========== 主题变化：applyOptions 更新样式，不销毁图表 ==========
  useEffect(() => {
    const chart = chartRef.current;
    const volumeChart = volumeChartRef.current;
    if (!chart || !volumeChart) return;

    const layoutOpts = {
      layout: {
        background: { color: chartColors.background },
        textColor: chartColors.textColor,
        fontFamily: CHART_FONT_FAMILY,
      },
      grid: { vertLines: { color: chartColors.gridColor }, horzLines: { color: chartColors.gridColor } },
      rightPriceScale: { borderColor: chartColors.gridColor },
      timeScale: { borderColor: chartColors.gridColor },
    };
    chart.applyOptions(layoutOpts);
    volumeChart.applyOptions(layoutOpts);

    // 更新 K线蜡烛颜色（颜色模式切换时生效）
    if (mainSeriesRef.current && seriesTypeRef.current === 'candle') {
      mainSeriesRef.current.applyOptions({
        upColor: chartColors.upColor,
        downColor: chartColors.downColor,
        wickUpColor: chartColors.upColor,
        wickDownColor: chartColors.downColor,
      });
    }

    // 更新成交量柱颜色（仅副图为成交量时）
    if (volumeSeriesRef.current && subChartTypeRef.current === 'volume' && safeData.length > 0) {
      const volData: HistogramData[] = safeData.map(d => ({
        time: parseTime(d.time),
        value: d.volume,
        color: d.close >= d.open ? chartColors.upColor + '99' : chartColors.downColor + '99',
      }));
      volumeSeriesRef.current.setData(volData);
    }
  }, [chartColors, safeData]);

  // ========== 周期变化：更新 timeScale 选项 + 交互模式 ==========
  useEffect(() => {
    const chart = chartRef.current;
    const volumeChart = volumeChartRef.current;
    if (!chart || !volumeChart) return;

    chart.applyOptions({
      timeScale: { timeVisible: isIntraday, secondsVisible: false },
      handleScroll: !isIntraday,
      handleScale: !isIntraday,
    });
    volumeChart.applyOptions({
      timeScale: { timeVisible: isIntraday, secondsVisible: false },
      handleScroll: !isIntraday,
      handleScale: !isIntraday,
    });
  }, [isIntraday]);

  // 切换周期时重置 fit 状态，避免沿用旧 X 轴可视范围
  useEffect(() => {
    hasFittedRef.current = false;
  }, [period]);

  // 分时模式固定显示成交量副图，避免隐藏 tab 后无法恢复
  useEffect(() => {
    if (!isIntraday) return;
    if (subChartTypeRef.current === 'volume') return;
    setSubChartType('volume');
    subChartTypeRef.current = 'volume';
  }, [isIntraday]);

  // ========== 副图辅助函数 ==========
  const clearSubChart = useCallback(() => {
    const volumeChart = volumeChartRef.current;
    if (!volumeChart) return;
    clearSeriesArray(volumeChart, subSeriesRefs);
    if (volumeSeriesRef.current) {
      try { volumeChart.removeSeries(volumeSeriesRef.current); } catch { /* already removed */ }
      volumeSeriesRef.current = null;
    }
  }, []);

  const renderSubChart = useCallback((type: SubChartType, chartData: KLineData[]) => {
    const volumeChart = volumeChartRef.current;
    if (!volumeChart || chartData.length === 0) return;

    if (type === 'volume') {
      volumeSeriesRef.current = volumeChart.addSeries(HistogramSeries, { priceFormat: { type: 'volume' } });
      const volData: HistogramData[] = chartData.map(d => ({
        time: parseTime(d.time), value: d.volume,
        color: d.close >= d.open ? chartColors.upColor + '99' : chartColors.downColor + '99',
      }));
      volumeSeriesRef.current.setData(volData);
    } else if (type === 'macd') {
      const { dif, dea, histogram } = calculateMACD(
        chartData, indicatorConfig.macd.fast, indicatorConfig.macd.slow, indicatorConfig.macd.signal,
      );
      const difSeries = volumeChart.addSeries(LineSeries, {
        color: '#3b82f6', lineWidth: 1, priceLineVisible: false, lastValueVisible: true, title: 'DIF',
      });
      difSeries.setData(dif);
      const deaSeries = volumeChart.addSeries(LineSeries, {
        color: '#eab308', lineWidth: 1, priceLineVisible: false, lastValueVisible: true, title: 'DEA',
      });
      deaSeries.setData(dea);
      const histSeries = volumeChart.addSeries(HistogramSeries, {
        priceLineVisible: false, lastValueVisible: true, title: 'MACD',
      });
      histSeries.setData(histogram as any);
      subSeriesRefs.current = [difSeries, deaSeries, histSeries];
    } else if (type === 'rsi') {
      const rsiData = calculateRSI(chartData, indicatorConfig.rsi.period);
      const rsiSeries = volumeChart.addSeries(LineSeries, {
        color: '#a855f7', lineWidth: 1, priceLineVisible: false, lastValueVisible: true, title: 'RSI',
      });
      rsiSeries.setData(rsiData);
      // 70/30 参考线
      rsiSeries.createPriceLine({ price: 70, color: '#ef444480', lineWidth: 1, lineStyle: LineStyle.Dashed, axisLabelVisible: false, title: '' });
      rsiSeries.createPriceLine({ price: 30, color: '#22c55e80', lineWidth: 1, lineStyle: LineStyle.Dashed, axisLabelVisible: false, title: '' });
      subSeriesRefs.current = [rsiSeries];
    } else if (type === 'kdj') {
      const { k, d, j } = calculateKDJ(
        chartData, indicatorConfig.kdj.period, indicatorConfig.kdj.k, indicatorConfig.kdj.d,
      );
      const kSeries = volumeChart.addSeries(LineSeries, {
        color: '#3b82f6', lineWidth: 1, priceLineVisible: false, lastValueVisible: true, title: 'K',
      });
      kSeries.setData(k);
      const dSeries = volumeChart.addSeries(LineSeries, {
        color: '#eab308', lineWidth: 1, priceLineVisible: false, lastValueVisible: true, title: 'D',
      });
      dSeries.setData(d);
      const jSeries = volumeChart.addSeries(LineSeries, {
        color: '#a855f7', lineWidth: 1, priceLineVisible: false, lastValueVisible: true, title: 'J',
      });
      jSeries.setData(j);
      subSeriesRefs.current = [kSeries, dSeries, jSeries];
    }
  }, [chartColors, indicatorConfig]);

  // ========== 核心：数据更新（切换股票/周期 = 全量，增量推送 = setData） ==========
  useEffect(() => {
    const chart = chartRef.current;
    const volumeChart = volumeChartRef.current;
    if (!chart || !volumeChart) return;

    // 数据为空时清除 series 并返回
    if (safeData.length === 0) {
      clearAllSeries();
      return;
    }

    const needType = isIntraday ? 'line' : 'candle';

    // series 类型不匹配时（分时 <-> K线切换），清除旧 series
    if (seriesTypeRef.current !== null && seriesTypeRef.current !== needType) {
      clearAllSeries();
    }

    // ---------- 分时图 ----------
    if (isIntraday) {
      if (!mainSeriesRef.current) {
        // 首次创建 series
        const lineSeries = chart.addSeries(LineSeries, {
          color: '#38bdf8', lineWidth: 2, priceLineVisible: false, lastValueVisible: true,
        });
        mainSeriesRef.current = lineSeries;
        seriesTypeRef.current = 'line';

        if (preClose > 0) {
          lineSeries.createPriceLine({
            price: preClose, color: chartColors.priceLineColor,
            lineWidth: 1, lineStyle: LineStyle.Dashed, axisLabelVisible: true, title: '昨收',
          });
        }

        // 均价线
        const avgSeries = chart.addSeries(LineSeries, {
          color: '#facc15', lineWidth: 1, priceLineVisible: false, lastValueVisible: false,
        });
        maSeriesRefs.current = [avgSeries];

        // 成交量
        volumeSeriesRef.current = volumeChart.addSeries(HistogramSeries, { priceFormat: { type: 'volume' } });
      }

      // 更新数据
      const lineData: LineData[] = safeData.map(d => ({ time: parseTime(d.time), value: d.close }));
      mainSeriesRef.current.setData(lineData);

      if (maSeriesRefs.current.length > 0) {
        const avgData: LineData[] = safeData.filter(d => d.avg).map(d => ({ time: parseTime(d.time), value: d.avg! }));
        maSeriesRefs.current[0].setData(avgData);
      }
    }
    // ---------- K线图 ----------
    else {
      if (!mainSeriesRef.current) {
        const candleSeries = chart.addSeries(CandlestickSeries, {
          upColor: chartColors.upColor, downColor: chartColors.downColor,
          wickUpColor: chartColors.upColor, wickDownColor: chartColors.downColor, borderVisible: false,
        });
        mainSeriesRef.current = candleSeries;
        seriesTypeRef.current = 'candle';
      }

      // 更新 K线 数据
      const candleData: CandlestickData[] = safeData.map(d => ({
        time: parseTime(d.time), open: d.open, high: d.high, low: d.low, close: d.close,
      }));
      mainSeriesRef.current.setData(candleData);

      // --- MA 均线（配置驱动） ---
      for (const s of maSeriesRefs.current) seriesIndicatorMap.current.delete(s);
      clearSeriesArray(chart, maSeriesRefs);
      if (indicatorConfig.ma.enabled) {
        indicatorConfig.ma.periods.forEach((p, idx) => {
          const maSeries = chart.addSeries(LineSeries, {
            color: MA_COLORS[idx % MA_COLORS.length], lineWidth: 1,
            priceLineVisible: false, lastValueVisible: true, title: `MA${p}`,
          });
          maSeries.setData(safeData.length >= p ? calculateSMA(safeData, p) : []);
          maSeriesRefs.current.push(maSeries);
          seriesIndicatorMap.current.set(maSeries, 'ma');
        });
      }

      // --- EMA 均线 ---
      for (const s of emaSeriesRefs.current) seriesIndicatorMap.current.delete(s);
      clearSeriesArray(chart, emaSeriesRefs);
      if (indicatorConfig.ema.enabled) {
        indicatorConfig.ema.periods.forEach((p, idx) => {
          const emaSeries = chart.addSeries(LineSeries, {
            color: EMA_COLORS[idx % EMA_COLORS.length], lineWidth: 1,
            priceLineVisible: false, lastValueVisible: true, title: `EMA${p}`,
          });
          emaSeries.setData(calculateEMA(safeData, p));
          emaSeriesRefs.current.push(emaSeries);
          seriesIndicatorMap.current.set(emaSeries, 'ema');
        });
      }

      // --- BOLL 布林带 ---
      for (const s of bollSeriesRefs.current) seriesIndicatorMap.current.delete(s);
      clearSeriesArray(chart, bollSeriesRefs);
      if (indicatorConfig.boll.enabled) {
        const { mid, upper, lower } = calculateBOLL(safeData, indicatorConfig.boll.period, indicatorConfig.boll.multiplier);
        const midSeries = chart.addSeries(LineSeries, {
          color: BOLL_COLOR, lineWidth: 1, priceLineVisible: false, lastValueVisible: true, title: 'BOLL:M',
        });
        midSeries.setData(mid);
        const upperSeries = chart.addSeries(LineSeries, {
          color: BOLL_COLOR, lineWidth: 1, lineStyle: LineStyle.Dashed,
          priceLineVisible: false, lastValueVisible: true, title: 'BOLL:U',
        });
        upperSeries.setData(upper);
        const lowerSeries = chart.addSeries(LineSeries, {
          color: BOLL_COLOR, lineWidth: 1, lineStyle: LineStyle.Dashed,
          priceLineVisible: false, lastValueVisible: true, title: 'BOLL:L',
        });
        lowerSeries.setData(lower);
        bollSeriesRefs.current = [midSeries, upperSeries, lowerSeries];
        for (const s of bollSeriesRefs.current) seriesIndicatorMap.current.set(s, 'boll');
      }
    }

    // ========== 副图渲染 ==========
    clearSubChart();
    const subChartTypeToRender: SubChartType = isIntraday ? 'volume' : subChartTypeRef.current;
    renderSubChart(subChartTypeToRender, safeData);

    // full: 用户主动切换股票/周期 → fitContent；refresh: 定时刷新 → 保留缩放；增量仅首次 fit
    const shouldFit = safeData.length > 0 && (
      updateMode === 'full' || (!hasFittedRef.current && safeData.length > 1)
    );
    if (shouldFit) {
      chart.timeScale().fitContent();
      volumeChart.timeScale().fitContent();
      hasFittedRef.current = true;
    }
  }, [safeData, updateMode, preClose, isIntraday, chartColors, clearAllSeries, clearSubChart, renderSubChart, indicatorConfig]);

  // ========== 副图指标禁用时自动回退到成交量 ==========
  useEffect(() => {
    const cur = subChartTypeRef.current;
    const shouldFallback =
      (cur === 'macd' && !indicatorConfig.macd.enabled) ||
      (cur === 'rsi' && !indicatorConfig.rsi.enabled) ||
      (cur === 'kdj' && !indicatorConfig.kdj.enabled);
    if (shouldFallback) {
      setSubChartType('volume');
      subChartTypeRef.current = 'volume';
      clearSubChart();
      renderSubChart('volume', safeData);
      volumeChartRef.current?.timeScale().fitContent();
    }
  }, [indicatorConfig.macd.enabled, indicatorConfig.rsi.enabled, indicatorConfig.kdj.enabled, clearSubChart, renderSubChart, safeData]);

  // ========== 副图切换 ==========
  const handleSubChartSwitch = useCallback((type: SubChartType) => {
    setSubChartType(type);
    subChartTypeRef.current = type;
    clearSubChart();
    renderSubChart(type, safeData);
    volumeChartRef.current?.timeScale().fitContent();
  }, [clearSubChart, renderSubChart, safeData]);

  // ========== 点击指标线弹出配置面板 ==========
  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;

    const getSeriesPrice = (raw: unknown): number | null => {
      if (!raw || typeof raw !== 'object') return null;
      const record = raw as Record<string, unknown>;
      if (typeof record.value === 'number') return record.value;
      if (typeof record.close === 'number') return record.close;
      return null;
    };

    const clickHandler = (param: MouseEventParams<Time>) => {
      if (!param.seriesData || !param.point) {
        setIndicatorPopup(null);
        return;
      }
      const clickY = param.point.y;
      const HIT_THRESHOLD = 12; // px
      let bestDist = Infinity;
      let bestType: MainIndicatorType | null = null;

      for (const [series, data] of param.seriesData) {
        const indType = seriesIndicatorMap.current.get(series);
        if (!indType) continue;
        const price = getSeriesPrice(data);
        if (price == null) continue;
        const coord = series.priceToCoordinate(price);
        if (coord == null) continue;
        const dist = Math.abs(coord - clickY);
        if (dist < bestDist) {
          bestDist = dist;
          bestType = indType;
        }
      }

      if (bestType && bestDist <= HIT_THRESHOLD) {
        const width = chartContainerRef.current?.clientWidth || 1;
        const height = chartContainerRef.current?.clientHeight || 1;
        setIndicatorPopup({
          type: bestType,
          xRatio: param.point.x / width,
          yRatio: param.point.y / height,
        });
      } else {
        setIndicatorPopup(null);
      }
    };

    chart.subscribeClick(clickHandler);
    return () => chart.unsubscribeClick(clickHandler);
  }, []);

  // ========== 十字光标 ==========
  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;

    const handler = (param: MouseEventParams<Time>) => {
      if (!param.time || !param.seriesData) {
        setHoverData(null);
        return;
      }
      const timeStr = typeof param.time === 'number'
        ? new Date(param.time * 1000).toISOString().slice(0, 19).replace('T', ' ')
        : String(param.time);
      const found = safeData.find(d => d.time.startsWith(timeStr.slice(0, 16)));
      setHoverData(found || null);
    };

    chart.subscribeCrosshairMove(handler);
    return () => chart.unsubscribeCrosshairMove(handler);
  }, [safeData]);

  // ========== 统计数据 memo ==========
  const todayHigh = useMemo(() => safeData.length > 0 ? Math.max(...safeData.map(d => d.high)) : 0, [safeData]);
  const todayLow = useMemo(() => safeData.length > 0 ? Math.min(...safeData.map(d => d.low)) : 0, [safeData]);
  const totalVolume = useMemo(() => safeData.reduce((sum, d) => sum + d.volume, 0), [safeData]);
  const currentPrice = stock?.price || lastData?.close || 0;
  const currentAvg = lastData?.avg || 0;

  // ========== 渲染（图表容器始终保留在 DOM 中，避免销毁重建） ==========
  const hasData = safeData.length > 0;
  const indicatorPopupLeft = indicatorPopup
    ? Math.min(
      Math.max(indicatorPopup.xRatio * chartViewport.width, 0),
      Math.max(chartViewport.width - 200, 0),
    )
    : 0;
  const indicatorPopupTop = indicatorPopup
    ? Math.min(
      Math.max(indicatorPopup.yRatio * chartViewport.height, 0),
      Math.max(chartViewport.height - 120, 0),
    )
    : 0;

  return (
    <div className="h-full w-full fin-panel flex flex-col relative" onMouseDown={() => setIndicatorPopup(null)}>
      {/* 加载提示（叠加在图表上方） */}
      {!hasData && (
        <div className="absolute inset-0 z-20 flex items-center justify-center fin-panel">
          <span className={`text-sm animate-pulse ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
            加载市场数据中...
          </span>
        </div>
      )}

      {/* Header */}
      <div className={`flex items-center px-2 py-1 border-b fin-divider fin-panel-strong z-10 ${!hasData ? 'invisible' : ''}`}>
        <div className="flex gap-1 shrink-0">
          {periods.map((p) => (
            <button
              key={p.id}
              onClick={() => onPeriodChange(p.id)}
              className={`text-xs px-3 py-1 rounded transition-colors ${
                period === p.id
                  ? (colors.isDark ? 'bg-slate-800/80' : 'bg-slate-200/80') + ' text-accent-2 font-bold'
                  : (colors.isDark ? 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/40')
              }`}
            >
              {p.label}
            </button>
          ))}
          {!isIntraday && (
            <div className={`flex items-center gap-2 ml-3 pl-3 border-l ${colors.isDark ? 'border-slate-700' : 'border-slate-300'}`}>
              <div className={`flex items-center gap-1 text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                <ZoomIn size={12} />
                <ZoomOut size={12} />
                <span>滚轮</span>
              </div>
              <div className={`flex items-center gap-1 text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                <MoveHorizontal size={12} />
                <span>拖拽</span>
              </div>
            </div>
          )}
        </div>

        {/* 数据信息栏：固定在右侧可用区域，避免切换周期时撑宽/抖动 */}
        <div className={`ml-3 min-w-0 flex-1 text-xs font-mono tabular-nums ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
          <div className="flex items-center justify-end gap-2 overflow-x-auto whitespace-nowrap [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden">
          {isIntraday ? (
            <>
              <span className="shrink-0 w-44 text-right">时间: <span className={colors.isDark ? 'text-slate-300' : 'text-slate-600'}>{displayData ? formatTimeDisplay(displayData.time) : '--'}</span></span>
              <span className="shrink-0 w-24 text-right">价格: <span className={getPriceColor(displayData?.close || 0)}>{displayData?.close?.toFixed(2) || '--'}</span></span>
              <span className="shrink-0 w-24 text-right">均价: <span className="text-yellow-500">{displayData?.avg?.toFixed(2) || '--'}</span></span>
              <span className="shrink-0 w-24 text-right">涨跌: <span className={getPriceColor(currentPrice)}>{formatChange(displayData?.close || preClose)}</span></span>
              <span className="shrink-0 w-24 text-right">幅度: <span className={getPriceColor(currentPrice)}>{formatChangePercent(displayData?.close || preClose)}</span></span>
            </>
          ) : (
            <>
              <span className="shrink-0 w-44 text-right">时间: <span className={colors.isDark ? 'text-slate-300' : 'text-slate-600'}>{displayData ? formatTimeDisplay(displayData.time) : '--'}</span></span>
              <span className="shrink-0 w-20 text-right">收: <span className="text-accent-2">{displayData?.close?.toFixed(2) || '--'}</span></span>
              <span className="shrink-0 w-20 text-right">开: {displayData?.open?.toFixed(2) || '--'}</span>
              <span className="shrink-0 w-20 text-right">高: <span className={cc.upClass}>{displayData?.high?.toFixed(2) || '--'}</span></span>
              <span className="shrink-0 w-20 text-right">低: <span className={cc.downClass}>{displayData?.low?.toFixed(2) || '--'}</span></span>
              <span className="shrink-0 w-24 text-right">MA5: <span className="text-yellow-500">{displayData?.ma5?.toFixed(2) || '--'}</span></span>
              <span className="shrink-0 w-24 text-right">MA10: <span className="text-purple-500">{displayData?.ma10?.toFixed(2) || '--'}</span></span>
              <span className="shrink-0 w-24 text-right">MA20: <span className="text-orange-500">{displayData?.ma20?.toFixed(2) || '--'}</span></span>
            </>
          )}
          </div>
        </div>
      </div>

      {/* 分时图专用信息栏 */}
      {isIntraday && (
        <div className={`flex items-center justify-between px-3 py-1.5 border-b fin-divider text-xs ${colors.isDark ? 'bg-slate-900/30' : 'bg-slate-100/50'}`}>
          <div className="flex gap-4">
            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>最高: <span className={cc.upClass}>{todayHigh.toFixed(2)}</span></span>
            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>最低: <span className={cc.downClass}>{todayLow.toFixed(2)}</span></span>
            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>昨收: <span className={colors.isDark ? 'text-slate-300' : 'text-slate-600'}>{preClose.toFixed(2)}</span></span>
          </div>
          <div className="flex gap-4">
            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>均价: <span className="text-yellow-500">{currentAvg.toFixed(2)}</span></span>
            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>总量: <span className={colors.isDark ? 'text-slate-300' : 'text-slate-600'}>{(totalVolume / 100).toFixed(0)}手</span></span>
          </div>
        </div>
      )}

      {/* 主图表区域 */}
      <div className="flex-1 min-h-0 relative">
        <div className="absolute inset-0" ref={chartContainerRef} />

        {/* 主图指标图例 - TradingView 风格 */}
        {!isIntraday && (
          <div className="absolute top-1 left-1 z-20 flex flex-col gap-0.5 pointer-events-auto">
            {/* MA */}
            <div className="flex items-center gap-1.5 text-[11px] font-mono">
              <button
                className="opacity-60 hover:opacity-100 transition-opacity"
                onClick={() => updateIndicator('ma', { enabled: !indicatorConfig.ma.enabled })}
                title={indicatorConfig.ma.enabled ? '隐藏 MA' : '显示 MA'}
              >
                {indicatorConfig.ma.enabled
                  ? <Eye size={12} className="text-yellow-500" />
                  : <EyeOff size={12} className={colors.isDark ? 'text-slate-600' : 'text-slate-400'} />}
              </button>
              {indicatorConfig.ma.enabled && indicatorConfig.ma.periods.map((p, i) => (
                <span key={p} style={{ color: MA_COLORS[i % MA_COLORS.length] }}>
                  MA{p}
                </span>
              ))}
              {!indicatorConfig.ma.enabled && (
                <span className={colors.isDark ? 'text-slate-600 line-through' : 'text-slate-400 line-through'}>MA</span>
              )}
            </div>
            {/* EMA */}
            <div className="flex items-center gap-1.5 text-[11px] font-mono">
              <button
                className="opacity-60 hover:opacity-100 transition-opacity"
                onClick={() => updateIndicator('ema', { enabled: !indicatorConfig.ema.enabled })}
                title={indicatorConfig.ema.enabled ? '隐藏 EMA' : '显示 EMA'}
              >
                {indicatorConfig.ema.enabled
                  ? <Eye size={12} className="text-cyan-500" />
                  : <EyeOff size={12} className={colors.isDark ? 'text-slate-600' : 'text-slate-400'} />}
              </button>
              {indicatorConfig.ema.enabled && indicatorConfig.ema.periods.map((p, i) => (
                <span key={p} style={{ color: EMA_COLORS[i % EMA_COLORS.length] }}>
                  EMA{p}
                </span>
              ))}
              {!indicatorConfig.ema.enabled && (
                <span className={colors.isDark ? 'text-slate-600 line-through' : 'text-slate-400 line-through'}>EMA</span>
              )}
            </div>
            {/* BOLL */}
            <div className="flex items-center gap-1.5 text-[11px] font-mono">
              <button
                className="opacity-60 hover:opacity-100 transition-opacity"
                onClick={() => updateIndicator('boll', { enabled: !indicatorConfig.boll.enabled })}
                title={indicatorConfig.boll.enabled ? '隐藏 BOLL' : '显示 BOLL'}
              >
                {indicatorConfig.boll.enabled
                  ? <Eye size={12} style={{ color: BOLL_COLOR }} />
                  : <EyeOff size={12} className={colors.isDark ? 'text-slate-600' : 'text-slate-400'} />}
              </button>
              {indicatorConfig.boll.enabled ? (
                <span style={{ color: BOLL_COLOR }}>BOLL({indicatorConfig.boll.period},{indicatorConfig.boll.multiplier})</span>
              ) : (
                <span className={colors.isDark ? 'text-slate-600 line-through' : 'text-slate-400 line-through'}>BOLL</span>
              )}
            </div>
          </div>
        )}

        {/* 指标快捷配置浮动面板 */}
        {indicatorPopup && (
          <div
            className={`absolute z-30 rounded shadow-lg border text-xs p-2 min-w-[180px] ${
              colors.isDark
                ? 'bg-slate-800 border-slate-700 text-slate-200'
                : 'bg-white border-slate-300 text-slate-700'
            }`}
            style={{
              left: indicatorPopupLeft,
              top: indicatorPopupTop,
            }}
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
          >
            {/* MA / EMA 配置（共用结构） */}
            {(indicatorPopup.type === 'ma' || indicatorPopup.type === 'ema') && (() => {
              const key = indicatorPopup.type;
              const label = key === 'ma' ? 'MA 均线' : 'EMA 指数均线';
              const cfg = indicatorConfig[key];
              return (
                <div className="flex flex-col gap-1.5">
                  <div className="flex items-center justify-between">
                    <span className="font-bold">{label}</span>
                    <label className="flex items-center gap-1 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={cfg.enabled}
                        onChange={(e) => updateIndicator(key, { enabled: e.target.checked })}
                        className="accent-blue-500"
                      />
                      <span className="text-[10px]">显示</span>
                    </label>
                  </div>
                  <div>
                    <span className={`text-[10px] ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>周期（逗号分隔）</span>
                    <input
                      className={`w-full mt-0.5 px-1.5 py-0.5 rounded text-xs border ${
                        colors.isDark ? 'bg-slate-700 border-slate-600' : 'bg-slate-50 border-slate-300'
                      }`}
                      value={cfg.periods.join(',')}
                      onChange={(e) => {
                        const periods = e.target.value.split(',').map(Number).filter(n => n > 0);
                        if (periods.length > 0) updateIndicator(key, { periods });
                      }}
                    />
                  </div>
                </div>
              );
            })()}

            {/* BOLL 配置 */}
            {indicatorPopup.type === 'boll' && (
              <div className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between">
                  <span className="font-bold">BOLL 布林带</span>
                  <label className="flex items-center gap-1 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={indicatorConfig.boll.enabled}
                      onChange={(e) => updateIndicator('boll', { enabled: e.target.checked })}
                      className="accent-blue-500"
                    />
                    <span className="text-[10px]">显示</span>
                  </label>
                </div>
                <div className="grid grid-cols-2 gap-1.5">
                  <div>
                    <span className={`text-[10px] ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>周期</span>
                    <input
                      type="number"
                      className={`w-full mt-0.5 px-1.5 py-0.5 rounded text-xs border ${
                        colors.isDark ? 'bg-slate-700 border-slate-600' : 'bg-slate-50 border-slate-300'
                      }`}
                      value={indicatorConfig.boll.period}
                      onChange={(e) => updateIndicator('boll', { period: Number(e.target.value) || 20 })}
                    />
                  </div>
                  <div>
                    <span className={`text-[10px] ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>倍数</span>
                    <input
                      type="number"
                      step="0.1"
                      className={`w-full mt-0.5 px-1.5 py-0.5 rounded text-xs border ${
                        colors.isDark ? 'bg-slate-700 border-slate-600' : 'bg-slate-50 border-slate-300'
                      }`}
                      value={indicatorConfig.boll.multiplier}
                      onChange={(e) => updateIndicator('boll', { multiplier: Number(e.target.value) || 2 })}
                    />
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* 副图切换 tab（仅 K线模式） */}
      {!isIntraday && hasData && (
        <div className={`flex items-center gap-1 px-2 py-0.5 border-t fin-divider ${colors.isDark ? 'bg-slate-900/50' : 'bg-slate-50'}`}>
          {([
            { id: 'volume' as SubChartType, label: '成交量' },
            ...(indicatorConfig.macd.enabled ? [{ id: 'macd' as SubChartType, label: 'MACD' }] : []),
            ...(indicatorConfig.rsi.enabled ? [{ id: 'rsi' as SubChartType, label: 'RSI' }] : []),
            ...(indicatorConfig.kdj.enabled ? [{ id: 'kdj' as SubChartType, label: 'KDJ' }] : []),
          ]).map(tab => (
            <button
              key={tab.id}
              onClick={() => handleSubChartSwitch(tab.id)}
              className={`text-[10px] px-2 py-0.5 rounded transition-colors ${
                subChartType === tab.id
                  ? 'text-accent-2 font-bold ' + (colors.isDark ? 'bg-slate-800/80' : 'bg-slate-200/80')
                  : (colors.isDark ? 'text-slate-500 hover:text-slate-300' : 'text-slate-400 hover:text-slate-600')
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      )}

      {/* 成交量拖拽分隔条 */}
      <ResizeHandle direction="vertical" onResize={handleVolumeResize} />

      {/* 副图区域 */}
      <div className="border-t fin-divider" style={{ height: volumeHeight }} ref={volumeContainerRef} />
    </div>
  );
};
