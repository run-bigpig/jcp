import React, { useState, useEffect } from 'react';
import { X, TrendingUp, RefreshCw, ExternalLink } from 'lucide-react';
import { GetAllHotTrends, OpenURL } from '../../wailsjs/go/main/App';
import { hottrend } from '../../wailsjs/go/models';
import { useTheme } from '../contexts/ThemeContext';

interface HotTrendDialogProps {
  isOpen: boolean;
  onClose: () => void;
}

export const HotTrendDialog: React.FC<HotTrendDialogProps> = ({ isOpen, onClose }) => {
  const { colors } = useTheme();
  const [results, setResults] = useState<hottrend.HotTrendResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedPlatform, setSelectedPlatform] = useState<string>('');

  // 加载热点数据
  const loadHotTrends = async () => {
    setLoading(true);
    try {
      const data = await GetAllHotTrends();
      setResults(data || []);
      if (data && data.length > 0 && !selectedPlatform) {
        setSelectedPlatform(data[0].platform);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      loadHotTrends();
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const currentResult = results.find(r => r.platform === selectedPlatform);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* 背景遮罩 */}
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />

      {/* 弹窗内容 */}
      <div className="relative w-[1120px] h-[760px] max-w-[92vw] max-h-[88vh] fin-panel border fin-divider rounded-xl shadow-2xl flex flex-col overflow-hidden">
        {/* 头部 */}
        <DialogHeader onClose={onClose} onRefresh={loadHotTrends} loading={loading} />

        {/* 主体 */}
        <div className="flex-1 flex overflow-hidden">
          {/* 左侧平台列表 */}
          <PlatformList
            results={results}
            selectedPlatform={selectedPlatform}
            onSelect={setSelectedPlatform}
            isDark={colors.isDark}
          />

          {/* 右侧热点列表 */}
          <HotItemList key={selectedPlatform} result={currentResult} loading={loading} isDark={colors.isDark} />
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
}> = ({ onClose, onRefresh, loading }) => {
  const { colors } = useTheme();
  return (
    <div className="flex items-center justify-between px-5 py-4 border-b fin-divider shrink-0">
      <div className="flex items-center gap-3">
        <div className="p-2 rounded-lg bg-gradient-to-br from-orange-500 to-red-500">
          <TrendingUp className="h-5 w-5 text-white" />
        </div>
        <div>
          <h2 className={`text-lg font-bold ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>全网热点</h2>
          <p className={`text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>实时舆情监控</p>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={onRefresh}
          disabled={loading}
          className={`p-2 rounded-lg transition-colors disabled:opacity-50 ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400 hover:text-white' : 'hover:bg-slate-200/50 text-slate-500 hover:text-slate-700'}`}
          title="刷新"
        >
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
        <button
          onClick={onClose}
          className={`p-2 rounded-lg transition-colors ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400 hover:text-white' : 'hover:bg-slate-200/50 text-slate-500 hover:text-slate-700'}`}
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
};

// 左侧平台列表组件
const PlatformList: React.FC<{
  results: hottrend.HotTrendResult[];
  selectedPlatform: string;
  onSelect: (platform: string) => void;
  isDark: boolean;
}> = ({ results, selectedPlatform, onSelect, isDark }) => (
  <div className="w-40 border-r fin-divider p-2 space-y-1 shrink-0">
    {results.map(result => (
      <button
        key={result.platform}
        onClick={() => onSelect(result.platform)}
        className={`w-full flex items-center gap-2 px-3 py-2.5 rounded-lg text-left transition-colors ${
          selectedPlatform === result.platform
            ? 'bg-accent/20 text-accent-2 border border-accent/30'
            : (isDark ? 'hover:bg-slate-700/50 text-slate-300' : 'hover:bg-slate-200/50 text-slate-600')
        }`}
      >
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium truncate">{result.platform_cn}</div>
          {result.error ? (
            <div className="text-xs text-red-400">加载失败</div>
          ) : (
            <div className={`text-xs ${isDark ? 'text-slate-500' : 'text-slate-400'}`}>{result.items?.length || 0} 条</div>
          )}
        </div>
      </button>
    ))}
  </div>
);

// 右侧热点列表组件
const HotItemList: React.FC<{
  result?: hottrend.HotTrendResult;
  loading: boolean;
  isDark: boolean;
}> = ({ result, loading, isDark }) => {
  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <RefreshCw className={`h-8 w-8 animate-spin ${isDark ? 'text-slate-500' : 'text-slate-400'}`} />
      </div>
    );
  }

  if (!result) {
    return (
      <div className={`flex-1 flex items-center justify-center ${isDark ? 'text-slate-500' : 'text-slate-400'}`}>
        请选择平台
      </div>
    );
  }

  if (result.error) {
    return (
      <div className="flex-1 flex items-center justify-center text-red-400">
        {result.error}
      </div>
    );
  }

  const items = result.items || [];

  return (
    <div className="flex-1 overflow-y-auto fin-scrollbar p-3">
      {items.map((item, idx) => (
        <HotItemRow key={item.id || idx} item={item} isDark={isDark} />
      ))}
    </div>
  );
};

// 单条热点行组件
const HotItemRow: React.FC<{ item: hottrend.HotItem; isDark: boolean }> = ({ item, isDark }) => {
  const handleClick = () => {
    if (item.url) {
      OpenURL(item.url);
    }
  };

  // 排名颜色
  const getRankColor = (rank: number) => {
    if (rank === 1) return 'bg-red-500 text-white';
    if (rank === 2) return 'bg-orange-500 text-white';
    if (rank === 3) return 'bg-yellow-500 text-white';
    return isDark ? 'bg-slate-600 text-slate-300' : 'bg-slate-300 text-slate-600';
  };

  return (
    <div
      onClick={handleClick}
      className={`flex items-center gap-3 px-3 py-2.5 rounded-lg cursor-pointer transition-colors group ${isDark ? 'hover:bg-slate-700/50' : 'hover:bg-slate-200/50'}`}
    >
      <span className={`w-6 h-6 flex items-center justify-center rounded text-xs font-bold ${getRankColor(item.rank)}`}>
        {item.rank}
      </span>
      <span className={`flex-1 text-sm truncate ${isDark ? 'text-slate-200 group-hover:text-white' : 'text-slate-700 group-hover:text-slate-900'}`}>
        {item.title}
      </span>
      {item.extra && (
        <span className={`text-xs shrink-0 ${isDark ? 'text-slate-500' : 'text-slate-400'}`}>{item.extra}</span>
      )}
      <ExternalLink className={`h-3.5 w-3.5 opacity-0 group-hover:opacity-100 transition-opacity shrink-0 ${isDark ? 'text-slate-500' : 'text-slate-400'}`} />
    </div>
  );
};
