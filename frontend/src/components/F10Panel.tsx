import React, { useEffect, useMemo, useState } from 'react';
import { RefreshCw, AlertTriangle, ChevronDown } from 'lucide-react';
import type {
  F10Overview,
  FinancialStatements,
  PerformanceEvents,
  FundFlowSeries,
  InstitutionalHoldings,
  IndustryCompare,
  BonusFinancing,
  BusinessAnalysis,
  ShareholderNumbers,
  EquityPledge,
  LockupRelease,
  ShareholderChanges,
  StockBuyback,
  StockValuation,
  F10OperationsRequired,
  F10CoreThemes,
  F10IndustryCompareMetrics,
  F10MainIndicators,
  F10ValuationTrend,
  F10Management,
  F10CapitalOperation,
  F10EquityStructure,
  F10RelatedStocks,
} from '../types';

type TabId =
  | 'overview'
  | 'operations'
  | 'news'
  | 'events'
  | 'themes'
  | 'forecast'
  | 'research'
  | 'industryMetrics'
  | 'financials'
  | 'fundflow'
  | 'bonus'
  | 'business'
  | 'shareholders'
  | 'equityStructure'
  | 'management'
  | 'capitalOperation'
  | 'relatedStocks'
  | 'valuation';

type FinancialSubTabId = 'statements' | 'mainIndicators' | 'percentStatements';
type FinancialStatementType = 'income' | 'balance' | 'cashflow';
type FinancialReportFilter = 'all' | 'year' | 'q3' | 'half' | 'q1';
type FinancialViewMode = 'report' | 'singleQuarter';

interface F10PanelProps {
  overview: F10Overview | null;
  loading?: boolean;
  error?: string | null;
  onRefresh?: () => void;
  onCollapse?: () => void;
}

const tabs: Array<{ id: TabId; label: string }> = [
  { id: 'overview', label: '公司概况' },
  { id: 'operations', label: '操盘必读' },
  { id: 'news', label: '资讯公告' },
  { id: 'events', label: '公司大事' },
  { id: 'themes', label: '核心题材' },
  { id: 'shareholders', label: '股东研究' },
  { id: 'business', label: '经营分析' },
  { id: 'industryMetrics', label: '同行比较' },
  { id: 'forecast', label: '盈利预测' },
  { id: 'research', label: '研究报告' },
  { id: 'financials', label: '财务分析' },
  { id: 'fundflow', label: '资金流向' },
  { id: 'bonus', label: '分红融资' },
  { id: 'equityStructure', label: '股本结构' },
  { id: 'management', label: '公司高管' },
  { id: 'capitalOperation', label: '资本运作' },
  { id: 'relatedStocks', label: '关联个股' },
  { id: 'valuation', label: '估值分析' },
];

const F10Panel: React.FC<F10PanelProps> = ({ overview, loading, error, onRefresh, onCollapse }) => {
  const [activeTab, setActiveTab] = useState<TabId>('overview');
  const [hideNoPlanDividends, setHideNoPlanDividends] = useState(false);
  const [financialSubTab, setFinancialSubTab] = useState<FinancialSubTabId>('statements');
  const [financialStatementType, setFinancialStatementType] = useState<FinancialStatementType>('income');
  const [financialReportFilter, setFinancialReportFilter] = useState<FinancialReportFilter>('all');
  const [financialViewMode, setFinancialViewMode] = useState<FinancialViewMode>('report');

  const overviewErrors = useMemo(() => {
    if (!overview?.errors) return [];
    return Object.values(overview.errors).filter(Boolean);
  }, [overview]);

  return (
    <div className="h-full flex flex-col text-left">
    <div className="flex items-center justify-between px-3 py-2 border-b fin-divider">
      <div>
        <div className="text-sm font-semibold text-slate-200">F10 全景数据</div>
        <div className="text-xs text-slate-500 flex flex-wrap gap-x-2">
          <span>{overview?.code ? `股票: ${overview.code}` : '暂无股票'}</span>
          {overview?.updatedAt && <span>更新: {extractDateOnly(overview.updatedAt) || overview.updatedAt}</span>}
          {overview?.source && <span>来源: {overview.source}</span>}
        </div>
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={onRefresh}
          className="flex items-center gap-1 text-xs text-slate-400 hover:text-accent-2 transition-colors"
          title="刷新F10数据"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          刷新
        </button>
        {onCollapse && (
          <button
            onClick={onCollapse}
            className="flex items-center gap-1 text-xs text-slate-400 hover:text-white transition-colors"
            title="收起F10"
          >
            <ChevronDown className="h-4 w-4" />
            收起
          </button>
        )}
      </div>
    </div>

      <div className="flex flex-wrap gap-2 px-3 py-2 border-b fin-divider text-xs">
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-2 py-1 rounded border transition-colors ${
              activeTab === tab.id
                ? 'border-accent text-accent-2 bg-accent/10'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto fin-scrollbar px-3 py-3 text-xs">
        {loading ? (
          <div className="text-slate-500">F10 数据加载中...</div>
        ) : (
          <>
            {(error || overviewErrors.length > 0) && (
              <div className="mb-3 rounded border border-orange-500/30 bg-orange-500/10 px-3 py-2 text-orange-200">
                <div className="flex items-center gap-2 font-medium">
                  <AlertTriangle className="h-4 w-4" />
                  数据获取提示
                </div>
                <ul className="mt-1 space-y-1 text-orange-200/80">
                  {error && <li>{error}</li>}
                  {overviewErrors.map((msg, idx) => (
                    <li key={idx}>{msg}</li>
                  ))}
                </ul>
              </div>
            )}

            {activeTab === 'overview' && renderOverview(overview)}
            {activeTab === 'operations' && renderOperations(overview?.operations)}
            {activeTab === 'news' && renderNewsAnnouncements(overview?.operations)}
            {activeTab === 'events' &&
              renderCompanyEvents(
                overview?.operations,
                overview?.pledge,
                overview?.lockup,
                overview?.holderChange,
                overview?.buyback,
              )}
            {activeTab === 'themes' && renderCoreThemes(overview?.coreThemes, overview?.operations, overview?.industry)}
            {activeTab === 'forecast' && renderForecastSection(overview?.operations)}
            {activeTab === 'research' && renderResearchSection(overview?.operations)}
            {activeTab === 'industryMetrics' && renderIndustryMetrics(overview?.industryMetrics)}
            {activeTab === 'financials' &&
              renderFinancialAnalysis(overview?.financials, overview?.mainIndicators, overview?.performance, {
                subTab: financialSubTab,
                onSubTabChange: setFinancialSubTab,
                statementType: financialStatementType,
                onStatementTypeChange: setFinancialStatementType,
                reportFilter: financialReportFilter,
                onReportFilterChange: setFinancialReportFilter,
                viewMode: financialViewMode,
                onViewModeChange: setFinancialViewMode,
              })}
            {activeTab === 'fundflow' && renderFundFlow(overview?.fundFlow)}
            {activeTab === 'bonus' && renderBonus(overview?.bonus, hideNoPlanDividends, setHideNoPlanDividends)}
            {activeTab === 'business' && renderBusiness(overview?.business)}
            {activeTab === 'shareholders' && renderShareholders(overview?.shareholders, overview?.institutions, overview?.holderChange)}
            {activeTab === 'equityStructure' && renderEquityStructure(overview?.equityStructure, overview?.lockup)}
            {activeTab === 'management' && renderManagement(overview?.management)}
            {activeTab === 'capitalOperation' && renderCapitalOperation(overview?.capitalOperation)}
            {activeTab === 'relatedStocks' && renderRelatedStocks(overview?.relatedStocks, overview?.industry)}
            {activeTab === 'valuation' && renderValuation(overview?.valuation, overview?.valuationTrend, overview?.updatedAt)}
          </>
        )}
      </div>
    </div>
  );
};

const renderOverview = (overview?: F10Overview | null) => {
  if (!overview?.company) {
    return <div className="text-slate-500">暂无公司概况数据</div>;
  }

  const highlights = buildCompanyHighlights(overview.company);
  const profile = pickCompanyText(overview.company, ['ORG_PROFILE', 'COMPANY_PROFILE', 'COMPANY_BRIEF', 'PROFILE']);
  const scope = pickCompanyText(overview.company, ['BUSINESS_SCOPE', 'MAIN_BUSINESS', 'BUSINESS']);

  if (highlights.length === 0 && !profile && !scope) {
    return <div className="text-slate-500">暂无公司概况数据</div>;
  }

  return (
    <div className="space-y-3">
      {highlights.length > 0 && (
        <div className="grid grid-cols-2 gap-2">
          {highlights.map(item => (
            <div key={item.label} className="rounded border border-slate-700/60 px-2 py-1.5 flex justify-between gap-2">
              <span className="text-slate-500 truncate">{item.label}</span>
              <span className="text-slate-200 text-right">{item.value}</span>
            </div>
          ))}
        </div>
      )}
      {profile && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-1">公司简介</div>
          <div className="text-slate-200 leading-relaxed whitespace-pre-wrap break-words">{profile}</div>
        </div>
      )}
      {scope && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-1">主营业务</div>
          <div className="text-slate-200 leading-relaxed whitespace-pre-wrap break-words">{scope}</div>
        </div>
      )}
    </div>
  );
};

const renderOperations = (operations?: F10OperationsRequired) => {
  if (!operations) {
    return <div className="text-slate-500">暂无操盘必读数据</div>;
  }

  const latestIndicators = mergeRecords(
    operations.latestIndicators,
    operations.latestIndicatorsExtra,
    operations.latestIndicatorsQuote,
  );

  return (
    <div className="space-y-3">
      {renderRecordGrid('最新指标', latestIndicators, [
        'REPORT_DATE',
        'REPORT_TYPE',
        'EPSJB',
        'EPSKCJB',
        'EPSXS',
        'BPS',
        'MGZBGJ',
        'MGZBGJJ',
        'MGWFPLR',
        'MGJYXJJE',
        'TOTAL_SHARE',
        'FREE_SHARE',
        'TOTAL_MARKET_CAP',
        'FLOAT_MARKET_CAP',
        'PE_DYNAMIC',
        'PE_TTM',
        'PE_STATIC',
        'PB',
        'PB_NEW_NOTICE',
        'PB_MRQ_REALTIME',
        'ROEJQ',
        'XSMLL',
        'ZCFZL',
      ], 24)}
      {renderEventList('大事提醒', operations.eventReminders, [
        'EVENT_TYPE',
        'SPECIFIC_EVENTTYPE',
        'LEVEL1_CONTENT',
        'LEVEL2_CONTENT',
        'EVENT_NAME',
        'EVENT_DESC',
        'CONTENT',
        'TITLE',
      ])}
      {renderSimpleTable('主要指标', operations.mainIndicators, [
        { label: '报告期', keys: ['REPORT_DATE_NAME', 'REPORT_DATE', 'REPORT_YEAR'] },
        { label: '每股收益', keys: ['EPSJB', 'EPSKCJB', 'EPSXS'] },
        { label: '营收同比', keys: ['TOTALOPERATEREVETZ', 'YYZSRGDHBZC'] },
        { label: '净利同比', keys: ['PARENTNETPROFITTZ', 'NETPROFITRPHBZC'] },
        { label: 'ROE(加权)', keys: ['ROEJQ'] },
      ], '暂无主要指标数据', 8)}
      {renderSimpleTable('股东分析', operations.shareholderAnalysis, [
        { label: '截止日', keys: ['END_DATE'] },
        { label: '股东户数', keys: ['HOLDER_TOTAL_NUM'] },
        { label: '户均持股', keys: ['AVG_FREE_SHARES'] },
        { label: '变动幅度', keys: ['TOTAL_NUM_RATIO'] },
      ], '暂无股东分析数据', 8)}
    </div>
  );
};

const renderNewsAnnouncements = (operations?: F10OperationsRequired) => {
  if (!operations || (!operations.news?.length && !operations.announcements?.length)) {
    return <div className="text-slate-500">暂无资讯公告数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderEventList('相关资讯', operations.news, ['NEWS_TITLE', 'TITLE', 'SUMMARY', 'summary', 'CONTENT', 'SOURCE'])}
      {renderEventList('相关公告', operations.announcements, ['ANNOUNCEMENT_TITLE', 'TITLE', 'CONTENT', 'NOTICE_DATE'])}
    </div>
  );
};

const renderForecastSection = (operations?: F10OperationsRequired) => {
  if (
    !operations ||
    (!operations.institutionForecast?.length && !operations.forecastChart?.length && !operations.forecastRevisionTrack?.length)
  ) {
    return <div className="text-slate-500">暂无盈利预测数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderInstitutionForecast(operations.institutionForecast)}
      {renderForecastChart(operations.forecastChart)}
      {renderForecastRevisionTrack(operations.forecastRevisionTrack)}
    </div>
  );
};

const renderResearchSection = (operations?: F10OperationsRequired) => {
  if (!operations || (!operations.reportSummary?.length && !operations.researchReports?.length)) {
    return <div className="text-slate-500">暂无研究报告数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderEventList('研报摘要', operations.reportSummary, [
        'REPORT_TITLE',
        'TITLE',
        'CONTENT',
        'ORG_NAME',
        'SOURCE',
      ])}
      {renderResearchReports(operations.researchReports)}
    </div>
  );
};

const renderCompanyEvents = (
  operations?: F10OperationsRequired,
  pledge?: EquityPledge,
  lockup?: LockupRelease,
  holderChange?: ShareholderChanges,
  buyback?: StockBuyback,
) => {
  const hasData = Boolean(
    operations?.eventReminders?.length ||
      operations?.dragonTigerList?.length ||
      operations?.blockTrades?.length ||
      operations?.marginTrading?.length ||
      pledge?.records?.length ||
      lockup?.records?.length ||
      holderChange?.records?.length ||
      buyback?.records?.length,
  );
  if (!hasData) {
    return <div className="text-slate-500">暂无公司大事数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderEventList('大事提醒', operations?.eventReminders, [
        'EVENT_TYPE',
        'SPECIFIC_EVENTTYPE',
        'LEVEL1_CONTENT',
        'LEVEL2_CONTENT',
      ])}
      {pledge?.records?.length ? renderPledge(pledge) : null}
      {lockup?.records?.length ? renderLockup(lockup) : null}
      {holderChange?.records?.length ? renderHolderChange(holderChange) : null}
      {buyback?.records?.length ? renderBuyback(buyback) : null}
      {renderSimpleTable('龙虎榜单', operations?.dragonTigerList, [
        { label: '交易日', keys: ['TRADE_DATE'] },
        { label: '上榜原因', keys: ['EXPLANATION'] },
        { label: '买入总额', keys: ['TOTAL_BUY'] },
        { label: '卖出总额', keys: ['TOTAL_SELL'] },
      ], '暂无龙虎榜单数据', 8)}
      {renderSimpleTable('大宗交易', operations?.blockTrades, [
        { label: '交易日', keys: ['TRADE_DATE'] },
        { label: '成交价', keys: ['DEAL_PRICE'] },
        { label: '成交量', keys: ['DEAL_VOLUME'] },
        { label: '成交额', keys: ['DEAL_AMT'] },
      ], '暂无大宗交易数据', 8)}
      {renderSimpleTable('融资融券', operations?.marginTrading, [
        { label: '交易日', keys: ['TRADE_DATE'] },
        { label: '融资余额', keys: ['FIN_BALANCE'] },
        { label: '融券余额', keys: ['LOAN_BALANCE'] },
        { label: '融资买入', keys: ['FIN_BUY_AMT'] },
      ], '暂无融资融券数据', 8)}
    </div>
  );
};

const renderFinancialAnalysis = (
  financials: FinancialStatements | undefined,
  indicators: F10MainIndicators | undefined,
  performance: PerformanceEvents | undefined,
  options: {
    subTab: FinancialSubTabId;
    onSubTabChange: (value: FinancialSubTabId) => void;
    statementType: FinancialStatementType;
    onStatementTypeChange: (value: FinancialStatementType) => void;
    reportFilter: FinancialReportFilter;
    onReportFilterChange: (value: FinancialReportFilter) => void;
    viewMode: FinancialViewMode;
    onViewModeChange: (value: FinancialViewMode) => void;
  },
) => {
  if (!financials && !indicators && !performance) {
    return <div className="text-slate-500">暂无财务分析数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderMainIndicators(indicators)}
      {renderFinancials(financials, indicators, options)}
      {renderPerformance(performance)}
    </div>
  );
};

const renderManagement = (management?: F10Management) => {
  if (!management || (!management.managementList?.length && !management.salaryDetails?.length && !management.holdingChanges?.length)) {
    return <div className="text-slate-500">暂无公司高管数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderSimpleTable('高管列表', management.managementList, [
        { label: '姓名', keys: ['PERSON_NAME'] },
        { label: '职务', keys: ['POSITION'] },
        { label: '任职时间', keys: ['INCUMBENT_TIME', 'INCUMBENT_DATE'] },
        { label: '持股数', keys: ['HOLD_NUM'] },
        { label: '薪酬', keys: ['SALARY'] },
      ], '暂无高管列表数据', 12)}
      {renderSimpleTable('高管薪酬', management.salaryDetails, [
        { label: '姓名', keys: ['PERSON_NAME'] },
        { label: '职务', keys: ['POSITION'] },
        { label: '报告期', keys: ['END_DATE'] },
        { label: '薪酬', keys: ['SALARY'] },
        { label: '行业均值', keys: ['AVG_SALARY'] },
      ], '暂无高管薪酬数据', 12)}
      {renderSimpleTable('高管持股变动', management.holdingChanges, [
        { label: '日期', keys: ['END_DATE'] },
        { label: '姓名', keys: ['EXECUTIVE_NAME', 'HOLDER_NAME'] },
        { label: '职务', keys: ['POSITION'] },
        { label: '变动股数', keys: ['CHANGE_NUM'] },
        { label: '成交均价', keys: ['AVERAGE_PRICE'] },
      ], '暂无高管持股变动数据', 12)}
    </div>
  );
};

const renderCapitalOperation = (capitalOperation?: F10CapitalOperation) => {
  if (!capitalOperation || (!capitalOperation.raiseSources?.length && !capitalOperation.projectProgress?.length)) {
    return <div className="text-slate-500">暂无资本运作数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderSimpleTable('募集资金来源', capitalOperation.raiseSources, [
        { label: '公告日', keys: ['NOTICE_DATE'] },
        { label: '发行类别', keys: ['FINANCE_TYPEE'] },
        { label: '募集净额', keys: ['NET_RAISE_FUNDS'] },
        { label: '发行起始日', keys: ['START_DATE'] },
      ], '暂无募集资金来源数据', 12)}
      {renderSimpleTable('项目进度', capitalOperation.projectProgress, [
        { label: '项目名称', keys: ['ITEM_NAME'] },
        { label: '公告日', keys: ['NOTICE_DATE'] },
        { label: '计划投入', keys: ['PLAN_INVEST_AMT'] },
        { label: '实际投入', keys: ['ACTUAL_INPUT_RF'] },
      ], '暂无项目进度数据', 12)}
    </div>
  );
};

const renderEquityStructure = (equityStructure?: F10EquityStructure, lockup?: LockupRelease) => {
  if (!equityStructure || (!equityStructure.latest?.length && !equityStructure.history?.length && !lockup?.records?.length)) {
    return <div className="text-slate-500">暂无股本结构数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderSimpleTable('股本结构', equityStructure.latest, [
        { label: '报告期', keys: ['END_DATE'] },
        { label: '总股本', keys: ['TOTAL_SHARES'] },
        { label: '流通股份', keys: ['UNLIMITED_SHARES'] },
        { label: '流通受限股份', keys: ['LIMITED_SHARES'] },
      ], '暂无股本结构数据', 1)}
      {renderSimpleTable('历年股本变动', equityStructure.history, [
        { label: '日期', keys: ['END_DATE'] },
        { label: '总股本', keys: ['TOTAL_SHARES'] },
        { label: '流通A股', keys: ['LISTED_A_SHARES'] },
        { label: '受限股份', keys: ['LIMITED_SHARES'] },
      ], '暂无历年股本变动数据', 12)}
      {renderSimpleTable('限售解禁', lockup?.records, [
        { label: '解禁日', keys: ['FREE_DATE', 'freeDate'] },
        { label: '解禁股数', keys: ['FREE_SHARES', 'freeShares'] },
        { label: '解禁比例', keys: ['FREE_RATIO', 'freeRatio'] },
        { label: '解禁市值', keys: ['LIFT_MARKET_CAP', 'liftMarketCap'] },
      ], '暂无限售解禁数据', 8)}
    </div>
  );
};

