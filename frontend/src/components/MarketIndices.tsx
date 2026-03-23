import React from 'react';
import { MarketIndex } from '../types';

interface MarketIndicesProps {
  indices: MarketIndex[];
  selectedCode?: string;
  onSelect?: (index: MarketIndex) => void;
}

export const MarketIndices: React.FC<MarketIndicesProps> = ({ indices, selectedCode, onSelect }) => {
  if (!indices || indices.length === 0) {
    return (
      <div className="flex items-center gap-4 text-xs text-slate-500">
        <span>大盘数据加载中...</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-3">
      {indices.map((index) => (
        <MarketIndexItem
          key={index.code}
          index={index}
          isSelected={selectedCode === index.code}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
};

interface MarketIndexItemProps {
  index: MarketIndex;
  isSelected?: boolean;
  onSelect?: (index: MarketIndex) => void;
}

const MarketIndexItem: React.FC<MarketIndexItemProps> = ({ index, isSelected, onSelect }) => {
  const isUp = index.change >= 0;
  const colorClass = isUp ? 'text-red-500' : 'text-green-500';
  const sign = isUp ? '+' : '';
  const selectable = Boolean(onSelect);

  return (
    <button
      type="button"
      disabled={!selectable}
      aria-pressed={isSelected}
      aria-label={`${index.name} 指数`}
      title={selectable ? '点击查看指数K线' : '指数不可选'}
      onClick={() => {
        if (onSelect) {
          onSelect(index);
        }
      }}
      className={`flex flex-col items-center min-w-[72px] px-2 py-1 rounded bg-transparent border transition-colors cursor-pointer focus:outline-none disabled:cursor-default disabled:hover:bg-transparent ${
        isSelected ? 'border-accent/40 bg-accent/10' : 'border-transparent hover:bg-slate-700/30'
      }`}
    >
      <span className="text-xs text-slate-400 truncate max-w-16">{index.name}</span>
      <span className={`text-sm font-mono font-medium tabular-nums ${colorClass}`}>
        {index.price.toFixed(2)}
      </span>
      <span className={`text-xs font-mono tabular-nums ${colorClass}`}>
        {sign}{index.changePercent.toFixed(2)}%
      </span>
    </button>
  );
};
