import React, { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { StockList } from './components/StockList';
import { StockChartLW } from './components/StockChartLW';
import { OrderBook as OrderBookComponent } from './components/OrderBook';
import { AgentRoom } from './components/AgentRoom';
import { SettingsDialog } from './components/SettingsDialog';
import { PositionDialog } from './components/PositionDialog';
import { HotTrendDialog } from './components/HotTrendDialog';
import { LongHuBangDialog } from './components/LongHuBangDialog';
import { WelcomePage } from './components/WelcomePage';
import { ThemeSwitcher } from './components/ThemeSwitcher';
import { useTheme } from './contexts/ThemeContext';
import { useCandleColor } from './contexts/CandleColorContext';
import { ResizeHandle } from './components/ResizeHandle';
import { getWatchlist, addToWatchlist, removeFromWatchlist } from './services/watchlistService';
import { getKLineData, getOrderBook } from './services/stockService';
import { getOrCreateSession, StockSession, updateStockPosition } from './services/sessionService';
import { getConfig, updateConfig } from './services/configService';
import { useMarketEvents } from './hooks/useMarketEvents';
import { useMarketStatus } from './hooks/useMarketStatus';
import { Stock, KLineData, OrderBook, TimePeriod, Telegraph, MarketIndex } from './types';
import { Radio, Settings, List, Minus, Square, X, Copy, Briefcase, TrendingUp, BarChart3 } from 'lucide-react';
import logo from './assets/images/logo.png';
import { GetTelegraphList, OpenURL, WindowMinimize, WindowMaximize, WindowClose } from '../wailsjs/go/main/App';
import { WindowIsMaximised, WindowSetSize, WindowGetSize } from '../wailsjs/runtime/runtime';

// 布局配置常量
const LAYOUT_DEFAULTS = {
  leftPanelWidth: 280,
  rightPanelWidth: 384,
  bottomPanelHeight: 180,
};
const LAYOUT_MIN = {
  leftPanelWidth: 280,
  rightPanelWidth: 384,
  bottomPanelHeight: 120,
};
const LAYOUT_MAX = {
  leftPanelWidth: 500,
  rightPanelWidth: 700,
  bottomPanelHeight: 450,
};

type KLineUpdateMode = 'full' | 'incremental' | 'refresh';

const App: React.FC = () => {
  const { colors } = useTheme();
  const cc = useCandleColor();
  const [watchlist, setWatchlist] = useState<Stock[]>([]);
  const [selectedSymbol, setSelectedSymbol] = useState<string>('');
  const [currentSession, setCurrentSession] = useState<StockSession | null>(null);
  const [timePeriod, setTimePeriod] = useState<TimePeriod>('1m');
  const [kLineData, setKLineData] = useState<KLineData[]>([]);
  const [kLineUpdateMode, setKLineUpdateMode] = useState<KLineUpdateMode>('full');
  const [orderBook, setOrderBook] = useState<OrderBook>({ bids: [], asks: [] });
  const [marketMessage, setMarketMessage] = useState<string>('市场数据加载中...');
  const [telegraphList, setTelegraphList] = useState<Telegraph[]>([]);
  const [showTelegraphList, setShowTelegraphList] = useState(false);
  const [telegraphLoading, setTelegraphLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [showSettings, setShowSettings] = useState(false);
  const [showPosition, setShowPosition] = useState(false);
  const [showHotTrend, setShowHotTrend] = useState(false);
  const [showLongHuBang, setShowLongHuBang] = useState(false);
  const [marketIndices, setMarketIndices] = useState<MarketIndex[]>([]);
  const [isMaximized, setIsMaximized] = useState(false);
  const klineRequestIdRef = useRef(0);

  // 使用纯前端市场状态判断
  const { status: marketStatus } = useMarketStatus();

  // 布局状态
  const [leftPanelWidth, setLeftPanelWidth] = useState(LAYOUT_DEFAULTS.leftPanelWidth);
  const [rightPanelWidth, setRightPanelWidth] = useState(LAYOUT_DEFAULTS.rightPanelWidth);
  const [bottomPanelHeight, setBottomPanelHeight] = useState(LAYOUT_DEFAULTS.bottomPanelHeight);
  const saveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const selectedStock = useMemo(() =>
    watchlist.find(s => s.symbol === selectedSymbol) || watchlist[0]
  , [selectedSymbol, watchlist]);

  // 处理股票数据更新（来自后端推送）
  const handleStockUpdate = useCallback((stocks: Stock[]) => {
    if (!stocks || !Array.isArray(stocks)) return;
    setWatchlist(prev => {
      // 更新已有股票的数据
      return prev.map(stock => {
        const updated = stocks.find(s => s.symbol === stock.symbol);
        return updated || stock;
      });
    });
  }, []);

  // 处理盘口数据更新（来自后端推送）
  const handleOrderBookUpdate = useCallback((data: OrderBook) => {
    setOrderBook(data);
  }, []);

  // 处理快讯数据更新（来自后端推送）
  const handleTelegraphUpdate = useCallback((data: Telegraph) => {
    if (data && data.content) {
      setMarketMessage(`[${data.time}] ${data.content}`);
    }
  }, []);

  // 处理大盘指数更新（来自后端推送）
  const handleMarketIndicesUpdate = useCallback((indices: MarketIndex[]) => {
    if (indices) {
      setMarketIndices(indices);
    }
  }, []);

  // 处理K线数据更新（来自后端推送，支持增量）
  const handleKLineUpdate = useCallback((data: { code: string; period: string; data: KLineData[]; incremental?: boolean }) => {
    if (!data || data.code !== selectedSymbol || data.period !== timePeriod) return;

    if (data.incremental && data.data.length > 0) {
      setKLineUpdateMode('incremental');
      // 增量更新：合并最新K线
      setKLineData(prev => {
        if (prev.length === 0) return data.data;
        const newBar = data.data[0];
        const lastIdx = prev.length - 1;
        // 同一时间戳则更新，否则追加
        if (prev[lastIdx].time === newBar.time) {
          const updated = [...prev];
          updated[lastIdx] = newBar;
          return updated;
        }
        return [...prev.slice(-239), newBar]; // 保持240根
      });
    } else {
      // 后端定时推送：用 refresh 模式更新数据但保留用户缩放状态
      if (Array.isArray(data.data) && data.data.length > 0) {
        setKLineUpdateMode('refresh');
        setKLineData(data.data);
      }
    }
  }, [selectedSymbol, timePeriod]);

  const syncWindowMaximizedState = useCallback(async () => {
    try {
      const maximized = await WindowIsMaximised();
      setIsMaximized(maximized);
    } catch {
      // ignore runtime query failures and keep current UI state
    }
  }, []);

  const toggleWindowMaximize = useCallback(async () => {
    try {
      await WindowMaximize();
      await syncWindowMaximizedState();
    } catch {
      // fallback to optimistic toggle when runtime query is unavailable
      setIsMaximized(prev => !prev);
    }
  }, [syncWindowMaximizedState]);

  // 保存布局配置（防抖）
  const saveLayoutConfig = useCallback(async (
    left: number, right: number, bottom: number,
    winWidth?: number, winHeight?: number
  ) => {
    if (saveTimeoutRef.current) {
      clearTimeout(saveTimeoutRef.current);
    }
    saveTimeoutRef.current = setTimeout(async () => {
      try {
        const config = await getConfig();
        const size = await WindowGetSize();
        config.layout = {
          leftPanelWidth: left,
          rightPanelWidth: right,
          bottomPanelHeight: bottom,
          windowWidth: winWidth ?? size.w,
          windowHeight: winHeight ?? size.h,
        };
        await updateConfig(config);
      } catch (err) {
        console.error('Failed to save layout config:', err);
      }
    }, 500);
  }, []);

  // 左侧面板 resize
  const handleLeftResize = useCallback((delta: number) => {
    setLeftPanelWidth(prev => {
      const newWidth = Math.max(LAYOUT_MIN.leftPanelWidth, Math.min(LAYOUT_MAX.leftPanelWidth, prev + delta));
      return newWidth;
    });
  }, []);

  // 右侧面板 resize
  const handleRightResize = useCallback((delta: number) => {
    setRightPanelWidth(prev => {
      const newWidth = Math.max(LAYOUT_MIN.rightPanelWidth, Math.min(LAYOUT_MAX.rightPanelWidth, prev - delta));
      return newWidth;
    });
  }, []);

  // 底部面板 resize
  const handleBottomResize = useCallback((delta: number) => {
    setBottomPanelHeight(prev => {
      const newHeight = Math.max(LAYOUT_MIN.bottomPanelHeight, Math.min(LAYOUT_MAX.bottomPanelHeight, prev - delta));
      return newHeight;
    });
  }, []);

  // resize 结束时保存配置
  const handleResizeEnd = useCallback(() => {
    saveLayoutConfig(leftPanelWidth, rightPanelWidth, bottomPanelHeight);
  }, [leftPanelWidth, rightPanelWidth, bottomPanelHeight, saveLayoutConfig]);

  // 监听窗口 resize 事件
  useEffect(() => {
    const windowResizeTimeoutRef = { current: null as ReturnType<typeof setTimeout> | null };
    const handleWindowResize = () => {
      if (windowResizeTimeoutRef.current) {
        clearTimeout(windowResizeTimeoutRef.current);
      }
      windowResizeTimeoutRef.current = setTimeout(() => {
        saveLayoutConfig(leftPanelWidth, rightPanelWidth, bottomPanelHeight);
      }, 500);
    };
    window.addEventListener('resize', handleWindowResize);
    return () => {
      window.removeEventListener('resize', handleWindowResize);
      if (windowResizeTimeoutRef.current) {
        clearTimeout(windowResizeTimeoutRef.current);
      }
    };
  }, [leftPanelWidth, rightPanelWidth, bottomPanelHeight, saveLayoutConfig]);

  // 获取快讯列表
  const handleShowTelegraphList = async () => {
    if (!showTelegraphList) {
      setShowTelegraphList(true);
      setTelegraphLoading(true);
      try {
        const list = await GetTelegraphList();
        setTelegraphList(list || []);
      } finally {
        setTelegraphLoading(false);
      }
    } else {
      setShowTelegraphList(false);
    }
  };

  // 打开快讯链接
  const handleOpenTelegraph = (telegraph: Telegraph) => {
    if (telegraph.url) {
      OpenURL(telegraph.url);
    }
    setShowTelegraphList(false);
  };

  // 使用市场事件 Hook
  const { subscribeOrderBook, subscribeKLine } = useMarketEvents({
    onStockUpdate: handleStockUpdate,
    onOrderBookUpdate: handleOrderBookUpdate,
    onTelegraphUpdate: handleTelegraphUpdate,
    onMarketIndicesUpdate: handleMarketIndicesUpdate,
    onKLineUpdate: handleKLineUpdate,
  });

  // Handle Adding Stock
  const handleAddStock = async (newStock: Stock) => {
    if (!watchlist.find(s => s.symbol === newStock.symbol)) {
      await addToWatchlist(newStock);
      setWatchlist(prev => [...prev, newStock]);
      // 添加后自动选中新股票并加载数据
      setSelectedSymbol(newStock.symbol);
      // 先清空 session，避免显示旧股票的消息
      setCurrentSession(null);
      subscribeOrderBook(newStock.symbol);
      // 加载 Session 和盘口数据
      const [session, orderBookData] = await Promise.all([
        getOrCreateSession(newStock.symbol, newStock.name),
        getOrderBook(newStock.symbol)
      ]);
      setCurrentSession(session);
      setOrderBook(orderBookData);
    }
  };

  // Handle Removing Stock
  const handleRemoveStock = async (symbol: string) => {
    await removeFromWatchlist(symbol);
    setWatchlist(prev => prev.filter(s => s.symbol !== symbol));
    // 如果删除的是当前选中的股票，切换到第一个
    if (symbol === selectedSymbol) {
      const remaining = watchlist.filter(s => s.symbol !== symbol);
      if (remaining.length > 0) {
        handleSelectStock(remaining[0].symbol);
      }
    }
  };

  // Handle Stock Selection - Load Session and sync data
  const handleSelectStock = async (symbol: string) => {
    setSelectedSymbol(symbol);
    // 订阅该股票的盘口推送
    subscribeOrderBook(symbol);
    const stock = watchlist.find(s => s.symbol === symbol);
    if (stock) {
      // 并行加载 Session 和盘口数据
      const [session, orderBookData] = await Promise.all([
        getOrCreateSession(symbol, stock.name),
        getOrderBook(symbol)
      ]);
      setCurrentSession(session);
      setOrderBook(orderBookData);
    }
  };

  // Load watchlist on mount
  useEffect(() => {
    const loadWatchlist = async () => {
      try {
        // 加载布局配置
        const config = await getConfig();
        if (config.layout) {
          if (config.layout.leftPanelWidth > 0) setLeftPanelWidth(config.layout.leftPanelWidth);
          if (config.layout.rightPanelWidth > 0) setRightPanelWidth(config.layout.rightPanelWidth);
          if (config.layout.bottomPanelHeight > 0) setBottomPanelHeight(config.layout.bottomPanelHeight);
          // 恢复窗口大小
          if (config.layout.windowWidth > 0 && config.layout.windowHeight > 0) {
            WindowSetSize(config.layout.windowWidth, config.layout.windowHeight);
          }
        }

        const list = await getWatchlist();
        setWatchlist(list);
        if (list.length > 0) {
          setSelectedSymbol(list[0].symbol);
          // 订阅第一个股票的盘口推送
          subscribeOrderBook(list[0].symbol);
          // 加载第一个股票的Session
          const session = await getOrCreateSession(list[0].symbol, list[0].name);
          setCurrentSession(session);
        }
        // 主动获取一次快讯数据（解决启动时后端推送早于前端监听注册的时序问题）
        const telegraphs = await GetTelegraphList();
        if (telegraphs && telegraphs.length > 0) {
          const latest = telegraphs[0];
          setMarketMessage(`[${latest.time}] ${latest.content}`);
        }
      } catch (err) {
        console.error('Failed to load watchlist:', err);
      } finally {
        setLoading(false);
      }
    };
    loadWatchlist();
  }, [subscribeOrderBook]);

  // Load K-line data when symbol or period changes
  useEffect(() => {
    if (!selectedSymbol) return;
    const requestId = ++klineRequestIdRef.current;
    // 切换时先切回 full 模式，等待全量数据到达
    setKLineUpdateMode('full');
    // 清空旧数据，避免切换期间出现“新股票 + 旧K线”错配
    setKLineData([]);
    // 订阅K线推送
    subscribeKLine(selectedSymbol, timePeriod);

    const loadKLineData = async () => {
      // 与后端推送统一数据长度，降低周/月K空响应概率
      const dataLen = timePeriod === '1m' ? 250 : 240;
      const maxRetries = 2;
      for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
        if (requestId !== klineRequestIdRef.current) return;
        try {
          const data = await getKLineData(selectedSymbol, timePeriod, dataLen);
          if (requestId !== klineRequestIdRef.current) return;
          if (Array.isArray(data) && data.length > 0) {
            setKLineUpdateMode('full');
            setKLineData(data);
            return;
          }
          console.warn(`[kline] empty data for ${selectedSymbol} ${timePeriod}, attempt=${attempt + 1}`);
        } catch (err) {
          if (requestId !== klineRequestIdRef.current) return;
          console.error(`[kline] load failed for ${selectedSymbol} ${timePeriod}, attempt=${attempt + 1}`, err);
        }
        if (attempt < maxRetries) {
          await new Promise(resolve => setTimeout(resolve, 600));
        }
      }
    };

    void loadKLineData();
  }, [selectedSymbol, timePeriod, subscribeKLine]);

  // 初始化窗口最大化状态
  useEffect(() => {
    void syncWindowMaximizedState();
  }, [syncWindowMaximizedState]);

  if (loading) return <div className="h-screen w-screen flex items-center justify-center fin-app text-white">加载中...</div>;

  // 没有自选股时显示欢迎页面
  if (watchlist.length === 0) {
    return <WelcomePage onAddStock={handleAddStock} />;
  }

  if (!selectedStock) return <div className="h-screen w-screen flex items-center justify-center fin-app text-white">请添加自选股</div>;

  return (
    <div className="flex flex-col h-screen text-slate-100 font-sans fin-app">
      {/* Top Navbar */}
      <header
        className="h-14 fin-panel border-b fin-divider flex items-center px-4 justify-between shrink-0 z-20"
        style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
        onDoubleClick={(e) => {
          // 排除 no-drag 区域的双击
          const target = e.target as HTMLElement;
          if (target.closest('[style*="no-drag"]') || target.closest('button') || target.closest('input')) return;
          void toggleWindowMaximize();
        }}
      >
        <div className="flex items-center gap-2" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <img src={logo} alt="logo" className="h-8 w-8 rounded-lg" />
          <span className={`font-bold text-lg tracking-tight ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>韭菜盘 <span className="text-accent-2">AI</span></span>
        </div>
        
        <div className="flex items-center gap-4 fin-panel-soft px-4 py-1.5 rounded-full border fin-divider relative" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <Radio className="h-3 w-3 animate-pulse text-accent-2" />
          <span className={`text-xs font-mono w-96 truncate text-center ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
            实时快讯: {marketMessage}
          </span>
          <button
            onClick={handleShowTelegraphList}
            className={`p-1 rounded transition-colors ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400' : 'hover:bg-slate-200/50 text-slate-500'} hover:text-accent-2`}
            title="查看快讯列表"
          >
            <List className="h-4 w-4" />
          </button>

          {/* 快讯下拉列表 */}
          {showTelegraphList && (
            <div
              className="absolute top-full left-0 right-0 mt-2 fin-panel border fin-divider rounded-lg shadow-xl z-50 max-h-96 overflow-y-auto fin-scrollbar text-left"
              onMouseLeave={() => setShowTelegraphList(false)}
            >
              <div className={`p-2 border-b fin-divider text-xs font-medium ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                财联社快讯
              </div>
              {telegraphLoading ? (
                <div className={`p-4 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>加载中...</div>
              ) : telegraphList.length === 0 ? (
                <div className={`p-4 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>暂无快讯</div>
              ) : (
                telegraphList.map((tg, idx) => (
                  <div
                    key={idx}
                    onClick={() => handleOpenTelegraph(tg)}
                    className={`p-3 border-b fin-divider last:border-b-0 cursor-pointer transition-colors ${colors.isDark ? 'hover:bg-slate-800/50' : 'hover:bg-slate-100/80'}`}
                  >
                    <div className="flex items-start gap-2">
                      <span className="text-xs text-accent-2 font-mono shrink-0">{tg.time}</span>
                      <span className={`text-xs line-clamp-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>{tg.content}</span>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        <div className="flex items-center gap-3" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <button
            onClick={() => setShowLongHuBang(true)}
            className={`p-2 rounded-lg fin-panel border fin-divider transition-colors ${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'} hover:border-red-400/40`}
            title="龙虎榜"
          >
            <BarChart3 className="h-4 w-4" />
          </button>
          <button
            onClick={() => setShowHotTrend(true)}
            className={`p-2 rounded-lg fin-panel border fin-divider transition-colors ${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'} hover:border-orange-400/40`}
            title="全网热点"
          >
            <TrendingUp className="h-4 w-4" />
          </button>
          <ThemeSwitcher />
          <button
            onClick={() => setShowSettings(true)}
            className={`p-2 rounded-lg fin-panel border fin-divider transition-colors ${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'} hover:border-accent/40`}
          >
            <Settings className="h-4 w-4" />
          </button>
          <div className="text-xs text-right hidden md:block">
            <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>市场状态</div>
            <div className={`font-bold ${
              marketStatus?.status === 'trading' ? 'text-green-500' :
              marketStatus?.status === 'pre_market' ? 'text-yellow-500' :
              marketStatus?.status === 'lunch_break' ? 'text-orange-500' :
              colors.isDark ? 'text-slate-500' : 'text-slate-400'
            }`}>
              {marketStatus?.statusText || '加载中...'}
            </div>
          </div>
          {/* 窗口控制按钮 */}
          <div className="flex items-center ml-2 border-l fin-divider pl-3">
            <button
              onClick={() => WindowMinimize()}
              className={`p-1.5 rounded transition-colors ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400 hover:text-white' : 'hover:bg-slate-200/50 text-slate-500 hover:text-slate-900'}`}
              title="最小化"
            >
              <Minus className="h-4 w-4" />
            </button>
            <button
              onClick={() => { void toggleWindowMaximize(); }}
              className={`p-1.5 rounded transition-colors ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400 hover:text-white' : 'hover:bg-slate-200/50 text-slate-500 hover:text-slate-900'}`}
              title={isMaximized ? "还原" : "最大化"}
            >
              {isMaximized ? <Copy className="h-3.5 w-3.5" /> : <Square className="h-3.5 w-3.5" />}
            </button>
            <button
              onClick={() => WindowClose()}
              className={`p-1.5 rounded hover:bg-red-500/80 hover:text-white transition-colors ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}
              title="关闭"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>
      </header>

      {/* Main Content Grid */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Sidebar: Watchlist */}
        <div style={{ width: leftPanelWidth }} className="shrink-0 fin-panel overflow-hidden">
          <StockList
            stocks={watchlist}
            selectedSymbol={selectedSymbol}
            onSelect={handleSelectStock}
            onAddStock={handleAddStock}
            onRemoveStock={handleRemoveStock}
            marketIndices={marketIndices}
          />
        </div>

        {/* Left Resize Handle */}
        <ResizeHandle direction="horizontal" onResize={handleLeftResize} onResizeEnd={handleResizeEnd} />

        {/* Center Panel: Charts & Data */}
        <div className="flex-1 flex flex-col min-w-0 fin-panel-center">
          {/* Stock Header - A股风格 */}
          <div className="px-6 py-3 shrink-0 border-b fin-divider-soft">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <span className={`text-lg font-bold ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{selectedStock.name}</span>
                <span className={`text-sm font-mono ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{selectedStock.symbol}</span>
                <button
                  onClick={() => setShowPosition(true)}
                  className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${colors.isDark ? 'text-slate-400 hover:bg-slate-700/50' : 'text-slate-500 hover:bg-slate-200/50'} hover:text-accent-2`}
                  title="持仓设置"
                >
                  <Briefcase className="h-3.5 w-3.5" />
                  {currentSession?.position && currentSession.position.shares > 0 ? (
                    (() => {
                      const pos = currentSession.position;
                      const marketValue = pos.shares * selectedStock.price;
                      const costAmount = pos.shares * pos.costPrice;
                      const profitLoss = marketValue - costAmount;
                      const profitPercent = costAmount > 0 ? (profitLoss / costAmount) * 100 : 0;
                      const isProfit = profitLoss >= 0;
                      return (
                        <span className={isProfit ? cc.upClass : cc.downClass}>
                          {pos.shares}股 {isProfit ? '+' : ''}{profitLoss.toFixed(0)} ({isProfit ? '+' : ''}{profitPercent.toFixed(2)}%)
                        </span>
                      );
                    })()
                  ) : (
                    <span>设置持仓</span>
                  )}
                </button>
              </div>
              <div className={`text-3xl font-mono font-bold ${cc.getColorClass(selectedStock.change >= 0)}`}>
                {selectedStock.price.toFixed(2)}
              </div>
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4 text-sm">
                <span className={`font-mono ${cc.getColorClass(selectedStock.change >= 0)}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.change.toFixed(2)}
                </span>
                <span className={`font-mono ${cc.getColorClass(selectedStock.change >= 0)}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.changePercent.toFixed(2)}%
                </span>
              </div>
              <div className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                {new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
              </div>
            </div>
          </div>

          {/* A股传统行情数据 */}
          <div className="grid grid-cols-4 gap-px p-2 border-b fin-divider-soft shrink-0 text-xs">
            <AStockStatItem label="今开" value={selectedStock.open} preClose={selectedStock.preClose} isDark={colors.isDark} />
            <AStockStatItem label="最高" value={selectedStock.high} preClose={selectedStock.preClose} isDark={colors.isDark} />
            <AStockStatItem label="成交量" value={formatVolume(selectedStock.volume)} isPlain isDark={colors.isDark} />
            <AStockStatItem label="昨收" value={selectedStock.preClose} isPlain isDark={colors.isDark} />
            <AStockStatItem label="最低" value={selectedStock.low} preClose={selectedStock.preClose} isDark={colors.isDark} />
            <AStockStatItem label="成交额" value={formatAmount(selectedStock.amount)} isPlain isDark={colors.isDark} />
            <AStockStatItem label="振幅" value={selectedStock.preClose > 0 ? ((selectedStock.high - selectedStock.low) / selectedStock.preClose * 100).toFixed(2) + '%' : '--'} isPlain isDark={colors.isDark} />
          </div>

          <div className="flex-1 flex flex-col min-h-0">
             {/* Chart Section */}
            <div className="flex-1 p-1 relative min-h-0">
               <StockChartLW
                  data={kLineData}
                  updateMode={kLineUpdateMode}
                  period={timePeriod}
                  onPeriodChange={setTimePeriod}
                  stock={selectedStock}
               />
            </div>

            {/* Bottom Resize Handle */}
            <ResizeHandle direction="vertical" onResize={handleBottomResize} onResizeEnd={handleResizeEnd} />

            {/* Bottom Info Panel: Order Book Only */}
            <div style={{ height: bottomPanelHeight }} className="border-t fin-divider-soft flex shrink-0">
               <div className="flex-1 overflow-hidden relative">
                  <OrderBookComponent data={orderBook} />
               </div>
            </div>
          </div>
        </div>

        {/* Right Resize Handle */}
        <ResizeHandle direction="horizontal" onResize={handleRightResize} onResizeEnd={handleResizeEnd} />

        {/* Right Panel: AI Agents */}
        <div style={{ width: rightPanelWidth }} className="shrink-0 fin-panel overflow-hidden">
          <AgentRoom
            stock={selectedStock}
            kLineData={kLineData}
            session={currentSession}
            onSessionUpdate={setCurrentSession}
          />
        </div>
      </div>

      <SettingsDialog isOpen={showSettings} onClose={() => setShowSettings(false)} />
      <PositionDialog
        isOpen={showPosition}
        onClose={() => setShowPosition(false)}
        stockCode={selectedStock.symbol}
        stockName={selectedStock.name}
        currentPrice={selectedStock.price}
        position={currentSession?.position}
        onSave={async (shares, costPrice) => {
          await updateStockPosition(selectedStock.symbol, shares, costPrice);
          const session = await getOrCreateSession(selectedStock.symbol, selectedStock.name);
          setCurrentSession(session);
        }}
      />
      <HotTrendDialog isOpen={showHotTrend} onClose={() => setShowHotTrend(false)} />
      <LongHuBangDialog isOpen={showLongHuBang} onClose={() => setShowLongHuBang(false)} />
    </div>
  );
};

// A股行情数据项组件
interface AStockStatItemProps {
  label: string;
  value: number | string;
  preClose?: number;
  isPlain?: boolean;
  isDark?: boolean;
}

const AStockStatItem: React.FC<AStockStatItemProps> = ({ label, value, preClose, isPlain, isDark = true }) => {
  const cc = useCandleColor();
  let colorClass = isDark ? 'text-slate-100' : 'text-slate-700';
  let displayValue = typeof value === 'string' ? value : value.toFixed(2);

  if (!isPlain && typeof value === 'number' && preClose) {
    if (value > preClose) colorClass = cc.upClass;
    else if (value < preClose) colorClass = cc.downClass;
  }

  return (
    <div className="flex justify-between items-center px-3 py-1.5">
      <span className={isDark ? 'text-slate-500' : 'text-slate-400'}>{label}</span>
      <span className={`font-mono ${colorClass}`}>{displayValue}</span>
    </div>
  );
};

// 格式化成交量
const formatVolume = (vol: number): string => {
  if (vol >= 100000000) return (vol / 100000000).toFixed(2) + '亿';
  if (vol >= 10000) return (vol / 10000).toFixed(2) + '万';
  return vol.toString();
};

// 格式化成交额
const formatAmount = (amount: number): string => {
  if (amount >= 100000000) return (amount / 100000000).toFixed(2) + '亿';
  if (amount >= 10000) return (amount / 10000).toFixed(2) + '万';
  return amount.toFixed(2);
};

export default App;
