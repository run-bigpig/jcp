import React, { useState, useRef, useCallback, useEffect } from 'react';
import { ZoomIn, ZoomOut, MoveHorizontal } from 'lucide-react';
import {
  ComposedChart,
  LineChart,
  Line,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell
} from 'recharts';
import { KLineData, TimePeriod, Stock } from '../types';

interface StockChartProps {
  data: KLineData[];
  period: TimePeriod;
  onPeriodChange: (p: TimePeriod) => void;
  stock?: Stock;
}

// Custom shape for candlestick
const Candlestick = (props: any) => {
  const { x, y, width, height, low, high, open, close } = props;
  const isGrowing = close > open;
  const color = isGrowing ? '#ef4444' : '#22c55e'; // 红涨绿跌（中国A股标准）

  const unitHeight = height / (high - low);
  
  const bodyTop = y + (high - Math.max(open, close)) * unitHeight;
  const bodyBottom = y + (high - Math.min(open, close)) * unitHeight;
  const bodyLen = Math.max(2, bodyBottom - bodyTop);

  return (
    <g>
      {/* Wick */}
      <line x1={x + width / 2} y1={y} x2={x + width / 2} y2={y + height} stroke={color} strokeWidth={1} />
      {/* Body */}
      <rect
        x={x}
        y={bodyTop}
        width={width}
        height={bodyLen}
        fill={color}
        stroke={color} // Fill the gap
      />
    </g>
  );
};

