import React, { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { StockList } from './components/StockList';
import { StockChart } from './components/StockChart';
import { OrderBook as OrderBookComponent } from './components/OrderBook';
import { F10Panel } from './components/F10Panel';
import { AgentRoom } from './components/AgentRoom';
import { SettingsDialog } from './components/SettingsDialog';
import { PositionDialog } from './components/PositionDialog';
import { HotTrendDialog } from './components/HotTrendDialog';
import { LongHuBangDialog } from './components/LongHuBangDialog';
import { MarketMovesDialog } from './components/MarketMovesDialog';
import { WelcomePage } from './components/WelcomePage';
import { ThemeSwitcher } from './components/ThemeSwitcher';
import { useTheme } from './contexts/ThemeContext';
import { ResizeHandle } from './components/ResizeHandle';
import { getWatchlist, addToWatchlist, removeFromWatchlist } from './services/watchlistService';
import { getKLineData, getOrderBook } from './services/stockService';
import { getF10Overview } from './services/f10Service';
import { getOrCreateSession, StockSession, updateStockPosition } from './services/sessionService';
import { getConfig, updateConfig } from './services/configService';
import { useMarketEvents } from './hooks/useMarketEvents';
import { Stock, KLineData, OrderBook, TimePeriod, Telegraph, MarketIndex, MarketStatus, F10Overview } from './types';
import { Radio, Settings, List, Minus, Square, X, Copy, Briefcase, TrendingUp, BarChart3, Activity } from 'lucide-react';
import logo from './assets/images/logo.png';
import { GetTelegraphList, OpenURL, WindowMinimize, WindowMaximize, WindowClose } from '../wailsjs/go/main/App';
import { WindowIsMaximised, WindowSetSize, WindowGetSize } from '../wailsjs/runtime/runtime';

// 布局配置常量
const LAYOUT_DEFAULTS = {
  leftPanelWidth: 280,
  rightPanelWidth: 384,
  bottomPanelHeight: 120,
};
const LAYOUT_MIN = {
  leftPanelWidth: 280,
  rightPanelWidth: 384,
  bottomPanelHeight: 104,
};
const LAYOUT_MAX = {
  leftPanelWidth: 500,
  rightPanelWidth: 700,
  bottomPanelHeight: 150,
};
const WINDOW_RESTORE_DEFAULT = {
  width: 1366,
  height: 768,
};

const clampLayoutValue = (value: number | undefined, min: number, max: number, fallback: number) => {
  if (typeof value !== 'number' || Number.isNaN(value) || value <= 0) return fallback;
  return Math.max(min, Math.min(max, value));
};

const formatClock = (): string =>
  new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

const App: React.FC = () => {
  const { colors } = useTheme();
  const [watchlist, setWatchlist] = useState<Stock[]>([]);
  const [selectedSymbol, setSelectedSymbol] = useState<string>('');
  const [currentSession, setCurrentSession] = useState<StockSession | null>(null);
  const [timePeriod, setTimePeriod] = useState<TimePeriod>('1m');
  const [kLineData, setKLineData] = useState<KLineData[]>([]);
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
  const [showMarketMoves, setShowMarketMoves] = useState(false);
  const [showF10, setShowF10] = useState(false);
  const [marketStatus, setMarketStatus] = useState<MarketStatus | null>(null);
  const [marketIndices, setMarketIndices] = useState<MarketIndex[]>([]);
  const [isMaximized, setIsMaximized] = useState(false);
  const [clock, setClock] = useState<string>(formatClock);
  const [f10Overview, setF10Overview] = useState<F10Overview | null>(null);
  const [f10Loading, setF10Loading] = useState(false);
  const [f10Error, setF10Error] = useState<string>('');
  const [pendingRemoveSymbol, setPendingRemoveSymbol] = useState<string>('');
  const [isRemovingStock, setIsRemovingStock] = useState(false);

  // 布局状态
  const [leftPanelWidth, setLeftPanelWidth] = useState(LAYOUT_DEFAULTS.leftPanelWidth);
  const [rightPanelWidth, setRightPanelWidth] = useState(LAYOUT_DEFAULTS.rightPanelWidth);
  const [bottomPanelHeight, setBottomPanelHeight] = useState(LAYOUT_DEFAULTS.bottomPanelHeight);
  const saveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const selectedStock = useMemo(() =>
    watchlist.find(s => s.symbol === selectedSymbol) || watchlist[0]
  , [selectedSymbol, watchlist]);
  const pendingRemoveStock = useMemo(
    () => watchlist.find(stock => stock.symbol === pendingRemoveSymbol) || null,
    [watchlist, pendingRemoveSymbol],
  );
  const marketOverview = useMemo(() => {
    if (!selectedStock) {
      return { quote: [], deal: [], capital: [] } as const;
    }

    const valuation = f10Overview?.valuation;
    const latestAvg = kLineData.length > 0 ? kLineData[kLineData.length - 1]?.avg : undefined;
    const quoteAvgCandidate = selectedStock.volume > 0 && selectedStock.amount > 0
      ? selectedStock.amount / selectedStock.volume
      : undefined;
    const isQuoteAvgPlausible = quoteAvgCandidate !== undefined
      && quoteAvgCandidate > selectedStock.price * 0.2
      && quoteAvgCandidate < selectedStock.price * 5;
    const avgPrice = latestAvg
      ?? (isQuoteAvgPlausible ? quoteAvgCandidate : undefined)
      ?? selectedStock.price;
    const amplitude = selectedStock.preClose > 0
      ? ((selectedStock.high - selectedStock.low) / selectedStock.preClose) * 100
      : undefined;
    const changeColor = selectedStock.change >= 0 ? 'text-red-500' : 'text-green-500';

    return {
      quote: [
        { label: '开', value: formatNumberOrDash(selectedStock.open), colorClass: getPriceColorClass(selectedStock.open, selectedStock.preClose) },
        { label: '高', value: formatNumberOrDash(selectedStock.high), colorClass: getPriceColorClass(selectedStock.high, selectedStock.preClose) },
        { label: '低', value: formatNumberOrDash(selectedStock.low), colorClass: getPriceColorClass(selectedStock.low, selectedStock.preClose) },
        { label: '昨', value: formatNumberOrDash(selectedStock.preClose) },
        { label: '振', value: formatPercentOrDash(amplitude) },
      ],
      deal: [
        { label: '量', value: formatVolume(selectedStock.volume) },
        { label: '额', value: formatAmount(selectedStock.amount) },
        { label: 'PE(TTM)', value: formatNumberOrDash(valuation?.peTtm) },
        { label: 'PB', value: formatNumberOrDash(valuation?.pb) },
        { label: '换', value: formatPercentOrDash(valuation?.turnoverRate) },
      ],
      capital: [
        { label: '总', value: formatCapValue(valuation?.totalMarketCap) },
        { label: '流', value: formatCapValue(valuation?.floatMarketCap) },
        { label: '均', value: formatNumberOrDash(avgPrice) },
        { label: '涨', value: `${selectedStock.change >= 0 ? '+' : ''}${selectedStock.changePercent.toFixed(2)}%`, colorClass: changeColor },
      ],
    } as const;
  }, [selectedStock, f10Overview, kLineData]);

  // 处理股票数据更新（来自后端推送）
  const handleStockUpdate = useCallback((stocks: Stock[]) => {
    if (!stocks || !Array.isArray(stocks)) return;
    setWatchlist(prev => {
      // 实时推送里通常不包含行业等静态字段，需保留本地已有值
      return prev.map(stock => {
        const updated = stocks.find(s => s.symbol === stock.symbol);
        if (!updated) return stock;
        return {
          ...stock,
          ...updated,
          name: updated.name || stock.name,
          sector: updated.sector || stock.sector,
          marketCap: updated.marketCap || stock.marketCap,
        };
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

  // 处理市场状态更新（来自后端推送）
  const handleMarketStatusUpdate = useCallback((status: MarketStatus) => {
    if (status) {
      setMarketStatus(status);
    }
  }, []);

  // 处理大盘指数更新（来自后端推送）
  const handleMarketIndicesUpdate = useCallback((indices: MarketIndex[]) => {
    if (indices) {
      setMarketIndices(indices);
    }
  }, []);

  // 处理K线数据更新（来自后端推送）
  const handleKLineUpdate = useCallback((data: { code: string; period: string; data: KLineData[] }) => {
    // 只更新当前选中股票和周期的K线数据
    if (data && data.code === selectedSymbol && data.period === timePeriod) {
      setKLineData(data.data);
    }
  }, [selectedSymbol, timePeriod]);

  const fetchF10Overview = useCallback(async (symbol: string) => {
    if (!symbol) return;
    setF10Loading(true);
    setF10Error('');
    try {
      const overview = await getF10Overview(symbol);
      setF10Overview(overview);
    } catch (err) {
      console.error('Failed to get F10 overview:', err);
      setF10Error(err instanceof Error ? err.message : '获取F10数据失败');
    } finally {
      setF10Loading(false);
    }
  }, []);

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
        const isWindowMaximized = await WindowIsMaximised();
        let windowWidth = config.layout?.windowWidth || WINDOW_RESTORE_DEFAULT.width;
        let windowHeight = config.layout?.windowHeight || WINDOW_RESTORE_DEFAULT.height;
        if (!isWindowMaximized) {
          const size = await WindowGetSize();
          windowWidth = Math.max(WINDOW_RESTORE_DEFAULT.width, winWidth ?? size.w);
          windowHeight = Math.max(WINDOW_RESTORE_DEFAULT.height, winHeight ?? size.h);
        }
        config.layout = {
          leftPanelWidth: left,
          rightPanelWidth: right,
          bottomPanelHeight: bottom,
          windowWidth,
          windowHeight,
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

  const handleShowTrend = useCallback(() => {
    setShowF10(false);
  }, []);

  const handleShowF10 = useCallback(() => {
    setShowF10(true);
    if (
      selectedStock?.symbol &&
      (!f10Overview || f10Overview.code !== selectedStock.symbol)
    ) {
      fetchF10Overview(selectedStock.symbol);
    }
  }, [selectedStock, f10Overview, fetchF10Overview]);

  // 使用市场事件 Hook
  const { subscribe, subscribeOrderBook, subscribeKLine } = useMarketEvents({
    onStockUpdate: handleStockUpdate,
    onOrderBookUpdate: handleOrderBookUpdate,
    onTelegraphUpdate: handleTelegraphUpdate,
    onMarketStatusUpdate: handleMarketStatusUpdate,
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
  const handleRemoveStock = (symbol: string) => {
    setPendingRemoveSymbol(symbol);
  };

  const handleCancelRemoveStock = () => {
    if (isRemovingStock) return;
    setPendingRemoveSymbol('');
  };

  const handleConfirmRemoveStock = async () => {
    if (!pendingRemoveSymbol || isRemovingStock) {
      return;
    }
    setIsRemovingStock(true);
    const symbol = pendingRemoveSymbol;

    try {
      await removeFromWatchlist(symbol);
      setWatchlist(prev => prev.filter(s => s.symbol !== symbol));
      setPendingRemoveSymbol('');
      // 如果删除的是当前选中的股票，切换到第一个
      if (symbol === selectedSymbol) {
        const remaining = watchlist.filter(s => s.symbol !== symbol);
        if (remaining.length > 0) {
          handleSelectStock(remaining[0].symbol);
        }
      }
    } finally {
      setIsRemovingStock(false);
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

  // Handle Market Index Selection - ensure index can be clicked from left-top panel
  const handleSelectIndex = async (index: MarketIndex) => {
    const existing = watchlist.find(s => s.symbol === index.code);
    if (existing) {
      await handleSelectStock(index.code);
      return;
    }

    const indexStock: Stock = {
      symbol: index.code,
      name: index.name,
      price: index.price,
      change: index.change,
      changePercent: index.changePercent,
      volume: index.volume,
      amount: index.amount,
      marketCap: '',
      sector: '指数',
      open: 0,
      high: 0,
      low: 0,
      preClose: 0,
    };
    await handleAddStock(indexStock);
  };

  // Load watchlist on mount
  useEffect(() => {
    const loadWatchlist = async () => {
      try {
        // 加载布局配置
        const config = await getConfig();
        if (config.layout) {
          setLeftPanelWidth(
            clampLayoutValue(
              config.layout.leftPanelWidth,
              LAYOUT_MIN.leftPanelWidth,
              LAYOUT_MAX.leftPanelWidth,
              LAYOUT_DEFAULTS.leftPanelWidth,
            ),
          );
          setRightPanelWidth(
            clampLayoutValue(
              config.layout.rightPanelWidth,
              LAYOUT_MIN.rightPanelWidth,
              LAYOUT_MAX.rightPanelWidth,
              LAYOUT_DEFAULTS.rightPanelWidth,
            ),
          );
          setBottomPanelHeight(
            clampLayoutValue(
              config.layout.bottomPanelHeight,
              LAYOUT_MIN.bottomPanelHeight,
              LAYOUT_MAX.bottomPanelHeight,
              LAYOUT_DEFAULTS.bottomPanelHeight,
            ),
          );
          // 恢复窗口大小：仅在非最大化状态下设置，避免破坏最大化/还原行为
          const isWindowMaximized = await WindowIsMaximised();
          if (!isWindowMaximized) {
            const restoreWidth = Math.max(WINDOW_RESTORE_DEFAULT.width, config.layout.windowWidth || WINDOW_RESTORE_DEFAULT.width);
            const restoreHeight = Math.max(WINDOW_RESTORE_DEFAULT.height, config.layout.windowHeight || WINDOW_RESTORE_DEFAULT.height);
            await WindowSetSize(restoreWidth, restoreHeight);
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
    // 切换时先清空数据，避免闪烁
    setKLineData([]);
    // 订阅K线推送
    subscribeKLine(selectedSymbol, timePeriod);
    const loadKLineData = async () => {
      // 分时图需要更多数据点（1分钟K线，一天约240根）
      const dataLen = timePeriod === '1m' ? 250 : 60;
      const data = await getKLineData(selectedSymbol, timePeriod, dataLen);
      setKLineData(data);
    };
    loadKLineData();
  }, [selectedSymbol, timePeriod, subscribeKLine]);

  // 初始化窗口最大化状态
  useEffect(() => {
    const syncMaximizedState = () => {
      WindowIsMaximised().then(setIsMaximized).catch(() => {});
    };
    syncMaximizedState();
    window.addEventListener('resize', syncMaximizedState);
    return () => {
      window.removeEventListener('resize', syncMaximizedState);
    };
  }, []);

  useEffect(() => {
    if (selectedSymbol) {
      fetchF10Overview(selectedSymbol);
    }
  }, [selectedSymbol, fetchF10Overview]);

  useEffect(() => {
    const timer = setInterval(() => {
      setClock(formatClock());
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!pendingRemoveSymbol) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !isRemovingStock) {
        setPendingRemoveSymbol('');
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [pendingRemoveSymbol, isRemovingStock]);

  // 自选股代码变化时同步后端订阅，避免新增/删除后实时价格滞后
  const watchlistSymbolKey = useMemo(
    () => watchlist.map(stock => stock.symbol).filter(Boolean).join(','),
    [watchlist],
  );

  useEffect(() => {
    if (!watchlistSymbolKey) return;
    subscribe(watchlistSymbolKey.split(','));
  }, [watchlistSymbolKey, subscribe]);

  if (loading) return <div className="h-screen w-screen flex items-center justify-center fin-app text-white">加载中...</div>;

  // 没有自选股时显示欢迎页面
  if (watchlist.length === 0) {
    return <WelcomePage onAddStock={handleAddStock} />;
  }

  if (!selectedStock) return <div className="h-screen w-screen flex items-center justify-center fin-app text-white">请添加自选股</div>;

  return (
    <div className="flex flex-col h-screen text-slate-100 font-sans fin-app">
      {/* Top Navbar */}
      <header className="h-14 fin-panel border-b fin-divider flex items-center px-4 justify-between shrink-0 z-20" style={{ '--wails-draggable': 'drag' } as React.CSSProperties}>
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
              className="absolute top-full left-0 right-0 mt-2 fin-panel border fin-divider rounded-lg shadow-xl z-50 max-h-96 overflow-y-auto fin-scrollbar"
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
          <button
            onClick={() => setShowMarketMoves(true)}
            className={`p-2 rounded-lg fin-panel border fin-divider transition-colors ${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'} hover:border-cyan-400/40`}
            title="异动中心"
          >
            <Activity className="h-4 w-4" />
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
            <div className={`font-mono text-[11px] mt-0.5 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              {clock}
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
              onClick={async () => {
                await WindowMaximize();
                const maximized = await WindowIsMaximised();
                setIsMaximized(maximized);
              }}
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
            selectedIndexCode={selectedSymbol}
            onSelectIndex={handleSelectIndex}
          />
        </div>

        {/* Left Resize Handle */}
        <ResizeHandle direction="horizontal" onResize={handleLeftResize} onResizeEnd={handleResizeEnd} />

        {/* Center Panel: Charts & Data */}
        <div className="flex-1 flex flex-col min-w-0 fin-panel-center relative z-0">
          {/* Stock Header - A股风格 */}
          <div className="px-6 py-2 shrink-0 border-b fin-divider-soft">
            <div className="flex items-center justify-between">
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
                        <span className={isProfit ? 'text-red-500' : 'text-green-500'}>
                          {pos.shares}股 {isProfit ? '+' : ''}{profitLoss.toFixed(0)} ({isProfit ? '+' : ''}{profitPercent.toFixed(2)}%)
                        </span>
                      );
                    })()
                  ) : (
                    <span>设置持仓</span>
                  )}
                </button>
              </div>
              <div className="flex items-end gap-3">
                <div className={`text-3xl leading-none font-mono font-bold ${selectedStock.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {selectedStock.price.toFixed(2)}
                </div>
                <div className="flex items-center gap-3 text-sm pb-0.5">
                <span className={`font-mono ${selectedStock.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.change.toFixed(2)}
                </span>
                <span className={`font-mono ${selectedStock.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.changePercent.toFixed(2)}%
                </span>
                </div>
              </div>
            </div>
          </div>

          {/* 行情 / 成交估值 / 市值资金 */}
          <div className="border-b fin-divider-soft shrink-0 text-xs">
            {isMaximized ? (
              <div className="grid grid-cols-1 xl:grid-cols-3">
                <div className="px-4 py-2 border-b xl:border-b-0 xl:border-r fin-divider-soft">
                  <div className={`text-[10px] uppercase tracking-wide mb-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>行情</div>
                  <div className="grid grid-cols-3 gap-x-3 gap-y-1.5">
                    {marketOverview.quote.map((item) => (
                      <div key={item.label} className="flex items-center gap-1.5 min-w-0">
                        <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>{item.label}</span>
                        <span className={`font-mono ${('colorClass' in item && item.colorClass) || (colors.isDark ? 'text-slate-200' : 'text-slate-700')}`}>{item.value}</span>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="px-4 py-2 border-b xl:border-b-0 xl:border-r fin-divider-soft">
                  <div className={`text-[10px] uppercase tracking-wide mb-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>成交 / 估值</div>
                  <div className="grid grid-cols-3 gap-x-3 gap-y-1.5">
                    {marketOverview.deal.map((item) => (
                      <div key={item.label} className="flex items-center gap-1.5 min-w-0">
                        <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>{item.label}</span>
                        <span className={`font-mono ${('colorClass' in item && item.colorClass) || (colors.isDark ? 'text-slate-200' : 'text-slate-700')}`}>{item.value}</span>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="px-4 py-2">
                  <div className={`text-[10px] uppercase tracking-wide mb-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>市值 / 资金</div>
                  <div className="grid grid-cols-2 gap-x-3 gap-y-1.5">
                    {marketOverview.capital.map((item) => (
                      <div key={item.label} className="flex items-center gap-1.5 min-w-0">
                        <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>{item.label}</span>
                        <span className={`font-mono ${('colorClass' in item && item.colorClass) || (colors.isDark ? 'text-slate-200' : 'text-slate-700')}`}>{item.value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ) : (
              <div className="px-4 py-0.5 relative z-[80]">
                <div className="flex items-center gap-3 text-[11px]">
                  <div className="relative group">
                    <button
                      type="button"
                      className={`h-6 px-1.5 rounded-sm transition-colors flex items-center justify-center whitespace-nowrap ${
                        colors.isDark ? 'text-slate-400 hover:text-slate-200' : 'text-slate-500 hover:text-slate-800'
                      }`}
                    >
                      <span className="font-medium">行情数据</span>
                    </button>
                    <div className={`absolute left-0 top-full z-[120] mt-1 w-[320px] rounded-lg border fin-divider-soft shadow-xl fin-panel p-2 hidden group-hover:block ${colors.isDark ? 'bg-slate-900/95' : 'bg-white/95'}`}>
                      <div className={`text-[10px] uppercase tracking-wide mb-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>行情</div>
                      <div className="grid grid-cols-3 gap-x-2.5 gap-y-1.5">
                        {marketOverview.quote.map((item) => (
                          <div key={item.label} className="flex items-center gap-1 min-w-0">
                            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>{item.label}</span>
                            <span className={`font-mono ${('colorClass' in item && item.colorClass) || (colors.isDark ? 'text-slate-200' : 'text-slate-700')}`}>{item.value}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>

                  <div className="relative group">
                    <button
                      type="button"
                      className={`h-6 px-1.5 rounded-sm transition-colors flex items-center justify-center whitespace-nowrap ${
                        colors.isDark ? 'text-slate-400 hover:text-slate-200' : 'text-slate-500 hover:text-slate-800'
                      }`}
                    >
                      <span className="font-medium">成交估值</span>
                    </button>
                    <div className={`absolute left-0 top-full z-[120] mt-1 w-[360px] rounded-lg border fin-divider-soft shadow-xl fin-panel p-2 hidden group-hover:block ${colors.isDark ? 'bg-slate-900/95' : 'bg-white/95'}`}>
                      <div className={`text-[10px] uppercase tracking-wide mb-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>成交 / 估值</div>
                      <div className="grid grid-cols-3 gap-x-2.5 gap-y-1.5">
                        {marketOverview.deal.map((item) => (
                          <div key={item.label} className="flex items-center gap-1 min-w-0">
                            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>{item.label}</span>
                            <span className={`font-mono ${('colorClass' in item && item.colorClass) || (colors.isDark ? 'text-slate-200' : 'text-slate-700')}`}>{item.value}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>

                  <div className="relative group">
                    <button
                      type="button"
                      className={`h-6 px-1.5 rounded-sm transition-colors flex items-center justify-center whitespace-nowrap ${
                        colors.isDark ? 'text-slate-400 hover:text-slate-200' : 'text-slate-500 hover:text-slate-800'
                      }`}
                    >
                      <span className="font-medium">市值资金</span>
                    </button>
                    <div className={`absolute left-0 top-full z-[120] mt-1 w-[300px] rounded-lg border fin-divider-soft shadow-xl fin-panel p-2 hidden group-hover:block ${colors.isDark ? 'bg-slate-900/95' : 'bg-white/95'}`}>
                      <div className={`text-[10px] uppercase tracking-wide mb-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>市值 / 资金</div>
                      <div className="grid grid-cols-2 gap-x-2.5 gap-y-1.5">
                        {marketOverview.capital.map((item) => (
                          <div key={item.label} className="flex items-center gap-1 min-w-0">
                            <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>{item.label}</span>
                            <span className={`font-mono ${('colorClass' in item && item.colorClass) || (colors.isDark ? 'text-slate-200' : 'text-slate-700')}`}>{item.value}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* 视图切换：趋势图 / F10全景 */}
          <div className="px-4 py-1 border-b fin-divider-soft shrink-0">
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={handleShowTrend}
                className={`text-xs px-2.5 py-0.5 rounded border transition-colors ${
                  !showF10
                    ? 'border-accent text-accent-2 bg-accent/10'
                    : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
                }`}
              >
                趋势图
              </button>
              <button
                type="button"
                onClick={handleShowF10}
                className={`text-xs px-2.5 py-0.5 rounded border transition-colors ${
                  showF10
                    ? 'border-accent text-accent-2 bg-accent/10'
                    : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
                }`}
              >
                F10 全景
              </button>
            </div>
          </div>

          <div className="flex-1 flex flex-col min-h-0 relative z-0">
            {!showF10 ? (
              <>
                 {/* Chart Section */}
                <div className="flex-1 p-1 relative min-h-0">
                   <StockChart
                      data={kLineData}
                      period={timePeriod}
                      onPeriodChange={setTimePeriod}
                      stock={selectedStock}
                      floatShares={f10Overview?.valuation?.floatShares}
                      fallbackTurnoverRate={f10Overview?.valuation?.turnoverRate}
                   />
                </div>

                {/* Bottom Resize Handle */}
                <ResizeHandle direction="vertical" onResize={handleBottomResize} onResizeEnd={handleResizeEnd} />

                {/* Bottom Info Panel: Order Book */}
                <div style={{ height: bottomPanelHeight }} className="border-t fin-divider-soft flex shrink-0">
                   <div className="flex-1 overflow-hidden relative">
                      <OrderBookComponent data={orderBook} />
                   </div>
                </div>
              </>
            ) : (
              <div className="flex-1 min-h-0 border-t fin-divider-soft overflow-hidden">
                <F10Panel
                  overview={f10Overview}
                  loading={f10Loading}
                  error={f10Error}
                  onRefresh={() => fetchF10Overview(selectedStock.symbol)}
                  onCollapse={() => setShowF10(false)}
                />
              </div>
            )}
          </div>
        </div>

        {/* Right Resize Handle */}
        <ResizeHandle direction="horizontal" onResize={handleRightResize} onResizeEnd={handleResizeEnd} />

        {/* Right Panel: AI Agents */}
        <div style={{ width: rightPanelWidth }} className="shrink-0 fin-panel overflow-hidden">
          <AgentRoom
            session={currentSession}
            onSessionUpdate={setCurrentSession}
          />
        </div>
      </div>

      {pendingRemoveSymbol && (
        <div
          className="fixed inset-0 z-[220] flex items-center justify-center bg-black/45"
          onClick={handleCancelRemoveStock}
        >
          <div
            className={`w-[360px] max-w-[92vw] rounded-xl border fin-divider shadow-2xl fin-panel p-4 ${
              colors.isDark ? 'bg-slate-900/95' : 'bg-white/95'
            }`}
            onClick={(event) => event.stopPropagation()}
          >
            <div className={`text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
              删除确认
            </div>
            <div className={`text-xs mt-2 leading-6 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
              确认删除自选股「{pendingRemoveStock?.name || '--'} {pendingRemoveSymbol}」吗？
            </div>
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={handleCancelRemoveStock}
                disabled={isRemovingStock}
                className={`px-3 py-1.5 rounded border text-xs transition-colors ${
                  colors.isDark
                    ? 'border-slate-600 text-slate-300 hover:bg-slate-800'
                    : 'border-slate-300 text-slate-600 hover:bg-slate-100'
                } disabled:opacity-50 disabled:cursor-not-allowed`}
              >
                取消
              </button>
              <button
                type="button"
                onClick={handleConfirmRemoveStock}
                disabled={isRemovingStock}
                className="px-3 py-1.5 rounded border border-red-500/40 text-xs text-red-300 hover:bg-red-500/15 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isRemovingStock ? '删除中...' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      )}

      <SettingsDialog isOpen={showSettings} onClose={() => setShowSettings(false)} />
      <PositionDialog
        isOpen={showPosition}
        onClose={() => setShowPosition(false)}
        stockCode={selectedStock.symbol}
        stockName={selectedStock.name}
        currentPrice={selectedStock.price}
        position={currentSession?.position}
        onSave={async (shares, costPrice) => {
          const result = await updateStockPosition(selectedStock.symbol, shares, costPrice);
          if (result !== 'success') {
            throw new Error(result || '持仓保存失败');
          }
          const session = await getOrCreateSession(selectedStock.symbol, selectedStock.name);
          setCurrentSession(session);
        }}
      />
      <HotTrendDialog isOpen={showHotTrend} onClose={() => setShowHotTrend(false)} />
      <LongHuBangDialog isOpen={showLongHuBang} onClose={() => setShowLongHuBang(false)} />
      <MarketMovesDialog isOpen={showMarketMoves} onClose={() => setShowMarketMoves(false)} />
    </div>
  );
};

const formatNumberOrDash = (value?: number, digits = 2): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '--';
  return value.toFixed(digits);
};

const formatPercentOrDash = (value?: number): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '--';
  return `${value.toFixed(2)}%`;
};

const formatCapValue = (value?: number): string => {
  if (value === undefined || value === null || Number.isNaN(value) || value <= 0) return '--';
  if (value >= 100000000) return `${(value / 100000000).toFixed(2)}亿`;
  return value.toFixed(2);
};

const getPriceColorClass = (value?: number, preClose?: number): string | undefined => {
  if (value === undefined || preClose === undefined || Number.isNaN(value) || Number.isNaN(preClose)) return undefined;
  if (value > preClose) return 'text-red-500';
  if (value < preClose) return 'text-green-500';
  return undefined;
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
