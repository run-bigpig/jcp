export interface Stock {
  symbol: string;
  name: string;
  price: number;
  change: number;
  changePercent: number;
  volume: number;
  amount: number;
  marketCap: string;
  sector: string;
  open: number;
  high: number;
  low: number;
  preClose: number;
}

// 股票持仓信息
export interface StockPosition {
  shares: number;    // 持仓数量
  costPrice: number; // 成本价
}

export interface KLineData {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  amount?: number;
  avg?: number; // For intraday average price line
  // 均线数据
  ma5?: number;
  ma10?: number;
  ma20?: number;
}

export interface OrderBookItem {
  price: number;
  size: number;
  total: number;
  percent: number; // For visual bar depth
}

export interface OrderBook {
  bids: OrderBookItem[];
  asks: OrderBookItem[];
}

export enum AgentRole {
  BULL = '多头分析师',
  BEAR = '空头怀疑论者',
  QUANT = '技术量化专家',
  MACRO = '宏观经济学家',
  NEWS = '市场情报员'
}

export interface Agent {
  id: string;
  name: string;
  role: AgentRole;
  avatar: string;
  color: string;
}

export interface ChatMessage {
  id: string;
  agentId: string;
  agentName?: string;
  role?: string;
  content: string;
  timestamp: number;
  replyTo?: string;
  mentions?: string[];
  round?: number;        // 讨论轮次
  msgType?: MsgType;     // 消息类型
}

// 消息类型
export type MsgType = 'opening' | 'opinion' | 'summary';

export type TimePeriod = '1m' | '1d' | '1w' | '1mo';

// 快讯数据结构
export interface Telegraph {
  time: string;
  content: string;
  url: string;
}

// MCP 传输类型
export type MCPTransportType = 'http' | 'sse' | 'command';

// MCP 服务器配置
export interface MCPServerConfig {
  id: string;
  name: string;
  transportType: MCPTransportType;
  endpoint: string;
  command: string;
  args: string[];
  toolFilter: string[];
  enabled: boolean;
}

// 大盘指数数据
export interface MarketIndex {
  code: string;          // 指数代码
  name: string;          // 指数名称
  price: number;         // 当前点位
  change: number;        // 涨跌点数
  changePercent: number; // 涨跌幅(%)
  volume: number;        // 成交量(手)
  amount: number;        // 成交额(万元)
}

// 市场状态
export interface MarketStatus {
  status: string;        // trading, closed, pre_market, lunch_break
  statusText: string;    // 中文状态描述
  isTradeDay: boolean;   // 是否交易日
  holidayName: string;   // 节假日名称
}

export interface StockPeer {
  symbol: string;
  name: string;
  market?: string;
}

export interface FinancialStatements {
  income?: Record<string, any>[];
  balance?: Record<string, any>[];
  cashflow?: Record<string, any>[];
}

export interface PerformanceEvents {
  forecast?: Record<string, any>[];
  express?: Record<string, any>[];
  schedule?: Record<string, any>[];
}

export interface FundFlowSeries {
  fields?: string[];
  lines?: string[][];
  labels?: Record<string, string>;
  latest?: Record<string, any>;
}

export interface InstitutionalHoldings {
  topHolders?: Record<string, any>[];
  controller?: Record<string, any>;
}

export interface IndustryCompare {
  industry?: string;
  peers?: StockPeer[];
}

export interface BonusFinancing {
  dividend?: Record<string, any>[];
  annual?: Record<string, any>[];
  financing?: Record<string, any>[];
  allotment?: Record<string, any>[];
}

export interface BusinessAnalysis {
  scope?: Record<string, any>[];
  composition?: Record<string, any>[];
  review?: Record<string, any>[];
}

export interface ShareholderNumbers {
  records?: Record<string, any>[];
  latest?: Record<string, any>;
}

export interface EquityPledge {
  records?: Record<string, any>[];
  latest?: Record<string, any>;
}

export interface LockupRelease {
  records?: Record<string, any>[];
  latest?: Record<string, any>;
}

