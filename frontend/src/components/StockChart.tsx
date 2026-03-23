import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  createChart,
  ColorType,
  CrosshairMode,
  UTCTimestamp,
  CandlestickData,
  LineData,
  HistogramData,
  WhitespaceData,
  CandlestickSeries,
  LineSeries,
  HistogramSeries,
} from 'lightweight-charts';
import { KLineData, TimePeriod, Stock } from '../types';
import { OpenURL } from '../../wailsjs/go/main/App';

interface StockChartProps {
  data: KLineData[];
  period: TimePeriod;
  onPeriodChange: (p: TimePeriod) => void;
  showF10?: boolean;
  onToggleF10?: () => void;
  stock?: Stock;
  floatShares?: number;
  fallbackTurnoverRate?: number;
  isTrading?: boolean;
  onNeedMore?: () => void;
  hasMore?: boolean;
  loading?: boolean;
  emptyText?: string;
}

const toUtcTimestamp = (value: string): UTCTimestamp => {
  const [date, time = '00:00:00'] = value.trim().split(' ');
  const [year, month, day] = date.split('-').map(Number);
  const [hour, minute, second] = time.split(':').map(Number);
  const utc = Date.UTC(year, month - 1, day, hour, minute, second || 0);
  return Math.floor(utc / 1000) as UTCTimestamp;
};

const resolveTime = (value: string): UTCTimestamp => toUtcTimestamp(value);

const resolveTradingSessionRange = (value?: string) => {
  if (!value) return null;
  const [date] = value.trim().split(' ');
  if (!date) return null;
  // 只保留极小边界缓冲，既避免标签裁切，又不制造大段空白区。
  const start = toUtcTimestamp(`${date} 09:29:00`);
  const end = toUtcTimestamp(`${date} 15:01:00`);
  if (!start || !end) return null;
  return { from: start, to: end } as const;
};

const buildIntradayTimeline = (value?: string): UTCTimestamp[] => {
  if (!value) return [];
  const [date] = value.trim().split(' ');
  if (!date) return [];
  const [year, month, day] = date.split('-').map(Number);
  if (!year || !month || !day) return [];

  const buildRange = (startHour: number, startMinute: number, endHour: number, endMinute: number) => {
    const start = Date.UTC(year, month - 1, day, startHour, startMinute, 0);
    const end = Date.UTC(year, month - 1, day, endHour, endMinute, 0);
    const points: UTCTimestamp[] = [];
    for (let t = start; t <= end; t += 60 * 1000) {
      points.push(Math.floor(t / 1000) as UTCTimestamp);
    }
    return points;
  };

  return [
    ...buildRange(9, 30, 11, 30),
    ...buildRange(13, 0, 15, 0),
  ];
};

const getISOWeek = (date: Date) => {
  const tmp = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
  const dayNum = tmp.getUTCDay() || 7;
  tmp.setUTCDate(tmp.getUTCDate() + 4 - dayNum);
  const yearStart = new Date(Date.UTC(tmp.getUTCFullYear(), 0, 1));
  const week = Math.ceil((((tmp.getTime() - yearStart.getTime()) / 86400000) + 1) / 7);
  return { year: tmp.getUTCFullYear(), week };
};

const formatTickLabel = (time: UTCTimestamp, period: TimePeriod): string => {
  const date = new Date(time * 1000);
  const year = date.getUTCFullYear();
  const month = date.getUTCMonth() + 1;
  const day = date.getUTCDate();

  if (period === '1m') {
    const hours = date.getUTCHours().toString().padStart(2, '0');
    const minutes = date.getUTCMinutes().toString().padStart(2, '0');
    return `${hours}:${minutes}`;
  }

  if (period === '1w') {
    const { year: weekYear, week } = getISOWeek(date);
    return `${weekYear}-W${week.toString().padStart(2, '0')}`;
  }

  if (period === '1mo') {
    return `${year}/${month.toString().padStart(2, '0')}`;
  }

  return `${month.toString().padStart(2, '0')}/${day.toString().padStart(2, '0')}`;
};

const formatCrosshairLabel = (time: UTCTimestamp, period: TimePeriod): string => {
  const date = new Date(time * 1000);
  const year = date.getUTCFullYear();
  const month = date.getUTCMonth() + 1;
  const day = date.getUTCDate();

  if (period === '1m') {
    const hours = date.getUTCHours().toString().padStart(2, '0');
    const minutes = date.getUTCMinutes().toString().padStart(2, '0');
    return `${year}-${month.toString().padStart(2, '0')}-${day.toString().padStart(2, '0')} ${hours}:${minutes}`;
  }

  if (period === '1w') {
    const { year: weekYear, week } = getISOWeek(date);
    return `${weekYear}-W${week.toString().padStart(2, '0')} (${year}-${month.toString().padStart(2, '0')}-${day.toString().padStart(2, '0')})`;
  }

  if (period === '1mo') {
    return `${year}-${month.toString().padStart(2, '0')} (${year}-${month.toString().padStart(2, '0')}-${day.toString().padStart(2, '0')})`;
  }

  return `${year}-${month.toString().padStart(2, '0')}-${day.toString().padStart(2, '0')}`;
};

