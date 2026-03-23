import React, { useState, useEffect, useRef } from 'react';
import { Stock, MarketIndex } from '../types';
import { searchStocks, StockSearchResult } from '../services/stockService';
import { TrendingUp, TrendingDown, Search, X } from 'lucide-react';
import { MarketIndices } from './MarketIndices';

interface StockListProps {
  stocks: Stock[]; // The current watchlist
  selectedSymbol: string;
  onSelect: (symbol: string) => void;
  onAddStock: (stock: Stock) => void;
  onRemoveStock?: (symbol: string) => void;
  marketIndices?: MarketIndex[];
  selectedIndexCode?: string;
  onSelectIndex?: (index: MarketIndex) => void;
}

export const StockList: React.FC<StockListProps> = ({
  stocks,
  selectedSymbol,
  onSelect,
  onAddStock,
  onRemoveStock,
  marketIndices,
  selectedIndexCode,
  onSelectIndex
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [searchResults, setSearchResults] = useState<StockSearchResult[]>([]);
  const [showDropdown, setShowDropdown] = useState(false);
  const [isSearching, setIsSearching] = useState(false);
  const searchRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  // 点击外部关闭下拉
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // 搜索防抖
  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }

    if (!searchTerm.trim()) {
      setSearchResults([]);
      setShowDropdown(false);
      return;
    }

    setIsSearching(true);
    debounceRef.current = setTimeout(async () => {
      const results = await searchStocks(searchTerm);
      // 确保 results 是数组，并过滤掉已在自选股中的股票
      const safeResults = Array.isArray(results) ? results : [];
      const filteredResults = safeResults.filter(
        r => !stocks.some(s => s.symbol === r.symbol)
      );
      setSearchResults(filteredResults);
      setShowDropdown(filteredResults.length > 0);
      setIsSearching(false);
    }, 300);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [searchTerm]);

  // 选择搜索结果添加股票
  const handleSelectResult = (result: StockSearchResult) => {
    const newStock: Stock = {
      symbol: result.symbol,
      name: result.name,
      price: 0,
      change: 0,
      changePercent: 0,
      volume: 0,
      amount: 0,
      marketCap: '',
      sector: result.industry,
      open: 0,
      high: 0,
      low: 0,
      preClose: 0,
    };
    onAddStock(newStock);
    setSearchTerm('');
    setShowDropdown(false);
  };

  return (
    <div className="flex flex-col h-full fin-panel border-r fin-divider w-full relative">
      <div className="p-4 fin-panel-strong border-b fin-divider">
        {/* 大盘指数 */}
        <div className="mb-4 pb-3 border-b fin-divider flex justify-center">
          <MarketIndices
            indices={marketIndices || []}
            selectedCode={selectedIndexCode}
            onSelect={onSelectIndex}
          />
        </div>
        <div ref={searchRef} className="relative z-50">
          <div className="relative">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              onFocus={() => searchResults.length > 0 && setShowDropdown(true)}
              placeholder="搜索股票代码或名称..."
              className="w-full fin-input rounded-lg pl-9 pr-4 py-2 text-sm placeholder-slate-500"
            />
            {isSearching && (
              <div className="absolute right-3 top-2.5 h-4 w-4 border-2 border-accent border-t-transparent rounded-full animate-spin" />
            )}
          </div>

          {/* 搜索下拉结果 */}
          {showDropdown && (
            <div className="absolute top-full left-0 right-0 mt-1 max-h-64 overflow-y-auto bg-slate-800 border border-slate-600 rounded-lg shadow-xl">
              {searchResults.map((result) => (
                <div
                  key={result.symbol}
                  onClick={() => handleSelectResult(result)}
                  className="px-3 py-2 hover:bg-slate-700 cursor-pointer border-b border-slate-700 last:border-b-0"
                >
                  <div className="flex justify-between items-center">
                    <div>
                      <span className="text-slate-200">{result.name}</span>
                      <span className="ml-2 font-mono text-accent-2 text-sm">{result.symbol}</span>
                    </div>
                    <span className="text-xs text-slate-500">{result.market}</span>
                  </div>
                  {result.industry && (
                    <div className="text-xs text-slate-500 mt-0.5">{result.industry}</div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
      
      <div className="flex-1 overflow-y-auto fin-scrollbar">
        {stocks.map((stock) => {
          const isSelected = stock.symbol === selectedSymbol;
          const isPositive = stock.change >= 0;

          return (
            <div
              key={stock.symbol}
              onClick={() => onSelect(stock.symbol)}
              className={`group px-3 py-2.5 border-b fin-divider cursor-pointer transition-colors hover:bg-slate-800/60 ${isSelected ? 'bg-slate-800/60 border-l-4 border-l-accent' : 'border-l-4 border-l-transparent'}`}
            >
              <div className="flex justify-between items-start mb-0.5">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <span className="font-bold text-slate-100 truncate">{stock.name}</span>
                    <span className="text-[10px] font-mono text-slate-400 fin-chip px-1 py-0.5 rounded shrink-0">
                      {stock.symbol}
                    </span>
                    {onRemoveStock && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          onRemoveStock(stock.symbol);
                        }}
                        className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-red-500/20 text-slate-500 hover:text-red-400 transition-all"
                      >
                        <X size={14} />
                      </button>
                    )}
                  </div>
                </div>
                <div className="text-right">
                  <div className={`font-mono ${isPositive ? 'text-red-500' : 'text-green-500'}`}>
                    {stock.price.toFixed(2)}
                  </div>
                  <div className={`text-xs font-mono flex items-center justify-end ${isPositive ? 'text-red-500' : 'text-green-500'}`}>
                    {isPositive ? <TrendingUp size={12} className="mr-1"/> : <TrendingDown size={12} className="mr-1"/>}
                    {isPositive ? '+' : ''}{stock.changePercent.toFixed(2)}%
                  </div>
                </div>
              </div>
              <div className="flex justify-between items-center text-[11px] text-slate-500 mt-1">
                <span>量: {formatVolume(stock.volume)}</span>
                {stock.sector && (
                  <span className="fin-chip px-1 py-0.5 rounded text-[10px] text-slate-300">{stock.sector}</span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

// 格式化成交量
const formatVolume = (vol: number): string => {
  if (vol >= 100000000) return (vol / 100000000).toFixed(2) + '亿';
  if (vol >= 10000) return (vol / 10000).toFixed(0) + '万';
  return vol.toString();
};
