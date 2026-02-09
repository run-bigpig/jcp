import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { StockList } from './components/StockList';
import { StockChart } from './components/StockChart';
import { OrderBook as OrderBookComponent } from './components/OrderBook';
import { AgentRoom } from './components/AgentRoom';
import { SettingsDialog } from './components/SettingsDialog';
import { PositionDialog } from './components/PositionDialog';
import { HotTrendDialog } from './components/HotTrendDialog';
import { WelcomePage } from './components/WelcomePage';
import { ThemeSwitcher } from './components/ThemeSwitcher';
import { getWatchlist, addToWatchlist, removeFromWatchlist } from './services/watchlistService';
import { getKLineData, getOrderBook } from './services/stockService';
import { getOrCreateSession, StockSession, updateStockPosition } from './services/sessionService';
import { useMarketEvents } from './hooks/useMarketEvents';
import { Stock, KLineData, OrderBook, TimePeriod, Telegraph, MarketIndex, MarketStatus } from './types';
import { Radio, Settings, List, Minus, Square, X, Copy, Briefcase, TrendingUp } from 'lucide-react';
import logo from './assets/images/logo.png';
import { GetTelegraphList, OpenURL, WindowMinimize, WindowMaximize, WindowClose } from '../wailsjs/go/main/App';
import { WindowIsMaximised } from '../wailsjs/runtime/runtime';