const formatLargeNumber = (value?: number) => {
  if (value === undefined || value === null) return '--';
  const abs = Math.abs(value);
  if (abs >= 100000000) return `${(value / 100000000).toFixed(2)}亿`;
  if (abs >= 10000) return `${(value / 10000).toFixed(2)}万`;
  if (abs >= 1) return value.toFixed(2);
  return value.toFixed(4);
};

const formatPercent = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '--';
  return `${value.toFixed(2)}%`;
};

const formatSignedPercent = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '--';
  const sign = value > 0 ? '+' : '';
  return `${sign}${value.toFixed(2)}%`;
};

const isIndexSymbol = (symbol?: string) => {
  if (!symbol) return false;
  const lower = symbol.toLowerCase();
  return lower.startsWith('sh000') || lower.startsWith('sz399') || lower.startsWith('s_sh000') || lower.startsWith('s_sz399');
};

const buildCrossMarkers = (rows: KLineData[]) => {
  const markers: any[] = [];
  for (let i = 1; i < rows.length; i += 1) {
    const prev = rows[i - 1];
    const curr = rows[i];
    if (
      prev.ma5 === undefined || prev.ma10 === undefined ||
      curr.ma5 === undefined || curr.ma10 === undefined
    ) {
      continue;
    }
    const prevDiff = prev.ma5 - prev.ma10;
    const currDiff = curr.ma5 - curr.ma10;
    if (prevDiff <= 0 && currDiff > 0) {
      markers.push({
        time: resolveTime(curr.time),
        position: 'belowBar',
        color: '#22c55e',
        shape: 'arrowUp',
        text: '金叉',
      });
    } else if (prevDiff >= 0 && currDiff < 0) {
      markers.push({
        time: resolveTime(curr.time),
        position: 'aboveBar',
        color: '#ef4444',
        shape: 'arrowDown',
        text: '死叉',
      });
    }
  }
  return markers;
};

const buildVolumeBreakMarkers = (rows: KLineData[], lookback = 20, volumeMultiplier = 1.8) => {
  const markers: any[] = [];
  for (let i = lookback; i < rows.length; i += 1) {
    const curr = rows[i];
    const window = rows.slice(i - lookback, i);
    if (window.length < lookback) continue;
    const highN = Math.max(...window.map(item => item.high));
    const lowN = Math.min(...window.map(item => item.low));
    const avgVolume = window.reduce((acc, item) => acc + item.volume, 0) / lookback;
    const isBreakUp = curr.close > highN && curr.volume > avgVolume * volumeMultiplier;
    const isBreakDown = curr.close < lowN && curr.volume > avgVolume * volumeMultiplier;
    if (isBreakUp) {
      markers.push({
        time: resolveTime(curr.time),
        position: 'belowBar',
        color: '#38bdf8',
        shape: 'arrowUp',
        text: '放量突破',
      });
    } else if (isBreakDown) {
      markers.push({
        time: resolveTime(curr.time),
        position: 'aboveBar',
        color: '#f97316',
        shape: 'arrowDown',
        text: '放量跌破',
      });
    }
  }
  return markers;
};

const resolveTurnoverRate = (
  volume: number | undefined,
  floatShares: number | undefined,
  fallbackTurnoverRate: number | undefined,
): number | undefined => {
  if (!volume || volume <= 0 || !floatShares || floatShares <= 0) {
    return fallbackTurnoverRate;
  }

  // 不同数据源的 volume 可能是“股”或“手”，同时尝试两种换算并选择更合理值。
  const bySharesUnit = (volume * 100) / floatShares;
  const byLotsUnit = (volume * 10000) / floatShares;
  const candidates = [bySharesUnit, byLotsUnit].filter(v => Number.isFinite(v) && v > 0);
  if (candidates.length === 0) return fallbackTurnoverRate;

  const plausible = candidates.filter(v => v <= 300);
  if (plausible.length === 0) return fallbackTurnoverRate;

  if (fallbackTurnoverRate && fallbackTurnoverRate > 0) {
    return plausible.reduce((best, curr) => (
      Math.abs(curr - fallbackTurnoverRate) < Math.abs(best - fallbackTurnoverRate) ? curr : best
    ), plausible[0]);
  }
  return plausible[0];
};