const renderRelatedStocks = (relatedStocks?: F10RelatedStocks, industry?: IndustryCompare) => {
  if (!relatedStocks || (!relatedStocks.industryRankings?.length && !relatedStocks.conceptRelations?.length && !industry?.peers?.length)) {
    return <div className="text-slate-500">暂无关联个股数据</div>;
  }

  const buildTopChanges = (rows: Record<string, any>[] | undefined, key: string, limit: number = 12) => {
    if (!rows || rows.length === 0) return [];
    return [...rows]
      .filter(row => findValue(row, [key]) !== undefined && findValue(row, [key]) !== null && findValue(row, [key]) !== '')
      .sort((a, b) => Number(findValue(b, [key]) || 0) - Number(findValue(a, [key]) || 0))
      .slice(0, limit);
  };

  const top3 = buildTopChanges(relatedStocks.industryRankings, 'Change3', 12);
  const top6 = buildTopChanges(relatedStocks.industryRankings, 'Change6', 12);
  const top12 = buildTopChanges(relatedStocks.industryRankings, 'Change12', 12);

  return (
    <div className="space-y-3">
      {renderSimpleTable('同行业个股排名', relatedStocks.industryRankings, [
        { label: '板块', keys: ['BOARD_NAME'] },
        { label: '公司', keys: ['SECURITY_NAME_ABBR'] },
        { label: '总市值', keys: ['TOTAL_CAP'] },
        { label: '营收同比', keys: ['TOTALOPERATEREVETZ'] },
        { label: '净利同比', keys: ['PARENTNETPROFITTZ'] },
      ], '暂无同行业个股排名数据', 12)}
      {(top3.length > 0 || top6.length > 0 || top12.length > 0) && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">3/6/12日涨幅最大</div>
          <div className="grid grid-cols-3 gap-3">
            {[
              { title: '3日涨幅最大', rows: top3, key: 'Change3' },
              { title: '6日涨幅最大', rows: top6, key: 'Change6' },
              { title: '12日涨幅最大', rows: top12, key: 'Change12' },
            ].map(group => (
              <div key={group.title} className="rounded border border-slate-700/40 px-2 py-1.5">
                <div className="text-slate-500 mb-1">{group.title}</div>
                <div className="space-y-1">
                  {group.rows.slice(0, 12).map((row, idx) => (
                    <div key={`${group.title}-${idx}`} className="grid grid-cols-[56px_1fr_56px] gap-2 text-xs">
                      <span className="text-slate-400">{formatValue(findValue(row, ['SECURITY_CODE']))}</span>
                      <span className="text-slate-200 truncate">{formatValue(findValue(row, ['SECURITY_NAME_ABBR']))}</span>
                      <span className="text-right text-slate-300">{formatPercentCell(findValue(row, [group.key]), true)}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
      {industry?.peers?.length ? (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">同业公司</div>
          <div className="flex flex-wrap gap-2">
            {industry.peers.slice(0, 16).map(peer => (
              <span key={peer.symbol} className="px-2 py-1 rounded fin-chip text-slate-300">
                {peer.name}
              </span>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
};

const renderInstitutionForecast = (records?: Record<string, any>[]) => {
  if (!records || records.length === 0) return null;
  const filtered = records.filter(item => Object.keys(item || {}).length > 0);
  if (filtered.length === 0) return null;

  const isAverage = (item: Record<string, any>) => {
    const org = String(findValue(item, ['ORG_NAME_ABBR', 'ORG_NAME']) || '');
    const code = String(findValue(item, ['ORG_CODE']) || '');
    return org.includes('平均') || code === '00000000';
  };

  const averageRows = filtered.filter(isAverage);
  const otherRows = filtered.filter(item => !isAverage(item));
  otherRows.sort((a, b) => {
    const da = String(findValue(a, ['PUBLISH_DATE', 'NOTICE_DATE', 'REPORT_DATE']) || '');
    const db = String(findValue(b, ['PUBLISH_DATE', 'NOTICE_DATE', 'REPORT_DATE']) || '');
    return db.localeCompare(da);
  });

  const rows = [...averageRows, ...otherRows].slice(0, 8);
  const sample = rows.find(row => findValue(row, ['YEAR1', 'YEAR2', 'YEAR3', 'YEAR4'])) || rows[0] || {};
  const yearLabels = [1, 2, 3, 4].map(idx => {
    const year = findValue(sample, [`YEAR${idx}`]);
    const mark = findValue(sample, [`YEAR_MARK${idx}`]);
    if (year) {
      return `${year}${mark ? String(mark) : ''}`;
    }
    return `年度${idx}`;
  });

  const renderCell = (row: Record<string, any>, idx: number) => {
    const eps = findValue(row, [`EPS${idx}`]);
    const pe = findValue(row, [`PE${idx}`]);
    const epsText = eps === undefined || eps === null || eps === '' ? '--' : formatValue(eps);
    const peText = pe === undefined || pe === null || pe === '' ? '--' : formatValue(pe);
    return (
      <div className="flex flex-col items-end gap-0.5 text-slate-200">
        <span>每股收益 {epsText}</span>
        <span className="text-slate-500">市盈率 {peText}</span>
      </div>
    );
  };

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">机构预测</div>
      <div className="grid grid-cols-[160px_repeat(4,minmax(120px,1fr))] gap-x-3 text-xs text-slate-500 pb-2 border-b border-slate-700/60">
        <div>机构名称 / 发布日</div>
        {yearLabels.map(label => (
          <div key={label} className="text-right">{label}</div>
        ))}
      </div>
      <div className="divide-y divide-slate-700/50 text-xs">
        {rows.map((row, idx) => {
          const org = formatValue(findValue(row, ['ORG_NAME_ABBR', 'ORG_NAME', 'ORG']) || '--');
          const date = formatValue(findValue(row, ['PUBLISH_DATE', 'NOTICE_DATE', 'REPORT_DATE']) || '');
          return (
            <div key={`${org}-${idx}`} className="grid grid-cols-[160px_repeat(4,minmax(120px,1fr))] gap-x-3 py-2">
              <div className="text-slate-200">
                <div className="truncate">{org}</div>
                {date && date !== '--' && <div className="text-slate-500">{date}</div>}
              </div>
              {renderCell(row, 1)}
              {renderCell(row, 2)}
              {renderCell(row, 3)}
              {renderCell(row, 4)}
            </div>
          );
        })}
      </div>
    </div>
  );
};

const renderForecastChart = (records?: Record<string, any>[]) => {
  if (!records || records.length === 0) return null;

  const rows = records
    .filter(item => Object.keys(item || {}).length > 0)
    .sort((a, b) => Number(findValue(a, ['RANK']) || 0) - Number(findValue(b, ['RANK']) || 0));

  if (rows.length === 0) return null;

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-1">盈利预测（年度）</div>
      <div className="text-[11px] text-slate-500 mb-2">说明: 这是机构一致预期的年度指标，不是走势图。重点看 EPS、PE 和净利润的年度变化。</div>
      <div className="overflow-x-auto">
        <table className="min-w-[980px] w-full text-xs">
          <thead>
            <tr className="text-slate-500 border-b border-slate-700/60">
              <th className="text-left py-1 pr-2">年度</th>
              <th className="text-right py-1 pr-2">EPS</th>
              <th className="text-right py-1 pr-2">PE</th>
              <th className="text-right py-1 pr-2">归母净利润</th>
              <th className="text-right py-1 pr-2">净利润同比</th>
              <th className="text-right py-1 pr-2">营业收入</th>
              <th className="text-right py-1 pr-2">营收同比</th>
              <th className="text-right py-1">ROE</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((item, idx) => {
              const year = findValue(item, ['YEAR']);
              const yearMark = findValue(item, ['YEAR_MARK']);
              const yearText = `${formatValue(year)}${yearMark ? String(yearMark) : ''}`;
              return (
                <tr key={`forecast-chart-${idx}`} className="border-b border-slate-800/70">
                  <td className="py-1 pr-2 text-slate-200">{yearText}</td>
                  <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(item, ['EPS']))}</td>
                  <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(item, ['PE']))}</td>
                  <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(item, ['PARENT_NETPROFIT']))}</td>
                  <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(item, ['PARENT_NETPROFIT_RATIO']), true)}</td>
                  <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(item, ['TOTAL_OPERATE_INCOME']))}</td>
                  <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(item, ['TOTAL_OPERATE_INCOME_RATIO']), true)}</td>
                  <td className="py-1 text-right text-slate-300">{formatPercentCell(findValue(item, ['ROE']))}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
};

const renderResearchReports = (records?: Record<string, any>[]) => {
  if (!records || records.length === 0) return null;

  const rows = records.filter(item => Object.keys(item || {}).length > 0);
  if (rows.length === 0) return null;

  const orgSet = new Set<string>();
  const ratingCounter = new Map<string, number>();
  const nowMs = Date.now();
  let recent30d = 0;

  rows.forEach(item => {
    const org = String(findValue(item, ['orgSName', 'ORG_S_NAME', 'ORG_NAME']) || '').trim();
    if (org) orgSet.add(org);

    const rating = String(findValue(item, ['emRatingName', 'EM_RATING_NAME', 'sRatingName', 'S_RATING_NAME']) || '').trim();
    if (rating) ratingCounter.set(rating, (ratingCounter.get(rating) || 0) + 1);

    const dateText = String(findValue(item, ['publishDate', 'PUBLISH_DATE', 'NOTICE_DATE']) || '').trim();
    const dateOnly = extractDateOnly(dateText);
    if (!dateOnly) return;
    const ts = new Date(dateOnly).getTime();
    if (!Number.isNaN(ts) && nowMs-ts <= 30 * 24 * 60 * 60 * 1000) {
      recent30d += 1;
    }
  });

  const ratingSummary = Array.from(ratingCounter.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 4)
    .map(([name, count]) => `${name}(${count})`)
    .join(' / ');

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">机构研报（公开）</div>
      <div className="grid grid-cols-3 gap-2 mb-2 text-slate-200">
        <div className="rounded border border-slate-700/60 px-2 py-1">
          <div className="text-slate-500">覆盖机构数</div>
          <div>{formatValue(orgSet.size)}</div>
        </div>
        <div className="rounded border border-slate-700/60 px-2 py-1">
          <div className="text-slate-500">近30天研报数</div>
          <div>{formatValue(recent30d)}</div>
        </div>
        <div className="rounded border border-slate-700/60 px-2 py-1">
          <div className="text-slate-500">样本总量</div>
          <div>{formatValue(rows.length)}</div>
        </div>
      </div>
      {ratingSummary && (
        <div className="text-slate-500 mb-2">评级分布: {ratingSummary}</div>
      )}
      <div className="space-y-2">
        {rows.slice(0, 8).map((item, idx) => {
          const title = formatValue(findValue(item, ['title', 'TITLE']) || '--');
          const org = formatValue(findValue(item, ['orgSName', 'ORG_S_NAME', 'ORG_NAME']) || '--');
          const date = formatValue(findValue(item, ['publishDate', 'PUBLISH_DATE']) || '--');
          const rating = formatValue(findValue(item, ['emRatingName', 'EM_RATING_NAME', 'sRatingName', 'S_RATING_NAME']) || '--');
          const epsThis = formatValue(findValue(item, ['predictThisYearEps', 'PREDICT_THIS_YEAR_EPS']) || '--');
          const epsNext = formatValue(findValue(item, ['predictNextYearEps', 'PREDICT_NEXT_YEAR_EPS']) || '--');

          return (
            <div key={`${title}-${idx}`} className="rounded border border-slate-700/40 px-2 py-1.5">
              <div className="text-slate-100 line-clamp-1">{title}</div>
              <div className="text-slate-500 mt-1">{org} · {date} · {rating}</div>
              <div className="text-slate-400 mt-1">EPS: 当年 {epsThis} / 次年 {epsNext}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

const renderForecastRevisionTrack = (records?: Record<string, any>[]) => {
  if (!records || records.length === 0) return null;
  const allRows = records.filter(item => Object.keys(item || {}).length > 0);
  const rows = allRows.slice(0, 16);
  if (rows.length === 0) return null;

  const parseNum = (value: any): number | null => {
    if (value === undefined || value === null || value === '') return null;
    const num = Number(String(value).replace(/,/g, '').trim());
    return Number.isNaN(num) ? null : num;
  };

  const findPrevByOrg = (org: string, start: number) => {
    for (let i = start + 1; i < rows.length; i += 1) {
      const candidateOrg = String(findValue(rows[i], ['orgName', 'ORG_NAME', 'ORG_NAME_ABBR', 'orgSName']) || '').trim();
      if (candidateOrg && candidateOrg === org) {
        return rows[i];
      }
    }
    return undefined;
  };

  const formatDelta = (current: any, previous: any) => {
    const c = parseNum(current);
    const p = parseNum(previous);
    if (c === null || p === null) return '--';
    const delta = c - p;
    const sign = delta > 0 ? '+' : '';
    return `${sign}${delta.toFixed(3)}`;
  };

  const hasForecastValue = (row: Record<string, any>) => {
    const epsThis = parseNum(findValue(row, ['epsThisYear', 'PREDICT_THIS_YEAR_EPS', 'EPS1', 'predictThisYearEps']));
    const epsNext = parseNum(findValue(row, ['epsNextYear', 'PREDICT_NEXT_YEAR_EPS', 'EPS2', 'predictNextYearEps', 'predictNextTwoYearEps']));
    const peThis = parseNum(findValue(row, ['peThisYear', 'PREDICT_THIS_YEAR_PE', 'PE1', 'predictThisYearPe']));
    const peNext = parseNum(findValue(row, ['peNextYear', 'PREDICT_NEXT_YEAR_PE', 'PE2', 'predictNextYearPe', 'predictNextTwoYearPe']));
    return epsThis !== null || epsNext !== null || peThis !== null || peNext !== null;
  };

  const valuedRows = rows.filter(hasForecastValue);
  const displayRows = valuedRows.length > 0 ? valuedRows : rows;
  const hiddenCount = rows.length - displayRows.length;

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">一致预期修正轨迹</div>
      {hiddenCount > 0 && (
        <div className="text-[11px] text-slate-500 mb-2">
          已隐藏 {hiddenCount} 条缺少预测字段的记录（上游未提供 EPS/PE）。
        </div>
      )}
      <div className="grid grid-cols-[96px_120px_70px_90px_90px_90px] gap-2 text-[11px] text-slate-500 pb-2 border-b border-slate-700/60">
        <span>日期</span>
        <span>机构</span>
        <span>来源</span>
        <span className="text-right">当年EPS</span>
        <span className="text-right">次年EPS</span>
        <span className="text-right">较上次修正</span>
      </div>
      <div className="divide-y divide-slate-700/50">
        {displayRows.map((row, idx) => {
          const date = formatValue(findValue(row, ['publishDate', 'PUBLISH_DATE', 'NOTICE_DATE']) || '--');
          const rawOrg = String(findValue(row, ['orgName', 'ORG_NAME', 'ORG_NAME_ABBR', 'orgSName', 'ORG_S_NAME']) || '').trim();
          const org = rawOrg ? formatValue(rawOrg) : '--';
          const source = formatValue(findValue(row, ['source', 'SOURCE']) || '--');
          const epsThis = findValue(row, ['epsThisYear', 'PREDICT_THIS_YEAR_EPS', 'EPS1']);
          const epsNext = findValue(row, ['epsNextYear', 'PREDICT_NEXT_YEAR_EPS', 'EPS2']);

          const prev = rawOrg ? findPrevByOrg(rawOrg, idx) : undefined;
          const prevEpsThis = prev ? findValue(prev, ['epsThisYear', 'PREDICT_THIS_YEAR_EPS', 'EPS1']) : null;
          const deltaText = formatDelta(epsThis, prevEpsThis);

          return (
            <div key={`${date}-${org}-${idx}`} className="grid grid-cols-[96px_120px_70px_90px_90px_90px] gap-2 py-1.5 text-xs">
              <span className="text-slate-300 truncate">{date}</span>
              <span className="text-slate-200 truncate">{org}</span>
              <span className="text-slate-500 truncate">{source}</span>
              <span className="text-right text-slate-200">{formatValue(epsThis)}</span>
              <span className="text-right text-slate-300">{formatValue(epsNext)}</span>
              <span className={`text-right ${deltaText.startsWith('+') ? 'text-red-300' : deltaText.startsWith('-') ? 'text-green-300' : 'text-slate-500'}`}>
                {deltaText}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
};

type ThemeTag = { name: string; change?: number };
type ThemeLeader = { name: string; change?: number };

const extractThemeTags = (
  items: Record<string, any>[] | undefined,
  nameKeys: string[],
  changeKeys: string[],
  tags: ThemeTag[],
) => {
  if (!items || items.length === 0) return;
  for (const item of items) {
    const name = findValue(item, nameKeys);
    if (!name) continue;
    if (tags.some(tag => tag.name === name)) continue;
    const changeRaw = findValue(item, changeKeys);
    const changeNum = changeRaw === undefined || changeRaw === null || changeRaw === ''
      ? undefined
      : Number(changeRaw);
    tags.push({ name, change: Number.isNaN(changeNum as number) ? undefined : changeNum });
  }
};

const buildThemeTags = (themes?: F10CoreThemes, operations?: F10OperationsRequired) => {
  const tags: ThemeTag[] = [];
  const boardTypes = themes?.boardTypes || [];

  if (boardTypes.length > 0) {
    const isPrecise = (value: any) => value === true || value === 1 || value === '1';
    const precise = boardTypes.filter(item => isPrecise(findValue(item, ['IS_PRECISE', 'IS_PRECISE_MATCH'])));
    const pick = precise.length > 0 ? precise : boardTypes;
    const sorted = [...pick].sort((a, b) => {
      const ra = Number(findValue(a, ['BOARD_RANK', 'RANK']));
      const rb = Number(findValue(b, ['BOARD_RANK', 'RANK']));
      if (Number.isNaN(ra) || Number.isNaN(rb)) return 0;
      return ra - rb;
    });
    extractThemeTags(
      sorted,
      ['BOARD_NAME', 'BK_NAME', 'BOARD', 'THEME_NAME', 'NAME'],
      ['BOARD_YIELD', 'PCT_CHANGE', 'CHANGE_PERCENT', 'RISE', 'ZDF', 'CHANGE_RATE', 'PERCENT', 'BOARD_ZDF'],
      tags,
    );
  }

  if (tags.length === 0) {
    const sectorTags = operations?.sectorTags || [];
    if (sectorTags.length > 0) {
      const isPrecise = (value: any) => value === true || value === 1 || value === '1';
      const precise = sectorTags.filter(item => isPrecise(findValue(item, ['IS_PRECISE', 'IS_PRECISE_MATCH'])));
      const pick = precise.length > 0 ? precise : sectorTags;
      const sorted = [...pick].sort((a, b) => {
        const ra = Number(findValue(a, ['BOARD_RANK', 'RANK']));
        const rb = Number(findValue(b, ['BOARD_RANK', 'RANK']));
        if (Number.isNaN(ra) || Number.isNaN(rb)) return 0;
        return ra - rb;
      });
      extractThemeTags(
        sorted,
        ['BOARD_NAME', 'BK_NAME', 'BOARD', 'THEME_NAME', 'NAME'],
        ['PCT_CHANGE', 'CHANGE_PERCENT', 'RISE', 'ZDF', 'CHANGE_RATE', 'PERCENT', 'BOARD_ZDF'],
        tags,
      );
    }
  }

  if (tags.length === 0) {
    extractThemeTags(
      themes?.themes,
      ['THEME_NAME', 'BOARD_NAME', 'BK_NAME', 'KEYWORD', 'NAME', 'CONCEPT_NAME'],
      ['PCT_CHANGE', 'CHANGE_PERCENT', 'RISE', 'ZDF', 'CHANGE_RATE', 'PERCENT', 'BOARD_ZDF'],
      tags,
    );
  }
  return tags;
};

const normalizeTextValue = (value: any): string => {
  if (value === undefined || value === null) return '';
  if (Array.isArray(value)) {
    return value.map(item => String(item)).join(' ');
  }
  if (typeof value === 'string') return value.replace(/\s+/g, ' ').trim();
  return String(value).replace(/\s+/g, ' ').trim();
};

const splitTextList = (value: string): string[] => {
  if (!value) return [];
  return value
    .split(/[，,、;；\n]/g)
    .map(item => item.trim())
    .filter(Boolean);
};

const pushUniqueText = (list: string[], value: string) => {
  if (!value) return;
  if (list.includes(value)) return;
  list.push(value);
};

const buildCoreThemeExtras = (themes?: F10CoreThemes, operations?: F10OperationsRequired) => {
  const reasons: string[] = [];
  const leaders: ThemeLeader[] = [];
  let reasonSources: Record<string, any>[] = [];
  if (themes?.selectedBoardReasons?.length) {
    reasonSources = themes.selectedBoardReasons;
  } else if (themes?.boardTypes?.length) {
    reasonSources = themes.boardTypes;
  } else if (operations?.coreThemes?.length) {
    reasonSources = operations.coreThemes;
  }

  let leaderSources: Record<string, any>[] = [];
  if (themes?.popularLeaders?.length) {
    leaderSources = themes.popularLeaders;
  } else if (operations?.coreThemes?.length) {
    leaderSources = operations.coreThemes;
  }

  if (reasonSources.length === 0 && leaderSources.length === 0) {
    return { reasons, leaders };
  }

  for (const item of reasonSources) {
    const reason = findValue(item, [
      'SELECTED_BOARD_REASON',
      'SELECT_REASON',
      'REASON',
      'REASON_DESC',
      'ENTRY_REASON',
      'CHOOSE_REASON',
      'REMARK',
      'CONTENT',
      'MAINPOINT',
      'POINT',
      'REASON_TEXT',
      'HIGHLIGHT',
      'DESCRIPTION',
    ]);
    const reasonText = normalizeTextValue(reason);
    if (reasonText) {
      const boardName = normalizeTextValue(
        findValue(item, ['BOARD_NAME', 'THEME_NAME', 'KEY_CLASSIF_NAME', 'NAME', 'CONCEPT_NAME']),
      );
      const reasonLine = boardName ? `${boardName}：${reasonText}` : reasonText;
      pushUniqueText(reasons, reasonLine);
    }
  }

  for (const item of leaderSources) {
    const leaderRaw = findValue(item, [
      'LEADER_NAME',
      'LEADER_STOCK',
      'LEADING_STOCK',
      'DRAGON_STOCK',
      'HOT_STOCK',
      'HOT_STOCKS',
      'POPULAR_STOCK',
      'POPULAR_STOCKS',
      'HEAD_STOCK',
      'LEADER',
    ]);
    const leaderText = normalizeTextValue(leaderRaw);
    const leaderNames = leaderText ? splitTextList(leaderText) : [];

    const fallbackName = findValue(item, [
      'STOCK_NAME',
      'SECURITY_NAME_ABBR',
      'SECURITY_NAME',
      'NAME',
    ]);
    if (leaderNames.length === 0 && fallbackName) {
      leaderNames.push(normalizeTextValue(fallbackName));
    }

    const changeRaw = findValue(item, [
      'CHANGE_PERCENT',
      'PCT_CHANGE',
      'CHANGE_RATE',
      'ZDF',
      'RISE',
      'BOARD_YIELD',
      'YIELD',
    ]);
    const changeNum = changeRaw === undefined || changeRaw === null || changeRaw === ''
      ? undefined
      : Number(changeRaw);
    const changeValue = Number.isNaN(changeNum as number) ? undefined : changeNum;

    leaderNames.forEach(name => {
      if (!name) return;
      if (leaders.some(leader => leader.name === name)) return;
      leaders.push({ name, change: changeValue });
    });
  }

  return { reasons, leaders };
};

const renderThemeExtras = (extras: ReturnType<typeof buildCoreThemeExtras>) => {
  const { reasons, leaders } = extras;
  if (reasons.length === 0 && leaders.length === 0) return null;
  return (
    <div className="rounded border border-slate-700/60 px-3 py-2 space-y-2">
      {reasons.length > 0 && (
        <div>
          <div className="text-slate-400 mb-1">入选理由</div>
          <ul className="space-y-1 text-slate-200">
            {reasons.slice(0, 2).map((reason, idx) => (
              <li key={idx} className="leading-relaxed">{reason}</li>
            ))}
          </ul>
        </div>
      )}
      {leaders.length > 0 && (
        <div>
          <div className="text-slate-400 mb-1">人气龙头</div>
          <div className="flex flex-wrap gap-2">
            {leaders.slice(0, 6).map(leader => {
              const changeText = leader.change !== undefined ? formatMetricValue(leader.change, 'signedPercent') : '';
              const changeColor =
                leader.change !== undefined && leader.change > 0
                  ? 'text-red-400'
                  : leader.change !== undefined && leader.change < 0
                    ? 'text-green-400'
                    : 'text-slate-300';
              return (
                <span
                  key={leader.name}
                  className="inline-flex items-center gap-1 rounded-full border border-slate-700/60 px-2 py-0.5 text-slate-200"
                >
                  <span>{leader.name}</span>
                  {changeText && <span className={changeColor}>{changeText}</span>}
                </span>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};

const renderThemeTags = (title: string, tags: ThemeTag[]) => {
  if (tags.length === 0) return null;
  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">{title}</div>
      <div className="flex flex-wrap gap-2">
        {tags.slice(0, 8).map(tag => {
          const change = tag.change;
          const changeText = change !== undefined ? formatMetricValue(change, 'signedPercent') : '';
          const changeColor =
            change !== undefined && change > 0
              ? 'text-red-400'
              : change !== undefined && change < 0
                ? 'text-green-400'
                : 'text-slate-300';
          return (
            <span
              key={tag.name}
              className="inline-flex items-center gap-1 rounded-full border border-slate-700/60 px-2 py-0.5 text-slate-200"
            >
              <span>{tag.name}</span>
              {changeText && <span className={changeColor}>{changeText}</span>}
            </span>
          );
        })}
      </div>
    </div>
  );
};

const renderCoreThemes = (themes?: F10CoreThemes, operations?: F10OperationsRequired, industry?: IndustryCompare) => {
  const hasThemes = Boolean(themes?.themes?.length || themes?.history?.length || themes?.boardTypes?.length);
  const tags = buildThemeTags(themes, operations);
  const extras = buildCoreThemeExtras(themes, operations);
  const hasIndustry = Boolean(industry?.industry || industry?.peers?.length);

  if (!hasThemes && tags.length === 0 && extras.reasons.length === 0 && extras.leaders.length === 0 && !hasIndustry) {
    return <div className="text-slate-500">暂无题材/行业数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderThemeTags('概念题材', tags)}
      {renderThemeExtras(extras)}
      {hasIndustry && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">行业信息</div>
          {industry?.industry && (
            <div className="mb-2">
              <div className="text-slate-500">所属行业</div>
              <div className="text-slate-200">{industry.industry}</div>
            </div>
          )}
          {industry?.peers?.length ? (
            <div>
              <div className="text-slate-500 mb-1">同业公司</div>
              <div className="flex flex-wrap gap-2">
                {industry.peers.slice(0, 12).map(peer => (
                  <span key={peer.symbol} className="px-2 py-1 rounded fin-chip text-slate-300">
                    {peer.name}
                  </span>
                ))}
              </div>
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
};

const renderMainIndicators = (indicators?: F10MainIndicators) => {
  const latest = indicators?.latest?.[0];
  if (!latest && !indicators?.yearly?.length && !indicators?.quarterly?.length) {
    return <div className="text-slate-500">暂无主要指标数据</div>;
  }

  const mainMetrics = [
    { label: '营业收入', keys: ['TOTAL_OPERATEINCOME', 'TOTAL_OPERATE_INCOME', 'OPERATE_INCOME', 'REVENUE'] },
    { label: '营收同比', keys: ['TOTALOPERATEREVETZ', 'TOTALOPERATEREVETZ_LAST', 'YYZSRGDHBZC', 'YYZSRGDHBZC_LAST', 'TOTAL_OPERATE_INCOME_YOY'] },
    { label: '归母净利', keys: ['PARENT_NETPROFIT', 'NETPROFIT', 'NET_PROFIT'] },
    { label: '净利同比', keys: ['PARENTNETPROFITTZ', 'PARENTNETPROFITTZ_LAST', 'NETPROFITRPHBZC', 'NETPROFITRPHBZC_LAST', 'NETPROFIT_YOY'] },
    { label: 'ROE(加权)', keys: ['ROEJQ', 'ROE', 'ROE_WEIGHTED', 'WEIGHTED_ROE'] },
    { label: '毛利率', keys: ['XSMLL', 'GROSS_PROFIT_RATIO', 'GROSS_MARGIN', 'GROSS_SALES_RATIO'] },
    { label: '资产负债率', keys: ['ZCFZL', 'DEBT_RATIO', 'ASSET_LIAB_RATIO'] },
    { label: '每股收益', keys: ['EPSJB', 'EPSKCJB', 'EPSXS', 'BASIC_EPS', 'EPS_BASIC', 'EPS'] },
  ];

  const renderIndicatorsTable = (title: string, rows: Record<string, any>[]) => {
    if (!rows || rows.length === 0) return null;
    const displayRows = rows;
    const excludeKeys = new Set([
      'SECUCODE',
      'SECURITY_CODE',
      'SECURITY_NAME_ABBR',
      'REPORT_DATE',
      'REPORTDATE',
      'END_DATE',
    ]);
    const keySet = new Set<string>();

    displayRows.forEach(row => {
      Object.entries(row || {}).forEach(([key, value]) => {
        if (excludeKeys.has(key)) return;
        if (!isPrimitive(value)) return;
        if (value === '' || value === null || value === undefined) return;
        keySet.add(key);
      });
    });

    const priorityKeys = [
      'BASIC_EPS',
      'EPSJB',
      'EPSKCJB',
      'EPSXS',
      'BPS',
      'MGZBGJ',
      'MGWFPLR',
      'MGJYXJJE',
      'TOTAL_OPERATEINCOME',
      'TOTAL_OPERATE_INCOME',
      'PARENT_NETPROFIT',
      'KCFJCXSYJLR',
      'ROEJQ',
      'ROE',
      'XSMLL',
      'ZCFZL',
      'TOTALOPERATEREVETZ',
      'PARENTNETPROFITTZ',
    ];

    const keys = Array.from(keySet);
    const orderedKeys = [
      ...priorityKeys.filter(key => keySet.has(key)),
      ...keys.filter(key => !priorityKeys.includes(key)).sort((a, b) => a.localeCompare(b)),
    ];
    const gridTemplateColumns = `180px repeat(${Math.max(displayRows.length, 1)}, minmax(96px, 1fr))`;

    return (
      <div className="rounded border border-slate-700/60 px-3 py-2">
        <div className="text-slate-400 mb-2">{title}（共{displayRows.length}期）</div>
        <div className="overflow-x-auto">
          <div className="min-w-max">
            <div className="grid gap-2 text-[11px] text-slate-500" style={{ gridTemplateColumns }}>
              <span>指标</span>
              {displayRows.map((row, idx) => (
                <span key={idx} className="text-right">
                  {formatValue(findValue(row, ['REPORT_DATE', 'REPORTDATE', 'END_DATE']))}
                </span>
              ))}
            </div>
            <div className="mt-2 space-y-1 max-h-80 overflow-auto pr-1">
              {orderedKeys.map(key => (
                <div key={key} className="grid gap-2" style={{ gridTemplateColumns }}>
                  <span className="text-slate-500">{mapKeyLabel(key)}</span>
                  {displayRows.map((row, idx) => (
                    <span key={idx} className="text-slate-200 text-right">
                      {formatValue(findValue(row, [key]))}
                    </span>
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-3">
      {renderLatestMetrics('核心指标', latest, mainMetrics)}
      {renderIndicatorsTable('主要指标（季度）', indicators?.quarterly || [])}
      {renderIndicatorsTable('主要指标（年度）', indicators?.yearly || [])}
    </div>
  );
};

const renderIndustryMetrics = (metrics?: F10IndustryCompareMetrics) => {
  if (!metrics || (!metrics.valuation?.length && !metrics.performance?.length && !metrics.growth?.length)) {
    return <div className="text-slate-500">暂无行业对比指标数据</div>;
  }

  const renderIndustryPeerTable = (title: string, rows: Record<string, any>[], columns: string[]) => {
    if (!rows || rows.length === 0) return null;
    const nameKeys = ['CORRE_SECURITY_NAME', 'SECURITY_NAME_ABBR', 'SECURITY_NAME', 'SECURITY_CODE', 'CORRE_SECURITY_CODE'];
    const getName = (row: Record<string, any>) => String(findValue(row, nameKeys) || '').trim();
    const isBenchmark = (row: Record<string, any>) => {
      const name = getName(row);
      return name === '行业中值' || name === '行业平均';
    };
    const benchmarkRows = rows.filter(isBenchmark);
    const median = benchmarkRows.find(row => getName(row) === '行业中值');
    const average = benchmarkRows.find(row => getName(row) === '行业平均');
    const peers = rows.filter(row => !isBenchmark(row));

    peers.sort((a, b) => {
      const ra = Number(findValue(a, ['PAIMING', 'RANK']));
      const rb = Number(findValue(b, ['PAIMING', 'RANK']));
      if (!Number.isNaN(ra) && !Number.isNaN(rb)) {
        return ra - rb;
      }
      return getName(a).localeCompare(getName(b));
    });

    const displayRows = [
      ...(median ? [median] : []),
      ...(average ? [average] : []),
      ...peers.slice(0, 10),
    ];

    const reportDate = formatValue(findValue(displayRows[0] || {}, ['REPORT_DATE', 'REPORTDATE', 'END_DATE']));
    const columnLabels = columns.map(key => mapKeyLabel(key));
    const gridStyle = {
      gridTemplateColumns: `160px repeat(${columns.length}, minmax(0, 1fr))`,
    } as React.CSSProperties;

    return (
      <div className="rounded border border-slate-700/60 px-3 py-2">
        <div className="text-slate-400">{title}</div>
        {reportDate && reportDate !== '--' && (
          <div className="text-[11px] text-slate-500 mb-2">报告期: {reportDate}</div>
        )}
        <div className="grid gap-2 text-[11px] text-slate-500" style={gridStyle}>
          <span>公司/类型</span>
          {columnLabels.map(label => (
            <span key={label} className="text-right">{label}</span>
          ))}
        </div>
        <div className="mt-2 divide-y divide-slate-700/50 text-xs">
          {displayRows.map((row, idx) => {
            const name = getName(row) || '--';
            const benchmark = isBenchmark(row);
            return (
              <div key={`${name}-${idx}`} className="grid gap-2 py-2" style={gridStyle}>
                <div className="flex items-center gap-2">
                  <span className={benchmark ? 'text-amber-300' : 'text-slate-200'}>{name}</span>
                  {benchmark && (
                    <span className="rounded-full border border-amber-500/40 px-2 py-0.5 text-[10px] text-amber-300">
                      基准
                    </span>
                  )}
                </div>
                {columns.map(key => (
                  <span key={key} className="text-right text-slate-200">
                    {formatValue(findValue(row, [key]))}
                  </span>
                ))}
              </div>
            );
          })}
        </div>
      </div>
    );
  };

  const valuationColumns = ['PE', 'PE_TTM', 'PB', 'PS_TTM', 'PCF_TTM', 'PEG', 'PAIMING'];
  const performanceColumns = ['ROE_AVG', 'XSJLL_AVG', 'TOAZZL_AVG', 'QYCS_AVG', 'PAIMING'];
  const growthColumns = ['YYSRTB', 'MGSYTB', 'JLRTB', 'YYSR_3Y', 'JLR_3Y', 'PAIMING'];

  return (
    <div className="space-y-3">
      {renderIndustryPeerTable('估值对比', metrics.valuation || [], valuationColumns)}
      {renderIndustryPeerTable('盈利能力', metrics.performance || [], performanceColumns)}
      {renderIndustryPeerTable('成长能力', metrics.growth || [], growthColumns)}
    </div>
  );
};

const renderValuationTrend = (trend?: F10ValuationTrend, showIntro: boolean = true) => {
  if (!trend || (!trend.pe?.length && !trend.pb?.length && !trend.ps?.length && !trend.pcf?.length)) {
    return <div className="text-slate-500">暂无估值走势数据</div>;
  }

  const toNumeric = (value: any) => {
    if (value === undefined || value === null || value === '') return NaN;
    if (typeof value === 'number') return value;
    if (typeof value === 'string') {
      const cleaned = value.replace(/,/g, '').replace(/%/g, '').trim();
      const num = Number(cleaned);
      return Number.isNaN(num) ? NaN : num;
    }
    return NaN;
  };

  const buildSeriesPoints = (series?: Record<string, any>[], limit: number = 240) => {
    if (!series || series.length === 0) return [];
    const points = series.map(item => {
      const date = formatValue(findValue(item, ['TRADE_DATE', 'REPORT_DATE', 'DATE']));
      const rawValue = findValue(item, ['INDICATOR_VALUE', 'VALUE', 'VAL', 'AVG', 'AVERAGE']);
      const value = toNumeric(rawValue);
      return { date, value };
    });
    const filtered = points.filter(point => point.date && point.date !== '--' && !Number.isNaN(point.value));
    return filtered.slice(-limit);
  };

  const seriesBlocks = [
    { key: 'pe', label: trend.labels?.pe || '市盈率', data: trend.pe },
    { key: 'pb', label: trend.labels?.pb || '市净率', data: trend.pb },
    { key: 'ps', label: trend.labels?.ps || '市销率', data: trend.ps },
    { key: 'pcf', label: trend.labels?.pcf || '市现率', data: trend.pcf },
  ];

  const MiniLineChart: React.FC<{
    points: Array<{ date: string; value: number }>;
    label: string;
  }> = ({ points, label }) => {
    if (!points || points.length < 2) return null;
    const width = 1000;
    const height = 220;
    const padding = { left: 48, right: 16, top: 16, bottom: 32 };
    const values = points.map(p => p.value);
    let min = Math.min(...values);
    let max = Math.max(...values);
    if (min === max) {
      min -= 1;
      max += 1;
    }
    const range = max - min;
    min -= range * 0.06;
    max += range * 0.06;
    const innerW = width - padding.left - padding.right;
    const innerH = height - padding.top - padding.bottom;

    const scaleX = (idx: number) => padding.left + (innerW * idx) / (points.length - 1);
    const scaleY = (value: number) => padding.top + (innerH * (max - value)) / (max - min);

    const path = points
      .map((p, idx) => `${idx === 0 ? 'M' : 'L'} ${scaleX(idx).toFixed(2)} ${scaleY(p.value).toFixed(2)}`)
      .join(' ');

    const ticks = 4;
    const yTicks = Array.from({ length: ticks }).map((_, idx) => {
      const value = max - (range * idx) / (ticks - 1);
      const y = scaleY(value);
      return { value, y };
    });

    const xTicks = [0, Math.floor((points.length - 1) / 3), Math.floor((points.length - 1) * 2 / 3), points.length - 1]
      .filter((v, idx, arr) => arr.indexOf(v) === idx);

    const mean = values.reduce((sum, val) => sum + val, 0) / values.length;
    const meanY = scaleY(mean);

    const [hoverIndex, setHoverIndex] = React.useState<number | null>(null);

    const handleMove = (event: React.MouseEvent<SVGSVGElement>) => {
      const rect = event.currentTarget.getBoundingClientRect();
      if (rect.width <= 0) return;
      // 将鼠标像素坐标映射到 viewBox 坐标系，避免缩放后光标与竖线错位
      const xInViewBox = ((event.clientX - rect.left) / rect.width) * width;
      const clamped = Math.min(Math.max(xInViewBox - padding.left, 0), innerW);
      const ratio = innerW <= 0 ? 0 : clamped / innerW;
      const idx = Math.round(ratio * (points.length - 1));
      setHoverIndex(idx);
    };

    const handleLeave = () => setHoverIndex(null);

    const latestIndex = points.length - 1;
    const latestPoint = points[latestIndex];
    const hoverPoint = hoverIndex !== null ? points[hoverIndex] : null;
    const hoverX = hoverIndex !== null ? scaleX(hoverIndex) : null;
    const hoverY = hoverPoint ? scaleY(hoverPoint.value) : null;

    return (
      <div className="relative w-full h-44">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          preserveAspectRatio="none"
          className="w-full h-full"
          onMouseMove={handleMove}
          onMouseLeave={handleLeave}
        >
          {yTicks.map((tick, idx) => (
            <g key={idx}>
              <line
                x1={padding.left}
                x2={width - padding.right}
                y1={tick.y}
                y2={tick.y}
                stroke="rgba(148,163,184,0.12)"
                strokeWidth="1"
              />
              <text x={padding.left - 6} y={tick.y + 3} textAnchor="end" fontSize="10" fill="rgba(148,163,184,0.8)">
                {tick.value.toFixed(2)}
              </text>
            </g>
          ))}
          {xTicks.map((idx, tickIdx) => (
            <text
              key={tickIdx}
              x={scaleX(idx)}
              y={height - 10}
              textAnchor="middle"
              fontSize="10"
              fill="rgba(148,163,184,0.8)"
            >
              {points[idx].date}
            </text>
          ))}
          <line
            x1={padding.left}
            x2={width - padding.right}
            y1={meanY}
            y2={meanY}
            stroke="rgba(249,115,22,0.6)"
            strokeDasharray="4 4"
            strokeWidth="1"
          />
          <text
            x={width - padding.right}
            y={meanY - 6}
            textAnchor="end"
            fontSize="10"
            fill="rgba(249,115,22,0.8)"
          >
            均值 {mean.toFixed(2)}
          </text>
          <path d={path} fill="none" stroke="rgba(56,189,248,0.9)" strokeWidth="2" />
          <circle
            cx={scaleX(latestIndex)}
            cy={scaleY(latestPoint.value)}
            r="3.5"
            fill="rgba(56,189,248,1)"
          />
          {hoverPoint && hoverX !== null && hoverY !== null && (
            <>
              <line
                x1={hoverX}
                x2={hoverX}
                y1={padding.top}
                y2={height - padding.bottom}
                stroke="rgba(148,163,184,0.4)"
                strokeDasharray="4 4"
              />
              <circle cx={hoverX} cy={hoverY} r="4" fill="rgba(14,165,233,1)" />
            </>
          )}
        </svg>
        {hoverPoint && hoverX !== null && hoverY !== null && (
          <div
            className="pointer-events-none absolute rounded border border-slate-700/60 bg-slate-900/90 px-2 py-1 text-[11px] text-slate-200 shadow"
            style={{
              left: `${(hoverX / width) * 100}%`,
              top: `${(hoverY / height) * 100}%`,
              transform: 'translate(8px, -50%)',
              maxWidth: '160px',
            }}
          >
            <div className="text-slate-400">{label}</div>
            <div>{hoverPoint.date}</div>
            <div>值: {hoverPoint.value.toFixed(2)}</div>
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="space-y-3">
      {showIntro && (
        <div className="text-[11px] text-slate-500">
          当前估值数值请查看“估值”标签；本页聚焦趋势变化。
        </div>
      )}
      {seriesBlocks.map(block => {
        const seriesPoints = buildSeriesPoints(block.data, 240);
        if (seriesPoints.length === 0) return null;
        return (
          <div key={block.key} className="rounded border border-slate-700/60 px-3 py-2">
            <div className="text-slate-400 mb-2">{block.label}</div>
            <MiniLineChart points={seriesPoints} label={block.label} />
          </div>
        );
      })}
    </div>
  );
};

const pickLatestRecord = (records: Record<string, any>[]) => {
  if (!records || records.length === 0) return undefined;
  if (records.length === 1) return records[0];
  const sorted = [...records].sort((a, b) => {
    const da = String(findValue(a, ['REPORT_DATE', 'END_DATE', 'NOTICE_DATE']) || '');
    const db = String(findValue(b, ['REPORT_DATE', 'END_DATE', 'NOTICE_DATE']) || '');
    return db.localeCompare(da);
  });
  return sorted[0];
};

const findNumericValue = (record: Record<string, any> | undefined, keys: string[]) => {
  const value = findValue(record, keys);
  if (value === undefined || value === null || value === '') return undefined;
  const normalized = Number(String(value).replace(/,/g, '').trim());
  return Number.isNaN(normalized) ? undefined : normalized;
};

const renderCashflowDerivedMetrics = (financials?: FinancialStatements, mainIndicators?: F10MainIndicators) => {
  const latestCashflow = pickLatestRecord(financials?.cashflow || []);
  if (!latestCashflow) return null;

  const latestMain = mainIndicators?.latest?.[0];
  const operateCash = findNumericValue(latestCashflow, ['NETCASH_OPERATE', 'NET_CASH_OPERATE', 'NET_OPERATE_CASH', 'NCO_OP']);
  const investCash = findNumericValue(latestCashflow, ['NETCASH_INVEST', 'NET_CASH_INVEST', 'NET_INVEST_CASH']);
  const financeCash = findNumericValue(latestCashflow, ['NETCASH_FINANCE', 'NET_CASH_FINANCE', 'NET_FINANCE_CASH']);

  let capex = findNumericValue(latestCashflow, [
    'CAPEX',
    'CASH_PAY_ACQ_CONST_FIOLTA',
    'PAY_ACQ_FIXED_INTANGIBLE',
    'INVEST_PAY_FIXED_ASSETS',
    'CASHPAID_PURCHCONSTFIXEDASS',
  ]);
  if (capex === undefined && investCash !== undefined && investCash < 0) {
    capex = Math.abs(investCash);
  }

  const netProfit = findNumericValue(latestMain, ['PARENT_NETPROFIT', 'NET_PROFIT', 'NETPROFIT']);
  const revenue = findNumericValue(latestMain, ['TOTAL_OPERATEINCOME', 'TOTAL_OPERATE_INCOME', 'OPERATE_INCOME', 'REVENUE']);

  const freeCashflow = operateCash !== undefined && capex !== undefined ? operateCash - capex : undefined;
  const cashToProfit = operateCash !== undefined && netProfit !== undefined && Math.abs(netProfit) > 0
    ? (operateCash / netProfit) * 100
    : undefined;
  const cashToRevenue = operateCash !== undefined && revenue !== undefined && Math.abs(revenue) > 0
    ? (operateCash / revenue) * 100
    : undefined;

  const rows: Array<{ label: string; value: any; format?: MetricFormat }> = [
    { label: '经营现金流净额', value: operateCash, format: 'signed' },
    { label: '投资现金流净额', value: investCash, format: 'signed' },
    { label: '筹资现金流净额', value: financeCash, format: 'signed' },
    { label: '资本开支(估算)', value: capex, format: 'signed' },
    { label: '自由现金流(估算)', value: freeCashflow, format: 'signed' },
    { label: '现金净利比', value: cashToProfit, format: 'signedPercent' },
    { label: '经营现金流/营收', value: cashToRevenue, format: 'signedPercent' },
  ];

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">现金流质量（衍生）</div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1">
        {rows.map(item => (
          <div key={item.label} className="flex items-center justify-between gap-2">
            <span className="text-slate-500">{item.label}</span>
            <span className="text-slate-200">
              {item.value === undefined ? '--' : formatMetricValue(item.value, item.format)}
            </span>
          </div>
        ))}
      </div>
      <div className="text-[11px] text-slate-600 mt-2">
        说明: 资本开支为公开字段估算值，缺失时以投资现金流净额近似。
      </div>
    </div>
  );
};

type FinancialRenderControls = {
  subTab: FinancialSubTabId;
  onSubTabChange: (tab: FinancialSubTabId) => void;
  statementType: FinancialStatementType;
  onStatementTypeChange: (next: FinancialStatementType) => void;
  reportFilter: FinancialReportFilter;
  onReportFilterChange: (next: FinancialReportFilter) => void;
  viewMode: FinancialViewMode;
  onViewModeChange: (next: FinancialViewMode) => void;
};

const financialMetaKeys = new Set([
  'SECUCODE',
  'SECURITY_CODE',
  'SECURITY_NAME_ABBR',
  'SECURITY_NAME',
  'SECURITY_INNER_CODE',
  'ORG_CODE',
  'SECURITY_TYPE_CODE',
  'SECURITY_TYPE',
  'SECURITY_TYPE_WEB',
  'REPORT_DATE',
  'REPORTDATE',
  'END_DATE',
  'REPORT_TYPE',
  'REPORT_TYPE_NAME',
  'REPORT_YEAR',
  'REPORT_DATE_NAME',
  'NOTICE_DATE',
  'UPDATE_DATE',
  'DATATYPE',
  'QDATE',
  'CURRENCY',
  'ISNEW',
  'ISLATEST',
  'SOURCE',
]);

const reportFilterLabels: Record<FinancialReportFilter, string> = {
  all: '全部',
  year: '年报',
  q3: '三季报',
  half: '中报',
  q1: '一季报',
};

type StatementFieldDef = {
  key: string;
  label: string;
};

const statementFieldSchemas: Record<FinancialStatementType, StatementFieldDef[]> = {
  income: [
    { key: 'TOTAL_OPERATE_INCOME', label: '营业总收入' },
    { key: 'TOTAL_OPERATE_COST', label: '营业总成本' },
    { key: 'OPERATE_COST', label: '营业成本' },
    { key: 'OPERATE_TAX_ADD', label: '税金及附加' },
    { key: 'SALE_EXPENSE', label: '销售费用' },
    { key: 'MANAGE_EXPENSE', label: '管理费用' },
    { key: 'RESEARCH_EXPENSE', label: '研发费用' },
    { key: 'FINANCE_EXPENSE', label: '财务费用' },
    { key: 'OTHER_INCOME', label: '其他收益' },
    { key: 'INVEST_INCOME', label: '投资收益' },
    { key: 'FAIRVALUE_CHANGE_INCOME', label: '公允价值变动收益' },
    { key: 'CREDIT_IMPAIRMENT_INCOME', label: '信用减值损益' },
    { key: 'ASSET_DISPOSAL_INCOME', label: '资产处置收益' },
    { key: 'OPERATE_PROFIT', label: '营业利润' },
    { key: 'NONBUSINESS_INCOME', label: '营业外收入' },
    { key: 'NONBUSINESS_EXPENSE', label: '营业外支出' },
    { key: 'TOTAL_PROFIT', label: '利润总额' },
    { key: 'INCOME_TAX', label: '所得税费用' },
    { key: 'NETPROFIT', label: '净利润' },
    { key: 'PARENT_NETPROFIT', label: '归母净利润' },
    { key: 'DEDUCT_PARENT_NETPROFIT', label: '扣非归母净利润' },
    { key: 'BASIC_EPS', label: '基本每股收益' },
    { key: 'DILUTED_EPS', label: '稀释每股收益' },
    { key: 'OPINION_TYPE', label: '审计意见(境内)' },
  ],
  balance: [
    { key: 'MONETARYFUNDS', label: '货币资金' },
    { key: 'TRADE_FINASSET_NOTFVTPL', label: '交易性金融资产' },
    { key: 'NOTE_ACCOUNTS_RECE', label: '应收票据及应收账款' },
    { key: 'NOTE_RECE', label: '应收票据' },
    { key: 'ACCOUNTS_RECE', label: '应收账款' },
    { key: 'FINANCE_RECE', label: '应收款项融资' },
    { key: 'PREPAYMENT', label: '预付款项' },
    { key: 'TOTAL_OTHER_RECE', label: '其他应收款合计' },
    { key: 'INVENTORY', label: '存货' },
    { key: 'CONTRACT_ASSET', label: '合同资产' },
    { key: 'OTHER_CURRENT_ASSET', label: '其他流动资产' },
    { key: 'TOTAL_CURRENT_ASSETS', label: '流动资产合计' },
    { key: 'INVEST_REALESTATE', label: '投资性房地产' },
    { key: 'FIXED_ASSET', label: '固定资产' },
    { key: 'CIP', label: '在建工程' },
    { key: 'USERIGHT_ASSET', label: '使用权资产' },
    { key: 'INTANGIBLE_ASSET', label: '无形资产' },
    { key: 'GOODWILL', label: '商誉' },
    { key: 'LONG_PREPAID_EXPENSE', label: '长期待摊费用' },
    { key: 'DEFER_TAX_ASSET', label: '递延所得税资产' },
    { key: 'OTHER_NONCURRENT_ASSET', label: '其他非流动资产' },
    { key: 'TOTAL_NONCURRENT_ASSETS', label: '非流动资产合计' },
    { key: 'TOTAL_ASSETS', label: '资产总计' },
    { key: 'NOTE_ACCOUNTS_PAYABLE', label: '应付票据及应付账款' },
    { key: 'NOTE_PAYABLE', label: '应付票据' },
    { key: 'ACCOUNTS_PAYABLE', label: '应付账款' },
    { key: 'ADVANCE_RECEIVABLES', label: '预收款项' },
    { key: 'CONTRACT_LIAB', label: '合同负债' },
    { key: 'STAFF_SALARY_PAYABLE', label: '应付职工薪酬' },
    { key: 'TAX_PAYABLE', label: '应交税费' },
    { key: 'TOTAL_OTHER_PAYABLE', label: '其他应付款合计' },
    { key: 'NONCURRENT_LIAB_1YEAR', label: '一年内到期的非流动负债' },
    { key: 'OTHER_CURRENT_LIAB', label: '其他流动负债' },
    { key: 'TOTAL_CURRENT_LIAB', label: '流动负债合计' },
    { key: 'LEASE_LIAB', label: '租赁负债' },
    { key: 'LONG_PAYABLE', label: '长期应付款' },
    { key: 'DEFER_INCOME', label: '递延收益' },
    { key: 'DEFER_TAX_LIAB', label: '递延所得税负债' },
    { key: 'TOTAL_NONCURRENT_LIAB', label: '非流动负债合计' },
    { key: 'TOTAL_LIABILITIES', label: '负债合计' },
    { key: 'SHARE_CAPITAL', label: '实收资本(或股本)' },
    { key: 'CAPITAL_RESERVE', label: '资本公积' },
    { key: 'OTHER_COMPRE_INCOME', label: '其他综合收益' },
    { key: 'SURPLUS_RESERVE', label: '盈余公积' },
    { key: 'UNASSIGN_RPOFIT', label: '未分配利润' },
    { key: 'TOTAL_PARENT_EQUITY', label: '归属于母公司股东权益总计' },
    { key: 'TOTAL_EQUITY', label: '股东权益合计' },
    { key: 'TOTAL_LIAB_EQUITY', label: '负债和股东权益总计' },
    { key: 'OPINION_TYPE', label: '审计意见(境内)' },
  ],
  cashflow: [
    { key: 'SALES_SERVICES', label: '销售商品、提供劳务收到的现金' },
    { key: 'RECEIVE_TAX_REFUND', label: '收到的税费返还' },
    { key: 'RECEIVE_OTHER_OPERATE', label: '收到其他与经营活动有关的现金' },
    { key: 'TOTAL_OPERATE_INFLOW', label: '经营活动现金流入小计' },
    { key: 'BUY_SERVICES', label: '购买商品、接受劳务支付的现金' },
    { key: 'PAY_STAFF_CASH', label: '支付给职工以及为职工支付的现金' },
    { key: 'PAY_ALL_TAX', label: '支付的各项税费' },
    { key: 'PAY_OTHER_OPERATE', label: '支付其他与经营活动有关的现金' },
    { key: 'TOTAL_OPERATE_OUTFLOW', label: '经营活动现金流出小计' },
    { key: 'NETCASH_OPERATE', label: '经营活动产生的现金流量净额' },
    { key: 'WITHDRAW_INVEST', label: '收回投资收到的现金' },
    { key: 'RECEIVE_INVEST_INCOME', label: '取得投资收益收到的现金' },
    { key: 'DISPOSAL_LONG_ASSET', label: '处置固定资产无形资产和其他长期资产收回的现金净额' },
    { key: 'TOTAL_INVEST_INFLOW', label: '投资活动现金流入小计' },
    { key: 'CONSTRUCT_LONG_ASSET', label: '购建固定资产无形资产和其他长期资产支付的现金' },
    { key: 'INVEST_PAY_CASH', label: '投资支付的现金' },
    { key: 'PAY_OTHER_INVEST', label: '支付其他与投资活动有关的现金' },
    { key: 'TOTAL_INVEST_OUTFLOW', label: '投资活动现金流出小计' },
    { key: 'NETCASH_INVEST', label: '投资活动产生的现金流量净额' },
    { key: 'ACCEPT_INVEST_CASH', label: '吸收投资收到的现金' },
    { key: 'RECEIVE_LOAN_CASH', label: '取得借款收到的现金' },
    { key: 'ISSUE_BOND', label: '发行债券收到的现金' },
    { key: 'RECEIVE_OTHER_FINANCE', label: '收到其他与筹资活动有关的现金' },
    { key: 'TOTAL_FINANCE_INFLOW', label: '筹资活动现金流入小计' },
    { key: 'PAY_DEBT_CASH', label: '偿还债务支付的现金' },
    { key: 'ASSIGN_DIVIDEND_PORFIT', label: '分配股利利润或偿付利息支付的现金' },
    { key: 'PAY_OTHER_FINANCE', label: '支付其他与筹资活动有关的现金' },
    { key: 'TOTAL_FINANCE_OUTFLOW', label: '筹资活动现金流出小计' },
    { key: 'NETCASH_FINANCE', label: '筹资活动产生的现金流量净额' },
    { key: 'RATE_CHANGE_EFFECT', label: '汇率变动对现金及现金等价物的影响' },
    { key: 'CCE_ADD', label: '现金及现金等价物净增加额' },
    { key: 'BEGIN_CCE', label: '期初现金及现金等价物余额' },
    { key: 'END_CCE', label: '期末现金及现金等价物余额' },
    { key: 'OPINION_TYPE', label: '审计意见(境内)' },
  ],
};

const statementPriorityKeys: Record<FinancialStatementType, string[]> = {
  income: [
    'TOTAL_OPERATEINCOME',
    'TOTAL_OPERATE_INCOME',
    'OPERATE_INCOME',
    'PARENT_NETPROFIT',
    'NETPROFIT',
    'KCFJCXSYJLR',
    'DEDUCT_NETPROFIT',
    'BASIC_EPS',
    'ROEJQ',
    'ROE',
    'GROSS_PROFIT_RATIO',
    'NET_PROFIT_RATIO',
    'TOTAL_OPERATE_INCOME_YOY',
    'NETPROFIT_YOY',
    'DEDUCT_NETPROFIT_YOY',
  ],
  balance: [
    'TOTAL_ASSETS',
    'TOTAL_LIABILITY',
    'TOTAL_EQUITY',
    'ASSET_LIAB_RATIO',
    'BPS',
    'TOTAL_SHARE',
    'FREE_SHARE',
  ],
  cashflow: [
    'SALES_SERVICES',
    'TAX_REFUND',
    'RECEIVE_OTHER_OPERATE',
    'TOTAL_OPERATE_INFLOW',
    'BUY_SERVICES',
    'PAY_STAFF_CASH',
    'PAY_ALL_TAX',
    'PAY_OTHER_OPERATE',
    'TOTAL_OPERATE_OUTFLOW',
    'NETCASH_OPERATE',
  ],
};

const getReportDateText = (row: Record<string, any>) => {
  return String(findValue(row, ['REPORT_DATE', 'REPORTDATE', 'END_DATE', 'QDATE']) || '').trim();
};

const getReportTypeText = (row: Record<string, any>) => {
  return String(findValue(row, ['REPORT_TYPE_NAME', 'REPORT_TYPE']) || '').trim();
};

const getReportYearAndMonth = (row: Record<string, any>) => {
  const dateOnly = extractDateOnly(getReportDateText(row));
  if (!dateOnly) return { year: '', month: 0 };
  const [year, month] = dateOnly.split('-');
  return { year: year || '', month: Number(month || 0) };
};

const inferReportFilter = (row: Record<string, any>): FinancialReportFilter => {
  const type = getReportTypeText(row);
  if (type.includes('年')) return 'year';
  if (type.includes('三季')) return 'q3';
  if (type.includes('中') || type.includes('半') || type.includes('二季')) return 'half';
  if (type.includes('一季')) return 'q1';

  const { month } = getReportYearAndMonth(row);
  if (month === 12) return 'year';
  if (month === 9) return 'q3';
  if (month === 6) return 'half';
  return 'q1';
};

const inferQuarterRank = (row: Record<string, any>) => {
  const type = getReportTypeText(row);
  if (type.includes('一季')) return 1;
  if (type.includes('中') || type.includes('半') || type.includes('二季')) return 2;
  if (type.includes('三季')) return 3;
  if (type.includes('年')) return 4;

  const { month } = getReportYearAndMonth(row);
  if (month <= 3) return 1;
  if (month <= 6) return 2;
  if (month <= 9) return 3;
  return 4;
};

const sortFinancialRows = (rows: Record<string, any>[]) => {
  return [...rows].sort((a, b) => {
    const da = extractDateOnly(getReportDateText(a));
    const db = extractDateOnly(getReportDateText(b));
    return db.localeCompare(da);
  });
};

const shouldDeriveSingleQuarterField = (key: string) => {
  const normalized = normalizeFieldKey(key);
  if (financialMetaKeys.has(normalized)) return false;
  if (
    hasFieldToken(normalized, 'RATIO') ||
    hasFieldToken(normalized, 'RATE') ||
    normalized.includes('MARGIN') ||
    normalized.includes('YOY') ||
    normalized.includes('QOQ') ||
    normalized.includes('PCT') ||
    normalized.includes('PERCENT') ||
    normalized.includes('EPS') ||
    normalized.includes('ROE') ||
    normalized.includes('ROA') ||
    normalized.includes('ROIC') ||
    normalized.includes('TURNOVER') ||
    normalized.includes('GROWTH') ||
    normalized.includes('TZ') ||
    normalized.includes('TB') ||
    normalized.includes('HB')
  ) {
    return false;
  }
  return true;
};

const toNumeric = (value: any) => {
  if (value === undefined || value === null || value === '') return NaN;
  if (typeof value === 'number') return value;
  if (typeof value === 'string') {
    const cleaned = value.replace(/,/g, '').replace(/%/g, '').trim();
    if (!cleaned) return NaN;
    const num = Number(cleaned);
    return Number.isNaN(num) ? NaN : num;
  }
  return NaN;
};

const toSingleQuarterRows = (rows: Record<string, any>[], statementType: FinancialStatementType) => {
  if (statementType === 'balance' || rows.length === 0) return rows;

  const index = new Map<string, Record<string, any>>();
  rows.forEach(row => {
    const { year } = getReportYearAndMonth(row);
    if (!year) return;
    const rank = inferQuarterRank(row);
    index.set(`${year}-${rank}`, row);
  });

  return rows.map(row => {
    const { year } = getReportYearAndMonth(row);
    const rank = inferQuarterRank(row);
    if (!year || rank <= 1) return row;

    const prev = index.get(`${year}-${rank - 1}`);
    if (!prev) return row;

    const next = { ...row };
    Object.keys(next).forEach(key => {
      if (!shouldDeriveSingleQuarterField(key)) return;
      const currentValue = toNumeric(row[key]);
      const prevValue = toNumeric(prev[key]);
      if (Number.isNaN(currentValue) || Number.isNaN(prevValue)) return;
      next[key] = currentValue - prevValue;
    });

    const type = getReportTypeText(row);
    if (type && !type.includes('单季度')) {
      next.REPORT_TYPE_NAME = `${type}(单季度)`;
    }
    return next;
  });
};

const hasFieldToken = (normalized: string, token: string) => {
  const parts = normalized.split('_').filter(Boolean);
  return parts.includes(token);
};

const isPercentLikeField = (key: string) => {
  const normalized = normalizeFieldKey(key);
  return (
    hasFieldToken(normalized, 'RATIO') ||
    hasFieldToken(normalized, 'RATE') ||
    normalized.includes('MARGIN') ||
    normalized.includes('YOY') ||
    normalized.includes('QOQ') ||
    normalized.includes('PCT') ||
    normalized.includes('PERCENT') ||
    normalized.includes('ROE') ||
    normalized.includes('ROA') ||
    normalized.includes('ROIC') ||
    normalized.endsWith('TZ') ||
    normalized.endsWith('TB') ||
    normalized.endsWith('HB') ||
    normalized.includes('GROWTH')
  );
};

const formatFinancialFieldValue = (value: any, key: string) => {
  if (value === undefined || value === null || value === '') return '--';
  if (typeof value === 'string' && value.trim().endsWith('%')) return value.trim();

  const num = toNumeric(value);
  if (!Number.isNaN(num) && isPercentLikeField(key)) {
    const sign = num > 0 ? '+' : '';
    return `${sign}${num.toFixed(2)}%`;
  }
  return formatValue(value);
};

const collectFinancialDisplayFields = (
  rows: Record<string, any>[],
  statementType: FinancialStatementType,
  percentOnly: boolean,
) => {
  const hasValue = (row: Record<string, any>, key: string) => {
    const value = findValue(row, [key]);
    return value !== '' && value !== null && value !== undefined;
  };

  if (!percentOnly) {
    const schema = statementFieldSchemas[statementType] || [];
    return schema.filter(field => rows.some(row => hasValue(row, field.key)));
  }

  const keySet = new Set<string>();
  rows.forEach(row => {
    Object.entries(row || {}).forEach(([key, value]) => {
      const normalized = normalizeFieldKey(key);
      if (financialMetaKeys.has(normalized)) return;
      if (!isPrimitive(value)) return;
      if (value === '' || value === null || value === undefined) return;
      if (percentOnly && !isPercentLikeField(key)) {
        const text = String(value).trim();
        if (!text.endsWith('%')) return;
      }
      keySet.add(key);
    });
  });

  const priority = statementPriorityKeys[statementType] || [];
  const orderedKeys = [
    ...priority.filter(key => keySet.has(key)),
    ...Array.from(keySet).filter(key => !priority.includes(key)).sort((a, b) => a.localeCompare(b)),
  ];
  return orderedKeys
    .map(key => ({ key, label: mapKeyLabel(key) }))
    .filter(field => field.label !== '未映射指标');
};

const renderFinancialMatrix = (
  title: string,
  rows: Record<string, any>[],
  statementType: FinancialStatementType,
  percentOnly: boolean = false,
) => {
  if (!rows.length) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        {title}: 暂无数据
      </div>
    );
  }

  const fields = collectFinancialDisplayFields(rows, statementType, percentOnly);
  if (!fields.length) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        {title}: 当前条件下无可展示字段
      </div>
    );
  }

  const gridTemplateColumns = `220px repeat(${Math.max(rows.length, 1)}, minmax(110px, 1fr))`;
  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">{title}</div>
      <div className="overflow-x-auto">
        <div className="min-w-max">
          <div className="grid gap-2 text-[11px] text-slate-500" style={{ gridTemplateColumns }}>
            <span>指标</span>
            {rows.map((row, idx) => {
              const date = extractDateOnly(getReportDateText(row)) || formatValue(getReportDateText(row));
              const type = formatValue(getReportTypeText(row));
              return (
                <span key={idx} className="text-right leading-tight">
                  <span className="block">{date || '--'}</span>
                  {type && type !== '--' && <span className="block text-[10px] text-slate-600">{type}</span>}
                </span>
              );
            })}
          </div>
          <div className="mt-2 space-y-1 max-h-96 overflow-auto pr-1">
            {fields.map(field => (
              <div key={field.key} className="grid gap-2" style={{ gridTemplateColumns }}>
                <span className="text-slate-500">{field.label}</span>
                {rows.map((row, idx) => (
                  <span key={idx} className="text-slate-200 text-right">
                    {formatFinancialFieldValue(findValue(row, [field.key]), field.key)}
                  </span>
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

const renderFinancials = (
  financials?: FinancialStatements,
  mainIndicators?: F10MainIndicators,
  controls?: FinancialRenderControls,
) => {
  const income = sortFinancialRows(financials?.income || []);
  const balance = sortFinancialRows(financials?.balance || []);
  const cashflow = sortFinancialRows(financials?.cashflow || []);

  if (income.length === 0 && balance.length === 0 && cashflow.length === 0) {
    return <div className="text-slate-500">暂无财务报表数据</div>;
  }
  if (!controls) {
    return <div className="text-slate-500">财务视图状态未初始化</div>;
  }

  const statementRowsMap: Record<FinancialStatementType, Record<string, any>[]> = {
    income,
    balance,
    cashflow,
  };
  const baseRows = statementRowsMap[controls.statementType] || [];
  const modeRows = controls.viewMode === 'singleQuarter'
    ? toSingleQuarterRows(baseRows, controls.statementType)
    : baseRows;
  const filteredRows = modeRows.filter(row => {
    if (controls.reportFilter === 'all') return true;
    return inferReportFilter(row) === controls.reportFilter;
  });

  const statementLabelMap: Record<FinancialStatementType, string> = {
    income: '利润表',
    balance: '资产负债表',
    cashflow: '现金流量表',
  };

  const renderSubTabButton = (tab: FinancialSubTabId, label: string) => (
    <button
      key={tab}
      onClick={() => controls.onSubTabChange(tab)}
      className={`px-2 py-1 rounded border text-xs transition-colors ${
        controls.subTab === tab
          ? 'border-accent text-accent-2 bg-accent/10'
          : 'border-transparent text-slate-400 hover:text-slate-200'
      }`}
    >
      {label}
    </button>
  );

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
        {renderSubTabButton('mainIndicators', '主要指标')}
        {renderSubTabButton('statements', '财务报表')}
        {renderSubTabButton('percentStatements', '百分比报表')}
      </div>

      {controls.subTab === 'mainIndicators' && renderMainIndicators(mainIndicators)}

      {(controls.subTab === 'statements' || controls.subTab === 'percentStatements') && (
        <div className="rounded border border-slate-700/60 px-3 py-2 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-slate-500">报表类型:</span>
            {(['income', 'balance', 'cashflow'] as FinancialStatementType[]).map(item => (
              <button
                key={item}
                onClick={() => controls.onStatementTypeChange(item)}
                className={`px-2 py-1 rounded border text-xs transition-colors ${
                  controls.statementType === item
                    ? 'border-accent text-accent-2 bg-accent/10'
                    : 'border-transparent text-slate-400 hover:text-slate-200'
                }`}
              >
                {statementLabelMap[item]}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-slate-500">报告筛选:</span>
            {(['all', 'year', 'q3', 'half', 'q1'] as FinancialReportFilter[]).map(item => (
              <button
                key={item}
                onClick={() => controls.onReportFilterChange(item)}
                className={`px-2 py-1 rounded border text-xs transition-colors ${
                  controls.reportFilter === item
                    ? 'border-accent text-accent-2 bg-accent/10'
                    : 'border-transparent text-slate-400 hover:text-slate-200'
                }`}
              >
                {reportFilterLabels[item]}
              </button>
            ))}
          </div>
          {controls.subTab === 'statements' && controls.statementType !== 'balance' && (
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-slate-500">口径:</span>
              {(['report', 'singleQuarter'] as FinancialViewMode[]).map(item => (
                <button
                  key={item}
                  onClick={() => controls.onViewModeChange(item)}
                  className={`px-2 py-1 rounded border text-xs transition-colors ${
                    controls.viewMode === item
                      ? 'border-accent text-accent-2 bg-accent/10'
                      : 'border-transparent text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {item === 'report' ? '按报告期' : '按单季度'}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {controls.subTab === 'statements' && renderFinancialMatrix(
        `${statementLabelMap[controls.statementType]}（${filteredRows.length}期）`,
        filteredRows,
        controls.statementType,
        false,
      )}
      {controls.subTab === 'statements' && controls.statementType === 'cashflow' && renderCashflowDerivedMetrics(financials, mainIndicators)}

      {controls.subTab === 'percentStatements' && renderFinancialMatrix(
        `${statementLabelMap[controls.statementType]}百分比指标（${filteredRows.length}期）`,
        filteredRows,
        controls.statementType,
        true,
      )}
    </div>
  );
};

const renderPerformance = (performance?: PerformanceEvents) => {
  if (!performance?.forecast?.length && !performance?.express?.length && !performance?.schedule?.length) {
    return <div className="text-slate-500">暂无业绩事件数据</div>;
  }

  const hasValue = (value: any) => value !== undefined && value !== null && value !== '';
  const toSummaryText = (item: Record<string, any>, keys: string[], maxLen: number = 90) => {
    const raw = String(findValue(item, keys) || '').replace(/\s+/g, ' ').trim();
    if (!raw) return '--';
    return raw.length > maxLen ? `${raw.slice(0, maxLen)}...` : raw;
  };
  const sortByDates = (rows: Record<string, any>[], keys: string[]) => (
    [...rows].sort((a, b) => {
      const da = String(findValue(a, keys) || '');
      const db = String(findValue(b, keys) || '');
      return db.localeCompare(da);
    })
  );

  const forecastRows = sortByDates(performance?.forecast || [], ['NOTICE_DATE', 'noticeDate', 'REPORT_DATE']).slice(0, 10);
  const expressRows = sortByDates(performance?.express || [], ['NOTICE_DATE', 'noticeDate', 'REPORT_DATE']).slice(0, 10);
  const scheduleRows = sortByDates(performance?.schedule || [], ['APPOINT_PUBLISH_DATE', 'appointDate', 'REPORT_DATE']).slice(0, 10);

  const renderForecastProfit = (row: Record<string, any>) => {
    const lower = findValue(row, ['netProfitLower', 'PREDICT_AMT_LOWER']);
    const upper = findValue(row, ['netProfitUpper', 'PREDICT_AMT_UPPER']);
    const amount = findValue(row, ['forecastAmount', 'FORECAST_JZ', 'NETPROFIT']);
    if (hasValue(lower) && hasValue(upper)) {
      return `${formatValue(lower)} ~ ${formatValue(upper)}`;
    }
    if (hasValue(amount)) {
      return formatValue(amount);
    }
    return '--';
  };

  const renderForecastRange = (row: Record<string, any>) => {
    const lower = findValue(row, ['ratioLower', 'PREDICT_RATIO_LOWER']);
    const upper = findValue(row, ['ratioUpper', 'PREDICT_RATIO_UPPER']);
    const mean = findValue(row, ['ratioMean', 'PREDICT_HBMEAN', 'CHANGE_RANGE']);
    if (hasValue(lower) && hasValue(upper)) {
      return `${formatPercentCell(lower, true)} ~ ${formatPercentCell(upper, true)}`;
    }
    if (hasValue(mean)) {
      return formatPercentCell(mean, true);
    }
    return '--';
  };

  return (
    <div className="space-y-3">
      {forecastRows.length > 0 && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">业绩预告</div>
          <div className="overflow-x-auto">
            <table className="min-w-[1120px] w-full text-xs">
              <thead>
                <tr className="text-slate-500 border-b border-slate-700/60">
                  <th className="text-left py-1 pr-2">公告日</th>
                  <th className="text-left py-1 pr-2">报告期</th>
                  <th className="text-left py-1 pr-2">预告类型</th>
                  <th className="text-right py-1 pr-2">预计净利润</th>
                  <th className="text-right py-1 pr-2">变动区间</th>
                  <th className="text-left py-1">内容摘要</th>
                </tr>
              </thead>
              <tbody>
                {forecastRows.map((row, idx) => (
                  <tr key={`forecast-${idx}`} className="border-b border-slate-800/70 align-top">
                    <td className="py-1 pr-2 text-slate-200 whitespace-nowrap">{formatValue(findValue(row, ['noticeDate', 'NOTICE_DATE']))}</td>
                    <td className="py-1 pr-2 text-slate-300 whitespace-nowrap">{formatValue(findValue(row, ['reportDate', 'REPORT_DATE']))}</td>
                    <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['predictType', 'PREDICT_TYPE', 'FORECAST_TYPE']))}</td>
                    <td className="py-1 pr-2 text-slate-200 text-right whitespace-nowrap">{renderForecastProfit(row)}</td>
                    <td className="py-1 pr-2 text-slate-300 text-right whitespace-nowrap">{renderForecastRange(row)}</td>
                    <td className="py-1 text-slate-300">{toSummaryText(row, ['predictContent', 'PREDICT_CONTENT', 'FORECAST_CONTENT'])}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {expressRows.length > 0 && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">业绩快报</div>
          <div className="overflow-x-auto">
            <table className="min-w-[1120px] w-full text-xs">
              <thead>
                <tr className="text-slate-500 border-b border-slate-700/60">
                  <th className="text-left py-1 pr-2">公告日</th>
                  <th className="text-left py-1 pr-2">报告期</th>
                  <th className="text-right py-1 pr-2">营业收入</th>
                  <th className="text-right py-1 pr-2">营收同比</th>
                  <th className="text-right py-1 pr-2">归母净利润</th>
                  <th className="text-right py-1 pr-2">净利同比</th>
                  <th className="text-right py-1 pr-2">EPS</th>
                  <th className="text-right py-1">ROE</th>
                </tr>
              </thead>
              <tbody>
                {expressRows.map((row, idx) => (
                  <tr key={`express-${idx}`} className="border-b border-slate-800/70">
                    <td className="py-1 pr-2 text-slate-200 whitespace-nowrap">{formatValue(findValue(row, ['noticeDate', 'NOTICE_DATE']))}</td>
                    <td className="py-1 pr-2 text-slate-300 whitespace-nowrap">{formatValue(findValue(row, ['reportDate', 'REPORT_DATE', 'QDATE']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(row, ['revenue', 'TOTAL_OPERATE_INCOME']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ['revenueYoY', 'YSTZ']), true)}</td>
                    <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(row, ['netProfit', 'PARENT_NETPROFIT', 'NETPROFIT']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ['netProfitYoY', 'JLRTBZCL']), true)}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['eps', 'BASIC_EPS']))}</td>
                    <td className="py-1 text-right text-slate-300">{formatPercentCell(findValue(row, ['roe', 'WEIGHTAVG_ROE']), true)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {scheduleRows.length > 0 && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">披露预约</div>
          <div className="overflow-x-auto">
            <table className="min-w-[980px] w-full text-xs">
              <thead>
                <tr className="text-slate-500 border-b border-slate-700/60">
                  <th className="text-left py-1 pr-2">报告期</th>
                  <th className="text-left py-1 pr-2">报告类型</th>
                  <th className="text-left py-1 pr-2">预约披露日</th>
                  <th className="text-left py-1 pr-2">实际披露日</th>
                  <th className="text-right py-1 pr-2">剩余天数</th>
                  <th className="text-left py-1">变更记录</th>
                </tr>
              </thead>
              <tbody>
                {scheduleRows.map((row, idx) => {
                  const changes = [
                    findValue(row, ['firstChangeDate', 'FIRST_CHANGE_DATE']),
                    findValue(row, ['secondChangeDate', 'SECOND_CHANGE_DATE']),
                    findValue(row, ['thirdChangeDate', 'THIRD_CHANGE_DATE']),
                  ].filter(v => hasValue(v));
                  return (
                    <tr key={`schedule-${idx}`} className="border-b border-slate-800/70">
                      <td className="py-1 pr-2 text-slate-200 whitespace-nowrap">{formatValue(findValue(row, ['reportDate', 'REPORT_DATE']))}</td>
                      <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['reportType', 'REPORT_TYPE_NAME', 'REPORT_TYPE']))}</td>
                      <td className="py-1 pr-2 text-slate-300 whitespace-nowrap">{formatValue(findValue(row, ['appointDate', 'APPOINT_PUBLISH_DATE', 'FIRST_APPOINT_DATE']))}</td>
                      <td className="py-1 pr-2 text-slate-300 whitespace-nowrap">{formatValue(findValue(row, ['actualDate', 'ACTUAL_PUBLISH_DATE']))}</td>
                      <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['residualDays', 'RESIDUAL_DAYS']))}</td>
                      <td className="py-1 text-slate-300">{changes.length > 0 ? changes.map(v => formatValue(v)).join(' / ') : '--'}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};

const renderFundFlow = (fundFlow?: FundFlowSeries) => {
  const fields = fundFlow?.fields || [];
  const lines = fundFlow?.lines || [];
  if (!fields.length || !lines.length) {
    return <div className="text-slate-500">暂无资金流数据</div>;
  }

  const indexOf = (field: string) => fields.findIndex(item => item.toLowerCase() === field.toLowerCase());
  const dateIdx = indexOf('f51');
  const mainIdx = indexOf('f52');
  const largeIdx = indexOf('f55');
  const superIdx = indexOf('f56');
  const changeIdx = indexOf('f63');
  const recentLines = [...lines].sort((a, b) => {
    if (dateIdx < 0) return 0;
    const dateA = String(getLineValue(a, dateIdx) || '');
    const dateB = String(getLineValue(b, dateIdx) || '');
    return dateB.localeCompare(dateA);
  });
  const recentDays = Math.min(15, recentLines.length);

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">最近资金流（近{recentDays}日）</div>
      <div className="grid grid-cols-5 gap-2 text-slate-500 text-[11px]">
        <span>日期</span>
        <span>主力净流入</span>
        <span>大单净流入</span>
        <span>超大单净流入</span>
        <span>涨跌幅</span>
      </div>
      <div className="space-y-1 mt-1">
        {recentLines.slice(0, recentDays).map((line, idx) => (
          <div key={idx} className="grid grid-cols-5 gap-2 text-slate-200">
            <span>{getLineValue(line, dateIdx)}</span>
            <span>{formatSignedNumber(getLineValue(line, mainIdx))}</span>
            <span>{formatSignedNumber(getLineValue(line, largeIdx))}</span>
            <span>{formatSignedNumber(getLineValue(line, superIdx))}</span>
            <span>{formatSignedNumber(getLineValue(line, changeIdx), true)}</span>
          </div>
        ))}
      </div>
    </div>
  );
};

const renderBonus = (
  bonus?: BonusFinancing,
  hideNoPlanDividends: boolean = false,
  setHideNoPlanDividends?: React.Dispatch<React.SetStateAction<boolean>>,
) => {
  if (!bonus?.dividend?.length && !bonus?.annual?.length && !bonus?.financing?.length && !bonus?.allotment?.length) {
    return <div className="text-slate-500">暂无分红融资数据</div>;
  }

  const sortByDateDesc = (items: Record<string, any>[], keys: string[]) => {
    return [...items].sort((a, b) => {
      const da = String(findValue(a, keys) || '');
      const db = String(findValue(b, keys) || '');
      return db.localeCompare(da);
    });
  };

  const dividendRows = sortByDateDesc(bonus?.dividend || [], ['NOTICE_DATE', 'noticeDate']);
  const annualRows = [...(bonus?.annual || [])].sort((a, b) => {
    const ya = Number(findValue(a, ['STATISTICS_YEAR', 'year']) || 0);
    const yb = Number(findValue(b, ['STATISTICS_YEAR', 'year']) || 0);
    return yb - ya;
  });
  const financingRows = sortByDateDesc(bonus?.financing || [], ['NOTICE_DATE', 'noticeDate']);
  const allotmentRows = sortByDateDesc(bonus?.allotment || [], ['NOTICE_DATE', 'noticeDate']);

  const BonusDividendTable: React.FC<{ rows: Record<string, any>[] }> = ({ rows }) => {
    const filteredRows = hideNoPlanDividends
      ? rows.filter(row => {
        const plan = String(findValue(row, ['IMPL_PLAN_PROFILE', 'plan']) || '').trim();
        return !plan.includes('不分配不转增');
      })
      : rows;

    return (
      <div className="rounded border border-slate-700/60 px-3 py-2">
        <div className="flex items-center justify-between mb-2">
          <div className="text-slate-400">分红方案</div>
          <label className="text-[11px] text-slate-500 inline-flex items-center gap-1 cursor-pointer select-none">
            <input
              type="checkbox"
              className="accent-sky-500"
              checked={hideNoPlanDividends}
              onChange={(e) => setHideNoPlanDividends?.(e.target.checked)}
            />
            隐藏“不分配不转增”
          </label>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-[980px] w-full text-xs">
            <thead>
              <tr className="text-slate-500 border-b border-slate-700/60">
                <th className="text-left py-1 pr-2">公告日期</th>
                <th className="text-left py-1 pr-2">分红方案</th>
                <th className="text-left py-1 pr-2">方案进度</th>
                <th className="text-left py-1 pr-2">股权登记日</th>
                <th className="text-left py-1 pr-2">除权除息日</th>
                <th className="text-left py-1">派息日</th>
              </tr>
            </thead>
            <tbody>
              {filteredRows.map((row, idx) => (
                <tr key={idx} className="border-b border-slate-800/70">
                  <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['NOTICE_DATE', 'noticeDate']))}</td>
                  <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['IMPL_PLAN_PROFILE', 'plan']))}</td>
                  <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['ASSIGN_PROGRESS', 'progress']))}</td>
                  <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['EQUITY_RECORD_DATE', 'recordDate']))}</td>
                  <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['EX_DIVIDEND_DATE', 'exDate']))}</td>
                  <td className="py-1 text-slate-300">{formatValue(findValue(row, ['PAY_CASH_DATE', 'payDate']))}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  };

  const BonusAnnualTable: React.FC<{ rows: Record<string, any>[] }> = ({ rows }) => (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">年度分红融资汇总</div>
      <div className="overflow-x-auto">
        <table className="min-w-[760px] w-full text-xs">
          <thead>
            <tr className="text-slate-500 border-b border-slate-700/60">
              <th className="text-left py-1 pr-2">统计年度</th>
              <th className="text-right py-1 pr-2">分红总额</th>
              <th className="text-right py-1 pr-2">增发数量</th>
              <th className="text-right py-1 pr-2">配股数量</th>
              <th className="text-right py-1">IPO数量</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, idx) => (
              <tr key={idx} className="border-b border-slate-800/70">
                <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['STATISTICS_YEAR', 'year']))}</td>
                <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(row, ['TOTAL_DIVIDEND', 'totalDividend']))}</td>
                <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['SEO_NUM', 'seoNum']))}</td>
                <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['ALLOTMENT_NUM', 'allotmentNum']))}</td>
                <td className="py-1 text-right text-slate-300">{formatValue(findValue(row, ['IPO_NUM', 'ipoNum']))}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );

  const BonusFinanceTable: React.FC<{ title: string; rows: Record<string, any>[]; isAllotment?: boolean }> = ({ title, rows, isAllotment }) => (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">{title}</div>
      <div className="overflow-x-auto">
        <table className="min-w-[980px] w-full text-xs">
          <thead>
            <tr className="text-slate-500 border-b border-slate-700/60">
              <th className="text-left py-1 pr-2">公告日期</th>
              <th className="text-right py-1 pr-2">发行数量</th>
              <th className="text-right py-1 pr-2">{isAllotment ? '募资总额' : '募资净额'}</th>
              <th className="text-right py-1 pr-2">发行价格</th>
              <th className="text-left py-1 pr-2">发行方式/说明</th>
              <th className="text-left py-1 pr-2">登记日</th>
              <th className="text-left py-1">上市/除权日</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, idx) => (
              <tr key={idx} className="border-b border-slate-800/70">
                <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['NOTICE_DATE', 'noticeDate']))}</td>
                <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(row, ['ISSUE_NUM', 'issueNum']))}</td>
                <td className="py-1 pr-2 text-right text-slate-300">
                  {formatValue(findValue(row, isAllotment ? ['TOTAL_RAISE_FUNDS', 'raiseFunds'] : ['NET_RAISE_FUNDS', 'raiseFunds']))}
                </td>
                <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['ISSUE_PRICE', 'issuePrice']))}</td>
                <td className="py-1 pr-2 text-slate-300">
                  {formatValue(findValue(row, isAllotment ? ['EVENT_EXPLAIN', 'issueWay'] : ['ISSUE_WAY_EXPLAIN', 'issueWay']))}
                </td>
                <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['REG_DATE', 'regDate', 'EQUITY_RECORD_DATE']))}</td>
                <td className="py-1 text-slate-300">{formatValue(findValue(row, ['LISTING_DATE', 'listDate', 'EX_DIVIDEND_DATEE']))}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );

  return (
    <div className="space-y-3">
      <BonusDividendTable rows={dividendRows} />
      <BonusAnnualTable rows={annualRows} />
      <BonusFinanceTable title="增发融资" rows={financingRows} />
      <BonusFinanceTable title="配股融资" rows={allotmentRows} isAllotment />
    </div>
  );
};

const businessTypeLabel = (value: any) => {
  const raw = String(value ?? '').trim();
  if (raw === '2') return '按产品分类';
  if (raw === '3') return '按地区分类';
  if (raw === '1') return '按行业分类';
  return raw ? `分类${raw}` : '其他分类';
};

const parseNumberLike = (value: any): number | null => {
  if (value === undefined || value === null || value === '') return null;
  const num = Number(String(value).replace(/,/g, '').trim());
  return Number.isNaN(num) ? null : num;
};

const formatRatioPercent = (value: any) => {
  const num = parseNumberLike(value);
  if (num === null) return '--';
  return `${(num * 100).toFixed(2)}%`;
};

const BusinessCompositionTable: React.FC<{ records?: Record<string, any>[] }> = ({ records }) => {
  const list = records || [];
  if (list.length === 0) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        主营构成: 暂无数据
      </div>
    );
  }

  const reportDates = useMemo(() => {
    const values = Array.from(new Set(list
      .map(item => String(findValue(item, ['REPORT_DATE', 'reportDate']) || '').trim())
      .filter(Boolean)));
    return values.sort((a, b) => b.localeCompare(a));
  }, [list]);

  const [selectedDate, setSelectedDate] = useState<string>(reportDates[0] || '');
  useEffect(() => {
    if (!reportDates.length) return;
    if (!selectedDate || !reportDates.includes(selectedDate)) {
      setSelectedDate(reportDates[0]);
    }
  }, [reportDates, selectedDate]);

  const currentRows = useMemo(() => {
    const filtered = selectedDate
      ? list.filter(item => String(findValue(item, ['REPORT_DATE', 'reportDate']) || '').trim() === selectedDate)
      : list;
    return [...filtered].sort((a, b) => {
      const ta = String(findValue(a, ['MAINOP_TYPE', 'type']) || '');
      const tb = String(findValue(b, ['MAINOP_TYPE', 'type']) || '');
      if (ta !== tb) return ta.localeCompare(tb);
      const ra = Number(findValue(a, ['RANK', 'rank']) ?? 0);
      const rb = Number(findValue(b, ['RANK', 'rank']) ?? 0);
      return ra - rb;
    });
  }, [list, selectedDate]);

  const grouped = useMemo(() => {
    const map = new Map<string, Record<string, any>[]>();
    currentRows.forEach(row => {
      const key = businessTypeLabel(findValue(row, ['MAINOP_TYPE', 'type']));
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(row);
    });
    return Array.from(map.entries());
  }, [currentRows]);

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">主营构成分析</div>
      <div className="flex gap-2 overflow-x-auto pb-2">
        {reportDates.slice(0, 16).map(date => (
          <button
            key={date}
            onClick={() => setSelectedDate(date)}
            className={`px-2 py-1 rounded border text-[11px] whitespace-nowrap ${
              date === selectedDate
                ? 'border-accent text-accent-2 bg-accent/10'
                : 'border-slate-700/60 text-slate-400 hover:text-slate-200'
            }`}
          >
            {extractDateOnly(date) || date}
          </button>
        ))}
      </div>
      <div className="overflow-x-auto">
        <table className="min-w-[980px] w-full text-xs">
          <thead>
            <tr className="text-slate-500 border-b border-slate-700/60">
              <th className="text-left py-1 pr-2">分类</th>
              <th className="text-left py-1 pr-2">主营构成</th>
              <th className="text-right py-1 pr-2">主营收入(元)</th>
              <th className="text-right py-1 pr-2">收入比例</th>
              <th className="text-right py-1 pr-2">主营成本(元)</th>
              <th className="text-right py-1 pr-2">成本比例</th>
              <th className="text-right py-1 pr-2">主营利润(元)</th>
              <th className="text-right py-1 pr-2">利润比例</th>
              <th className="text-right py-1">毛利率</th>
            </tr>
          </thead>
          <tbody>
            {grouped.map(([groupName, rows]) =>
              rows.map((row, idx) => (
                <tr key={`${groupName}-${idx}`} className="border-b border-slate-800/70">
                  {idx === 0 && (
                    <td rowSpan={rows.length} className="align-top py-1 pr-2 text-slate-400 whitespace-nowrap">
                      {groupName}
                    </td>
                  )}
                  <td className="py-1 pr-2 text-slate-200 whitespace-nowrap">
                    {formatValue(findValue(row, ['ITEM_NAME', 'itemName']) || '--')}
                  </td>
                  <td className="py-1 pr-2 text-right text-slate-200">
                    {formatValue(findValue(row, ['MAIN_BUSINESS_INCOME', 'income']))}
                  </td>
                  <td className="py-1 pr-2 text-right text-slate-300">
                    {formatRatioPercent(findValue(row, ['MBI_RATIO', 'incomeRatio']))}
                  </td>
                  <td className="py-1 pr-2 text-right text-slate-200">
                    {formatValue(findValue(row, ['MAIN_BUSINESS_COST', 'cost']))}
                  </td>
                  <td className="py-1 pr-2 text-right text-slate-300">
                    {formatRatioPercent(findValue(row, ['MBC_RATIO', 'costRatio']))}
                  </td>
                  <td className="py-1 pr-2 text-right text-slate-200">
                    {formatValue(findValue(row, ['MAIN_BUSINESS_RPOFIT', 'MAIN_BUSINESS_PROFIT', 'profit']))}
                  </td>
                  <td className="py-1 pr-2 text-right text-slate-300">
                    {formatRatioPercent(findValue(row, ['MBR_RATIO', 'profitRatio']))}
                  </td>
                  <td className="py-1 text-right text-slate-300">
                    {formatRatioPercent(findValue(row, ['GROSS_RPOFIT_RATIO', 'GROSS_PROFIT_RATIO', 'grossMargin']))}
                  </td>
                </tr>
              )),
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};

const BusinessReviewPanel: React.FC<{ records?: Record<string, any>[] }> = ({ records }) => {
  const list = records || [];
  if (list.length === 0) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        经营评述: 暂无数据
      </div>
    );
  }

  const reportDates = useMemo(() => {
    const values = Array.from(new Set(list
      .map(item => String(findValue(item, ['REPORT_DATE', 'reportDate']) || '').trim())
      .filter(Boolean)));
    return values.sort((a, b) => b.localeCompare(a));
  }, [list]);
  const [selectedDate, setSelectedDate] = useState<string>(reportDates[0] || '');
  useEffect(() => {
    if (!reportDates.length) return;
    if (!selectedDate || !reportDates.includes(selectedDate)) {
      setSelectedDate(reportDates[0]);
    }
  }, [reportDates, selectedDate]);

  const selected = useMemo(() => {
    if (!selectedDate) return list[0];
    return list.find(item => String(findValue(item, ['REPORT_DATE', 'reportDate']) || '').trim() === selectedDate) || list[0];
  }, [list, selectedDate]);

  const content = String(findValue(selected, ['reviewContent', 'BUSINESS_REVIEW']) || '').trim();
  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">经营评述</div>
      {reportDates.length > 1 && (
        <div className="flex gap-2 overflow-x-auto pb-2">
          {reportDates.map(date => (
            <button
              key={date}
              onClick={() => setSelectedDate(date)}
              className={`px-2 py-1 rounded border text-[11px] whitespace-nowrap ${
                date === selectedDate
                  ? 'border-accent text-accent-2 bg-accent/10'
                  : 'border-slate-700/60 text-slate-400 hover:text-slate-200'
              }`}
            >
              {extractDateOnly(date) || date}
            </button>
          ))}
        </div>
      )}
      <div className="text-slate-200 leading-relaxed whitespace-pre-wrap break-words">
        {content || '暂无评述正文'}
      </div>
    </div>
  );
};

const BusinessScopePanel: React.FC<{ records?: Record<string, any>[] }> = ({ records }) => {
  const list = records || [];
  if (list.length === 0) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        主营范围: 暂无数据
      </div>
    );
  }
  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">主营范围</div>
      <div className="space-y-2">
        {list.map((item, idx) => (
          <div key={idx} className="text-slate-200 leading-relaxed whitespace-pre-wrap break-words">
            {String(findValue(item, ['businessScope', 'BUSINESS_SCOPE']) || '').trim() || '暂无数据'}
          </div>
        ))}
      </div>
    </div>
  );
};

const renderBusiness = (business?: BusinessAnalysis) => {
  if (!business?.scope?.length && !business?.composition?.length && !business?.review?.length) {
    return <div className="text-slate-500">暂无经营分析数据</div>;
  }

  return (
    <div className="space-y-3">
      <BusinessScopePanel records={business?.scope} />
      <BusinessCompositionTable records={business?.composition} />
      <BusinessReviewPanel records={business?.review} />
    </div>
  );
};

const pickArrayByKeys = (obj: Record<string, any> | undefined, keys: string[]): Record<string, any>[] => {
  if (!obj) return [];
  const objKeys = Object.keys(obj);
  for (const key of keys) {
    const direct = obj[key];
    if (Array.isArray(direct)) return direct as Record<string, any>[];
    const match = objKeys.find(item => item.toLowerCase() === key.toLowerCase());
    if (match && Array.isArray(obj[match])) return obj[match] as Record<string, any>[];
  }
  return [];
};

const formatPercentCell = (value: any, signed: boolean = false) => {
  if (value === undefined || value === null || value === '') return '--';
  const num = Number(value);
  if (Number.isNaN(num)) {
    const text = String(value).trim();
    if (!text) return '--';
    return text.includes('%') ? text : text;
  }
  const sign = signed && num > 0 ? '+' : '';
  return `${sign}${num.toFixed(2)}%`;
};

const sortByDateDesc = (items: Record<string, any>[], keys: string[]) => {
  return [...items].sort((a, b) => {
    const da = String(findValue(a, keys) || '');
    const db = String(findValue(b, keys) || '');
    return db.localeCompare(da);
  });
};

const renderShareholders = (
  shareholders?: ShareholderNumbers,
  institutions?: InstitutionalHoldings,
  holderChange?: ShareholderChanges,
) => {
  const controllerPayload = institutions?.controller as Record<string, any> | undefined;
  const gdrsFromController = pickArrayByKeys(controllerPayload, ['gdrs']);
  const numberRecords = shareholders?.records?.length ? shareholders.records : gdrsFromController;
  const latestNumbers = shareholders?.latest || numberRecords?.[0];
  const topShareholders = pickArrayByKeys(controllerPayload, ['sdgd']);
  const topShareholdersDisplay = topShareholders.length > 0 ? topShareholders : (institutions?.topHolders || []);
  const topFloatShareholders = pickArrayByKeys(controllerPayload, ['sdltgd']);
  const topHolderChanges = pickArrayByKeys(controllerPayload, ['sdgdcgbd']);
  const institutionSummaryRows = pickArrayByKeys(controllerPayload, ['jgcc']);
  const controllerRows = pickArrayByKeys(controllerPayload, ['sjkzr']);
  const controllerInfo = controllerRows[0];
  const fallbackHolderChange = holderChange?.records || [];
  const hasAnyData =
    numberRecords.length > 0 ||
    topShareholdersDisplay.length > 0 ||
    topFloatShareholders.length > 0 ||
    topHolderChanges.length > 0 ||
    fallbackHolderChange.length > 0 ||
    institutionSummaryRows.length > 0;

  if (!hasAnyData) {
    return <div className="text-slate-500">暂无股东研究数据</div>;
  }

  const numberTrendRows = sortByDateDesc(numberRecords, ['END_DATE', 'endDate']).slice(0, 12);
  const institutionSummaryDisplayRows = (() => {
    const sorted = sortByDateDesc(institutionSummaryRows, ['REPORT_DATE']);
    const overallRows = sorted.filter(row => {
      const orgType = String(findValue(row, ['ORG_TYPE']) || '').trim();
      return orgType === '' || orgType === '00';
    });
    return overallRows.length > 0 ? overallRows : sorted;
  })().slice(0, 10);

  const institutionSummaryMetricDefs: Array<{ label: string; keys: string[]; percent?: boolean }> = [
    { label: '机构总数(家)', keys: ['TOTAL_ORG_NUM'] },
    { label: '合计持股(股)', keys: ['TOTAL_FREE_SHARES'] },
    { label: '合计市值(元)', keys: ['TOTAL_HOLD_VALUE', 'TOTAL_MARKET_VALUE', 'TOTAL_MARKET_CAP', 'TOTAL_HOLD_MARKET_CAP', 'TOTAL_FREE_MARKET_CAP'] },
    { label: '占流通股比(%)', keys: ['TOTAL_SHARES_RATIO'], percent: true },
    { label: '占总股本比例(%)', keys: ['ALL_SHARES_RATIO'], percent: true },
  ];
  const institutionSummaryMetrics = institutionSummaryMetricDefs;
  const hasMetricValue = (value: any) => value !== undefined && value !== null && String(value).trim() !== '';
  const institutionSummaryRenderableRows = institutionSummaryDisplayRows.filter(row =>
    institutionSummaryMetrics.some(metric => hasMetricValue(findValue(row, metric.keys)))
  );
  const institutionSummaryMinWidth = Math.max(420, 180 + institutionSummaryRenderableRows.length * 120);

  const renderTopHolderTable = (title: string, rows: Record<string, any>[], ratioKeys: string[]) => {
    if (!rows.length) return null;
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2">
        <div className="text-slate-400 mb-2">{title}</div>
        <div className="overflow-x-auto">
          <table className="min-w-[920px] w-full text-xs">
            <thead>
              <tr className="text-slate-500 border-b border-slate-700/60">
                <th className="text-left py-1 pr-2">名次</th>
                <th className="text-left py-1 pr-2">股东名称</th>
                <th className="text-left py-1 pr-2">股份类型</th>
                <th className="text-right py-1 pr-2">持股数(股)</th>
                <th className="text-right py-1 pr-2">持股比例</th>
                <th className="text-right py-1 pr-2">增减(股)</th>
                <th className="text-right py-1">变动比例</th>
              </tr>
            </thead>
            <tbody>
              {rows.slice(0, 10).map((row, idx) => (
                <tr key={`${title}-${idx}`} className="border-b border-slate-800/70">
                  <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['HOLDER_RANK']))}</td>
                  <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['HOLDER_NAME']))}</td>
                  <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['SHARES_TYPE', 'HOLDER_TYPE']))}</td>
                  <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(row, ['HOLD_NUM', 'HOLD_SHARES']))}</td>
                  <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ratioKeys))}</td>
                  <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['HOLD_NUM_CHANGE', 'CHANGE_NUM']))}</td>
                  <td className="py-1 text-right text-slate-300">{formatPercentCell(findValue(row, ['CHANGE_RATIO', 'CHANGE_RATE']), true)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-3">
      {controllerInfo && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-1">实控人/控股股东</div>
          <div className="text-slate-200">
            {formatValue(findValue(controllerInfo, ['HOLDER_NAME']))}
            {findValue(controllerInfo, ['HOLD_RATIO']) !== undefined && (
              <span className="text-slate-400 ml-2">({formatPercentCell(findValue(controllerInfo, ['HOLD_RATIO']))})</span>
            )}
          </div>
        </div>
      )}

      {renderLatestMetrics('股东人数与筹码概览', latestNumbers, [
        { label: '截止日', keys: ['endDate', 'END_DATE'] },
        { label: '股东人数', keys: ['holderNum', 'HOLDER_TOTAL_NUM', 'HOLDER_NUM'] },
        { label: '较上期变化', keys: ['holderChangeRate', 'TOTAL_NUM_RATIO', 'HOLDER_NUM_RATIO'], format: 'signedPercent' },
        { label: '人均流通股', keys: ['avgFreeShares', 'avgHoldNum', 'AVG_FREE_SHARES', 'AVG_HOLD_NUM'] },
        { label: '人均流通股变化', keys: ['avgFreeSharesRatio', 'AVG_FREESHARES_RATIO'], format: 'signedPercent' },
        { label: '筹码集中度', keys: ['focusLevel', 'HOLD_FOCUS'] },
        { label: '股价', keys: ['price', 'PRICE'] },
        { label: '人均持股金额', keys: ['avgHoldAmt', 'AVG_HOLD_AMT'] },
        { label: '十大股东持股合计', keys: ['top10HoldRatio', 'HOLD_RATIO_TOTAL'], format: 'percent' },
        { label: '十大流通股东持股合计', keys: ['top10FreeHoldRatio', 'FREEHOLD_RATIO_TOTAL'], format: 'percent' },
      ])}

      {numberTrendRows.length > 0 && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">股东人数与股价比</div>
          <div className="overflow-x-auto">
            <table className="min-w-[1200px] w-full text-xs">
              <thead>
                <tr className="text-slate-500 border-b border-slate-700/60">
                  <th className="text-left py-1 pr-2">截止日</th>
                  <th className="text-right py-1 pr-2">股东人数(户)</th>
                  <th className="text-right py-1 pr-2">较上期变化</th>
                  <th className="text-right py-1 pr-2">人均流通股(股)</th>
                  <th className="text-right py-1 pr-2">人均流通股变化</th>
                  <th className="text-left py-1 pr-2">筹码集中度</th>
                  <th className="text-right py-1 pr-2">股价(元)</th>
                  <th className="text-right py-1 pr-2">人均持股金额(元)</th>
                  <th className="text-right py-1 pr-2">十大股东持股合计</th>
                  <th className="text-right py-1">十大流通股东持股合计</th>
                </tr>
              </thead>
              <tbody>
                {numberTrendRows.map((row, idx) => (
                  <tr key={`gdrs-${idx}`} className="border-b border-slate-800/70">
                    <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['END_DATE', 'endDate']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(row, ['HOLDER_TOTAL_NUM', 'holderNum', 'HOLDER_NUM']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ['TOTAL_NUM_RATIO', 'holderChangeRate', 'HOLDER_NUM_RATIO']), true)}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['AVG_FREE_SHARES', 'avgFreeShares', 'AVG_HOLD_NUM']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ['AVG_FREESHARES_RATIO', 'avgFreeSharesRatio']), true)}</td>
                    <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['HOLD_FOCUS', 'focusLevel']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['PRICE', 'price']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['AVG_HOLD_AMT', 'avgHoldAmt']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ['HOLD_RATIO_TOTAL', 'top10HoldRatio']))}</td>
                    <td className="py-1 text-right text-slate-300">{formatPercentCell(findValue(row, ['FREEHOLD_RATIO_TOTAL', 'top10FreeHoldRatio']))}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {renderTopHolderTable('十大股东', topShareholdersDisplay, ['HOLD_NUM_RATIO', 'HOLDER_RATIO'])}
      {renderTopHolderTable('十大流通股东', topFloatShareholders, ['FREE_HOLDNUM_RATIO', 'HOLD_NUM_RATIO'])}

      {(topHolderChanges.length > 0 || fallbackHolderChange.length > 0) && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">十大股东持股变动</div>
          <div className="overflow-x-auto">
            <table className="min-w-[1100px] w-full text-xs">
              <thead>
                <tr className="text-slate-500 border-b border-slate-700/60">
                  <th className="text-left py-1 pr-2">截止日</th>
                  <th className="text-left py-1 pr-2">名次</th>
                  <th className="text-left py-1 pr-2">股东名称</th>
                  <th className="text-left py-1 pr-2">股份类型</th>
                  <th className="text-right py-1 pr-2">持股数(股)</th>
                  <th className="text-right py-1 pr-2">占总股本</th>
                  <th className="text-right py-1 pr-2">增减(股)</th>
                  <th className="text-right py-1 pr-2">变动比例</th>
                  <th className="text-left py-1">变动原因</th>
                </tr>
              </thead>
              <tbody>
                {(topHolderChanges.length > 0 ? topHolderChanges : fallbackHolderChange).slice(0, 12).map((row, idx) => (
                  <tr key={`sdgdcgbd-${idx}`} className="border-b border-slate-800/70">
                    <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['END_DATE', 'endDate', 'NOTICE_DATE']))}</td>
                    <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['HOLDER_RANK']))}</td>
                    <td className="py-1 pr-2 text-slate-200">{formatValue(findValue(row, ['HOLDER_NAME', 'holderName']))}</td>
                    <td className="py-1 pr-2 text-slate-300">{formatValue(findValue(row, ['SHARES_TYPE', 'direction']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-200">{formatValue(findValue(row, ['HOLD_NUM', 'changeShares']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ['HOLD_NUM_RATIO', 'holdRatio', 'AFTER_CHANGE_RATE']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatValue(findValue(row, ['HOLD_CHANGE', 'CHANGE_NUM', 'changeShares']))}</td>
                    <td className="py-1 pr-2 text-right text-slate-300">{formatPercentCell(findValue(row, ['CHANGE_RATIO', 'changeRate']), true)}</td>
                    <td className="py-1 text-slate-300">{formatValue(findValue(row, ['CHANGE_REASON', 'direction']))}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {institutionSummaryRenderableRows.length > 0 && institutionSummaryMetrics.length > 0 && (
        <div className="rounded border border-slate-700/60 px-3 py-2">
          <div className="text-slate-400 mb-2">机构持仓统计</div>
          <div className="overflow-x-auto">
            <table className="w-full text-xs" style={{ minWidth: `${institutionSummaryMinWidth}px` }}>
              <thead>
                <tr className="text-slate-500 border-b border-slate-700/60">
                  <th className="text-left py-1 pr-2">指标</th>
                  {institutionSummaryRenderableRows.map((row, idx) => (
                    <th key={`jgcc-date-${idx}`} className="text-right py-1 pr-2">
                      {formatValue(findValue(row, ['REPORT_DATE']))}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {institutionSummaryMetrics.map(metric => (
                  <tr key={metric.label} className="border-b border-slate-800/70">
                    <td className="py-1 pr-2 text-slate-200 whitespace-nowrap">{metric.label}</td>
                    {institutionSummaryRenderableRows.map((row, idx) => {
                      const value = findValue(row, metric.keys);
                      return (
                        <td key={`${metric.label}-${idx}`} className="py-1 pr-2 text-right text-slate-300">
                          {metric.percent ? formatPercentCell(value) : formatValue(value)}
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};

const renderPledge = (pledge?: EquityPledge) => {
  if (!pledge?.records?.length) {
    return <div className="text-slate-500">暂无股权质押数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderLatestMetrics('最新质押概况', pledge.latest, [
        { label: '统计日', keys: ['tradeDate', 'TRADE_DATE'] },
        { label: '质押比例', keys: ['pledgeRatio', 'PLEDGE_RATIO'] },
        { label: '质押市值', keys: ['pledgeMarketCap', 'PLEDGE_MARKET_CAP'] },
        { label: '质押笔数', keys: ['pledgeDealNum', 'PLEDGE_DEAL_NUM'] },
        { label: '回购余额', keys: ['repurchaseBalance', 'REPURCHASE_BALANCE'] },
        { label: '行业', keys: ['industry', 'INDUSTRY'] },
        { label: '一年涨跌', keys: ['yearChangeRate', 'Y1_CLOSE_ADJCHRATE'] },
      ])}
      {renderEventList('质押历史', pledge.records, [
        'pledgeRatio',
        'pledgeMarketCap',
        'pledgeDealNum',
        'repurchaseBalance',
      ])}
    </div>
  );
};

const renderLockup = (lockup?: LockupRelease) => {
  if (!lockup?.records?.length) {
    return <div className="text-slate-500">暂无限售解禁数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderLatestMetrics('最近解禁', lockup.latest, [
        { label: '解禁日', keys: ['freeDate', 'FREE_DATE'] },
        { label: '解禁类型', keys: ['freeSharesType', 'FREE_SHARES_TYPE'] },
        { label: '解禁股数', keys: ['freeShares', 'FREE_SHARES'] },
        { label: '本次流通', keys: ['currentFreeShares', 'CURRENT_FREE_SHARES'] },
        { label: '流通比例', keys: ['freeRatio', 'FREE_RATIO'] },
        { label: '总股本比例', keys: ['totalRatio', 'TOTAL_RATIO'] },
        { label: '解禁市值', keys: ['liftMarketCap', 'LIFT_MARKET_CAP'] },
        { label: '解禁股东', keys: ['batchHolderNum', 'BATCH_HOLDER_NUM'] },
      ])}
      {renderEventList('解禁安排', lockup.records, [
        'freeSharesType',
        'freeShares',
        'freeRatio',
        'liftMarketCap',
      ])}
    </div>
  );
};

const renderHolderChange = (holderChange?: ShareholderChanges) => {
  if (!holderChange?.records?.length) {
    return <div className="text-slate-500">暂无股东增减持数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderLatestMetrics('最新增减持', holderChange.latest, [
        { label: '股东名称', keys: ['holderName', 'HOLDER_NAME'] },
        { label: '方向', keys: ['direction', 'DIRECTION'] },
        { label: '变动股数', keys: ['changeShares', 'CHANGE_NUM'] },
        { label: '变动比例', keys: ['changeRate', 'CHANGE_RATE'] },
        { label: '变动后比例', keys: ['afterChangeRate', 'AFTER_CHANGE_RATE'] },
        { label: '持股比例', keys: ['holdRatio', 'HOLD_RATIO'] },
        { label: '公告日', keys: ['noticeDate', 'NOTICE_DATE'] },
      ])}
      {renderEventList('增减持记录', holderChange.records, [
        'holderName',
        'direction',
        'changeShares',
        'changeRate',
        'afterChangeRate',
      ])}
    </div>
  );
};

const renderBuyback = (buyback?: StockBuyback) => {
  if (!buyback?.records?.length) {
    return <div className="text-slate-500">暂无股票回购数据</div>;
  }

  return (
    <div className="space-y-3">
      {renderLatestMetrics('最新回购进度', buyback.latest, [
        { label: '公告日', keys: ['noticeDate', 'DIM_DATE'] },
        { label: '回购进度', keys: ['progressLabel', 'REPURPROGRESS'] },
        { label: '回购目的', keys: ['objective', 'REPUROBJECTIVE'] },
        { label: '计划金额', keys: ['planAmountLower', 'planAmountUpper', 'REPURAMOUNTLIMIT'] },
        { label: '计划股数', keys: ['planSharesLower', 'planSharesUpper', 'REPURNUMCAP'] },
        { label: '已回购股数', keys: ['repurchasedShares', 'REPURNUM'] },
        { label: '已回购金额', keys: ['repurchasedAmount', 'REPURAMOUNT'] },
        { label: '更新日', keys: ['updateDate', 'UPD', 'UPDATEDATE'] },
      ])}
      {renderEventList('回购记录', buyback.records, [
        'progressLabel',
        'objective',
        'planAmountUpper',
        'repurchasedAmount',
        'repurchasedShares',
      ])}
    </div>
  );
};

const renderValuation = (valuation?: StockValuation, trend?: F10ValuationTrend, updatedAt?: string) => {
  const hasTrendData = Boolean(trend && (trend.pe?.length || trend.pb?.length || trend.ps?.length || trend.pcf?.length));
  if (!valuation && !hasTrendData) {
    return <div className="text-slate-500">暂无估值数据</div>;
  }

  const trendAsOf = (() => {
    const groups = [trend?.pe, trend?.pb, trend?.ps, trend?.pcf];
    for (const group of groups) {
      if (!group || group.length === 0) continue;
      const latest = group[group.length - 1];
      const date = findValue(latest, ['TRADE_DATE', 'REPORT_DATE', 'DATE']);
      const normalized = typeof date === 'string' ? extractDateOnly(date) : '';
      if (normalized) return normalized;
    }
    return '';
  })();

  const asOf = extractDateOnly(updatedAt || '') || trendAsOf;
  const valuationTitle = asOf ? `估值指标（截至 ${asOf}）` : '估值指标';

  return (
    <div className="space-y-3">
      {valuation &&
        renderLatestMetrics(valuationTitle, valuation as unknown as Record<string, any>, [
          { label: '最新价', keys: ['price'] },
          { label: 'PE(TTM)', keys: ['peTtm'] },
          { label: 'PB', keys: ['pb'] },
          { label: '总市值', keys: ['totalMarketCap'] },
          { label: '流通市值', keys: ['floatMarketCap'] },
          { label: '换手率', keys: ['turnoverRate'], format: 'percent' },
          { label: '振幅', keys: ['amplitude'], format: 'percent' },
          { label: '总股本', keys: ['totalShares'] },
          { label: '流通股', keys: ['floatShares'] },
        ])}
      {hasTrendData && renderValuationTrend(trend, false)}
    </div>
  );
};

type SimpleTableColumn = {
  label: string;
  keys: string[];
};

const renderSimpleTable = (
  title: string,
  rows: Record<string, any>[] | undefined,
  columns: SimpleTableColumn[],
  emptyText: string,
  limit: number = 10,
) => {
  if (!rows || rows.length === 0) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        {emptyText}
      </div>
    );
  }

  const displayRows = rows.slice(0, limit);
  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">{title}</div>
      <div className="overflow-x-auto">
        <table className="min-w-full text-xs">
          <thead>
            <tr className="text-slate-500 border-b border-slate-700/60">
              {columns.map(col => (
                <th key={col.label} className="text-left py-1 pr-3 whitespace-nowrap">
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {displayRows.map((row, idx) => (
              <tr key={`${title}-${idx}`} className="border-b border-slate-800/70">
                {columns.map(col => (
                  <td key={col.label} className="py-1 pr-3 text-slate-200 whitespace-nowrap">
                    {formatValue(findValue(row, col.keys))}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

const renderEventList = (title: string, items?: Record<string, any>[], preferKeys?: string[]) => {
  if (!items || items.length === 0) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        {title}: 暂无数据
      </div>
    );
  }

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">{title}</div>
      <div className="space-y-2">
        {items.slice(0, 4).map((item, idx) => {
          const formatEventDate = (value: any) => {
            if (value === undefined || value === null || value === '') return '无日期';
            const asNumber = Number(value);
            if (!Number.isNaN(asNumber) && asNumber > 1000000000) {
              const millis = asNumber > 100000000000 ? asNumber : asNumber * 1000;
              const date = new Date(millis);
              if (!Number.isNaN(date.getTime())) {
                const y = date.getFullYear();
                const m = String(date.getMonth() + 1).padStart(2, '0');
                const d = String(date.getDate()).padStart(2, '0');
                return `${y}-${m}-${d}`;
              }
            }
            const text = formatValue(value);
            return text === '--' ? '无日期' : text;
          };

          const dateValue = findValue(item, [
            'NOTICE_DATE',
            'REPORT_DATE',
            'APPOINT_DATE',
            'END_DATE',
            'TRADE_DATE',
            'FREE_DATE',
            'DIM_DATE',
            'START_DATE',
            'UPDATE_DATE',
            'noticeDate',
            'reportDate',
            'appointDate',
            'endDate',
            'tradeDate',
            'freeDate',
            'startDate',
            'updateDate',
            'showDateTime',
            'SHOWDATETIME',
            'publishDate',
            'PUBLISH_DATE',
            'publish_time',
            'publishTime',
            'PUBLISH_TIME',
            'display_time',
            'DISPLAY_TIME',
          ]);
          const typeValue = findValue(item, ['FORECAST_TYPE', 'REPORT_TYPE', 'NOTICE_TYPE']);
          const typeDisplay = formatValue(typeValue);
          return (
            <div key={idx} className="space-y-0.5">
              <div className="flex items-center justify-between gap-2 text-slate-200">
                <span>{formatEventDate(dateValue)}</span>
                {typeDisplay && typeDisplay !== '--' && typeDisplay !== '0' ? (
                  <span className="text-slate-500">{typeDisplay}</span>
                ) : (
                  <span className="text-slate-500"></span>
                )}
              </div>
              <div className="text-slate-500 line-clamp-2">{summarizeRecord(item, preferKeys)}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

const buildCompanyHighlights = (company: Record<string, any>) => {
  const info = findObject(company, ['CompanyInfo', 'companyInfo', 'COMPANYINFO', 'COMPANY_INFO', 'jbzl']);
  const security = findObject(company, ['SecurityInfo', 'securityInfo', 'SECURITYINFO', 'SECURITY_INFO', 'jbzl']);
  const sources = [info, security, company].filter(Boolean) as Record<string, any>[];

  const pick = (keys: string[]) => {
    for (const source of sources) {
      const value = findValue(source, keys);
      if (value !== undefined && value !== null && value !== '') {
        return value;
      }
    }
    return undefined;
  };

  const highlights: Array<{ label: string; value: string }> = [];
  const push = (label: string, value: any) => {
    if (value === undefined || value === null || value === '') return;
    highlights.push({ label, value: formatValue(value) });
  };

  push('公司全称', pick(['COMPANY_NAME', 'ORG_NAME', 'COMPANYNAME', 'FULL_NAME']));
  push('股票简称', pick(['SECURITY_NAME_ABBR', 'SECURITY_NAME', 'SECURITY_NAME_A']));
  push('所属行业', pick(['INDUSTRY', 'INDUSTRY_CSRC', 'INDUSTRY_NAME', 'EM2016', 'INDUSTRYCSRC1']));
  push('上市日期', pick(['LISTING_DATE', 'LISTINGDATE', 'LIST_DATE']));
  push('证券类型', pick(['SECURITY_TYPE', 'SECURITYTYPE']));
  push('交易所', pick(['TRADE_MARKET', 'TRADEMARKET']));
  push('注册资本', pick(['REG_CAPITAL', 'REGISTERED_CAPITAL', 'REGCAPITAL']));
  push('法人代表', pick(['LEGAL_PERSON', 'LEGAL_REPRESENTATIVE']));
  push('董事长', pick(['CHAIRMAN', 'CHAIRMAN_NAME']));
  push('员工人数', pick(['STAFF_NUM', 'EMPLOYEES', 'EMPLOYEE_NUM', 'EMP_NUM']));
  push('公司电话', pick(['ORG_TEL', 'TEL', 'PHONE']));
  push('公司邮箱', pick(['ORG_EMAIL', 'EMAIL']));
  push('公司网站', pick(['ORG_WEB', 'WEBSITE']));
  push('注册地址', pick(['REG_ADDRESS', 'ADDRESS']));
  push('办公地址', pick(['OFFICE_ADDRESS', 'OFFICE_ADDR']));
  push('证券代码', pick(['SECURITY_CODE', 'SECUCODE', 'TS_CODE']));

  if (highlights.length > 0) {
    return highlights.slice(0, 18);
  }

  const fallback = Object.entries(company)
    .filter(([, value]) => isPrimitive(value))
    .slice(0, 6)
    .map(([key, value]) => ({ label: key, value: formatValue(value) }));
  return fallback;
};

const pickCompanyText = (company: Record<string, any>, keys: string[]) => {
  const info = findObject(company, ['CompanyInfo', 'companyInfo', 'COMPANYINFO', 'COMPANY_INFO', 'jbzl']);
  const security = findObject(company, ['SecurityInfo', 'securityInfo', 'SECURITYINFO', 'SECURITY_INFO', 'jbzl']);
  const sources = [info, security, company].filter(Boolean) as Record<string, any>[];

  for (const source of sources) {
    const value = findValue(source, keys);
    if (value !== undefined && value !== null && value !== '') {
      return String(value).replace(/\s+/g, ' ').trim();
    }
  }
  return '';
};

const findObject = (obj: Record<string, any>, keys: string[]) => {
  for (const key of keys) {
    const value = obj[key];
    if (value) {
      if (Array.isArray(value) && value.length > 0 && typeof value[0] === 'object') {
        return value[0] as Record<string, any>;
      }
      if (typeof value === 'object') {
        return value as Record<string, any>;
      }
    }
    const lowerKey = Object.keys(obj).find(item => item.toLowerCase() === key.toLowerCase());
    if (lowerKey) {
      const lowerValue = obj[lowerKey];
      if (Array.isArray(lowerValue) && lowerValue.length > 0 && typeof lowerValue[0] === 'object') {
        return lowerValue[0] as Record<string, any>;
      }
      if (typeof lowerValue === 'object') {
        return lowerValue as Record<string, any>;
      }
    }
  }
  return null;
};

const findValue = (record: Record<string, any> | undefined, keys: string[]) => {
  if (!record) return undefined;
  const found = findValueInRecord(record, keys);
  if (found !== undefined && found !== null && found !== '') {
    return found;
  }
  const normalized = record.normalized;
  if (normalized && typeof normalized === 'object') {
    return findValueInRecord(normalized as Record<string, any>, keys);
  }
  return undefined;
};

const findValueInRecord = (record: Record<string, any>, keys: string[]) => {
  const recordKeys = Object.keys(record);
  for (const key of keys) {
    const direct = record[key];
    if (direct !== undefined && direct !== null && direct !== '') {
      return direct;
    }
    const match = recordKeys.find(item => item.toLowerCase() === key.toLowerCase());
    if (match && record[match] !== undefined && record[match] !== null && record[match] !== '') {
      return record[match];
    }
  }
  return undefined;
};

const summarizeRecord = (record: Record<string, any>, preferKeys?: string[]) => {
  if (!record) return '暂无数据';
  const entries: Array<[string, any]> = [];
  if (preferKeys && preferKeys.length > 0) {
    preferKeys.forEach(key => {
      const value = findValue(record, [key]);
      if (value !== undefined && value !== null && value !== '') {
        entries.push([key, value]);
      }
    });
  }
  const allowFallback = !preferKeys || preferKeys.length === 0 || entries.length === 0;
  if (allowFallback && entries.length < 3) {
    Object.entries(record).some(([key, value]) => {
      if (entries.length >= 3) return true;
      if (!isPrimitive(value)) return false;
      if (value === '' || value === null || value === undefined) return false;
      entries.push([key, value]);
      return false;
    });
  }

  if (entries.length === 0) return '暂无数据';
  return entries
    .slice(0, 3)
    .map(([key, value]) => `${mapKeyLabel(key)} ${formatValue(value)}`)
    .join(' · ');
};

const mapKeyLabel = (key: string) => {
  const map: Record<string, string> = {
    REPORT_DATE: '报告期',
    NOTICE_DATE: '公告日',
    END_DATE: '截止日',
    APPOINT_DATE: '预约日',
    EVENT_TYPE: '事件类型',
    SPECIFIC_EVENTTYPE: '具体事件',
    LEVEL1_CONTENT: '事件内容',
    LEVEL2_CONTENT: '事件详情',
    FORECAST_TYPE: '预告类型',
    CHANGE_RANGE: '变动区间',
    NETPROFIT: '净利润',
    TOTAL_OPERATE_INCOME: '营业收入',
    BASIC_EPS: '每股收益',
    ROE: '净资产收益率(ROE)',
    PLAN: '方案',
    PROGRESS: '进度',
    RECORD_DATE: '股权登记',
    EX_DATE: '除权日',
    PAY_DATE: '派息日',
    ISSUE_NUM: '发行数量',
    RAISE_FUNDS: '募资金额',
    ISSUE_PRICE: '发行价',
    ISSUE_WAY: '发行方式',
    YEAR: '年度',
    TOTAL_DIVIDEND: '分红总额',
    SEO_NUM: '增发数量',
    ALLOTMENT_NUM: '配股数量',
    IPO_NUM: 'IPO数量',
    REG_DATE: '注册日',
    LIST_DATE: '上市日',
    BUSINESS_SCOPE: '经营范围',
    REVIEW_CONTENT: '经营评述',
    ITEM_NAME: '项目',
    MAINOP_TYPE: '类型',
    INCOME: '收入',
    INCOME_RATIO: '收入占比',
    COST: '成本',
    COST_RATIO: '成本占比',
    PROFIT: '利润',
    PROFIT_RATIO: '利润占比',
    GROSS_MARGIN: '毛利率',
    RANK: '排名',
    HOLDER_NUM: '股东户数',
    HOLDER_NUM_CHANGE: '户数变动',
    HOLDER_NUM_RATIO: '变动幅度',
    AVG_HOLD_NUM: '户均持股',
    TOTAL_MARKET_CAP: '总市值',
    FLOAT_MARKET_CAP: '流通市值',
    TOTAL_A_SHARES: '总股本',
    ACCOUNTS_PAYABLE_TR: '应付账款周转率',
    BPSTZ: '每股净资产同比(BPS)',
    AVG_NET_PROFIT: '人均净利润',
    AVG_TOI: '人均营收',
    AVG_FREESHARES_RATIO: '户均持股比例',
    AVG_FREE_SHARES: '户均持股(股)',
    AVG_HOLD_AMT: '户均持股市值',
    CHANGE_REASON: '变动原因',
    TRADE_DATE: '统计日',
    INDICATOR_VALUE: '指标值',
    INDICATOR_NAME: '指标',
    PRICE: '价格',
    VOLUME: '成交量',
    AMOUNT: '成交额',
    F43: '最新价',
    F44: '最高价',
    F45: '最低价',
    F46: '今开',
    F47: '成交量',
    F48: '成交额',
    F57: '证券代码',
    F58: '证券简称',
    F60: '昨收',
    F84: '总手',
    F85: '总额',
    F107: '指标标记',
    F116: '总市值',
    F117: '流通市值',
    F162: '市盈率(动态)',
    F163: '市盈率(静态)',
    F164: '市盈率(TTM)',
    F167: '市净率',
    PE: '市盈率',
    PB: '市净率',
    PE_DYNAMIC: '市盈率(动态)',
    PE_STATIC: '市盈率(静态)',
    PE_TTM: '市盈率(TTM)',
    PE_1Y: '市盈率(1年)',
    PE_2Y: '市盈率(2年)',
    PE_3Y: '市盈率(3年)',
    PB_NEW_NOTICE: '市净率(最新)',
    PB_MRQ_REALTIME: '市净率(报告期)',
    PS: '市销率',
    PS_TTM: '市销率(TTM)',
    PCF: '市现率',
    PCF_TTM: '市现率(TTM)',
    PEG: '市盈增长比(PEG)',
    ROE_AVG: 'ROE(均值)',
    ROEPJ_L1: 'ROE(近1年)',
    ROEPJ_L2: 'ROE(近2年)',
    ROEPJ_L3: 'ROE(近3年)',
    XSJLL_AVG: '销售净利率(均值)',
    XSJLL_L1: '销售净利率(近1年)',
    XSJLL_L2: '销售净利率(近2年)',
    XSJLL_L3: '销售净利率(近3年)',
    TOAZZL_AVG: '总资产周转率(均值)',
    TOAZZL_L1: '总资产周转率(近1年)',
    TOAZZL_L2: '总资产周转率(近2年)',
    TOAZZL_L3: '总资产周转率(近3年)',
    QYCS_AVG: '权益乘数(均值)',
    QYCS_L1: '权益乘数(近1年)',
    QYCS_L2: '权益乘数(近2年)',
    QYCS_L3: '权益乘数(近3年)',
    YYSRTB: '营收同比',
    MGSYTB: '每股收益同比',
    JLRTB: '净利润同比',
    YYSR_3Y: '营收3年复合',
    MGSY_3Y: '每股收益3年复合',
    JLR_3Y: '净利润3年复合',
    PAIMING: '排名',
    SECURITY_CODE: '代码',
    SECURITY_NAME_ABBR: '名称',
    SECUCODE: '证券代码',
    SECURITY_NAME: '证券简称',
    EPSJB: '每股收益(基本)',
    EPSJB_PL: '每股收益(基本)',
    EPSKCJB: '每股收益(扣非)',
    EPSXS: '每股收益(稀释)',
    BPS: '每股净资产',
    BPS_PL: '每股净资产',
    MGZBGJ: '每股资本公积',
    MGZBGJJ: '每股资本公积',
    MGWFPLR: '每股未分配利润',
    MGJYXJJE: '每股经营现金流',
    PER_CAPITAL_RESERVE: '每股资本公积',
    PER_UNASSIGN_PROFIT: '每股未分配利润',
    PER_NETCASH: '每股经营现金流',
    TOTAL_SHARE: '总股本',
    FREE_SHARE: '流通股本',
    EQUITY_NEW_REPORT: '净资产(最新报告期)',
    ROEJQ: 'ROE(加权)',
    ROE_DILUTED: 'ROE(摊薄)',
    XSMLL: '销售毛利率',
    ZCFZL: '资产负债率',
    TOTAL_OPERATEINCOME: '营业收入',
    TOTAL_OPERATEINCOME_LAST: '营业收入(上期)',
    TOTAL_OPERATE_INCOME_LAST: '营业收入(上期)',
    PARENT_NETPROFIT: '归母净利润',
    KCFJCXSYJLR: '扣非净利润',
    TOTALOPERATEREVETZ: '营收同比',
    TOTALOPERATEREVETZ_LAST: '营收同比(上期)',
    YYZSRGDHBZC: '营收同比',
    YYZSRGDHBZC_LAST: '营收同比(上期)',
    PARENTNETPROFITTZ: '净利同比',
    PARENTNETPROFITTZ_LAST: '净利同比(上期)',
    ROEJQ_LAST: 'ROE(加权,上期)',
    NETPROFITRPHBZC: '净利同比',
    NETPROFITRPHBZC_LAST: '净利同比(上期)',
    PARENT_NETPROFIT_LAST: '归母净利润(上期)',
    KCFJCXSYJLR_LAST: '扣非净利润(上期)',
    KCFJCXSYJLRTZ_LAST: '扣非净利同比(上期)',
    KFJLRGDHBZC_LAST: '扣非净利环比(上期)',
    XSMLL_LAST: '销售毛利率(上期)',
    ZCFZL_LAST: '资产负债率(上期)',
    ASSET_DISPOSAL_INCOME: '资产处置收益',
    ASSET_DISPOSAL_INCOME_YOY: '资产处置收益同比',
    ASSET_IMPAIRMENT_INCOME: '资产减值损益',
    ASSET_IMPAIRMENT_INCOME_YOY: '资产减值损益同比',
    BASIC_EPS_YOY: '基本每股收益同比',
    CONTINUED_NETPROFIT: '持续经营净利润',
    CONTINUED_NETPROFIT_YOY: '持续经营净利润同比',
    CREDIT_IMPAIRMENT_INCOME: '信用减值损益',
    CREDIT_IMPAIRMENT_INCOME_YOY: '信用减值损益同比',
    DEDUCT_PARENT_NETPROFIT: '扣非归母净利润',
    DEDUCT_PARENT_NETPROFIT_YOY: '扣非归母净利润同比',
    DILUTED_EPS: '稀释每股收益',
    FORMERNAME: '曾用名',
    GROSS_PROFIT: '毛利润',
    DEDU_PARENT_PROFIT: '扣非归母净利润',
    DPNP_YOY_RATIO: '扣非归母净利同比',
    JROA: '净资产收益率',
    SEASON_LABEL: '季度标签',
    F600: '扩展行情600',
    F732: '扩展行情732',
    PLEDGE_RATIO: '质押比例',
    PLEDGE_MARKET_CAP: '质押市值',
    PLEDGE_DEAL_NUM: '质押笔数',
    REPURCHASE_BALANCE: '回购余额',
    INDUSTRY: '行业',
    YEAR_CHANGE_RATE: '一年涨跌',
    FREE_DATE: '解禁日',
    FREE_SHARES_TYPE: '解禁类型',
    FREE_SHARES: '解禁股数',
    FREE_RATIO: '解禁比例',
    CURRENT_FREE_SHARES: '本次流通',
    TOTAL_RATIO: '总股本比例',
    LIFT_MARKET_CAP: '解禁市值',
    BATCH_HOLDER_NUM: '解禁股东',
    HOLDER_NAME: '股东名称',
    DIRECTION: '方向',
    CHANGE_NUM: '变动股数',
    CHANGE_RATE: '变动比例',
    AFTER_CHANGE_RATE: '变动后比例',
    HOLD_RATIO: '持股比例',
    CHANGE_FREE_RATIO: '流通变动',
    START_DATE: '开始日',
    MARKET: '交易市场',
    THEME_NAME: '题材',
    KEY_CLASSIF_NAME: '题材分类',
    KEY_CLASSIF_CODE: '题材编码',
    IS_HISTORY: '是否历史题材',
    MAINPOINT: '要点',
    BOARD_NAME: '板块',
    BOARD_TYPE: '板块类型',
    BOARD_CODE: '板块代码',
    BOARD_RANK: '板块排名',
    BOARD_YIELD: '板块涨跌幅',
    BOARD_ZDF: '板块涨跌幅',
    BK_NAME: '板块',
    BOARD_SECUCODE: '板块证券代码',
    BOARD: '板块',
    SELECT_REASON: '入选理由',
    REASON: '入选理由',
    REASON_DESC: '入选理由',
    ENTRY_REASON: '入选理由',
    CHOOSE_REASON: '入选理由',
    LEADER_STOCK: '人气龙头',
    LEADING_STOCK: '人气龙头',
    DRAGON_STOCK: '人气龙头',
    HOT_STOCK: '人气龙头',
    HOT_STOCKS: '人气龙头',
    POPULAR_STOCK: '人气龙头',
    POPULAR_STOCKS: '人气龙头',
    HEAD_STOCK: '人气龙头',
    NEWS_TITLE: '资讯标题',
    ANNOUNCEMENT_TITLE: '公告标题',
    REPORT_TITLE: '研报标题',
    STOCK_NAME: '股票名称',
    STOCK_CODE: '股票代码',
    ORG_NAME: '机构',
    ORG_S_NAME: '机构简称',
    ORG_CODE: '机构代码',
    RESEARCHER: '研究员',
    INFO_CODE: '研报编码',
    INDV_INDU_NAME: '行业',
    INDUSTRY_NAME: '行业',
    RZYE: '融资余额',
    RQYE: '融券余额',
    BUY: '买入',
    SELL: '卖出',
    NET: '净额',
    EXPLANATION: '上榜原因',
    OPERATEDEPT_NAME: '营业部',
    BUY_AMT_REAL: '买入金额',
    BUY_RATIO: '买入占比',
    SELL_AMT_REAL: '卖出金额',
    SELL_RATIO: '卖出占比',
    BUY_BUY_TOTAL: '买入总额',
    BUY_RATIO_TOTAL: '总占比',
    DEAL_PRICE: '成交价',
    DEAL_VOLUME: '成交量',
    DEAL_AMT: '成交额',
    PREMIUM_RATIO: '溢价率',
    BUYER_NAME: '买方营业部',
    SELLER_NAME: '卖方营业部',
    FIN_BALANCE: '融资余额',
    FIN_BUY_AMT: '融资买入',
    FIN_REPAY_AMT: '融资偿还',
    LOAN_BALANCE: '融券余额',
    LOAN_SELL_VOL: '融券卖出',
    LOAN_REPAY_VOL: '融券偿还',
    TOTAL_OPERATE_INCOME_YOY: '营收同比',
    NETPROFIT_YOY: '净利同比',
    DEDUCT_NETPROFIT_YOY: '扣非同比',
    GROSS_PROFIT_RATIO: '毛利率',
    NET_PROFIT_RATIO: '净利率',
    SALES_NETRATIO: '销售净利率',
    ROA: '总资产收益率(ROA)',
    AVG_PRICE: '均价',
    CLOSE_PRICE: '收盘价',
    PROGRESSLABEL: '回购进度',
    REPURAMOUNT: '回购金额',
    PROGRESS_LABEL: '回购进度',
    OBJECTIVE: '回购目的',
    PLAN_PRICE_LOWER: '计划价下限',
    PLAN_PRICE_UPPER: '计划价上限',
    PLAN_SHARES_LOWER: '计划股数下限',
    PLAN_SHARES_UPPER: '计划股数上限',
    PLAN_AMOUNT_LOWER: '计划金额下限',
    PLAN_AMOUNT_UPPER: '计划金额上限',
    REPURCHASED_SHARES: '已回购股数',
    REPURCHASED_AMOUNT: '已回购金额',
    REPURCHASED_PRICE_LOW: '回购价下限',
    REPURCHASED_PRICE_HIGH: '回购价上限',
    ADVANCED_DATE: '回购进展日',
    UPDATE_DATE: '更新日',
    NOTICE_DATE_LOCAL: '公告日',
    TURNOVER_RATE: '换手率',
    AMPLITUDE: '振幅',
    TOTAL_SHARES: '总股本',
    FLOAT_SHARES: '流通股',
    ADD_AMP_UPPER: '增幅上限',
    ADD_AMP_LOWER: '增幅下限',
    TITLE: '标题',
    SUMMARY: '摘要',
    CONTENT: '内容',
    NAME: '名称',
    REPORT_TYPE: '报告类型',
    REPORT_TYPE_NAME: '报告类型',
    REPORT_DATE_NAME: '报告期',
    REPORT_YEAR: '报告年度',
    NOTICE_TYPE: '公告类型',
    ART_CODE: '公告编号',
    DISPLAY_TIME: '展示时间',
    PUBLISH_DATE: '发布日期',
    PUBLISH_TIME: '发布时间',
    SOURCE: '来源',
    RATING: '评级',
    RATING_CHANGE: '评级变化',
    EM_RATING_NAME: '东财评级',
    EM_RATING_CODE: '东财评级代码',
    S_RATING_NAME: '评级名称',
    AIM_PRICE: '目标价',
    INDU_OLD_INDUSTRY_NAME: '行业',
    INDU_OLD_INDUSTRY_CODE: '行业代码',
    ORG_NAME_ABBR: '机构简称',
    SECURITY_TYPE: '证券类型',
    SECURITY_TYPE_WEB: '证券类型',
    SECURITY_TYPE_CODE: '证券类型代码',
    SECURITY_INNER_CODE: '内部代码',
    FORECAST_CONTENT: '预告内容',
    PREDICT_CONTENT: '预告内容',
    FORECAST_STATE: '预告状态',
    PREDICT_TYPE: '预告类型',
    PREDICT_FINANCE: '预告口径',
    PREDICT_AMT_LOWER: '预测净利下限',
    PREDICT_AMT_UPPER: '预测净利上限',
    PREDICT_RATIO_LOWER: '预测增幅下限',
    PREDICT_RATIO_UPPER: '预测增幅上限',
    PREDICT_HBMEAN: '预测增幅均值',
    FORECAST_JZ: '预测净利润',
    PREYEAR_SAME_PERIOD: '上年同期',
    CHANGE_REASON_EXPLAIN: '变动原因',
    APPOINT_PUBLISH_DATE: '预约披露日',
    FIRST_APPOINT_DATE: '首次预约日',
    ACTUAL_PUBLISH_DATE: '实际披露日',
    RESIDUAL_DAYS: '剩余天数',
    FIRST_CHANGE_DATE: '变更日1',
    SECOND_CHANGE_DATE: '变更日2',
    THIRD_CHANGE_DATE: '变更日3',
    YSTZ: '营收同比',
    JLRTBZCL: '净利同比',
    WEIGHTAVG_ROE: '加权ROE',
    PARENT_BVPS: '每股净资产',
    QDATE: '报告期',
    DATATYPE: '数据类型',
    PUBLISHNAME: '行业',
    EPS1: '每股收益预测1(EPS)',
    EPS2: '每股收益预测2(EPS)',
    EPS3: '每股收益预测3(EPS)',
    EPS4: '每股收益预测4(EPS)',
    PREDICT_THIS_YEAR_EPS: '当年EPS预测',
    PREDICT_NEXT_YEAR_EPS: '次年EPS预测',
    PREDICT_THIS_YEAR_PE: '当年PE预测',
    PREDICT_NEXT_YEAR_PE: '次年PE预测',
    PE1: '市盈率预测1(PE)',
    PE2: '市盈率预测2(PE)',
    PE3: '市盈率预测3(PE)',
    PE4: '市盈率预测4(PE)',
    YEAR1: '预测年度1',
    YEAR2: '预测年度2',
    YEAR3: '预测年度3',
    YEAR4: '预测年度4',
    YEAR_MARK1: '年度标记1',
    YEAR_MARK2: '年度标记2',
    YEAR_MARK3: '年度标记3',
    YEAR_MARK4: '年度标记4',
    EPS_BASIC: '每股收益',
    EPSJBTZ: '每股收益同比',
    MGJYXJJETZ: '每股经营现金流同比',
    MGWFPLRTZ: '每股未分配利润同比',
    MGZBGJTZ: '每股资本公积同比',
    ROEJQTZ: 'ROE同比',
    ZCFZLTZ: '资产负债率同比',
    XSMLL_TB: '毛利率同比',
    KCFJCXSYJLRTZ: '扣非净利同比',
    KFJLRGDHBZC: '扣非净利环比',
    TOTALOPERATEREVE: '营业收入',
    PARENTNETPROFIT: '归母净利润',
    NETCASH_OPERATE: '经营现金流',
    NETCASH_INVEST: '投资现金流',
    NETCASH_FINANCE: '筹资现金流',
    NET_CASH_OPERATE: '经营现金流',
    NET_CASH_INVEST: '投资现金流',
    NET_CASH_FINANCE: '筹资现金流',
    NET_OPERATE_CASH: '经营现金流',
    NET_MARGIN: '净利率',
    GROSS_SALES_RATIO: '毛利率',
    ROE_WEIGHTED: 'ROE(加权)',
    WEIGHTED_ROE: 'ROE(加权)',
    ASSET_TOTAL: '总资产',
    TOTAL_ASSETS: '总资产',
    LIABILITY_TOTAL: '总负债',
    TOTAL_LIABILITY: '总负债',
    OWNER_EQUITY: '净资产',
    TOTAL_EQUITY: '净资产',
    EQUITY_TOTAL: '净资产',
    ASSET_LIAB_RATIO: '资产负债率',
    DEBT_RATIO: '资产负债率',
    OPERATE_INCOME: '营业收入',
    REVENUE: '营业收入',
    NET_PROFIT: '净利润',
    DEDUCTED_NET_PROFIT: '扣非净利润',
    DEDUCT_NETPROFIT: '扣非净利润',
    MAIN_BUSINESS_INCOME: '主营收入',
    MAIN_BUSINESS_COST: '主营成本',
    MAIN_BUSINESS_RPOFIT: '主营利润',
    MBI_RATIO: '收入占比',
    MBC_RATIO: '成本占比',
    MBR_RATIO: '利润占比',
    GROSS_RPOFIT_RATIO: '毛利率',
    BUSINESS_REVIEW: '经营评述',
    MAIN_BUSINESS: '主营业务',
    ASSIGN_PROGRESS: '分红进度',
    IMPL_PLAN_PROFILE: '分红方案',
    EQUITY_RECORD_DATE: '股权登记日',
    EX_DIVIDEND_DATE: '除权除息日',
    PAY_CASH_DATE: '派息日',
    NET_RAISE_FUNDS: '募资净额',
    ISSUE_WAY_EXPLAIN: '发行方式',
    STATISTICS_YEAR: '统计年度',
    HOLDER_TOTAL_NUM: '股东总户数',
    TOTAL_NUM_RATIO: '户数变动幅度',
    HOLD_FOCUS: '持股集中度',
    HOLD_RATIO_TOTAL: '总持股比例',
    FREEHOLD_RATIO_TOTAL: '流通持股比例',
    HOLDER_CHANGE: '户数变动',
    HOLDER_RATIO: '持股比例',
    HOLDER_SHARES: '持股数量',
    HOLD_SHARES: '持股数量',
    HOLD_NUM: '持股数量',
    HOLDPCT: '持股比例',
    HOLDERNAME: '股东名称',
    HOLDER: '股东',
    CHANGE_SHARES: '变动股数',
    HOLDER_CHANGE_RATE: '变动幅度',
    TYPE: '类型',
    BUYER_CODE: '买方编号',
    SELLER_CODE: '卖方编号',
    TRADE_UNIT: '交易单位',
    CHANGE_RATE_1DAYS: '近1日涨跌',
    CHANGE_RATE_5DAYS: '近5日涨跌',
    DAILY_RANK: '当日排名',
    DISCOUNT_TURNOVER: '折价成交额',
    PREMIUM_TURNOVER: '溢价成交额',
    TRADE_MARKET_OLD: '交易市场',
    UNLIMITED_A_SHARES: '流通A股',
    CHANGE_PERCENT: '涨跌幅',
    PCT_CHANGE: '涨跌幅',
    RISE: '涨跌幅',
    ZDF: '涨跌幅',
    PERCENT: '涨跌幅',
    IS_PRECISE: '精确匹配',
    IS_PRECISE_MATCH: '精确匹配',
    CONCEPT_NAME: '概念',
    SECURITY_NAME_A: '证券简称',
    TRADE_MARKET: '交易市场',
    KEYWORD: '关键词',
    KEY_CLASSIF: '题材分类',
    MAINPOINT_CONTENT: '要点内容',
    IS_POINT: '重点标记',
    EVENT_NAME: '事件',
    EVENT_DESC: '事件说明',
    FORECAST: '预测',
    MAIN_NET: '主力净流入',
    LARGE_NET: '大单净流入',
    MEDIUM_NET: '中单净流入',
    SMALL_NET: '小单净流入',
    SUPER_NET: '超大单净流入',
    MAIN_RATIO: '主力净占比',
    LARGE_RATIO: '大单净占比',
    MEDIUM_RATIO: '中单净占比',
    SMALL_RATIO: '小单净占比',
    SUPER_RATIO: '超大单净占比',
    STAFF_NUM: '员工人数',
    TAXRATE: '实际税率',
    TOAZZL: '总资产周转率',
    XSJLL: '销售净利率',
    XJLLB: '现金流量比率',
    XSJXLYYSR: '销售净现金流/营业收入',
    SS_OI: '销售费用率',
    SS_TA: '销售费用/总资产',
    ROEKCJQ: '扣非ROE(加权)',
    ROIC: '投入资本回报率',
    ROICTZ: '投入资本回报率同比',
    SD: '速动比率',
    BLDKBBL: '不良贷款比率',
    CAPITAL_LEVERAGE_RATIO: '资本杠杆率',
    CAPITAL_PROVISIONS_SUM: '资本公积合计',
    CASH_RATIO: '现金比率',
    CA_TA: '流动资产/总资产',
    CHZZL: '存货周转率',
    CHZZTS: '存货周转天数',
    COMPENSATE_EXPENSE: '赔付支出',
    CQBL: '长期负债比率',
    CURRENCY: '币种',
    CURRENT_ASSET_TR: '流动资产周转率',
    DJD_DEDUCTDPNP_QOQ: '单季度扣非净利环比',
    DJD_DEDUCTDPNP_YOY: '单季度扣非净利同比',
    DJD_DPNP_QOQ: '单季度净利环比',
    DJD_DPNP_YOY: '单季度净利同比',
    DJD_TOI_QOQ: '单季度营收环比',
    DJD_TOI_YOY: '单季度营收同比',
    EARNED_PREMIUM: '已赚保费',
    FCFF_BACK: '企业自由现金流(回溯)',
    FCFF_FORWARD: '企业自由现金流(前瞻)',
    FC_LIABILITIES: '负债总额',
    FIRST_ADEQUACY_RATIO: '一级资本充足率',
    FIXED_ASSET_TR: '固定资产周转率',
    GROSSLOANS: '贷款总额',
    GUARD_SPEED_RATIO: '速动比率',
    HXYJBCZL: '核心一级资本充足率',
    INTEREST_COVERAGE_RATIO: '利息保障倍数',
    INTEREST_DEBT_RATIO: '有息负债率',
    JJYWFXZB: '基金业务分析指标',
    JYXJLYYSR: '经营现金流/营业收入',
    JZB: '净值比',
    JZBJZC: '净资本/净资产',
    JZC: '净资产',
    LD: '流动比率',
    LIABILITY: '负债',
    LIQUIDATION_RATIO: '清算比率',
    LIQUIDITY_COVERAGE_RATIO: '流动性覆盖率',
    LOAN_ADVANCES: '发放贷款及垫款',
    LOAN_PROVISION_RATIO: '贷款拨备率',
    LTDRR: '贷款拨备覆盖率',
    MLR: '毛利润',
    NBV_LIFE: '寿险新业务价值',
    NBV_RATE: '新业务价值率',
    NCA_TA: '非流动资产/总资产',
    NCO_FIXED: '固定资产净现金流',
    NCO_NETPROFIT: '净利润现金含量',
    NCO_OP: '经营活动净现金流',
    NET_ASSETS_LIABILITIES: '净资产/负债',
    NET_CAPITAL_LIABILITIES: '净资本/负债',
    NET_FUNDING_RATIO: '净稳定资金率',
    NET_INTEREST_MARGIN: '净息差',
    NET_INTEREST_SPREAD: '净利差',
    NET_ROI: '净投资收益率',
    NEWCAPITALADER: '新资本充足率',
    NHJZ_CURRENT_AMT: '年化净值(本期)',
    NONPERLOAN: '不良贷款',
    NON_PERFORMING_LOAN: '不良贷款',
    NZBJE: '内在资本净额',
    OPERATE_CYCLE: '营业周期',
    ORG_TYPE: '机构类型',
    OVERDUE_LOANS: '逾期贷款',
    PAYABLE_TDAYS: '应付账款周转天数',
    PER_EBIT: '每股息税前利润',
    PER_OI: '每股营业收入',
    PER_TOI: '每股总营收',
    PREPAID_ACCOUNTS_RATIO: '预付账款占比',
    PREPAID_ACCOUNTS_TDAYS: '预付账款周转天数',
    PROPRIETARY_CAPITAL: '自有资本',
    QYCS: '权益乘数',
    REVENUE_RATIO: '收入占比',
    RISK_COVERAGE: '风险覆盖率',
    RZRQYWFXZB: '融资融券业务分析指标',
    SOLVENCY_AR: '偿付能力充足率',
    SURRENDER_RATE_LIFE: '寿险退保率',
    TOTALDEPOSITS: '存款总额',
    TOTAL_ROI: '总资产报酬率',
    YSZKYYSR: '应收账款/营业收入',
    YSZKZZL: '应收账款周转率',
    YSZKZZTS: '应收账款周转天数',
    YYFXZB: '营运分析指标',
    ZQCXYWFXZB: '证券承销业务分析指标',
    ZQZYYWFXZB: '证券自营业务分析指标',
    ZYGDSYLZQJZB: '自有固收类证券净值比',
    ZYGPGMJZC: '自营股票规模/净资产',
    ZZCJLL: '总资产净利率',
    ZZCJLLTZ: '总资产净利率同比',
    ZZCZZTS: '总资产周转天数',
  };
  const normalized = normalizeFieldKey(key);
  if (map[normalized]) return map[normalized];
  if (normalized.endsWith('_LAST')) {
    const base = normalized.slice(0, -5);
    const baseLabel = map[base];
    if (baseLabel) return `${baseLabel}(上期)`;
  }
  if (/^F\d+$/.test(normalized)) return `行情字段${normalized.slice(1)}`;
  const mixedLabel = localizeMixedFieldLabel(key);
  if (mixedLabel) return mixedLabel;
  const autoLabel = autoGenerateChineseFieldLabel(normalized);
  if (autoLabel) return autoLabel;
  if (/[A-Za-z]/.test(key)) return '未映射指标';
  return key;
};

function localizeMixedFieldLabel(key: string): string {
  if (!key || !/[A-Za-z]/.test(key)) return '';
  let result = key;

  const replacements: Array<[RegExp, string]> = [
    [/FINANCE/gi, '筹资'],
    [/INVEST/gi, '投资'],
    [/INVENTORY/gi, '存货'],
    [/NETCASH/gi, '净现金流'],
    [/INFLOW/gi, '流入'],
    [/OUTFLOW/gi, '流出'],
    [/AMORTIZE/gi, '摊销'],
    [/IA/gi, '无形资产'],
    [/LPE/gi, '长期待摊费用'],
    [/PAY/gi, '支付'],
    [/NETPROFIT/gi, '净利润'],
    [/PARENT/gi, '归母'],
    [/COMPREHENSIVE/gi, '综合'],
    [/COMPRE/gi, '综合'],
    [/TCI/gi, '综合收益'],
    [/RESEARCH/gi, '研发'],
    [/SALE/gi, '销售'],
    [/OTHER/gi, '其他'],
    [/OPINION/gi, '意见'],
    [/TYPE/gi, '类型'],
    [/BALANCE/gi, '差额'],
    [/ADD/gi, '附加'],
    [/REDUCE/gi, '减少'],
    [/TAX/gi, '税'],
    [/INCOME/gi, '收益'],
    [/PROFIT/gi, '利润'],
    [/EXPENSE/gi, '费用'],
    [/YOY/gi, '同比'],
    [/QOQ/gi, '环比'],
    [/_+/g, ''],
  ];

  replacements.forEach(([pattern, value]) => {
    result = result.replace(pattern, value);
  });
  result = result.replace(/[A-Za-z]+/g, '项');
  result = result.replace(/\s+/g, '');
  result = result.replace(/项+/g, '项');
  result = result.replace(/项同比/g, '同比');
  result = result.replace(/项环比/g, '环比');
  result = result.replace(/^项+|项+$/g, '');

  if (result === key) return '';
  if (/[A-Za-z]/.test(result)) return '';
  return result;
}

function autoGenerateChineseFieldLabel(normalized: string): string {
  if (!normalized || !/^[A-Z0-9_]+$/.test(normalized)) return '';
  if (!normalized.includes('_')) {
    if (/^[A-Z]+$/.test(normalized)) return '未映射指标';
    return '';
  }

  const tokenMap: Record<string, string> = {
    ASSET: '资产',
    LIABILITY: '负债',
    EQUITY: '权益',
    INCOME: '收益',
    COST: '成本',
    PROFIT: '利润',
    LOSS: '损失',
    NET: '净',
    GROSS: '毛',
    OPERATE: '经营',
    OPERATING: '经营',
    TOTAL: '总',
    PARENT: '归母',
    DEDUCT: '扣非',
    BASIC: '基本',
    DILUTED: '稀释',
    EPS: '每股收益',
    CASH: '现金',
    FLOW: '流量',
    CREDIT: '信用',
    IMPAIRMENT: '减值',
    DISPOSAL: '处置',
    CONTINUED: '持续经营',
    YOY: '同比',
    QOQ: '环比',
    RATIO: '比率',
    RATE: '比率',
    MARGIN: '率',
    REVENUE: '营收',
    SHARE: '股本',
    SHARES: '股本',
    CURRENT: '本期',
    NON: '非',
    TAX: '税',
    EXPENSE: '费用',
    TURNOVER: '周转',
    CAPITAL: '资本',
    RESERVE: '公积',
    VALUE: '值',
    RETURN: '回报',
    OPINION: '意见',
    OTHER: '其他',
    RESEARCH: '研发',
    SALE: '销售',
    COMPRE: '综合',
    TCI: '综合收益',
    FINANCE: '筹资',
    INVEST: '投资',
    INVENTORY: '存货',
    NETCASH: '净现金流',
    INFLOW: '流入',
    OUTFLOW: '流出',
    AMORTIZE: '摊销',
    IA: '无形资产',
    LPE: '长期待摊费用',
    PAY: '支付',
    REDUCE: '减少',
    BALANCE: '差额',
    ADD: '附加',
    DEBT: '债务',
    INTEREST: '利息',
    PER: '每股',
  };

  const parts = normalized.split('_').filter(Boolean);
  if (parts.length < 2) return '';

  const translated = parts.map(part => {
    if (tokenMap[part]) return tokenMap[part];
    if (/^\d+$/.test(part)) return part;
    return '项';
  });

  const label = translated.join('').replace(/项+/g, '项').replace(/^项+|项+$/g, '');
  if (!label || label === normalized) return '';
  return label;
}

const normalizeFieldKey = (key: string) => {
  if (!key) return '';
  const withUnderscore = key.replace(/([a-z0-9])([A-Z])/g, '$1_$2');
  return withUnderscore.replace(/[^a-zA-Z0-9_]/g, '_').toUpperCase();
};

const getLineValue = (line: string[], idx: number) => {
  if (!line || idx < 0 || idx >= line.length) return undefined;
  return line[idx];
};

const formatSignedNumber = (value?: string, percent?: boolean) => {
  if (value === undefined || value === null || value === '') return '--';
  const num = Number(value);
  if (Number.isNaN(num)) return value;
  const formatted = percent ? `${num.toFixed(2)}%` : formatNumber(num);
  return num > 0 ? `+${formatted}` : formatted;
};

const formatMetricValue = (value: any, format?: MetricFormat) => {
  if (value === undefined || value === null || value === '') return '--';
  const num = typeof value === 'number' ? value : Number(value);
  if (format === 'percent') {
    if (!Number.isNaN(num)) {
      return `${num.toFixed(2)}%`;
    }
    return formatValue(value);
  }
  if (format === 'signed') {
    if (!Number.isNaN(num)) {
      const base = formatNumber(num);
      return num > 0 ? `+${base}` : base;
    }
    return formatValue(value);
  }
  if (format === 'signedPercent') {
    if (!Number.isNaN(num)) {
      const sign = num > 0 ? '+' : '';
      return `${sign}${num.toFixed(2)}%`;
    }
    return formatValue(value);
  }
  return formatValue(value);
};

const extractDateOnly = (value: string) => {
  const trimmed = value.trim();
  const match = trimmed.match(/^(\d{4})[/-](\d{1,2})[/-](\d{1,2})(?:[ T].*)?$/);
  if (!match) return '';
  const year = match[1];
  const month = match[2].padStart(2, '0');
  const day = match[3].padStart(2, '0');
  return `${year}-${month}-${day}`;
};

const formatValue = (value: any) => {
  if (value === undefined || value === null || value === '') return '--';
  if (typeof value === 'number') return formatNumber(value);
  if (typeof value === 'string') {
    const trimmed = value.trim();
    const dateOnly = extractDateOnly(trimmed);
    if (dateOnly) return dateOnly;
    const translated = translateEnumValue(trimmed);
    if (translated) return translated;
    const num = Number(trimmed);
    if (!Number.isNaN(num) && trimmed !== '') {
      return formatNumber(num);
    }
    return trimmed;
  }
  if (typeof value === 'boolean') return value ? '是' : '否';
  if (Array.isArray(value)) return value.join(', ');
  return String(value);
};

const translateEnumValue = (value: string): string | null => {
  const normalized = value.replace(/\s+/g, '_').toUpperCase();
  const map: Record<string, string> = {
    BUY: '买入',
    SELL: '卖出',
    NET: '净额',
    HOLD: '持有',
    INCREASE: '增持',
    DECREASE: '减持',
    TRADING: '交易中',
    PRE_MARKET: '开盘前',
    AFTER_HOURS: '收盘后',
    LUNCH_BREAK: '午间休市',
    ANNUAL_REPORT: '年报',
    SEMI_ANNUAL_REPORT: '中报',
    Q1_REPORT: '一季报',
    Q3_REPORT: '三季报',
    NONE: '无',
  };
  return map[normalized] || null;
};

const formatNumber = (num: number) => {
  const abs = Math.abs(num);
  if (abs >= 100000000) return `${(num / 100000000).toFixed(2)}亿`;
  if (abs >= 10000) return `${(num / 10000).toFixed(2)}万`;
  if (abs < 1 && abs > 0) return num.toFixed(4);
  if (Number.isInteger(num)) return num.toString();
  return num.toFixed(2);
};

type MetricFormat = 'percent' | 'signed' | 'signedPercent';
type MetricField = { label: string; keys: string[]; format?: MetricFormat };

const renderLatestMetrics = (
  title: string,
  record: Record<string, any> | undefined,
  fields: MetricField[],
) => {
  if (!record) return null;
  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">{title}</div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-slate-200">
        {fields.map(field => {
          const value = findValue(record, field.keys);
          if (value === undefined || value === null || value === '') {
            return null;
          }
          return (
            <div key={field.label} className="flex justify-between gap-2">
              <span className="text-slate-500">{field.label}</span>
              <span className="text-right">{formatMetricValue(value, field.format)}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
};

const renderRecordGrid = (
  title: string,
  record: Record<string, any> | undefined,
  preferredKeys?: string[],
  limit: number = 8,
) => {
  if (!record) return null;
  const entries: Array<[string, any]> = [];

  if (preferredKeys && preferredKeys.length > 0) {
    preferredKeys.forEach(key => {
      const value = findValue(record, [key]);
      if (value !== undefined && value !== null && value !== '') {
        entries.push([key, value]);
      }
    });
  }

  const allowFallback = !preferredKeys || preferredKeys.length === 0 || entries.length === 0;
  if (allowFallback && entries.length < limit) {
    Object.entries(record).some(([key, value]) => {
      if (entries.length >= limit) return true;
      if (!isPrimitive(value)) return false;
      if (value === '' || value === null || value === undefined) return false;
      entries.push([key, value]);
      return false;
    });
  }

  if (entries.length === 0) {
    return (
      <div className="rounded border border-slate-700/60 px-3 py-2 text-slate-500">
        {title}: 暂无数据
      </div>
    );
  }

  return (
    <div className="rounded border border-slate-700/60 px-3 py-2">
      <div className="text-slate-400 mb-2">{title}</div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-slate-200">
        {entries.slice(0, limit).map(([key, value]) => (
          <div key={key} className="flex justify-between gap-2">
            <span className="text-slate-500">{mapKeyLabel(key)}</span>
            <span className="text-right">{formatValue(value)}</span>
          </div>
        ))}
      </div>
    </div>
  );
};

const isPrimitive = (value: any) => {
  return ['string', 'number', 'boolean'].includes(typeof value);
};

const mergeRecords = (...records: Array<Record<string, any> | undefined>) => {
  const merged: Record<string, any> = {};
  records.forEach(record => {
    if (record && typeof record === 'object') {
      Object.assign(merged, record);
    }
  });
  return Object.keys(merged).length > 0 ? merged : undefined;
};

export { F10Panel };