const App: React.FC = () => {
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
  const [marketStatus, setMarketStatus] = useState<MarketStatus | null>(null);
  const [marketIndices, setMarketIndices] = useState<MarketIndex[]>([]);
  const [isMaximized, setIsMaximized] = useState(false);

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
  const { subscribeOrderBook } = useMarketEvents({
    onStockUpdate: handleStockUpdate,
    onOrderBookUpdate: handleOrderBookUpdate,
    onTelegraphUpdate: handleTelegraphUpdate,
    onMarketStatusUpdate: handleMarketStatusUpdate,
    onMarketIndicesUpdate: handleMarketIndicesUpdate,
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
    const loadKLineData = async () => {
      // 分时图需要更多数据点（1分钟K线，一天约240根）
      const dataLen = timePeriod === '1m' ? 250 : 60;
      const data = await getKLineData(selectedSymbol, timePeriod, dataLen);
      setKLineData(data);
    };
    loadKLineData();
  }, [selectedSymbol, timePeriod]);

  // 初始化窗口最大化状态
  useEffect(() => {
    WindowIsMaximised().then(setIsMaximized);
  }, []);

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
          <span className="font-bold text-lg tracking-tight">韭菜盘 <span className="text-accent-2">AI</span></span>
        </div>
        
        <div className="flex items-center gap-4 fin-panel-soft px-4 py-1.5 rounded-full border fin-divider relative" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <Radio className="h-3 w-3 animate-pulse text-accent-2" />
          <span className="text-xs font-mono text-slate-300 w-96 truncate text-center">
            实时快讯: {marketMessage}
          </span>
          <button
            onClick={handleShowTelegraphList}
            className="p-1 rounded hover:bg-slate-700/50 text-slate-400 hover:text-accent-2 transition-colors"
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
              <div className="p-2 border-b fin-divider text-xs text-slate-400 font-medium">
                财联社快讯
              </div>
              {telegraphLoading ? (
                <div className="p-4 text-center text-slate-500 text-sm">加载中...</div>
              ) : telegraphList.length === 0 ? (
                <div className="p-4 text-center text-slate-500 text-sm">暂无快讯</div>
              ) : (
                telegraphList.map((tg, idx) => (
                  <div
                    key={idx}
                    onClick={() => handleOpenTelegraph(tg)}
                    className="p-3 border-b fin-divider last:border-b-0 hover:bg-slate-800/50 cursor-pointer transition-colors"
                  >
                    <div className="flex items-start gap-2">
                      <span className="text-xs text-accent-2 font-mono shrink-0">{tg.time}</span>
                      <span className="text-xs text-slate-300 line-clamp-2">{tg.content}</span>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        <div className="flex items-center gap-3" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <button
            onClick={() => setShowHotTrend(true)}
            className="p-2 rounded-lg fin-panel border fin-divider text-slate-300 hover:text-white hover:border-orange-400/40 transition-colors"
            title="全网热点"
          >
            <TrendingUp className="h-4 w-4" />
          </button>
          <ThemeSwitcher />
          <button
            onClick={() => setShowSettings(true)}
            className="p-2 rounded-lg fin-panel border fin-divider text-slate-300 hover:text-white hover:border-accent/40 transition-colors"
          >
            <Settings className="h-4 w-4" />
          </button>
          <div className="text-xs text-right hidden md:block">
            <div className="text-slate-400">市场状态</div>
            <div className={`font-bold ${
              marketStatus?.status === 'trading' ? 'text-green-500' :
              marketStatus?.status === 'pre_market' ? 'text-yellow-500' :
              marketStatus?.status === 'lunch_break' ? 'text-orange-500' :
              'text-slate-500'
            }`}>
              {marketStatus?.statusText || '加载中...'}
            </div>
          </div>
          {/* 窗口控制按钮 */}
          <div className="flex items-center ml-2 border-l fin-divider pl-3">
            <button
              onClick={() => WindowMinimize()}
              className="p-1.5 rounded hover:bg-slate-700/50 text-slate-400 hover:text-white transition-colors"
              title="最小化"
            >
              <Minus className="h-4 w-4" />
            </button>
            <button
              onClick={() => { WindowMaximize(); setIsMaximized(!isMaximized); }}
              className="p-1.5 rounded hover:bg-slate-700/50 text-slate-400 hover:text-white transition-colors"
              title={isMaximized ? "还原" : "最大化"}
            >
              {isMaximized ? <Copy className="h-3.5 w-3.5" /> : <Square className="h-3.5 w-3.5" />}
            </button>
            <button
              onClick={() => WindowClose()}
              className="p-1.5 rounded hover:bg-red-500/80 text-slate-400 hover:text-white transition-colors"
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
        <StockList
          stocks={watchlist}
          selectedSymbol={selectedSymbol}
          onSelect={handleSelectStock}
          onAddStock={handleAddStock}
          onRemoveStock={handleRemoveStock}
          marketIndices={marketIndices}
        />

        {/* Center Panel: Charts & Data */}
        <div className="flex-1 flex flex-col min-w-0 bg-transparent">
          {/* Stock Header - A股风格 */}
          <div className="fin-panel-strong border-b fin-divider px-6 py-3 shrink-0">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <span className="text-lg font-bold text-white">{selectedStock.name}</span>
                <span className="text-sm text-slate-400 font-mono">{selectedStock.symbol}</span>
                <button
                  onClick={() => setShowPosition(true)}
                  className="flex items-center gap-1 px-2 py-1 rounded text-xs text-slate-400 hover:text-accent-2 hover:bg-slate-700/50 transition-colors"
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
              <div className={`text-3xl font-mono font-bold ${selectedStock.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                {selectedStock.price.toFixed(2)}
              </div>
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4 text-sm">
                <span className={`font-mono ${selectedStock.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.change.toFixed(2)}
                </span>
                <span className={`font-mono ${selectedStock.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.changePercent.toFixed(2)}%
                </span>
              </div>
              <div className="text-xs text-slate-500">
                {new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
              </div>
            </div>
          </div>
          
          {/* A股传统行情数据 */}
          <div className="grid grid-cols-4 gap-px p-2 fin-panel border-b fin-divider shrink-0 text-xs">
            <AStockStatItem label="今开" value={selectedStock.open} preClose={selectedStock.preClose} />
            <AStockStatItem label="最高" value={selectedStock.high} preClose={selectedStock.preClose} />
            <AStockStatItem label="成交量" value={formatVolume(selectedStock.volume)} isPlain />
            <AStockStatItem label="昨收" value={selectedStock.preClose} isPlain />
            <AStockStatItem label="最低" value={selectedStock.low} preClose={selectedStock.preClose} />
            <AStockStatItem label="成交额" value={formatAmount(selectedStock.amount)} isPlain />
            <AStockStatItem label="振幅" value={selectedStock.preClose > 0 ? ((selectedStock.high - selectedStock.low) / selectedStock.preClose * 100).toFixed(2) + '%' : '--'} isPlain />
          </div>

          <div className="flex-1 flex flex-col min-h-0">
             {/* Chart Section */}
            <div className="flex-1 p-1 relative min-h-0">
               <StockChart
                  data={kLineData}
                  period={timePeriod}
                  onPeriodChange={setTimePeriod}
                  stock={selectedStock}
               />
            </div>
            
            {/* Bottom Info Panel: Order Book Only */}
            <div className="h-64 border-t fin-divider flex fin-panel shrink-0">
               <div className="flex-1 overflow-hidden relative">
                  <OrderBookComponent data={orderBook} />
               </div>
            </div>
          </div>
        </div>

        {/* Right Panel: AI Agents */}
        <AgentRoom
          stock={selectedStock}
          kLineData={kLineData}
          session={currentSession}
          onSessionUpdate={setCurrentSession}
        />
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
    </div>
  );
};

// A股行情数据项组件
interface AStockStatItemProps {
  label: string;
  value: number | string;
  preClose?: number;
  isPlain?: boolean;
}

const AStockStatItem: React.FC<AStockStatItemProps> = ({ label, value, preClose, isPlain }) => {
  let colorClass = 'text-slate-100';
  let displayValue = typeof value === 'string' ? value : value.toFixed(2);

  if (!isPlain && typeof value === 'number' && preClose) {
    if (value > preClose) colorClass = 'text-red-500';
    else if (value < preClose) colorClass = 'text-green-500';
  }

  return (
    <div className="flex justify-between items-center px-3 py-1.5">
      <span className="text-slate-500">{label}</span>
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
