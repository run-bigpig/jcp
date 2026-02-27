export namespace hottrend {
	
	export class HotItem {
	    id: string;
	    title: string;
	    url: string;
	    hot_score: number;
	    rank: number;
	    platform: string;
	    extra: string;
	
	    static createFrom(source: any = {}) {
	        return new HotItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.hot_score = source["hot_score"];
	        this.rank = source["rank"];
	        this.platform = source["platform"];
	        this.extra = source["extra"];
	    }
	}
	export class HotTrendResult {
	    platform: string;
	    platform_cn: string;
	    items: HotItem[];
	    // Go type: time
	    updated_at: any;
	    from_cache: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new HotTrendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.platform_cn = source["platform_cn"];
	        this.items = this.convertValues(source["items"], HotItem);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.from_cache = source["from_cache"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlatformInfo {
	    ID: string;
	    Name: string;
	    HomeURL: string;
	
	    static createFrom(source: any = {}) {
	        return new PlatformInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.HomeURL = source["HomeURL"];
	    }
	}

}

export namespace main {
	
	export class EnhancePromptRequest {
	    originalPrompt: string;
	    agentRole: string;
	    agentName: string;
	
	    static createFrom(source: any = {}) {
	        return new EnhancePromptRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.originalPrompt = source["originalPrompt"];
	        this.agentRole = source["agentRole"];
	        this.agentName = source["agentName"];
	    }
	}
	export class EnhancePromptResponse {
	    success: boolean;
	    enhancedPrompt?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new EnhancePromptResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.enhancedPrompt = source["enhancedPrompt"];
	        this.error = source["error"];
	    }
	}
	export class GenerateStrategyRequest {
	    prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new GenerateStrategyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt = source["prompt"];
	    }
	}
	export class GenerateStrategyResponse {
	    success: boolean;
	    error?: string;
	    strategy?: models.Strategy;
	    reasoning?: string;
	
	    static createFrom(source: any = {}) {
	        return new GenerateStrategyResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.strategy = this.convertValues(source["strategy"], models.Strategy);
	        this.reasoning = source["reasoning"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MeetingMessageRequest {
	    stockCode: string;
	    content: string;
	    mentionIds: string[];
	    replyToId: string;
	    replyContent: string;
	
	    static createFrom(source: any = {}) {
	        return new MeetingMessageRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.content = source["content"];
	        this.mentionIds = source["mentionIds"];
	        this.replyToId = source["replyToId"];
	        this.replyContent = source["replyContent"];
	    }
	}

}

export namespace mcp {
	
	export class ServerStatus {
	    id: string;
	    connected: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.connected = source["connected"];
	        this.error = source["error"];
	    }
	}
	export class ToolInfo {
	    name: string;
	    description: string;
	    serverId: string;
	    serverName: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.serverId = source["serverId"];
	        this.serverName = source["serverName"];
	    }
	}

}

export namespace models {
	
	export class AIConfig {
	    id: string;
	    name: string;
	    provider: string;
	    baseUrl: string;
	    apiKey: string;
	    modelName: string;
	    maxTokens: number;
	    temperature: number;
	    timeout: number;
	    isDefault: boolean;
	    useResponses: boolean;
	    noSystemRole: boolean;
	    project: string;
	    location: string;
	    credentialsJson: string;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.modelName = source["modelName"];
	        this.maxTokens = source["maxTokens"];
	        this.temperature = source["temperature"];
	        this.timeout = source["timeout"];
	        this.isDefault = source["isDefault"];
	        this.useResponses = source["useResponses"];
	        this.noSystemRole = source["noSystemRole"];
	        this.project = source["project"];
	        this.location = source["location"];
	        this.credentialsJson = source["credentialsJson"];
	    }
	}
	export class AgentConfig {
	    id: string;
	    name: string;
	    role: string;
	    avatar: string;
	    color: string;
	    instruction: string;
	    tools: string[];
	    mcpServers: string[];
	    enabled: boolean;
	    aiConfigId: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.avatar = source["avatar"];
	        this.color = source["color"];
	        this.instruction = source["instruction"];
	        this.tools = source["tools"];
	        this.mcpServers = source["mcpServers"];
	        this.enabled = source["enabled"];
	        this.aiConfigId = source["aiConfigId"];
	    }
	}
	export class OpenClawConfig {
	    enabled: boolean;
	    port: number;
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenClawConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.apiKey = source["apiKey"];
	    }
	}
	export class LayoutConfig {
	    leftPanelWidth: number;
	    rightPanelWidth: number;
	    bottomPanelHeight: number;
	    windowWidth: number;
	    windowHeight: number;
	
	    static createFrom(source: any = {}) {
	        return new LayoutConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.leftPanelWidth = source["leftPanelWidth"];
	        this.rightPanelWidth = source["rightPanelWidth"];
	        this.bottomPanelHeight = source["bottomPanelHeight"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	    }
	}
	export class ProxyConfig {
	    mode: string;
	    customUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.customUrl = source["customUrl"];
	    }
	}
	export class MemoryConfig {
	    enabled: boolean;
	    aiConfigId: string;
	    maxRecentRounds: number;
	    maxKeyFacts: number;
	    maxSummaryLength: number;
	    compressThreshold: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.aiConfigId = source["aiConfigId"];
	        this.maxRecentRounds = source["maxRecentRounds"];
	        this.maxKeyFacts = source["maxKeyFacts"];
	        this.maxSummaryLength = source["maxSummaryLength"];
	        this.compressThreshold = source["compressThreshold"];
	    }
	}
	export class MCPServerConfig {
	    id: string;
	    name: string;
	    transportType: string;
	    endpoint: string;
	    command: string;
	    args: string[];
	    toolFilter: string[];
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.transportType = source["transportType"];
	        this.endpoint = source["endpoint"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.toolFilter = source["toolFilter"];
	        this.enabled = source["enabled"];
	    }
	}
	export class AppConfig {
	    theme: string;
	    aiConfigs: AIConfig[];
	    defaultAiId: string;
	    strategyAiId: string;
	    moderatorAiId: string;
	    mcpServers: MCPServerConfig[];
	    memory: MemoryConfig;
	    proxy: ProxyConfig;
	    layout: LayoutConfig;
	    openClaw: OpenClawConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.aiConfigs = this.convertValues(source["aiConfigs"], AIConfig);
	        this.defaultAiId = source["defaultAiId"];
	        this.strategyAiId = source["strategyAiId"];
	        this.moderatorAiId = source["moderatorAiId"];
	        this.mcpServers = this.convertValues(source["mcpServers"], MCPServerConfig);
	        this.memory = this.convertValues(source["memory"], MemoryConfig);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.layout = this.convertValues(source["layout"], LayoutConfig);
	        this.openClaw = this.convertValues(source["openClaw"], OpenClawConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChatMessage {
	    id: string;
	    agentId: string;
	    agentName: string;
	    role: string;
	    content: string;
	    timestamp: number;
	    replyTo?: string;
	    mentions?: string[];
	    round?: number;
	    msgType?: string;
	    error?: string;
	    meetingMode?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.agentId = source["agentId"];
	        this.agentName = source["agentName"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.timestamp = source["timestamp"];
	        this.replyTo = source["replyTo"];
	        this.mentions = source["mentions"];
	        this.round = source["round"];
	        this.msgType = source["msgType"];
	        this.error = source["error"];
	        this.meetingMode = source["meetingMode"];
	    }
	}
	export class KLineData {
	    time: string;
	    open: number;
	    high: number;
	    low: number;
	    close: number;
	    volume: number;
	    amount?: number;
	    avg?: number;
	    ma5?: number;
	    ma10?: number;
	    ma20?: number;
	
	    static createFrom(source: any = {}) {
	        return new KLineData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.open = source["open"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.close = source["close"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	        this.avg = source["avg"];
	        this.ma5 = source["ma5"];
	        this.ma10 = source["ma10"];
	        this.ma20 = source["ma20"];
	    }
	}
	
	export class LongHuBangDetail {
	    rank: number;
	    operName: string;
	    buyAmt: number;
	    buyPercent: number;
	    sellAmt: number;
	    sellPercent: number;
	    netAmt: number;
	    direction: string;
	
	    static createFrom(source: any = {}) {
	        return new LongHuBangDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.operName = source["operName"];
	        this.buyAmt = source["buyAmt"];
	        this.buyPercent = source["buyPercent"];
	        this.sellAmt = source["sellAmt"];
	        this.sellPercent = source["sellPercent"];
	        this.netAmt = source["netAmt"];
	        this.direction = source["direction"];
	    }
	}
	export class LongHuBangItem {
	    tradeDate: string;
	    code: string;
	    secuCode: string;
	    name: string;
	    closePrice: number;
	    changePercent: number;
	    netBuyAmt: number;
	    buyAmt: number;
	    sellAmt: number;
	    totalAmt: number;
	    turnoverRate: number;
	    freeCap: number;
	    reason: string;
	    reasonDetail: string;
	    accumAmount: number;
	    dealRatio: number;
	    netRatio: number;
	    d1Change: number;
	    d2Change: number;
	    d5Change: number;
	    d10Change: number;
	    securityType: string;
	
	    static createFrom(source: any = {}) {
	        return new LongHuBangItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tradeDate = source["tradeDate"];
	        this.code = source["code"];
	        this.secuCode = source["secuCode"];
	        this.name = source["name"];
	        this.closePrice = source["closePrice"];
	        this.changePercent = source["changePercent"];
	        this.netBuyAmt = source["netBuyAmt"];
	        this.buyAmt = source["buyAmt"];
	        this.sellAmt = source["sellAmt"];
	        this.totalAmt = source["totalAmt"];
	        this.turnoverRate = source["turnoverRate"];
	        this.freeCap = source["freeCap"];
	        this.reason = source["reason"];
	        this.reasonDetail = source["reasonDetail"];
	        this.accumAmount = source["accumAmount"];
	        this.dealRatio = source["dealRatio"];
	        this.netRatio = source["netRatio"];
	        this.d1Change = source["d1Change"];
	        this.d2Change = source["d2Change"];
	        this.d5Change = source["d5Change"];
	        this.d10Change = source["d10Change"];
	        this.securityType = source["securityType"];
	    }
	}
	
	
	
	export class OrderBookItem {
	    price: number;
	    size: number;
	    total: number;
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new OrderBookItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.price = source["price"];
	        this.size = source["size"];
	        this.total = source["total"];
	        this.percent = source["percent"];
	    }
	}
	export class OrderBook {
	    bids: OrderBookItem[];
	    asks: OrderBookItem[];
	
	    static createFrom(source: any = {}) {
	        return new OrderBook(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bids = this.convertValues(source["bids"], OrderBookItem);
	        this.asks = this.convertValues(source["asks"], OrderBookItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Stock {
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
	
	    static createFrom(source: any = {}) {
	        return new Stock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.change = source["change"];
	        this.changePercent = source["changePercent"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	        this.marketCap = source["marketCap"];
	        this.sector = source["sector"];
	        this.open = source["open"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.preClose = source["preClose"];
	    }
	}
	export class StockPosition {
	    shares: number;
	    costPrice: number;
	
	    static createFrom(source: any = {}) {
	        return new StockPosition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shares = source["shares"];
	        this.costPrice = source["costPrice"];
	    }
	}
	export class StockSession {
	    id: string;
	    stockCode: string;
	    stockName: string;
	    messages: ChatMessage[];
	    position?: StockPosition;
	    createdAt: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new StockSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.messages = this.convertValues(source["messages"], ChatMessage);
	        this.position = this.convertValues(source["position"], StockPosition);
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StrategyAgent {
	    id: string;
	    name: string;
	    role: string;
	    avatar: string;
	    color: string;
	    instruction: string;
	    tools: string[];
	    mcpServers: string[];
	    enabled: boolean;
	    aiConfigId: string;
	
	    static createFrom(source: any = {}) {
	        return new StrategyAgent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.avatar = source["avatar"];
	        this.color = source["color"];
	        this.instruction = source["instruction"];
	        this.tools = source["tools"];
	        this.mcpServers = source["mcpServers"];
	        this.enabled = source["enabled"];
	        this.aiConfigId = source["aiConfigId"];
	    }
	}
	export class Strategy {
	    id: string;
	    name: string;
	    description: string;
	    color: string;
	    agents: StrategyAgent[];
	    isBuiltin: boolean;
	    source: string;
	    sourceMeta: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Strategy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.color = source["color"];
	        this.agents = this.convertValues(source["agents"], StrategyAgent);
	        this.isBuiltin = source["isBuiltin"];
	        this.source = source["source"];
	        this.sourceMeta = source["sourceMeta"];
	        this.createdAt = source["createdAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace services {
	
	export class LongHuBangListResult {
	    items: models.LongHuBangItem[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new LongHuBangListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], models.LongHuBangItem);
	        this.total = source["total"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockSearchResult {
	    symbol: string;
	    name: string;
	    industry: string;
	    market: string;
	
	    static createFrom(source: any = {}) {
	        return new StockSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.industry = source["industry"];
	        this.market = source["market"];
	    }
	}
	export class Telegraph {
	    time: string;
	    content: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new Telegraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.content = source["content"];
	        this.url = source["url"];
	    }
	}
	export class TradingPeriod {
	    status: string;
	    text: string;
	    startTime: string;
	    endTime: string;
	
	    static createFrom(source: any = {}) {
	        return new TradingPeriod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.text = source["text"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	    }
	}
	export class TradingSchedule {
	    isTradeDay: boolean;
	    holidayName: string;
	    periods: TradingPeriod[];
	
	    static createFrom(source: any = {}) {
	        return new TradingSchedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isTradeDay = source["isTradeDay"];
	        this.holidayName = source["holidayName"];
	        this.periods = this.convertValues(source["periods"], TradingPeriod);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdateInfo {
	    hasUpdate: boolean;
	    latestVersion: string;
	    currentVersion: string;
	    releaseUrl: string;
	    releaseNotes: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.latestVersion = source["latestVersion"];
	        this.currentVersion = source["currentVersion"];
	        this.releaseUrl = source["releaseUrl"];
	        this.releaseNotes = source["releaseNotes"];
	        this.error = source["error"];
	    }
	}

}

export namespace tools {
	
	export class ToolInfo {
	    name: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}

}

