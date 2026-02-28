import React, { useEffect, useRef, useCallback, useMemo, useState } from 'react';
import { ZoomIn, ZoomOut, MoveHorizontal } from 'lucide-react';
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

// 计算简单移动平均线
function calculateMA(data: KLineData[], period: number): LineData[] {
  const result: LineData[] = [];
  for (let i = period - 1; i < data.length; i++) {
    let sum = 0;
    for (let j = 0; j < period; j++) {
      sum += data[i - j].close;
    }
    result.push({
      time: parseTime(data[i].time),
      value: sum / period,
    });
  }
  return result;
}

// 解析时间字符串为 lightweight-charts 时间格式
function parseTime(timeStr: string): Time {
  if (timeStr.length > 10) {
    const [datePart, timePart] = timeStr.split(' ');
    const [year, month, day] = datePart.split('-').map(Number);
    const [hour, minute, second] = timePart.split(':').map(Number);
    const utcTimestamp = Date.UTC(year, month - 1, day, hour, minute, second || 0);
    return Math.floor(utcTimestamp / 1000) as Time;
  }
  return timeStr as Time;
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
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const volumeContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const volumeChartRef = useRef<IChartApi | null>(null);
  const mainSeriesRef = useRef<ISeriesApi<SeriesType, Time> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<SeriesType, Time> | null>(null);
  const maSeriesRefs = useRef<ISeriesApi<SeriesType, Time>[]>([]);
  const seriesTypeRef = useRef<'line' | 'candle' | null>(null);
  const hasFittedRef = useRef(false);
  const isIntradayRef = useRef(false);

  const [volumeHeight, setVolumeHeight] = useState(VOLUME_DEFAULT);

  const handleVolumeResize = useCallback((delta: number) => {
    setVolumeHeight(prev => Math.max(VOLUME_MIN, Math.min(VOLUME_MAX, prev - delta)));
  }, []);

  const safeData = data || [];
  const isIntraday = period === '1m';
  isIntradayRef.current = isIntraday;
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
    for (const s of maSeriesRefs.current) {
      try { chart.removeSeries(s); } catch { /* already removed */ }
    }
    maSeriesRefs.current = [];
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
      layout: { background: { color: '#0f172a' }, textColor: '#94a3b8', attributionLogo: false },
      grid: { vertLines: { color: '#1e293b' }, horzLines: { color: '#1e293b' } },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: '#1e293b', scaleMargins: { top: 0.1, bottom: 0.1 } },
      timeScale: { borderColor: '#1e293b', timeVisible: true, secondsVisible: false },
      localization: { timeFormatter: (time: Time) => typeof time === 'number' ? formatTimestamp(time) : String(time) },
      handleScroll: true,
      handleScale: true,
    });

    const volumeChart = createChart(volumeContainerRef.current, {
      layout: { background: { color: '#0f172a' }, textColor: '#94a3b8', attributionLogo: false },
      grid: { vertLines: { color: '#1e293b' }, horzLines: { color: '#1e293b' } },
      rightPriceScale: { borderColor: '#1e293b', scaleMargins: { top: 0.1, bottom: 0 } },
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
    let resizeTimer: ReturnType<typeof setTimeout> | null = null;
    const resizeObserver = new ResizeObserver(() => {
      if (chartContainerRef.current && volumeContainerRef.current) {
        const w = chartContainerRef.current.clientWidth;
        const h = chartContainerRef.current.clientHeight;
        const vh = volumeContainerRef.current.clientHeight;
        if (w > 0 && h > 0) chart.applyOptions({ width: w, height: h });
        if (w > 0 && vh > 0) volumeChart.applyOptions({ width: w, height: vh });
        // 分时模式下防抖 fitContent，确保最大化/还原后数据填满新宽度
        // K线模式保留用户当前浏览位置，不强制 fit
        if (isIntradayRef.current) {
          if (resizeTimer) clearTimeout(resizeTimer);
          resizeTimer = setTimeout(() => {
            chart.timeScale().fitContent();
            volumeChart.timeScale().fitContent();
          }, 100);
        }
      }
    });
    resizeObserver.observe(chartContainerRef.current);
    resizeObserver.observe(volumeContainerRef.current);

    // 仅在组件卸载时销毁
    return () => {
      if (resizeTimer) clearTimeout(resizeTimer);
      resizeObserver.disconnect();
      chart.remove();
      volumeChart.remove();
      chartRef.current = null;
      volumeChartRef.current = null;
      mainSeriesRef.current = null;
      volumeSeriesRef.current = null;
      maSeriesRefs.current = [];
      seriesTypeRef.current = null;
      hasFittedRef.current = false;
    };
  }, []); // 空依赖 —— 只执行一次

  // ========== 主题变化：applyOptions 更新样式，不销毁图表 ==========
  useEffect(() => {
    const chart = chartRef.current;
    const volumeChart = volumeChartRef.current;
    if (!chart || !volumeChart) return;

    const layoutOpts = {
      layout: { background: { color: chartColors.background }, textColor: chartColors.textColor },
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

    // 更新成交量柱颜色
    if (volumeSeriesRef.current && safeData.length > 0) {
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

        // 均线
        const maColors = ['#facc15', '#a855f7', '#f97316'];
        maColors.forEach(color => {
          const maSeries = chart.addSeries(LineSeries, {
            color, lineWidth: 1, priceLineVisible: false, lastValueVisible: false,
          });
          maSeriesRefs.current.push(maSeries);
        });

        // 成交量
        volumeSeriesRef.current = volumeChart.addSeries(HistogramSeries, { priceFormat: { type: 'volume' } });
      }

      // 更新数据
      const candleData: CandlestickData[] = safeData.map(d => ({
        time: parseTime(d.time), open: d.open, high: d.high, low: d.low, close: d.close,
      }));
      mainSeriesRef.current.setData(candleData);

      const maPeriods = [5, 10, 20];
      maSeriesRefs.current.forEach((maSeries, idx) => {
        const p = maPeriods[idx];
        if (safeData.length >= p) {
          maSeries.setData(calculateMA(safeData, p));
        } else {
          maSeries.setData([]);
        }
      });
    }

    // 更新成交量数据
    if (volumeSeriesRef.current) {
      const volumeData: HistogramData[] = safeData.map(d => ({
        time: parseTime(d.time), value: d.volume,
        color: d.close >= d.open ? chartColors.upColor + '99' : chartColors.downColor + '99',
      }));
      volumeSeriesRef.current.setData(volumeData);
    }

    // full: 用户主动切换股票/周期 → fitContent；refresh: 定时刷新 → 保留缩放；增量仅首次 fit
    const shouldFit = safeData.length > 0 && (
      updateMode === 'full' || (!hasFittedRef.current && safeData.length > 1)
    );
    if (shouldFit) {
      chart.timeScale().fitContent();
      volumeChart.timeScale().fitContent();
      hasFittedRef.current = true;
    }
  }, [safeData, updateMode, preClose, isIntraday, chartColors, clearAllSeries]);

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

  return (
    <div className="h-full w-full fin-panel flex flex-col relative">
      {/* 加载提示（叠加在图表上方） */}
      {!hasData && (
        <div className="absolute inset-0 z-20 flex items-center justify-center pointer-events-none">
          <span className={`text-sm animate-pulse px-3 py-1 rounded ${colors.isDark ? 'text-slate-400 bg-slate-900/40' : 'text-slate-600 bg-slate-100/70'}`}>
            暂无K线数据，可切换周期重试...
          </span>
        </div>
      )}

      {/* Header */}
      <div className="flex items-center justify-between px-2 py-1 border-b fin-divider fin-panel-strong z-10 flex-nowrap overflow-hidden">
        <div className="flex gap-1 shrink-0 flex-nowrap">
          {periods.map((p) => (
            <button
              key={p.id}
              onClick={() => onPeriodChange(p.id)}
              className={`text-xs px-3 py-1 rounded transition-colors whitespace-nowrap ${
                period === p.id
                  ? (colors.isDark ? 'bg-slate-800/80' : 'bg-slate-200/80') + ' text-accent-2 font-bold'
                  : (colors.isDark ? 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/40')
              }`}
            >
              {p.label}
            </button>
          ))}
          {!isIntraday && (
            <div className={`hidden lg:flex items-center gap-2 ml-3 pl-3 border-l ${colors.isDark ? 'border-slate-700' : 'border-slate-300'}`}>
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

        {/* 数据信息栏 */}
        <div className={`text-xs font-mono flex gap-3 min-w-0 overflow-hidden whitespace-nowrap ${colors.isDark ? 'text-slate-400' : 'text-slate-500'} ${!hasData ? 'opacity-60' : ''}`}>
          {isIntraday ? (
            <>
              <span>时间: <span className={colors.isDark ? 'text-slate-300' : 'text-slate-600'}>{displayData ? formatTimeDisplay(displayData.time) : '--'}</span></span>
              <span>价格: <span className={getPriceColor(displayData?.close || 0)}>{displayData?.close?.toFixed(2) || '--'}</span></span>
              <span>均价: <span className="text-yellow-500">{displayData?.avg?.toFixed(2) || '--'}</span></span>
              <span>涨跌: <span className={getPriceColor(currentPrice)}>{formatChange(displayData?.close || preClose)}</span></span>
              <span>幅度: <span className={getPriceColor(currentPrice)}>{formatChangePercent(displayData?.close || preClose)}</span></span>
            </>
          ) : (
            <>
              <span>时间: <span className={colors.isDark ? 'text-slate-300' : 'text-slate-600'}>{displayData ? formatTimeDisplay(displayData.time) : '--'}</span></span>
              <span>收: <span className="text-accent-2">{displayData?.close?.toFixed(2)}</span></span>
              <span>开: {displayData?.open?.toFixed(2)}</span>
              <span>高: <span className={cc.upClass}>{displayData?.high?.toFixed(2)}</span></span>
              <span>低: <span className={cc.downClass}>{displayData?.low?.toFixed(2)}</span></span>
              {displayData?.ma5 && (
                <>
                  <span>MA5: <span className="text-yellow-500">{displayData?.ma5?.toFixed(2)}</span></span>
                  <span>MA10: <span className="text-purple-500">{displayData?.ma10?.toFixed(2)}</span></span>
                  <span>MA20: <span className="text-orange-500">{displayData?.ma20?.toFixed(2)}</span></span>
                </>
              )}
            </>
          )}
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
      <div className="flex-1 min-h-0" ref={chartContainerRef} />

      {/* 成交量拖拽分隔条 */}
      <ResizeHandle direction="vertical" onResize={handleVolumeResize} />

      {/* 成交量图表区域 */}
      <div className="border-t fin-divider" style={{ height: volumeHeight }} ref={volumeContainerRef} />
    </div>
  );
};
