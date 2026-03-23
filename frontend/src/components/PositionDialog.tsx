import React, { useState, useEffect } from 'react';
import { X, Briefcase } from 'lucide-react';
import type { StockPosition } from '../types';
import { useTheme } from '../contexts/ThemeContext';

interface PositionDialogProps {
  isOpen: boolean;
  onClose: () => void;
  stockCode: string;
  stockName: string;
  currentPrice: number;
  position?: StockPosition;
  onSave: (shares: number, costPrice: number) => Promise<void> | void;
}

export const PositionDialog: React.FC<PositionDialogProps> = ({
  isOpen,
  onClose,
  stockCode,
  stockName,
  currentPrice,
  position,
  onSave,
}) => {
  const { colors } = useTheme();
  const [shares, setShares] = useState<string>('');
  const [costPrice, setCostPrice] = useState<string>('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (isOpen && position) {
      setShares(position.shares > 0 ? position.shares.toString() : '');
      setCostPrice(position.costPrice > 0 ? position.costPrice.toString() : '');
    } else if (isOpen) {
      setShares('');
      setCostPrice('');
    }
    if (isOpen) {
      setSaving(false);
      setError('');
    }
  }, [isOpen, position]);

  if (!isOpen) return null;

  const sharesNum = parseInt(shares) || 0;
  const costPriceNum = parseFloat(costPrice) || 0;
  const costAmount = sharesNum * costPriceNum;
  const marketValue = sharesNum * currentPrice;
  const profitLoss = marketValue - costAmount;
  const profitLossPercent = costAmount > 0 ? (profitLoss / costAmount) * 100 : 0;

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      await onSave(sharesNum, costPriceNum);
      onClose();
    } catch (e) {
      const msg = e instanceof Error ? e.message : '持仓保存失败';
      setError(msg || '持仓保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleClear = async () => {
    setSaving(true);
    setError('');
    try {
      await onSave(0, 0);
      onClose();
    } catch (e) {
      const msg = e instanceof Error ? e.message : '清空持仓失败';
      setError(msg || '清空持仓失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div className="relative w-96 fin-panel border fin-divider rounded-xl shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b fin-divider">
          <div className="flex items-center gap-2">
            <Briefcase className="h-5 w-5 text-accent-2" />
            <span className={`font-bold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>持仓设置</span>
          </div>
          <button
            onClick={onClose}
            className={`p-1 rounded transition-colors ${colors.isDark ? 'hover:bg-slate-700 text-slate-400 hover:text-white' : 'hover:bg-slate-200 text-slate-500 hover:text-slate-700'}`}
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Stock Info */}
        <div className={`px-4 py-3 border-b fin-divider ${colors.isDark ? 'bg-slate-800/30' : 'bg-slate-100/50'}`}>
          <div className="flex justify-between items-center">
            <div>
              <span className={`font-medium ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>{stockName}</span>
              <span className={`ml-2 text-sm font-mono ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{stockCode}</span>
            </div>
            <span className="text-lg font-mono text-accent-2">{currentPrice.toFixed(2)}</span>
          </div>
        </div>

        {/* Form */}
        <div className="p-4 space-y-4">
          <div>
            <label className={`block text-sm mb-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>持仓数量（股）</label>
            <input
              type="number"
              value={shares}
              onChange={(e) => setShares(e.target.value)}
              placeholder="请输入持仓数量"
              className="w-full fin-input rounded-lg px-3 py-2 text-sm"
              min="0"
              step="100"
              disabled={saving}
            />
          </div>
          <div>
            <label className={`block text-sm mb-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>成本价（元）</label>
            <input
              type="number"
              value={costPrice}
              onChange={(e) => setCostPrice(e.target.value)}
              placeholder="请输入成本价"
              className="w-full fin-input rounded-lg px-3 py-2 text-sm"
              min="0"
              step="0.01"
              disabled={saving}
            />
          </div>

          {error && (
            <div className="text-xs text-red-400">{error}</div>
          )}

          {/* Calculated Info */}
          {sharesNum > 0 && costPriceNum > 0 && (
            <div className={`p-3 rounded-lg space-y-2 text-sm ${colors.isDark ? 'bg-slate-800/50' : 'bg-slate-100'}`}>
              <div className="flex justify-between">
                <span className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>成本金额</span>
                <span className={`font-mono ${colors.isDark ? 'text-slate-200' : 'text-slate-700'}`}>{costAmount.toFixed(2)}</span>
              </div>
              <div className="flex justify-between">
                <span className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>市值</span>
                <span className={`font-mono ${colors.isDark ? 'text-slate-200' : 'text-slate-700'}`}>{marketValue.toFixed(2)}</span>
              </div>
              <div className="flex justify-between">
                <span className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>盈亏</span>
                <span className={`font-mono ${profitLoss >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {profitLoss >= 0 ? '+' : ''}{profitLoss.toFixed(2)} ({profitLossPercent >= 0 ? '+' : ''}{profitLossPercent.toFixed(2)}%)
                </span>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex gap-2 p-4 border-t fin-divider">
          {position && position.shares > 0 && (
            <button
              onClick={handleClear}
              className="px-4 py-2 rounded-lg text-sm text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-50"
              disabled={saving}
            >
              清空持仓
            </button>
          )}
          <div className="flex-1" />
          <button
            onClick={onClose}
            className={`px-4 py-2 rounded-lg text-sm transition-colors disabled:opacity-50 ${colors.isDark ? 'text-slate-400 hover:bg-slate-700' : 'text-slate-500 hover:bg-slate-200'}`}
            disabled={saving}
          >
            取消
          </button>
          <button
            onClick={handleSave}
            className="px-4 py-2 rounded-lg text-sm bg-accent hover:bg-accent text-white transition-colors disabled:opacity-50"
            disabled={saving}
          >
            {saving ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
};
