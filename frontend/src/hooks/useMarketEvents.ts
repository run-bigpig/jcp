import { useEffect, useCallback, useRef } from 'react';
import { EventsOn, EventsOff, EventsEmit } from '@wailsjs/runtime/runtime';
import { Stock, OrderBook, Telegraph, MarketIndex, MarketStatus, KLineData } from '../types';

// K线推送数据结构
interface KLineUpdateData {
  code: string;
  period: string;
  data: KLineData[];
}

// 事件名称常量，与后端保持一致
const EVENT_STOCK_UPDATE = 'market:stock:update';
const EVENT_ORDERBOOK_UPDATE = 'market:orderbook:update';
const EVENT_TELEGRAPH_UPDATE = 'market:telegraph:update';
const EVENT_MARKET_STATUS_UPDATE = 'market:status:update';
const EVENT_MARKET_INDICES_UPDATE = 'market:indices:update';
const EVENT_MARKET_SUBSCRIBE = 'market:subscribe';
const EVENT_ORDERBOOK_SUBSCRIBE = 'market:orderbook:subscribe';
const EVENT_KLINE_UPDATE = 'market:kline:update';
const EVENT_KLINE_SUBSCRIBE = 'market:kline:subscribe';

interface UseMarketEventsOptions {
  onStockUpdate?: (stocks: Stock[]) => void;
  onOrderBookUpdate?: (orderBook: OrderBook) => void;
  onTelegraphUpdate?: (telegraph: Telegraph) => void;
  onMarketStatusUpdate?: (status: MarketStatus) => void;
  onMarketIndicesUpdate?: (indices: MarketIndex[]) => void;
  onKLineUpdate?: (data: KLineUpdateData) => void;
}

/**
 * 市场数据事件 Hook
 * 监听后端推送的实时市场数据
 */
export function useMarketEvents(options: UseMarketEventsOptions) {
  const { onStockUpdate, onOrderBookUpdate, onTelegraphUpdate, onMarketStatusUpdate, onMarketIndicesUpdate, onKLineUpdate } = options;

  // 使用 ref 保存回调，避免重复注册
  const stockCallbackRef = useRef(onStockUpdate);
  const orderBookCallbackRef = useRef(onOrderBookUpdate);
  const telegraphCallbackRef = useRef(onTelegraphUpdate);
  const marketStatusCallbackRef = useRef(onMarketStatusUpdate);
  const marketIndicesCallbackRef = useRef(onMarketIndicesUpdate);
  const klineCallbackRef = useRef(onKLineUpdate);

  // 更新 ref
  useEffect(() => {
    stockCallbackRef.current = onStockUpdate;
    orderBookCallbackRef.current = onOrderBookUpdate;
    telegraphCallbackRef.current = onTelegraphUpdate;
    marketStatusCallbackRef.current = onMarketStatusUpdate;
    marketIndicesCallbackRef.current = onMarketIndicesUpdate;
    klineCallbackRef.current = onKLineUpdate;
  }, [onStockUpdate, onOrderBookUpdate, onTelegraphUpdate, onMarketStatusUpdate, onMarketIndicesUpdate, onKLineUpdate]);

  // 注册事件监听
  useEffect(() => {
    // 监听股票数据更新
    EventsOn(EVENT_STOCK_UPDATE, (stocks: Stock[]) => {
      stockCallbackRef.current?.(stocks);
    });

    // 监听盘口数据更新
    EventsOn(EVENT_ORDERBOOK_UPDATE, (orderBook: OrderBook) => {
      orderBookCallbackRef.current?.(orderBook);
    });

    // 监听快讯数据更新
    EventsOn(EVENT_TELEGRAPH_UPDATE, (telegraph: Telegraph) => {
      telegraphCallbackRef.current?.(telegraph);
    });

    // 监听市场状态更新
    EventsOn(EVENT_MARKET_STATUS_UPDATE, (status: MarketStatus) => {
      marketStatusCallbackRef.current?.(status);
    });

    // 监听大盘指数更新
    EventsOn(EVENT_MARKET_INDICES_UPDATE, (indices: MarketIndex[]) => {
      marketIndicesCallbackRef.current?.(indices);
    });

    // 监听K线数据更新
    EventsOn(EVENT_KLINE_UPDATE, (data: KLineUpdateData) => {
      klineCallbackRef.current?.(data);
    });

    // 清理函数
    return () => {
      EventsOff(EVENT_STOCK_UPDATE);
      EventsOff(EVENT_ORDERBOOK_UPDATE);
      EventsOff(EVENT_TELEGRAPH_UPDATE);
      EventsOff(EVENT_MARKET_STATUS_UPDATE);
      EventsOff(EVENT_MARKET_INDICES_UPDATE);
      EventsOff(EVENT_KLINE_UPDATE);
    };
  }, []);

  // 订阅股票
  const subscribe = useCallback((codes: string[]) => {
    EventsEmit(EVENT_MARKET_SUBSCRIBE, codes);
  }, []);

  // 订阅盘口（指定当前选中的股票）
  const subscribeOrderBook = useCallback((code: string) => {
    EventsEmit(EVENT_ORDERBOOK_SUBSCRIBE, code);
  }, []);

  // 订阅K线（指定股票代码和周期）
  const subscribeKLine = useCallback((code: string, period: string) => {
    EventsEmit(EVENT_KLINE_SUBSCRIBE, code, period);
  }, []);

  return { subscribe, subscribeOrderBook, subscribeKLine };
}
