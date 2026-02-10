import React, { useState, useEffect } from 'react';
import { X, TrendingUp, RefreshCw, Calendar } from 'lucide-react';
import { GetLongHuBangList, GetLongHuBangDetail } from '../../wailsjs/go/main/App';
import { models } from '../../wailsjs/go/models';

interface LongHuBangDialogProps {
  isOpen: boolean;
  onClose: () => void;
}

// 获取默认交易日期：16点前用前一天，16点后用当天
const getDefaultTradeDate = (): string => {
  const now = new Date();
  const hour = now.getHours();
  if (hour < 16) {
    now.setDate(now.getDate() - 1);
  }
  return now.toISOString().split('T')[0];
};

export const LongHuBangDialog: React.FC<LongHuBangDialogProps> = ({ isOpen, onClose }) => {
  const [items, setItems] = useState<models.LongHuBangItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [selectedItem, setSelectedItem] = useState<models.LongHuBangItem | null>(null);
  const [details, setDetails] = useState<models.LongHuBangDetail[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [tradeDate, setTradeDate] = useState('');
  const [pageNumber, setPageNumber] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const pageSize = 30;

  const loadList = async (page: number, date: string, append = false) => {
    if (append) {
      setLoadingMore(true);
    } else {
      setLoading(true);
    }
    try {
      const result = await GetLongHuBangList(pageSize, page, date);
      if (result) {
        const newItems = result.items || [];
        if (append) {
          setItems(prev => [...prev, ...newItems]);
        } else {
          setItems(newItems);
        }
        setHasMore(newItems.length >= pageSize);
      } else {
        if (!append) setItems([]);
        setHasMore(false);
      }
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      const defaultDate = getDefaultTradeDate();
      setPageNumber(1);
      setTradeDate(defaultDate);
      setHasMore(true);
      loadList(1, defaultDate, false);
      setSelectedItem(null);
      setDetails([]);
    }
  }, [isOpen]);

  const handleDateChange = (date: string) => {
    setTradeDate(date);
    setPageNumber(1);
    setHasMore(true);
    loadList(1, date, false);
  };

  const handleLoadMore = () => {
    if (!loadingMore && hasMore) {
      const nextPage = pageNumber + 1;
      setPageNumber(nextPage);
      loadList(nextPage, tradeDate, true);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative w-[950px] h-[700px] fin-panel border fin-divider rounded-xl shadow-2xl flex flex-col overflow-hidden">
        <DialogHeader
          onClose={onClose}
          onRefresh={() => loadList(1, tradeDate, false)}
          loading={loading}
          tradeDate={tradeDate}
          onDateChange={handleDateChange}
        />
        <div className="flex-1 flex overflow-hidden">
          <ItemList
            items={items}
            loading={loading}
            loadingMore={loadingMore}
            hasMore={hasMore}
            selectedItem={selectedItem}
            onSelect={setSelectedItem}
            setDetails={setDetails}
            setDetailLoading={setDetailLoading}
            onLoadMore={handleLoadMore}
          />
          <DetailPanel
            item={selectedItem}
            details={details}
            loading={detailLoading}
          />
        </div>
      </div>
    </div>
  );
};

// 头部组件
const DialogHeader: React.FC<{
  onClose: () => void;
  onRefresh: () => void;
  loading: boolean;
  tradeDate: string;
  onDateChange: (date: string) => void;
}> = ({ onClose, onRefresh, loading, tradeDate, onDateChange }) => (
  <div className="flex items-center justify-between px-5 py-4 border-b fin-divider">
    <div className="flex items-center gap-3">
      <TrendingUp className="w-5 h-5 text-red-500" />
      <h2 className="text-lg font-semibold fin-text-primary">龙虎榜</h2>
    </div>
    <div className="flex items-center gap-3">
      <div className="flex items-center gap-2">
        <Calendar className="w-4 h-4 fin-text-tertiary" />
        <input
          type="date"
          value={tradeDate}
          onChange={(e) => onDateChange(e.target.value)}
          className="px-2 py-1 text-sm rounded-lg fin-panel border fin-divider fin-text-primary bg-transparent focus:outline-none focus:ring-1 focus:ring-accent"
          max={new Date().toISOString().split('T')[0]}
        />
        {tradeDate && (
          <button
            onClick={() => onDateChange('')}
            className="text-xs fin-text-tertiary hover:fin-text-secondary"
          >
            清除
          </button>
        )}
      </div>
      <button
        onClick={onRefresh}
        disabled={loading}
        className="p-2 rounded-lg fin-hover transition-colors"
      >
        <RefreshCw className={`w-4 h-4 fin-text-secondary ${loading ? 'animate-spin' : ''}`} />
      </button>
      <button onClick={onClose} className="p-2 rounded-lg fin-hover transition-colors">
        <X className="w-4 h-4 fin-text-secondary" />
      </button>
    </div>
  </div>
);

// 列表组件
const ItemList: React.FC<{
  items: models.LongHuBangItem[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  selectedItem: models.LongHuBangItem | null;
  onSelect: (item: models.LongHuBangItem) => void;
  setDetails: (details: models.LongHuBangDetail[]) => void;
  setDetailLoading: (loading: boolean) => void;
  onLoadMore: () => void;
}> = ({ items, loading, loadingMore, hasMore, selectedItem, onSelect, setDetails, setDetailLoading, onLoadMore }) => {
  const listRef = React.useRef<HTMLDivElement>(null);

  // 滚动到底部时加载更多
  const handleScroll = () => {
    const el = listRef.current;
    if (!el || loadingMore || !hasMore) return;
    if (el.scrollHeight - el.scrollTop - el.clientHeight < 100) {
      onLoadMore();
    }
  };

  const handleSelect = async (item: models.LongHuBangItem) => {
    onSelect(item);
    setDetailLoading(true);
    try {
      const data = await GetLongHuBangDetail(item.code, item.tradeDate);
      setDetails(data || []);
    } finally {
      setDetailLoading(false);
    }
  };

  const formatAmount = (amt: number) => {
    if (Math.abs(amt) >= 100000000) {
      return (amt / 100000000).toFixed(2) + '亿';
    }
    return (amt / 10000).toFixed(0) + '万';
  };

  if (loading) {
    return (
      <div className="w-[380px] border-r fin-divider flex items-center justify-center">
        <RefreshCw className="w-6 h-6 fin-text-secondary animate-spin" />
      </div>
    );
  }

  return (
    <div
      ref={listRef}
      onScroll={handleScroll}
      className="w-[380px] border-r fin-divider overflow-y-auto fin-scrollbar"
    >
      {items.map((item, idx) => (
        <div
          key={`${item.code}-${item.tradeDate}-${idx}`}
          onClick={() => handleSelect(item)}
          className={`px-4 py-3 border-b fin-divider cursor-pointer transition-all ${
            selectedItem?.code === item.code && selectedItem?.tradeDate === item.tradeDate
              ? 'bg-accent/10 border-l-2 border-l-accent'
              : 'hover:bg-slate-500/5 border-l-2 border-l-transparent'
          }`}
        >
          <div className="flex items-center justify-between mb-1.5">
            <div className="flex items-center gap-2">
              <span className="font-medium fin-text-primary">{item.name}</span>
              <span className="text-xs fin-text-tertiary font-mono">{item.code}</span>
            </div>
            <span className={`text-sm font-mono font-medium ${item.changePercent >= 0 ? 'text-red-500' : 'text-green-500'}`}>
              {item.changePercent >= 0 ? '+' : ''}{item.changePercent.toFixed(2)}%
            </span>
          </div>
          <div className="flex items-center justify-between text-xs">
            <span className="fin-text-tertiary">{item.tradeDate}</span>
            <span className={`font-mono ${item.netBuyAmt >= 0 ? 'text-red-400' : 'text-green-400'}`}>
              净买入 {formatAmount(item.netBuyAmt)}
            </span>
          </div>
          <div className="text-xs fin-text-tertiary mt-1.5 truncate">{item.reason}</div>
        </div>
      ))}
      {loadingMore && (
        <div className="py-4 flex items-center justify-center">
          <RefreshCw className="w-4 h-4 fin-text-secondary animate-spin" />
          <span className="ml-2 text-xs fin-text-tertiary">加载中...</span>
        </div>
      )}
      {!hasMore && items.length > 0 && (
        <div className="py-4 text-center text-xs fin-text-tertiary">
          没有更多数据了
        </div>
      )}
    </div>
  );
};

// 营业部行组件
const BrokerRow: React.FC<{
  index: number;
  detail: models.LongHuBangDetail;
  type: 'buy' | 'sell';
  formatAmount: (amt: number) => string;
}> = ({ index, detail, type, formatAmount }) => {
  const amt = type === 'buy' ? detail.buyAmt : detail.sellAmt;
  const percent = type === 'buy' ? detail.buyPercent : detail.sellPercent;

  return (
    <div className="flex items-center text-sm px-2 py-2 rounded hover:bg-slate-500/5 transition-colors">
      <span className="w-5 text-xs fin-text-tertiary">{index + 1}</span>
      <span className="flex-1 truncate fin-text-primary text-xs">{detail.operName}</span>
      <span className={`w-20 text-right font-mono ${type === 'buy' ? 'text-red-400' : 'text-green-400'}`}>
        {formatAmount(amt)}
      </span>
      <span className="w-16 text-right text-xs fin-text-tertiary">
        {percent.toFixed(2)}%
      </span>
    </div>
  );
};

// 营业部列表组件
const BrokerSection: React.FC<{
  title: string;
  details: models.LongHuBangDetail[];
  type: 'buy' | 'sell';
  formatAmount: (amt: number) => string;
}> = ({ title, details, type, formatAmount }) => (
  <div className="mb-5">
    <div className={`flex items-center gap-2 mb-3 pb-2 border-b ${type === 'buy' ? 'border-red-500/20' : 'border-green-500/20'}`}>
      <div className={`w-1 h-4 rounded ${type === 'buy' ? 'bg-red-500' : 'bg-green-500'}`} />
      <h3 className={`text-sm font-medium ${type === 'buy' ? 'text-red-500' : 'text-green-500'}`}>
        {title}
      </h3>
    </div>
    {details.length === 0 ? (
      <div className="text-sm fin-text-tertiary text-center py-4">暂无数据</div>
    ) : (
      <div className="space-y-1">
        {details.slice(0, 5).map((d, idx) => (
          <BrokerRow key={idx} index={idx} detail={d} type={type} formatAmount={formatAmount} />
        ))}
      </div>
    )}
  </div>
);

// 统计卡片组件
const StatCard: React.FC<{
  label: string;
  value: string;
  valueClass?: string;
}> = ({ label, value, valueClass = 'fin-text-primary' }) => (
  <div className="px-3 py-2 rounded-lg bg-slate-500/5">
    <div className="text-xs fin-text-tertiary mb-1">{label}</div>
    <div className={`text-sm font-mono font-medium ${valueClass}`}>{value}</div>
  </div>
);

// 股票头部信息
const StockHeader: React.FC<{
  item: models.LongHuBangItem;
  formatAmount: (amt: number) => string;
}> = ({ item, formatAmount }) => (
  <div className="mb-5">
    <div className="flex items-baseline gap-3 mb-4">
      <span className="text-2xl font-bold fin-text-primary">{item.name}</span>
      <span className="text-sm fin-text-tertiary font-mono">{item.code}</span>
      <span className={`text-lg font-mono font-semibold ml-auto ${item.changePercent >= 0 ? 'text-red-500' : 'text-green-500'}`}>
        {item.changePercent >= 0 ? '+' : ''}{item.changePercent.toFixed(2)}%
      </span>
    </div>
    <div className="grid grid-cols-2 gap-3">
      <StatCard label="收盘价" value={item.closePrice.toFixed(2)} />
      <StatCard label="换手率" value={`${item.turnoverRate.toFixed(2)}%`} />
      <StatCard label="净买入" value={formatAmount(item.netBuyAmt)} valueClass="text-red-500" />
      <StatCard label="成交占比" value={`${item.dealRatio.toFixed(2)}%`} />
      <StatCard label="买入额" value={formatAmount(item.buyAmt)} valueClass="text-red-400" />
      <StatCard label="卖出额" value={formatAmount(item.sellAmt)} valueClass="text-green-400" />
    </div>
    <div className="mt-3 px-3 py-2 rounded-lg bg-slate-500/5">
      <span className="text-xs fin-text-tertiary">上榜原因: </span>
      <span className="text-xs fin-text-secondary">{item.reason}</span>
    </div>
  </div>
);

// 详情面板组件
const DetailPanel: React.FC<{
  item: models.LongHuBangItem | null;
  details: models.LongHuBangDetail[];
  loading: boolean;
}> = ({ item, details, loading }) => {
  const formatAmount = (amt: number) => {
    if (Math.abs(amt) >= 100000000) {
      return (amt / 100000000).toFixed(2) + '亿';
    }
    return (amt / 10000).toFixed(0) + '万';
  };

  if (!item) {
    return (
      <div className="flex-1 flex items-center justify-center fin-text-tertiary">
        请选择左侧股票查看营业部明细
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <RefreshCw className="w-6 h-6 fin-text-secondary animate-spin" />
      </div>
    );
  }

  const buyDetails = details.filter(d => d.direction === 'buy');
  const sellDetails = details.filter(d => d.direction === 'sell');

  return (
    <div className="flex-1 overflow-y-auto p-4 fin-scrollbar">
      <StockHeader item={item} formatAmount={formatAmount} />
      <BrokerSection title="买入前五营业部" details={buyDetails} type="buy" formatAmount={formatAmount} />
      <BrokerSection title="卖出前五营业部" details={sellDetails} type="sell" formatAmount={formatAmount} />
    </div>
  );
};
