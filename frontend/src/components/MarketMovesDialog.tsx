import React, { useEffect, useState } from 'react';
import { Activity, RefreshCw, TrendingUp, X } from 'lucide-react';
import { GetBoardFundFlow, GetBoardLeaders, GetStockMoves } from '../../wailsjs/go/main/App';

interface MarketMovesDialogProps {
  isOpen: boolean;
  onClose: () => void;
}

type MoveTab = 'stock' | 'board';
type BoardCategory = 'industry' | 'concept' | 'region';
type StockMoveType = 'surge' | 'drop' | 'change_up' | 'change_down' | 'mainflow' | 'turnover';

interface BoardItem {
  code: string;
  name: string;
  changePercent: number;
  mainNetInflow: number;
  superNetInflow: number;
  largeNetInflow: number;
}

interface BoardFlowResult {
  category: string;
  items: BoardItem[];
  updateTime?: string;
}

interface LeaderItem {
  rank: number;
  code: string;
  name: string;
  price: number;
  changePercent: number;
  mainNetInflow: number;
  mainNetInflowRatio: number;
  score: number;
}

interface BoardLeaderResult {
  boardCode: string;
  items: LeaderItem[];
  updateTime?: string;
}

interface StockMoveItem {
  rank: number;
  code: string;
  name: string;
  price: number;
  changePercent: number;
  speed: number;
  turnoverRate: number;
  volume: number;
  amount: number;
  mainNetInflow: number;
  mainNetInflowRatio: number;
  updateTime?: string;
}

interface StockMoveResult {
  moveType: string;
  items: StockMoveItem[];
  updateTime?: string;
}

const CATEGORY_LABELS: Record<BoardCategory, string> = {
  industry: '行业',
  concept: '概念',
  region: '地域',
};

const STOCK_MOVE_LABELS: Record<StockMoveType, string> = {
  surge: '涨速榜',
  drop: '跌速榜',
  change_up: '涨幅榜',
  change_down: '跌幅榜',
  mainflow: '主力净流入',
  turnover: '换手活跃',
};

const formatAmount = (value: number): string => {
  if (!Number.isFinite(value) || value === 0) return '0';
  const abs = Math.abs(value);
  if (abs >= 1e8) return `${(value / 1e8).toFixed(2)}亿`;
  if (abs >= 1e4) return `${(value / 1e4).toFixed(2)}万`;
  return value.toFixed(0);
};