export const StockChart: React.FC<StockChartProps> = ({ data, period, onPeriodChange, stock }) => {
  // 确保 data 不为 null
  const safeData = data || [];
  // 缩放和滑动状态
  const [visibleCount, setVisibleCount] = useState(60);
  const [startIndex, setStartIndex] = useState(0);
  const [isHovering, setIsHovering] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const chartRef = useRef<HTMLDivElement>(null);
  const lastX = useRef(0);
  const prevPeriod = useRef(period);

  // 统一处理数据和周期变化
  useEffect(() => {
    // 周期切换时重置
    if (prevPeriod.current !== period) {
      prevPeriod.current = period;
      setVisibleCount(60);
      setStartIndex(0);
      return;
    }
    // 数据变化时调整视图到最新
    if (safeData.length > 0) {
      const newStart = Math.max(0, safeData.length - visibleCount);
      setStartIndex(newStart);
    }
  }, [safeData, period, visibleCount]);

  // 滚轮缩放处理 - 必须在条件返回之前
  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault();
    const delta = e.deltaY > 0 ? 10 : -10;
    setVisibleCount(prev => {
      const newCount = Math.max(20, Math.min(safeData.length, prev + delta));
      const newStart = Math.max(0, safeData.length - newCount);
      setStartIndex(newStart);
      return newCount;
    });
  }, [safeData.length]);

  // 拖拽滑动处理
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    // 只响应鼠标左键
    if (e.button !== 0) return;
    e.preventDefault();
    setIsDragging(true);
    lastX.current = e.clientX;
  }, []);

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (!isDragging) return;
    const deltaX = e.clientX - lastX.current;
    const sensitivity = Math.max(1, Math.floor(visibleCount / 30));
    if (Math.abs(deltaX) > 10) {
      const move = deltaX > 0 ? -sensitivity : sensitivity;
      setStartIndex(prev => Math.max(0, Math.min(safeData.length - visibleCount, prev + move)));
      lastX.current = e.clientX;
    }
  }, [isDragging, safeData.length, visibleCount]);

  // 全局监听 mouseup，解决鼠标移出组件后松开导致的事件粘连问题
  useEffect(() => {
    const handleGlobalMouseUp = () => {
      setIsDragging(false);
    };

    window.addEventListener('mouseup', handleGlobalMouseUp);
    return () => {
      window.removeEventListener('mouseup', handleGlobalMouseUp);
    };
  }, []);

  // 拖动时强制设置全局鼠标样式（避免被子元素覆盖）
  useEffect(() => {
    if (isDragging) {
      document.body.classList.add('grabbing');
    } else {
      document.body.classList.remove('grabbing');
    }
    return () => {
      document.body.classList.remove('grabbing');
    };
  }, [isDragging]);

  // Guard clause for empty data
  if (safeData.length === 0) {
    return (
      <div className="h-full w-full fin-panel flex items-center justify-center">
        <span className="text-slate-500 text-sm animate-pulse">加载市场数据中...</span>
      </div>
    );
  }

  // 计算可见数据
  const visibleData = safeData.slice(startIndex, startIndex + visibleCount);
  const lastVisible = visibleData[visibleData.length - 1];

  // Transform data for the chart
  const chartData = visibleData.map(d => ({
    ...d,
    range: [d.low, d.high] as [number, number],
  }));

  const lastClose = lastVisible?.close || 0;
  const isIntraday = period === '1m';

  const periods: { id: TimePeriod; label: string }[] = [
    { id: '1m', label: '分时' },
    { id: '1d', label: '日K' },
    { id: '1w', label: '周K' },
    { id: '1mo', label: '月K' },
  ];

  return (
    <div className="h-full w-full fin-panel flex flex-col">
      {/* Header with Tabs and Info */}
      <div className="flex items-center justify-between px-2 py-1 border-b fin-divider fin-panel-strong z-10">
         <div className="flex gap-1">
            {periods.map((p) => (
              <button
                key={p.id}
                onClick={() => onPeriodChange(p.id)}
                className={`text-xs px-3 py-1 rounded transition-colors ${
                  period === p.id
                    ? 'bg-slate-800/80 text-accent-2 font-bold'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
                }`}
              >
                {p.label}
              </button>
            ))}
            {/* 操作提示 */}
            {!isIntraday && (
              <div className="flex items-center gap-2 ml-3 pl-3 border-l border-slate-700">
                <div className="flex items-center gap-1 text-slate-500 text-xs">
                  <ZoomIn size={12} />
                  <ZoomOut size={12} />
                  <span>滚轮</span>
                </div>
                <div className="flex items-center gap-1 text-slate-500 text-xs">
                  <MoveHorizontal size={12} />
                  <span>拖拽</span>
                </div>
              </div>
            )}
         </div>
         <div className="text-xs text-slate-400 font-mono flex gap-4">
           {isIntraday ? (
             <>
               <span>现价: <span className="text-accent-2">{(stock?.price || lastClose).toFixed(2)}</span></span>
               <span>均价: <span className="text-yellow-400">{safeData[safeData.length - 1].avg?.toFixed(2) || '--'}</span></span>
               <span>最高: <span className="text-red-400">{(stock?.high || Math.max(...safeData.map(d => d.high))).toFixed(2)}</span></span>
               <span>最低: <span className="text-green-400">{(stock?.low || Math.min(...safeData.map(d => d.low))).toFixed(2)}</span></span>
             </>
           ) : (
             <>
               <span>收: <span className="text-accent-2">{lastClose.toFixed(2)}</span></span>
               <span>开: {lastVisible?.open.toFixed(2)}</span>
               <span>高: <span className="text-red-400">{lastVisible?.high.toFixed(2)}</span></span>
               <span>低: <span className="text-green-400">{lastVisible?.low.toFixed(2)}</span></span>
               {lastVisible?.ma5 && (
                 <>
                   <span>MA5: <span className="text-yellow-400">{lastVisible?.ma5?.toFixed(2)}</span></span>
                   <span>MA10: <span className="text-purple-400">{lastVisible?.ma10?.toFixed(2)}</span></span>
                   <span>MA20: <span className="text-orange-400">{lastVisible?.ma20?.toFixed(2)}</span></span>
                 </>
               )}
             </>
           )}
         </div>
      </div>

      <div
        ref={chartRef}
        className={`flex-1 min-h-0 relative transition-all duration-200 ${
          !isIntraday
            ? (isDragging ? 'cursor-grabbing' : 'cursor-grab') + ' ' + (isHovering ? 'ring-1 ring-slate-600/50 ring-inset bg-slate-800/20' : '')
            : ''
        }`}
        onWheel={handleWheel}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseEnter={() => setIsHovering(true)}
        onMouseLeave={() => setIsHovering(false)}
      >
        {/* 悬停提示 */}
        {!isIntraday && isHovering && (
          <div className="absolute top-2 left-1/2 -translate-x-1/2 z-20 flex items-center gap-3 px-3 py-1.5 rounded-full bg-slate-900/90 border border-slate-700/50 text-xs text-slate-400 backdrop-blur-sm">
            <span className="flex items-center gap-1">
              <ZoomIn size={12} className="text-slate-500" />
              <ZoomOut size={12} className="text-slate-500" />
              滚轮缩放
            </span>
            <span className="w-px h-3 bg-slate-700" />
            <span className="flex items-center gap-1">
              <MoveHorizontal size={12} className="text-slate-500" />
              拖拽滑动
            </span>
          </div>
        )}
        <ResponsiveContainer width="100%" height="100%">
          {isIntraday ? (
             <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" vertical={false} />
                <XAxis
                  dataKey="time"
                  tick={{ fill: '#94a3b8', fontSize: 10 }}
                  axisLine={{ stroke: '#334155' }}
                  tickLine={false}
                  minTickGap={30}
                  interval="preserveStartEnd"
                />
                <YAxis
                  domain={['auto', 'auto']}
                  orientation="right"
                  tick={{ fill: '#94a3b8', fontSize: 10 }}
                  axisLine={false}
                  tickLine={false}
                  tickFormatter={(val) => val.toFixed(2)}
                />
                <Tooltip
                  active={!isDragging}
                  contentStyle={{ backgroundColor: '#0f172a', border: '1px solid #1e293b', color: '#e2e8f0' }}
                  itemStyle={{ color: '#38bdf8' }}
                  labelStyle={{ color: '#94a3b8' }}
                  content={({ active, payload, label }) => {
                    if (!active || !payload || payload.length === 0) return null;
                    const d = payload[0]?.payload;
                    if (!d) return null;
                    return (
                      <div style={{ backgroundColor: '#0f172a', border: '1px solid #1e293b', padding: '8px 12px', borderRadius: '4px' }}>
                        <div style={{ color: '#94a3b8', marginBottom: '4px' }}>{label}</div>
                        <div style={{ color: '#38bdf8' }}>价格: {d.close?.toFixed(2)}</div>
                        {d.avg && <div style={{ color: '#facc15' }}>均价: {d.avg?.toFixed(2)}</div>}
                      </div>
                    );
                  }}
                />
                <Line
                  type="linear"
                  dataKey="close"
                  stroke="#38bdf8"
                  strokeWidth={1.5}
                  dot={false}
                  isAnimationActive={false}
                  name="价格"
                />
                {/* 分时均价线 */}
                <Line
                  type="linear"
                  dataKey="avg"
                  stroke="#facc15"
                  strokeWidth={1}
                  dot={false}
                  isAnimationActive={false}
                  name="均价"
                />
             </LineChart>
          ) : (
            <ComposedChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" vertical={false} />
              <XAxis 
                dataKey="time" 
                tick={{ fill: '#94a3b8', fontSize: 10 }} 
                axisLine={{ stroke: '#334155' }}
                tickLine={false}
                minTickGap={30}
              />
              <YAxis 
                domain={['auto', 'auto']} 
                orientation="right" 
                tick={{ fill: '#94a3b8', fontSize: 10 }} 
                axisLine={false}
                tickLine={false}
                tickFormatter={(val) => val.toFixed(1)}
              />
              <Tooltip
                active={!isDragging}
                contentStyle={{ backgroundColor: '#0f172a', border: '1px solid #1e293b', color: '#e2e8f0' }}
                itemStyle={{ color: '#cbd5f5' }}
                labelStyle={{ color: '#94a3b8' }}
                content={({ active, payload, label }) => {
                  if (!active || !payload || payload.length === 0) return null;
                  const d = payload[0]?.payload;
                  if (!d) return null;
                  return (
                    <div style={{ backgroundColor: '#0f172a', border: '1px solid #1e293b', padding: '8px 12px', borderRadius: '4px' }}>
                      <div style={{ color: '#94a3b8', marginBottom: '4px' }}>{label}</div>
                      <div style={{ color: '#e2e8f0' }}>开: {d.open?.toFixed(2)}</div>
                      <div style={{ color: '#ef4444' }}>高: {d.high?.toFixed(2)}</div>
                      <div style={{ color: '#22c55e' }}>低: {d.low?.toFixed(2)}</div>
                      <div style={{ color: '#38bdf8' }}>收: {d.close?.toFixed(2)}</div>
                      {d.ma5 && <div style={{ color: '#facc15' }}>MA5: {d.ma5?.toFixed(2)}</div>}
                      {d.ma10 && <div style={{ color: '#a855f7' }}>MA10: {d.ma10?.toFixed(2)}</div>}
                      {d.ma20 && <div style={{ color: '#f97316' }}>MA20: {d.ma20?.toFixed(2)}</div>}
                    </div>
                  );
                }}
              />
              <Bar
                dataKey="range"
                shape={<Candlestick />}
                isAnimationActive={false}
              >
                {
                  chartData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.close > entry.open ? '#ef4444' : '#22c55e'} />
                  ))
                }
              </Bar>
              {/* 均线 */}
              <Line type="linear" dataKey="ma5" stroke="#facc15" strokeWidth={1} dot={false} isAnimationActive={false} name="MA5" />
              <Line type="linear" dataKey="ma10" stroke="#a855f7" strokeWidth={1} dot={false} isAnimationActive={false} name="MA10" />
              <Line type="linear" dataKey="ma20" stroke="#f97316" strokeWidth={1} dot={false} isAnimationActive={false} name="MA20" />
            </ComposedChart>
          )}
        </ResponsiveContainer>
      </div>
      
      {/* Volume Chart at bottom - Color Coded for Buy/Sell */}
      <div className="h-16 border-t fin-divider">
         <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={chartData} margin={{ top: 0, right: 10, left: 0, bottom: 0 }}>
             <Bar 
              dataKey="volume" 
              isAnimationActive={false} 
            >
               {chartData.map((entry, index) => (
                  <Cell 
                    key={`vol-${index}`} 
                    fill={entry.close >= entry.open ? '#ef4444' : '#22c55e'} 
                    opacity={0.5} 
                  />
              ))}
            </Bar>
          </ComposedChart>
         </ResponsiveContainer>
      </div>
    </div>
  );
};
