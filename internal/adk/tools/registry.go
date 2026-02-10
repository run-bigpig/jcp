package tools

import (
	"github.com/run-bigpig/jcp/internal/services"
	"github.com/run-bigpig/jcp/internal/services/hottrend"

	"google.golang.org/adk/tool"
)

// ToolInfo 工具信息
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Registry 工具注册中心
type Registry struct {
	marketService         *services.MarketService
	newsService           *services.NewsService
	configService         *services.ConfigService
	researchReportService *services.ResearchReportService
	hotTrendService       *hottrend.HotTrendService
	longHuBangService     *services.LongHuBangService
	tools                 map[string]tool.Tool
	toolInfos             map[string]ToolInfo // 工具信息映射
}

// NewRegistry 创建工具注册中心
func NewRegistry(
	marketService *services.MarketService,
	newsService *services.NewsService,
	configService *services.ConfigService,
	researchReportService *services.ResearchReportService,
	hotTrendService *hottrend.HotTrendService,
	longHuBangService *services.LongHuBangService,
) *Registry {
	r := &Registry{
		marketService:         marketService,
		newsService:           newsService,
		configService:         configService,
		researchReportService: researchReportService,
		hotTrendService:       hotTrendService,
		longHuBangService:     longHuBangService,
		tools:                 make(map[string]tool.Tool),
		toolInfos:             make(map[string]ToolInfo),
	}
	r.registerAllTools()
	return r
}

// registerAllTools 注册所有工具
func (r *Registry) registerAllTools() {
	// 注册股票实时数据工具
	r.registerTool("get_stock_realtime", "获取股票实时行情数据，包括当前价格、涨跌幅、开盘价、最高价、最低价、成交量等", r.createStockRealtimeTool)

	// 注册K线数据工具
	r.registerTool("get_kline_data", "获取股票K线数据，支持5分钟线、日线、周线、月线", r.createKLineTool)

	// 注册盘口数据工具
	r.registerTool("get_orderbook", "获取股票五档盘口数据，包括买卖五档价格和数量", r.createOrderBookTool)

	// 注册快讯工具
	r.registerTool("get_news", "获取最新财经快讯，来源于财联社", r.createNewsTool)

	// 注册股票搜索工具
	r.registerTool("search_stocks", "搜索股票，根据关键词搜索股票代码和名称", r.createSearchStocksTool)

	// 注册研报查询工具
	r.registerTool("get_research_report", "获取个股研报列表，包括券商评级、研究员、预测EPS/PE等信息", r.createResearchReportTool)

	// 注册研报内容查询工具
	r.registerTool("get_report_content", "获取研报正文内容，需要先通过 get_research_report 获取 infoCode", r.createReportContentTool)

	// 注册舆情热点工具
	r.registerTool("get_hottrend", "获取全网舆情热点，支持微博、知乎、B站、百度、抖音、头条等平台的实时热搜榜单", r.createHotTrendTool)

	// 注册龙虎榜工具
	r.registerTool("get_longhubang", "获取A股龙虎榜数据，包括上榜股票、净买入金额、买卖金额、上榜原因等信息", r.createLongHuBangTool)

	// 注册龙虎榜营业部明细工具
	r.registerTool("get_longhubang_detail", "获取个股龙虎榜营业部买卖明细，需要提供股票代码和交易日期", r.createLongHuBangDetailTool)
}

// registerTool 注册单个工具并保存信息
func (r *Registry) registerTool(name, description string, creator func() (tool.Tool, error)) {
	if t, err := creator(); err == nil {
		r.tools[name] = t
		r.toolInfos[name] = ToolInfo{Name: name, Description: description}
	}
}

// GetTool 获取指定工具
func (r *Registry) GetTool(name string) (tool.Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// GetTools 根据名称列表获取工具
func (r *Registry) GetTools(names []string) []tool.Tool {
	var result []tool.Tool
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// GetAllTools 获取所有工具
func (r *Registry) GetAllTools() []tool.Tool {
	var result []tool.Tool
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// GetAllToolNames 获取所有工具名称
func (r *Registry) GetAllToolNames() []string {
	var names []string
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// GetAllToolInfos 获取所有工具信息
func (r *Registry) GetAllToolInfos() []ToolInfo {
	var infos []ToolInfo
	for _, info := range r.toolInfos {
		infos = append(infos, info)
	}
	return infos
}

// GetToolInfosByNames 根据名称列表获取工具信息
func (r *Registry) GetToolInfosByNames(names []string) []ToolInfo {
	var infos []ToolInfo
	for _, name := range names {
		if info, ok := r.toolInfos[name]; ok {
			infos = append(infos, info)
		}
	}
	return infos
}