export const MarketMovesDialog: React.FC<MarketMovesDialogProps> = ({ isOpen, onClose }) => {
  const [activeTab, setActiveTab] = useState<MoveTab>('board');
  const [category, setCategory] = useState<BoardCategory>('industry');
  const [stockMoveType, setStockMoveType] = useState<StockMoveType>('surge');
  const [loading, setLoading] = useState(false);
  const [boardFlow, setBoardFlow] = useState<BoardFlowResult | null>(null);
  const [error, setError] = useState<string>('');
  const [stockLoading, setStockLoading] = useState(false);
  const [stockError, setStockError] = useState('');
  const [stockMoves, setStockMoves] = useState<StockMoveResult | null>(null);
  const [selectedBoard, setSelectedBoard] = useState<BoardItem | null>(null);
  const [leaderLoading, setLeaderLoading] = useState(false);
  const [leaderError, setLeaderError] = useState('');
  const [leaderResult, setLeaderResult] = useState<BoardLeaderResult | null>(null);

  const loadBoardLeaders = async (boardCode: string) => {
    setLeaderLoading(true);
    setLeaderError('');
    try {
      const result = await GetBoardLeaders(boardCode, 6);
      const normalized: BoardLeaderResult = {
        boardCode: result?.boardCode || boardCode,
        items: Array.isArray(result?.items) ? result.items : [],
        updateTime: result?.updateTime,
      };
      setLeaderResult(normalized);
      if (!normalized.items.length) {
        setLeaderError('该板块暂未返回可用龙头候选');
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '获取龙头推荐失败';
      setLeaderError(message);
      setLeaderResult(null);
    } finally {
      setLeaderLoading(false);
    }
  };

  const loadBoardFlow = async (nextCategory: BoardCategory) => {
    setLoading(true);
    setError('');
    try {
      const result = await GetBoardFundFlow(nextCategory, 1, 30);
      const normalized: BoardFlowResult = {
        category: result?.category || nextCategory,
        items: Array.isArray(result?.items) ? result.items : [],
        updateTime: result?.updateTime,
      };
      setBoardFlow(normalized);
      if (normalized.items.length > 0) {
        const first = normalized.items[0];
        setSelectedBoard(first);
        void loadBoardLeaders(first.code);
      } else {
        setSelectedBoard(null);
        setLeaderResult(null);
        setLeaderError('');
        setError('暂无可展示的板块异动数据');
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '获取板块异动失败';
      setError(message);
      setBoardFlow(null);
      setSelectedBoard(null);
      setLeaderResult(null);
      setLeaderError('');
    } finally {
      setLoading(false);
    }
  };

  const loadStockMoves = async (nextType: StockMoveType) => {
    setStockLoading(true);
    setStockError('');
    try {
      const result = await GetStockMoves(nextType, 1, 50);
      const normalized: StockMoveResult = {
        moveType: result?.moveType || nextType,
        items: Array.isArray(result?.items) ? result.items : [],
        updateTime: result?.updateTime,
      };
      setStockMoves(normalized);
      if (!normalized.items.length) {
        setStockError('暂无可展示的盘口异动数据');
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '获取盘口异动失败';
      setStockError(message);
      setStockMoves(null);
    } finally {
      setStockLoading(false);
    }
  };

  useEffect(() => {
    if (!isOpen) return;
    setActiveTab('board');
    setCategory('industry');
    setStockMoveType('surge');
    void loadBoardFlow('industry');
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen || activeTab !== 'stock') return;
    void loadStockMoves(stockMoveType);
  }, [isOpen, activeTab, stockMoveType]);

  const onChangeCategory = (nextCategory: BoardCategory) => {
    setCategory(nextCategory);
    void loadBoardFlow(nextCategory);
  };

  const onChangeStockMoveType = (nextType: StockMoveType) => {
    setStockMoveType(nextType);
    void loadStockMoves(nextType);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative w-[1120px] h-[760px] max-w-[92vw] max-h-[88vh] fin-panel border fin-divider rounded-xl shadow-2xl flex flex-col overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b fin-divider">
          <div className="flex items-center gap-3">
            <Activity className="w-5 h-5 text-accent-2" />
            <h2 className="text-lg font-semibold fin-text-primary">异动中心</h2>
          </div>
          <button onClick={onClose} className="p-2 rounded-lg fin-hover transition-colors">
            <X className="w-4 h-4 fin-text-secondary" />
          </button>
        </div>

        <div className="px-4 pt-3">
          <div className="flex items-center gap-2">
            <button
              onClick={() => setActiveTab('board')}
              className={`px-3 py-1.5 rounded-lg text-sm border transition-colors ${
                activeTab === 'board'
                  ? 'bg-accent/20 text-accent border-accent/40'
                  : 'fin-panel fin-text-secondary fin-divider hover:fin-text-primary'
              }`}
            >
              板块异动
            </button>
            <button
              onClick={() => setActiveTab('stock')}
              className={`px-3 py-1.5 rounded-lg text-sm border transition-colors ${
                activeTab === 'stock'
                  ? 'bg-accent/20 text-accent border-accent/40'
                  : 'fin-panel fin-text-secondary fin-divider hover:fin-text-primary'
              }`}
            >
              盘口异动
            </button>
          </div>
        </div>

        <div className="flex-1 px-4 py-3 overflow-hidden">
          {activeTab === 'stock' ? (
            <div className="h-full fin-panel border fin-divider rounded-lg p-4 overflow-auto fin-scrollbar">
              <div className="flex items-center justify-between mb-3">
                <div className="text-sm fin-text-primary font-medium">盘口异动</div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => loadStockMoves(stockMoveType)}
                    className="p-1.5 rounded fin-hover transition-colors"
                    disabled={stockLoading}
                    title="刷新"
                  >
                    <RefreshCw className={`w-4 h-4 fin-text-secondary ${stockLoading ? 'animate-spin' : ''}`} />
                  </button>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2 mb-3">
                {(Object.keys(STOCK_MOVE_LABELS) as StockMoveType[]).map((item) => (
                  <button
                    key={item}
                    onClick={() => onChangeStockMoveType(item)}
                    className={`px-2.5 py-1 rounded text-xs border ${stockMoveType === item
                      ? 'bg-accent/20 text-accent border-accent/40'
                      : 'fin-panel fin-text-secondary fin-divider hover:fin-text-primary'}`}
                  >
                    {STOCK_MOVE_LABELS[item]}
                  </button>
                ))}
                {stockMoves?.updateTime ? (
                  <span className="text-xs fin-text-tertiary">更新: {stockMoves.updateTime}</span>
                ) : null}
              </div>
              {stockLoading ? (
                <div className="h-[460px] flex items-center justify-center">
                  <RefreshCw className="w-5 h-5 fin-text-secondary animate-spin" />
                </div>
              ) : stockError ? (
                <div className="text-sm text-amber-300">{stockError}</div>
              ) : (
                <div className="overflow-auto fin-scrollbar h-[500px] rounded border fin-divider">
                  <table className="w-full text-sm">
                    <thead className="sticky top-0 fin-panel border-b fin-divider z-10">
                      <tr className="text-left">
                        <th className="px-3 py-2 font-medium fin-text-secondary">股票</th>
                        <th className="px-3 py-2 font-medium fin-text-secondary">现价</th>
                        <th className="px-3 py-2 font-medium fin-text-secondary">涨跌幅</th>
                        <th className="px-3 py-2 font-medium fin-text-secondary">涨速</th>
                        <th className="px-3 py-2 font-medium fin-text-secondary">主力净流入</th>
                        <th className="px-3 py-2 font-medium fin-text-secondary">换手率</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(stockMoves?.items || []).map((item) => (
                        <tr key={item.code} className="border-b fin-divider/40 hover:bg-slate-700/20">
                          <td className="px-3 py-2">
                            <div className="font-medium fin-text-primary">{item.rank}. {item.name}</div>
                            <div className="text-xs fin-text-tertiary font-mono">{item.code}</div>
                          </td>
                          <td className="px-3 py-2 font-mono fin-text-primary">{item.price.toFixed(2)}</td>
                          <td className={`px-3 py-2 font-mono ${item.changePercent >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                            {item.changePercent >= 0 ? '+' : ''}
                            {item.changePercent.toFixed(2)}%
                          </td>
                          <td className={`px-3 py-2 font-mono ${item.speed >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                            {item.speed >= 0 ? '+' : ''}
                            {item.speed.toFixed(2)}%
                          </td>
                          <td className={`px-3 py-2 font-mono ${item.mainNetInflow >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                            {formatAmount(item.mainNetInflow)}
                          </td>
                          <td className="px-3 py-2 font-mono fin-text-primary">{item.turnoverRate.toFixed(2)}%</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          ) : (
            <div className="h-full fin-panel border fin-divider rounded-lg overflow-hidden flex flex-col">
              <div className="px-4 py-3 border-b fin-divider flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <TrendingUp className="w-4 h-4 text-accent-2" />
                  <span className="text-sm fin-text-primary font-medium">板块异动（资金流）</span>
                  {boardFlow?.updateTime ? (
                    <span className="text-xs fin-text-tertiary">更新: {boardFlow.updateTime}</span>
                  ) : null}
                </div>
                <div className="flex items-center gap-2">
                  {(['industry', 'concept', 'region'] as BoardCategory[]).map((item) => (
                    <button
                      key={item}
                      onClick={() => onChangeCategory(item)}
                      className={`px-2.5 py-1 rounded text-xs border ${
                        category === item
                          ? 'bg-accent/20 text-accent border-accent/40'
                          : 'fin-panel fin-text-secondary fin-divider hover:fin-text-primary'
                      }`}
                    >
                      {CATEGORY_LABELS[item]}
                    </button>
                  ))}
                  <button
                    onClick={() => loadBoardFlow(category)}
                    className="p-1.5 rounded fin-hover transition-colors"
                    disabled={loading}
                    title="刷新"
                  >
                    <RefreshCw className={`w-4 h-4 fin-text-secondary ${loading ? 'animate-spin' : ''}`} />
                  </button>
                </div>
              </div>

              {loading ? (
                <div className="flex-1 flex items-center justify-center">
                  <RefreshCw className="w-5 h-5 fin-text-secondary animate-spin" />
                </div>
              ) : error ? (
                <div className="flex-1 px-4 py-3 text-sm text-amber-300">{error}</div>
              ) : (
                <div className="flex-1 min-h-0 flex">
                  <div className="w-[58%] overflow-auto fin-scrollbar border-r fin-divider">
                    <table className="w-full text-sm">
                      <thead className="sticky top-0 fin-panel border-b fin-divider z-10">
                        <tr className="text-left">
                          <th className="px-3 py-2 font-medium fin-text-secondary">板块</th>
                          <th className="px-3 py-2 font-medium fin-text-secondary">涨跌幅</th>
                          <th className="px-3 py-2 font-medium fin-text-secondary">主力净流入</th>
                          <th className="px-3 py-2 font-medium fin-text-secondary">超大单</th>
                          <th className="px-3 py-2 font-medium fin-text-secondary">大单</th>
                        </tr>
                      </thead>
                      <tbody>
                        {(boardFlow?.items || []).map((item) => {
                          const active = selectedBoard?.code === item.code;
                          return (
                            <tr
                              key={item.code}
                              className={`border-b fin-divider/40 cursor-pointer ${
                                active ? 'bg-accent/10' : 'hover:bg-slate-700/20'
                              }`}
                              onClick={() => {
                                setSelectedBoard(item);
                                void loadBoardLeaders(item.code);
                              }}
                            >
                              <td className="px-3 py-2">
                                <div className="font-medium fin-text-primary">{item.name}</div>
                                <div className="text-xs fin-text-tertiary font-mono">{item.code}</div>
                              </td>
                              <td className={`px-3 py-2 font-mono ${item.changePercent >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                                {item.changePercent >= 0 ? '+' : ''}
                                {item.changePercent.toFixed(2)}%
                              </td>
                              <td className={`px-3 py-2 font-mono ${item.mainNetInflow >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                                {formatAmount(item.mainNetInflow)}
                              </td>
                              <td className={`px-3 py-2 font-mono ${item.superNetInflow >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                                {formatAmount(item.superNetInflow)}
                              </td>
                              <td className={`px-3 py-2 font-mono ${item.largeNetInflow >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                                {formatAmount(item.largeNetInflow)}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                  <div className="flex-1 min-w-0 p-3 overflow-auto fin-scrollbar">
                    <div className="flex items-center justify-between mb-2">
                      <div className="text-sm fin-text-primary font-medium">龙头推荐</div>
                      {leaderResult?.updateTime ? (
                        <div className="text-xs fin-text-tertiary">更新: {leaderResult.updateTime}</div>
                      ) : null}
                    </div>
                    <div className="text-xs fin-text-tertiary mb-3">
                      推荐逻辑：涨跌幅 + 主力净流入 + 主力净流入占比综合评分
                    </div>
                    {selectedBoard ? (
                      <div className="text-xs fin-text-secondary mb-3">
                        当前板块：<span className="fin-text-primary">{selectedBoard.name}</span>
                        <span className="ml-1 font-mono fin-text-tertiary">{selectedBoard.code}</span>
                      </div>
                    ) : (
                      <div className="text-sm fin-text-tertiary">请先在左侧选择板块</div>
                    )}
                    {leaderLoading ? (
                      <div className="pt-6 flex items-center justify-center">
                        <RefreshCw className="w-4 h-4 fin-text-secondary animate-spin" />
                      </div>
                    ) : leaderError ? (
                      <div className="text-sm text-amber-300">{leaderError}</div>
                    ) : (
                      <div className="space-y-2">
                        {(leaderResult?.items || []).map((leader) => (
                          <div key={leader.code} className="fin-panel border fin-divider rounded-lg px-3 py-2">
                            <div className="flex items-center justify-between">
                              <div className="text-sm fin-text-primary">
                                {leader.rank}. {leader.name}
                                <span className="ml-1 text-xs font-mono fin-text-tertiary">{leader.code}</span>
                              </div>
                              <div className={`text-sm font-mono ${leader.changePercent >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                                {leader.changePercent >= 0 ? '+' : ''}
                                {leader.changePercent.toFixed(2)}%
                              </div>
                            </div>
                            <div className="mt-1 text-xs fin-text-secondary flex items-center justify-between">
                              <span>主力净流入: {formatAmount(leader.mainNetInflow)}</span>
                              <span>占比: {leader.mainNetInflowRatio.toFixed(2)}%</span>
                              <span>评分: {leader.score.toFixed(2)}</span>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