export const StockChart: React.FC<StockChartProps> = ({
  data,
  period,
  onPeriodChange,
  showF10 = false,
  onToggleF10,
  stock,
  floatShares,
  fallbackTurnoverRate,
  onNeedMore,
  hasMore = true,
  loading = false,
  emptyText = '暂无K线数据',
}) => {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<ReturnType<typeof createChart> | null>(null);
  const candleSeriesRef = useRef<any>(null);
  const lineSeriesRef = useRef<any>(null);
  const avgSeriesRef = useRef<any>(null);
  const ma5SeriesRef = useRef<any>(null);
  const ma10SeriesRef = useRef<any>(null);
  const ma20SeriesRef = useRef<any>(null);
  const supportSeriesRef = useRef<any>(null);
  const resistanceSeriesRef = useRef<any>(null);
  const volumeSeriesRef = useRef<any>(null);
  const timeAnchorSeriesRef = useRef<any>(null);
  const signalMenuRef = useRef<HTMLDivElement | null>(null);
  const lastPeriodRef = useRef<TimePeriod | null>(null);
  const lastSymbolRef = useRef<string | undefined>(undefined);
  const lastDataLengthRef = useRef(0);
  const fitOnResizeRef = useRef(false);
  const sessionRangeRef = useRef<{ from: UTCTimestamp; to: UTCTimestamp } | null>(null);
  const [hoverInfo, setHoverInfo] = useState<{
    timeLabel: string;
    open?: number;
    high?: number;
    low?: number;
    close?: number;
    price?: number;
    avg?: number;
    volume?: number;
    amount?: number;
    changePercent?: number;
    amplitude?: number;
    turnoverRate?: number;
    point: { x: number; y: number };
  } | null>(null);
  const [enableSignals, setEnableSignals] = useState(false);
  const [showSignalMenu, setShowSignalMenu] = useState(false);
  const [showSupportResistance, setShowSupportResistance] = useState(true);
  const [showCrossSignals, setShowCrossSignals] = useState(true);
  const [showVolumeSignals, setShowVolumeSignals] = useState(true);

  const isIntraday = period === '1m';
  const isIndex = stock?.sector === '指数' || isIndexSymbol(stock?.symbol);

  const lastPoint = data[data.length - 1];
  const dataIndexMap = useMemo(() => {
    const map = new Map<number, number>();
    data.forEach((item, index) => {
      map.set(resolveTime(item.time), index);
    });
    return map;
  }, [data]);
  const chartStats = useMemo(() => {
    if (!data || data.length === 0) {
      return {
        lastClose: 0,
        high: 0,
        low: 0,
        avg: 0,
        ma5: 0,
        ma10: 0,
        ma20: 0,
        lastOpen: 0,
      };
    }

    const high = Math.max(...data.map(item => item.high));
    const low = Math.min(...data.map(item => item.low));

    return {
      lastClose: lastPoint?.close ?? 0,
      lastOpen: lastPoint?.open ?? 0,
      high,
      low,
      avg: lastPoint?.avg ?? 0,
      ma5: lastPoint?.ma5 ?? 0,
      ma10: lastPoint?.ma10 ?? 0,
      ma20: lastPoint?.ma20 ?? 0,
    };
  }, [data, lastPoint]);

  useEffect(() => {
    if (!showSignalMenu) return;
    const handleClickOutside = (event: MouseEvent) => {
      if (signalMenuRef.current && !signalMenuRef.current.contains(event.target as Node)) {
        setShowSignalMenu(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [showSignalMenu]);

  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: '#94a3b8',
        attributionLogo: false,
      },
      grid: {
        vertLines: { color: '#1e293b' },
        horzLines: { color: '#1e293b' },
      },
      rightPriceScale: {
        borderColor: '#1e293b',
      },
      timeScale: {
        borderColor: '#1e293b',
        timeVisible: true,
      },
      crosshair: {
        mode: CrosshairMode.Magnet,
        vertLine: { color: '#334155', width: 1, style: 0 },
        horzLine: { color: '#334155', width: 1, style: 0 },
      },
      handleScroll: {
        mouseWheel: true,
        pressedMouseMove: true,
        horzTouchDrag: true,
      },
      handleScale: {
        mouseWheel: true,
        pinch: true,
        axisPressedMouseMove: true,
      },
    });

    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#ef4444',
      downColor: '#22c55e',
      wickUpColor: '#ef4444',
      wickDownColor: '#22c55e',
      borderVisible: false,
    });

    const lineSeries = chart.addSeries(LineSeries, {
      color: '#38bdf8',
      lineWidth: 2,
      priceLineVisible: false,
    });

    const avgSeries = chart.addSeries(LineSeries, {
      color: '#facc15',
      lineWidth: 1,
      priceLineVisible: false,
    });

    const ma5Series = chart.addSeries(LineSeries, {
      color: '#facc15',
      lineWidth: 1,
      priceLineVisible: false,
    });

    const ma10Series = chart.addSeries(LineSeries, {
      color: '#a855f7',
      lineWidth: 1,
      priceLineVisible: false,
    });

    const ma20Series = chart.addSeries(LineSeries, {
      color: '#f97316',
      lineWidth: 1,
      priceLineVisible: false,
    });

    const supportSeries = chart.addSeries(LineSeries, {
      color: 'rgba(34, 197, 94, 0.75)',
      lineWidth: 1,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });

    const resistanceSeries = chart.addSeries(LineSeries, {
      color: 'rgba(239, 68, 68, 0.75)',
      lineWidth: 1,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });

    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceScaleId: 'volume',
      priceFormat: { type: 'volume' },
    });
    const timeAnchorSeries = chart.addSeries(LineSeries, {
      color: 'rgba(0,0,0,0)',
      lineWidth: 1,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });

    chart.priceScale('right').applyOptions({
      scaleMargins: { top: 0.12, bottom: 0.25 },
    });

    chart.priceScale('volume').applyOptions({
      scaleMargins: { top: 0.75, bottom: 0 },
      visible: false,
    });

    chartRef.current = chart;
    candleSeriesRef.current = candleSeries;
    lineSeriesRef.current = lineSeries;
    avgSeriesRef.current = avgSeries;
    ma5SeriesRef.current = ma5Series;
    ma10SeriesRef.current = ma10Series;
    ma20SeriesRef.current = ma20Series;
    supportSeriesRef.current = supportSeries;
    resistanceSeriesRef.current = resistanceSeries;
    volumeSeriesRef.current = volumeSeries;
    timeAnchorSeriesRef.current = timeAnchorSeries;

    const resizeObserver = new ResizeObserver(entries => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        chart.applyOptions({ width, height });
        if (fitOnResizeRef.current) {
          chart.timeScale().fitContent();
          return;
        }
        if (sessionRangeRef.current) {
          chart.timeScale().setVisibleRange(sessionRangeRef.current);
        }
      }
    });

    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      chart.remove();
      chartRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (!chartRef.current) return;
    const chart = chartRef.current;
    const enable = !isIntraday;
    fitOnResizeRef.current = !isIntraday;
    if (!isIntraday) {
      sessionRangeRef.current = null;
    }
    chart.applyOptions({
      handleScroll: {
        mouseWheel: enable,
        pressedMouseMove: enable,
        horzTouchDrag: enable,
      },
      handleScale: {
        mouseWheel: enable,
        pinch: enable,
        axisPressedMouseMove: enable,
      },
    });
  }, [isIntraday]);

  useEffect(() => {
    if (!chartRef.current) return;
    if (!data || data.length === 0) return;

    const chart = chartRef.current;
    const prevRange = chart.timeScale().getVisibleRange();
    const candleSeries = candleSeriesRef.current;
    const lineSeries = lineSeriesRef.current;
    const avgSeries = avgSeriesRef.current;
    const ma5Series = ma5SeriesRef.current;
    const ma10Series = ma10SeriesRef.current;
    const ma20Series = ma20SeriesRef.current;
    const supportSeries = supportSeriesRef.current;
    const resistanceSeries = resistanceSeriesRef.current;
    const volumeSeries = volumeSeriesRef.current;
    const timeAnchorSeries = timeAnchorSeriesRef.current;

    chart.applyOptions({
      localization: {
        timeFormatter: (time: UTCTimestamp) => formatTickLabel(time, period),
      },
      timeScale: {
        timeVisible: isIntraday,
        secondsVisible: isIntraday,
        tickMarkFormatter: (time: UTCTimestamp) => formatTickLabel(time, period),
        rightOffset: 0,
        fixLeftEdge: false,
        fixRightEdge: false,
      },
    });

    const timeline = isIntraday ? buildIntradayTimeline(lastPoint?.time) : [];
    const getItemByTime = (time: UTCTimestamp) => {
      const index = dataIndexMap.get(time);
      return index !== undefined ? data[index] : undefined;
    };

    const volumeData: Array<HistogramData | WhitespaceData> = timeline.length > 0
      ? timeline.map(time => {
        const item = getItemByTime(time);
        if (!item) return { time };
        return {
          time,
          value: item.volume,
          color: item.close >= item.open ? '#ef4444' : '#22c55e',
        };
      })
      : data.map(item => ({
        time: resolveTime(item.time),
        value: item.volume,
        color: item.close >= item.open ? '#ef4444' : '#22c55e',
      }));
    volumeSeries.setData(volumeData as any);

    if (enableSignals && showSupportResistance && !isIntraday && supportSeries && resistanceSeries && data.length > 1) {
      const lookback = Math.min(isIntraday ? 120 : 60, data.length);
      const recent = data.slice(-lookback);
      const support = Math.min(...recent.map(item => item.low));
      const resistance = Math.max(...recent.map(item => item.high));
      const firstTime = resolveTime(data[0].time);
      const endTime = resolveTime(data[data.length - 1].time);
      supportSeries.applyOptions({ visible: true });
      resistanceSeries.applyOptions({ visible: true });
      supportSeries.setData([{ time: firstTime, value: support }, { time: endTime, value: support }]);
      resistanceSeries.setData([{ time: firstTime, value: resistance }, { time: endTime, value: resistance }]);
    } else {
      supportSeries?.setData([]);
      resistanceSeries?.setData([]);
      supportSeries?.applyOptions({ visible: false });
      resistanceSeries?.applyOptions({ visible: false });
    }

    if (isIntraday) {
      candleSeries.applyOptions({ visible: false });
      lineSeries.applyOptions({ visible: true });
      avgSeries.applyOptions({ visible: !isIndex });
      ma5Series.applyOptions({ visible: false });
      ma10Series.applyOptions({ visible: false });
      ma20Series.applyOptions({ visible: false });

      const lineData: Array<LineData | WhitespaceData> = timeline.length > 0
        ? timeline.map(time => {
          const item = getItemByTime(time);
          if (!item) return { time };
          return { time, value: item.close };
        })
        : data.map(item => ({
          time: resolveTime(item.time),
          value: item.close,
        }));
      const avgData: Array<LineData | WhitespaceData> = isIndex ? [] : (
        timeline.length > 0
          ? timeline.map(time => {
            const item = getItemByTime(time);
            if (!item || item.avg === undefined || item.avg === null) return { time };
            return { time, value: item.avg as number };
          })
          : data
            .filter(item => item.avg !== undefined && item.avg !== null)
            .map(item => ({
              time: resolveTime(item.time),
              value: item.avg as number,
            }))
      );

      candleSeries.setData([]);
      ma5Series.setData([]);
      ma10Series.setData([]);
      ma20Series.setData([]);
      lineSeries.setData(lineData);
      avgSeries.setData(avgData);
      if (typeof lineSeries.setMarkers === 'function') {
        lineSeries.setMarkers([]);
      }
      if (typeof candleSeries.setMarkers === 'function') {
        candleSeries.setMarkers([]);
      }
      if (timeAnchorSeries) {
        const sessionRange = resolveTradingSessionRange(lastPoint?.time);
        sessionRangeRef.current = sessionRange;
        const anchors = sessionRange ? [{ time: sessionRange.from }, { time: sessionRange.to }] : [];
        timeAnchorSeries.setData(anchors as any);
      }
    } else {
      candleSeries.applyOptions({ visible: true });
      lineSeries.applyOptions({ visible: false });
      avgSeries.applyOptions({ visible: false });

      const candleData = data.map(item => ({
        time: resolveTime(item.time),
        open: item.open,
        high: item.high,
        low: item.low,
        close: item.close,
      }));

      const ma5Data = data
        .filter(item => item.ma5 !== undefined && item.ma5 !== null)
        .map(item => ({
          time: resolveTime(item.time),
          value: item.ma5 as number,
        }));

      const ma10Data = data
        .filter(item => item.ma10 !== undefined && item.ma10 !== null)
        .map(item => ({
          time: resolveTime(item.time),
          value: item.ma10 as number,
        }));

      const ma20Data = data
        .filter(item => item.ma20 !== undefined && item.ma20 !== null)
        .map(item => ({
          time: resolveTime(item.time),
          value: item.ma20 as number,
        }));

      candleSeries.setData(candleData);
      ma5Series.applyOptions({ visible: ma5Data.length > 0 });
      ma10Series.applyOptions({ visible: ma10Data.length > 0 });
      ma20Series.applyOptions({ visible: ma20Data.length > 0 });
      ma5Series.setData(ma5Data);
      ma10Series.setData(ma10Data);
      ma20Series.setData(ma20Data);
      lineSeries.setData([]);
      avgSeries.setData([]);
      const signalMarkers = enableSignals
        ? [
          ...(showCrossSignals ? buildCrossMarkers(data) : []),
          ...(showVolumeSignals ? buildVolumeBreakMarkers(data) : []),
        ].sort((a, b) => Number(a.time) - Number(b.time))
        : [];
      if (typeof candleSeries.setMarkers === 'function') {
        candleSeries.setMarkers(signalMarkers);
      }
      if (typeof lineSeries.setMarkers === 'function') {
        lineSeries.setMarkers([]);
      }
      timeAnchorSeries?.setData([]);
    }

    const periodChanged = lastPeriodRef.current && lastPeriodRef.current !== period;
    const symbolChanged = lastSymbolRef.current && lastSymbolRef.current !== stock?.symbol;
    const shouldPreserveRange =
      !periodChanged &&
      !symbolChanged &&
      lastDataLengthRef.current > 0 &&
      data.length > lastDataLengthRef.current;

    if (isIntraday) {
      const sessionRange = resolveTradingSessionRange(lastPoint?.time);
      sessionRangeRef.current = sessionRange;
      if (sessionRange) {
        chart.timeScale().setVisibleRange(sessionRange);
      } else if (shouldPreserveRange && prevRange) {
        chart.timeScale().setVisibleRange(prevRange);
      } else {
        chart.timeScale().fitContent();
      }
    } else if (shouldPreserveRange && prevRange) {
      chart.timeScale().setVisibleRange(prevRange);
    } else {
      chart.timeScale().fitContent();
    }

    lastPeriodRef.current = period;
    lastSymbolRef.current = stock?.symbol;
    lastDataLengthRef.current = data.length;
  }, [
    data,
    isIntraday,
    period,
    stock?.symbol,
    showSupportResistance,
    showCrossSignals,
    showVolumeSignals,
    enableSignals,
  ]);

  useEffect(() => {
    if (!chartRef.current) return;
    if (data && data.length > 0) return;
    candleSeriesRef.current?.setData([]);
    if (typeof candleSeriesRef.current?.setMarkers === 'function') {
      candleSeriesRef.current.setMarkers([]);
    }
    lineSeriesRef.current?.setData([]);
    if (typeof lineSeriesRef.current?.setMarkers === 'function') {
      lineSeriesRef.current.setMarkers([]);
    }
    avgSeriesRef.current?.setData([]);
    ma5SeriesRef.current?.setData([]);
    ma10SeriesRef.current?.setData([]);
    ma20SeriesRef.current?.setData([]);
    supportSeriesRef.current?.setData([]);
    resistanceSeriesRef.current?.setData([]);
    volumeSeriesRef.current?.setData([]);
  }, [data]);

  useEffect(() => {
    if (!chartRef.current) return;
    if (!data || data.length === 0) {
      setHoverInfo(null);
      return;
    }

    const chart = chartRef.current;

    const handleMove = (param: {
      time?: UTCTimestamp | string;
      point?: { x: number; y: number };
      seriesData: Map<any, CandlestickData | LineData | HistogramData>;
    }) => {
      if (!param.time || !param.point) {
        setHoverInfo(null);
        return;
      }

      const candle = param.seriesData.get(candleSeriesRef.current) as CandlestickData | undefined;
      const line = param.seriesData.get(lineSeriesRef.current) as LineData | undefined;
      const avg = param.seriesData.get(avgSeriesRef.current) as LineData | undefined;
      const volume = param.seriesData.get(volumeSeriesRef.current) as HistogramData | undefined;

      const container = containerRef.current;
      const width = container?.clientWidth ?? 0;
      const height = container?.clientHeight ?? 0;
      const tooltipWidth = 220;
      const tooltipHeight = isIntraday ? 110 : 170;
      let x = param.point.x + 12;
      let y = param.point.y + 12;
      if (x + tooltipWidth > width) {
        x = Math.max(8, param.point.x - tooltipWidth - 12);
      }
      if (y + tooltipHeight > height) {
        y = Math.max(8, param.point.y - tooltipHeight - 12);
      }

      const timeValue = typeof param.time === 'string' ? toUtcTimestamp(param.time) : param.time;

      const index = dataIndexMap.get(timeValue as number);
      const item = index !== undefined ? data[index] : undefined;
      const prevClose = index !== undefined && index > 0 ? data[index - 1].close : undefined;
      const changePercent =
        prevClose && item ? ((item.close - prevClose) / prevClose) * 100 : undefined;
      const amplitude =
        prevClose && item ? ((item.high - item.low) / prevClose) * 100 : undefined;
      const turnoverRate = !isIntraday
        ? resolveTurnoverRate(item?.volume, floatShares, fallbackTurnoverRate)
        : undefined;

      setHoverInfo({
        timeLabel: formatCrosshairLabel(timeValue, period),
        open: candle?.open ?? item?.open,
        high: candle?.high ?? item?.high,
        low: candle?.low ?? item?.low,
        close: candle?.close ?? item?.close,
        price: line?.value ?? item?.close,
        avg: avg?.value ?? item?.avg,
        volume: volume?.value ?? item?.volume,
        amount: item?.amount,
        changePercent,
        amplitude,
        turnoverRate,
        point: { x, y },
      });
    };

    chart.subscribeCrosshairMove(handleMove as any);
    return () => {
      chart.unsubscribeCrosshairMove(handleMove as any);
    };
  }, [data, dataIndexMap, floatShares, fallbackTurnoverRate, isIntraday, period]);

  useEffect(() => {
    if (!chartRef.current) return;
    if (isIntraday) return;
    if (!onNeedMore || !hasMore || data.length === 0) return;

    const chart = chartRef.current;
    const timeScale = chart.timeScale();
    let lastTrigger = 0;

    const handleRangeChange = (range: { from: number; to: number } | null) => {
      if (!range || !hasMore) return;
      if (range.from < 5) {
        const now = Date.now();
        if (now - lastTrigger < 800) return;
        lastTrigger = now;
        onNeedMore();
      }
    };

    timeScale.subscribeVisibleLogicalRangeChange(handleRangeChange);
    return () => {
      timeScale.unsubscribeVisibleLogicalRangeChange(handleRangeChange);
    };
  }, [data.length, hasMore, onNeedMore]);

  const isEmpty = !data || data.length === 0;

  const periods: { id: TimePeriod; label: string }[] = [
    { id: '1m', label: '分时' },
    { id: '1d', label: '日' },
    { id: '1w', label: '周' },
    { id: '1mo', label: '月' },
  ];

  return (
    <div className="h-full w-full fin-panel flex flex-col relative z-0">
      <div className="flex items-center justify-between gap-2 px-2 py-1 border-b fin-divider fin-panel-strong z-10">
        <div className="flex items-center gap-1 min-w-0 flex-1 overflow-x-auto fin-scrollbar pr-1">
          {periods.map((p) => (
            <button
              key={p.id}
              onClick={() => onPeriodChange(p.id)}
              className={`text-xs px-3 py-1 rounded transition-colors whitespace-nowrap shrink-0 ${
                period === p.id
                  ? 'bg-slate-800/80 text-accent-2 font-bold'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
              }`}
            >
              {p.label}
            </button>
          ))}
          {onToggleF10 && (
            <button
              type="button"
              onClick={onToggleF10}
              className={`text-xs px-3 py-1 rounded transition-colors whitespace-nowrap shrink-0 ${
                showF10
                  ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-400/40'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 border border-transparent'
              }`}
              title={showF10 ? '返回K线' : '查看F10'}
            >
              F10
            </button>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {!isIntraday && (
            <div className="relative shrink-0" ref={signalMenuRef}>
              <button
                type="button"
                onClick={() => setShowSignalMenu(prev => !prev)}
                aria-haspopup="menu"
                aria-expanded={showSignalMenu}
                className={`text-xs px-2.5 py-1 rounded border transition-colors whitespace-nowrap flex items-center gap-1 cursor-pointer ${
                  showSignalMenu
                    ? 'border-accent/70 text-accent-2 bg-accent/15'
                    : enableSignals
                      ? 'border-accent/50 text-accent-2 bg-accent/8 hover:bg-accent/15'
                      : 'border-slate-600/40 text-slate-300 hover:text-slate-100 hover:border-slate-500 hover:bg-slate-700/40'
                }`}
                title="信号设置（点击展开）"
              >
                信号
                <span
                  className={`text-[10px] transition-transform ${showSignalMenu ? 'rotate-180' : ''} ${
                    enableSignals ? 'text-accent-2' : 'text-slate-400'
                  }`}
                >
                  ▼
                </span>
              </button>
              {showSignalMenu && (
                <div className="absolute right-0 top-full mt-1 z-50 w-40 rounded-md border fin-divider fin-panel p-2 shadow-lg">
                  <label className="flex items-center gap-2 text-xs text-slate-200 cursor-pointer py-1">
                    <input
                      type="checkbox"
                      checked={enableSignals}
                      onChange={(e) => setEnableSignals(e.target.checked)}
                      className="h-3.5 w-3.5 accent-accent-2"
                    />
                    <span>总开关</span>
                  </label>
                  <div className="my-1 border-t fin-divider-soft" />
                  <label className={`flex items-center gap-2 text-xs cursor-pointer py-1 ${enableSignals ? 'text-slate-300' : 'text-slate-500'}`}>
                    <input
                      type="checkbox"
                      checked={showSupportResistance}
                      disabled={!enableSignals}
                      onChange={(e) => setShowSupportResistance(e.target.checked)}
                      className="h-3.5 w-3.5 accent-emerald-400"
                    />
                    <span>压阻线</span>
                  </label>
                  <label className={`flex items-center gap-2 text-xs cursor-pointer py-1 ${enableSignals ? 'text-slate-300' : 'text-slate-500'}`}>
                    <input
                      type="checkbox"
                      checked={showCrossSignals}
                      disabled={!enableSignals}
                      onChange={(e) => setShowCrossSignals(e.target.checked)}
                      className="h-3.5 w-3.5 accent-yellow-400"
                    />
                    <span>金死叉</span>
                  </label>
                  <label className={`flex items-center gap-2 text-xs cursor-pointer py-1 ${enableSignals ? 'text-slate-300' : 'text-slate-500'}`}>
                    <input
                      type="checkbox"
                      checked={showVolumeSignals}
                      disabled={!enableSignals}
                      onChange={(e) => setShowVolumeSignals(e.target.checked)}
                      className="h-3.5 w-3.5 accent-sky-400"
                    />
                    <span>放量突破</span>
                  </label>
                </div>
              )}
            </div>
          )}
          <div className="hidden md:flex text-xs text-slate-400 font-mono items-center gap-3 whitespace-nowrap">
            {isIntraday ? (
              <>
                <span>现 <span className="text-accent-2">{(stock?.price || chartStats.lastClose).toFixed(2)}</span></span>
                <span>均 <span className="text-yellow-400">{chartStats.avg ? chartStats.avg.toFixed(2) : '--'}</span></span>
                <span>高 <span className="text-red-400">{chartStats.high.toFixed(2)}</span></span>
                <span>低 <span className="text-green-400">{chartStats.low.toFixed(2)}</span></span>
              </>
            ) : (
              <>
                <span>收 <span className="text-accent-2">{chartStats.lastClose.toFixed(2)}</span></span>
                <span className="hidden lg:inline">开 {chartStats.lastOpen.toFixed(2)}</span>
                <span>高 <span className="text-red-400">{chartStats.high.toFixed(2)}</span></span>
                <span>低 <span className="text-green-400">{chartStats.low.toFixed(2)}</span></span>
                {chartStats.ma5 > 0 && (
                  <>
                    <span className="hidden xl:inline">MA5 <span className="text-yellow-400">{chartStats.ma5.toFixed(2)}</span></span>
                    <span className="hidden 2xl:inline">MA10 <span className="text-purple-400">{chartStats.ma10.toFixed(2)}</span></span>
                    <span className="hidden 2xl:inline">MA20 <span className="text-orange-400">{chartStats.ma20.toFixed(2)}</span></span>
                  </>
                )}
              </>
            )}
          </div>
          <button
            type="button"
            onClick={() => OpenURL('https://www.tradingview.com/')}
            className="hidden xl:inline text-[10px] text-slate-500 hover:text-slate-300 transition-colors whitespace-nowrap"
            title="Charting by TradingView"
          >
            Charting by TradingView
          </button>
        </div>
      </div>
      <div className="flex-1 min-h-0 relative">
        <div ref={containerRef} className="w-full h-full" />
        {isEmpty && (
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
            <span className={`text-sm ${loading ? 'text-slate-500 animate-pulse' : 'text-slate-500'}`}>
              {loading ? '加载市场数据中...' : emptyText}
            </span>
          </div>
        )}
        {hoverInfo && (
          <div
            className="absolute z-20 rounded border border-slate-700 bg-slate-900/90 px-2 py-1 text-[11px] text-slate-200 shadow-lg pointer-events-none"
            style={{ left: hoverInfo.point.x, top: hoverInfo.point.y, width: 200 }}
          >
            <div className="text-slate-400 mb-1">{hoverInfo.timeLabel}</div>
            {isIntraday ? (
              <div className="grid grid-cols-2 gap-x-2 gap-y-0.5">
                <span>价</span><span className="text-slate-100">{formatLargeNumber(hoverInfo.price)}</span>
                <span>均</span><span className="text-yellow-300">{formatLargeNumber(hoverInfo.avg)}</span>
                <span>量</span><span>{formatLargeNumber(hoverInfo.volume)}</span>
                <span>额</span><span>{formatLargeNumber(hoverInfo.amount)}</span>
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-x-2 gap-y-0.5">
                <span>开</span><span>{formatLargeNumber(hoverInfo.open)}</span>
                <span>高</span><span className="text-red-300">{formatLargeNumber(hoverInfo.high)}</span>
                <span>低</span><span className="text-green-300">{formatLargeNumber(hoverInfo.low)}</span>
                <span>收</span><span>{formatLargeNumber(hoverInfo.close)}</span>
                <span>量</span><span>{formatLargeNumber(hoverInfo.volume)}</span>
                <span>额</span><span>{formatLargeNumber(hoverInfo.amount)}</span>
                <span>涨幅</span><span>{formatSignedPercent(hoverInfo.changePercent)}</span>
                <span>振幅</span><span>{formatPercent(hoverInfo.amplitude)}</span>
                <span>换手</span><span>{formatPercent(hoverInfo.turnoverRate)}</span>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