export interface ShareholderChanges {
  records?: Record<string, any>[];
  latest?: Record<string, any>;
}

export interface StockBuyback {
  records?: Record<string, any>[];
  latest?: Record<string, any>;
}

export interface F10OperationsRequired {
  latestIndicators?: Record<string, any>;
  latestIndicatorsExtra?: Record<string, any>;
  latestIndicatorsQuote?: Record<string, any>;
  eventReminders?: Record<string, any>[];
  news?: Record<string, any>[];
  announcements?: Record<string, any>[];
  shareholderAnalysis?: Record<string, any>[];
  dragonTigerList?: Record<string, any>[];
  blockTrades?: Record<string, any>[];
  marginTrading?: Record<string, any>[];
  mainIndicators?: Record<string, any>[];
  sectorTags?: Record<string, any>[];
  coreThemes?: Record<string, any>[];
  institutionForecast?: Record<string, any>[];
  forecastChart?: Record<string, any>[];
  reportSummary?: Record<string, any>[];
  researchReports?: Record<string, any>[];
  forecastRevisionTrack?: Record<string, any>[];
}

export interface F10Management {
  managementList?: Record<string, any>[];
  salaryDetails?: Record<string, any>[];
  holdingChanges?: Record<string, any>[];
}

export interface F10CapitalOperation {
  raiseSources?: Record<string, any>[];
  projectProgress?: Record<string, any>[];
}

export interface F10EquityStructure {
  latest?: Record<string, any>[];
  history?: Record<string, any>[];
  composition?: Record<string, any>[];
}

export interface F10RelatedStocks {
  industryRankings?: Record<string, any>[];
  conceptRelations?: Record<string, any>[];
}

export interface F10CoreThemes {
  boardTypes?: Record<string, any>[];
  themes?: Record<string, any>[];
  history?: Record<string, any>[];
  selectedBoardReasons?: Record<string, any>[];
  popularLeaders?: Record<string, any>[];
}

export interface F10IndustryCompareMetrics {
  valuation?: Record<string, any>[];
  performance?: Record<string, any>[];
  growth?: Record<string, any>[];
}

export interface F10MainIndicators {
  latest?: Record<string, any>[];
  yearly?: Record<string, any>[];
  quarterly?: Record<string, any>[];
}

export interface F10ValuationTrend {
  source?: string;
  range?: string;
  requestedRange?: string;
  fallback?: boolean;
  dateType?: number;
  labels?: Record<string, string>;
  pe?: Record<string, any>[];
  pb?: Record<string, any>[];
  ps?: Record<string, any>[];
  pcf?: Record<string, any>[];
}

export interface StockValuation {
  price?: number;
  peTtm?: number;
  pb?: number;
  totalMarketCap?: number;
  floatMarketCap?: number;
  turnoverRate?: number;
  amplitude?: number;
  totalShares?: number;
  floatShares?: number;
}

export interface F10Overview {
  code: string;
  updatedAt?: string;
  source?: string;
  company?: Record<string, any>;
  financials?: FinancialStatements;
  performance?: PerformanceEvents;
  fundFlow?: FundFlowSeries;
  institutions?: InstitutionalHoldings;
  industry?: IndustryCompare;
  bonus?: BonusFinancing;
  business?: BusinessAnalysis;
  shareholders?: ShareholderNumbers;
  pledge?: EquityPledge;
  lockup?: LockupRelease;
  holderChange?: ShareholderChanges;
  buyback?: StockBuyback;
  valuation?: StockValuation;
  operations?: F10OperationsRequired;
  coreThemes?: F10CoreThemes;
  industryMetrics?: F10IndustryCompareMetrics;
  mainIndicators?: F10MainIndicators;
  management?: F10Management;
  capitalOperation?: F10CapitalOperation;
  equityStructure?: F10EquityStructure;
  relatedStocks?: F10RelatedStocks;
  valuationTrend?: F10ValuationTrend;
  errors?: Record<string, string>;
}
