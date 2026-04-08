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
	    tokenParamMode: string;
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
	        this.tokenParamMode = source["tokenParamMode"];
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
	export class KDJConfig {
	    enabled: boolean;
	    period: number;
	    k: number;
	    d: number;
	
	    static createFrom(source: any = {}) {
	        return new KDJConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.period = source["period"];
	        this.k = source["k"];
	        this.d = source["d"];
	    }
	}
	export class RSIConfig {
	    enabled: boolean;
	    period: number;
	
	    static createFrom(source: any = {}) {
	        return new RSIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.period = source["period"];
	    }
	}
	export class MACDConfig {
	    enabled: boolean;
	    fast: number;
	    slow: number;
	    signal: number;
	
	    static createFrom(source: any = {}) {
	        return new MACDConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.fast = source["fast"];
	        this.slow = source["slow"];
	        this.signal = source["signal"];
	    }
	}
	export class BOLLConfig {
	    enabled: boolean;
	    period: number;
	    multiplier: number;
	
	    static createFrom(source: any = {}) {
	        return new BOLLConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.period = source["period"];
	        this.multiplier = source["multiplier"];
	    }
	}
	export class EMAConfig {
	    enabled: boolean;
	    periods: number[];
	
	    static createFrom(source: any = {}) {
	        return new EMAConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.periods = source["periods"];
	    }
	}
	export class MAConfig {
	    enabled: boolean;
	    periods: number[];
	
	    static createFrom(source: any = {}) {
	        return new MAConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.periods = source["periods"];
	    }
	}
	export class IndicatorConfig {
	    ma: MAConfig;
	    ema: EMAConfig;
	    boll: BOLLConfig;
	    macd: MACDConfig;
	    rsi: RSIConfig;
	    kdj: KDJConfig;
	
	    static createFrom(source: any = {}) {
	        return new IndicatorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ma = this.convertValues(source["ma"], MAConfig);
	        this.ema = this.convertValues(source["ema"], EMAConfig);
	        this.boll = this.convertValues(source["boll"], BOLLConfig);
	        this.macd = this.convertValues(source["macd"], MACDConfig);
	        this.rsi = this.convertValues(source["rsi"], RSIConfig);
	        this.kdj = this.convertValues(source["kdj"], KDJConfig);
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
	    candleColorMode: string;
	    aiConfigs: AIConfig[];
	    defaultAiId: string;
	    strategyAiId: string;
	    moderatorAiId: string;
	    aiRetryCount: number;
	    verboseAgentIO: boolean;
	    agentSelectionStyle: string;
	    enableSecondReview: boolean;
	    mcpServers: MCPServerConfig[];
	    memory: MemoryConfig;
	    proxy: ProxyConfig;
	    layout: LayoutConfig;
	    openClaw: OpenClawConfig;
	    indicators: IndicatorConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.candleColorMode = source["candleColorMode"];
	        this.aiConfigs = this.convertValues(source["aiConfigs"], AIConfig);
	        this.defaultAiId = source["defaultAiId"];
	        this.strategyAiId = source["strategyAiId"];
	        this.moderatorAiId = source["moderatorAiId"];
	        this.aiRetryCount = source["aiRetryCount"];
	        this.verboseAgentIO = source["verboseAgentIO"];
	        this.agentSelectionStyle = source["agentSelectionStyle"];
	        this.enableSecondReview = source["enableSecondReview"];
	        this.mcpServers = this.convertValues(source["mcpServers"], MCPServerConfig);
	        this.memory = this.convertValues(source["memory"], MemoryConfig);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.layout = this.convertValues(source["layout"], LayoutConfig);
	        this.openClaw = this.convertValues(source["openClaw"], OpenClawConfig);
	        this.indicators = this.convertValues(source["indicators"], IndicatorConfig);
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
	
	export class BoardFundFlowItem {
	    code: string;
	    name: string;
	    price: number;
	    changePercent: number;
	    mainNetInflow: number;
	    mainNetInflowRatio: number;
	    superNetInflow: number;
	    superNetInflowRatio: number;
	    largeNetInflow: number;
	    largeNetInflowRatio: number;
	    mediumNetInflow: number;
	    mediumNetInflowRatio: number;
	    smallNetInflow: number;
	    smallNetInflowRatio: number;
	    updateTime?: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardFundFlowItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.changePercent = source["changePercent"];
	        this.mainNetInflow = source["mainNetInflow"];
	        this.mainNetInflowRatio = source["mainNetInflowRatio"];
	        this.superNetInflow = source["superNetInflow"];
	        this.superNetInflowRatio = source["superNetInflowRatio"];
	        this.largeNetInflow = source["largeNetInflow"];
	        this.largeNetInflowRatio = source["largeNetInflowRatio"];
	        this.mediumNetInflow = source["mediumNetInflow"];
	        this.mediumNetInflowRatio = source["mediumNetInflowRatio"];
	        this.smallNetInflow = source["smallNetInflow"];
	        this.smallNetInflowRatio = source["smallNetInflowRatio"];
	        this.updateTime = source["updateTime"];
	    }
	}
	export class BoardFundFlowList {
	    category: string;
	    items: BoardFundFlowItem[];
	    total?: number;
	    updateTime?: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardFundFlowList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.items = this.convertValues(source["items"], BoardFundFlowItem);
	        this.total = source["total"];
	        this.updateTime = source["updateTime"];
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
	export class BoardLeaderItem {
	    rank: number;
	    code: string;
	    name: string;
	    price: number;
	    changePercent: number;
	    turnoverRate?: number;
	    mainNetInflow: number;
	    mainNetInflowRatio: number;
	    score: number;
	    updateTime?: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardLeaderItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.changePercent = source["changePercent"];
	        this.turnoverRate = source["turnoverRate"];
	        this.mainNetInflow = source["mainNetInflow"];
	        this.mainNetInflowRatio = source["mainNetInflowRatio"];
	        this.score = source["score"];
	        this.updateTime = source["updateTime"];
	    }
	}
	export class BoardLeaderList {
	    boardCode: string;
	    items: BoardLeaderItem[];
	    updateTime?: string;
	
	    static createFrom(source: any = {}) {
	        return new BoardLeaderList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.boardCode = source["boardCode"];
	        this.items = this.convertValues(source["items"], BoardLeaderItem);
	        this.updateTime = source["updateTime"];
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
	export class BonusFinancing {
	    dividend?: any[];
	    annual?: any[];
	    financing?: any[];
	    allotment?: any[];
	
	    static createFrom(source: any = {}) {
	        return new BonusFinancing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dividend = source["dividend"];
	        this.annual = source["annual"];
	        this.financing = source["financing"];
	        this.allotment = source["allotment"];
	    }
	}
	export class BusinessAnalysis {
	    scope?: any[];
	    composition?: any[];
	    review?: any[];
	
	    static createFrom(source: any = {}) {
	        return new BusinessAnalysis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.composition = source["composition"];
	        this.review = source["review"];
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
	
	export class EquityPledge {
	    records?: any[];
	    latest?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new EquityPledge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = source["records"];
	        this.latest = source["latest"];
	    }
	}
	export class F10CapitalOperation {
	    raiseSources?: any[];
	    projectProgress?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10CapitalOperation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.raiseSources = source["raiseSources"];
	        this.projectProgress = source["projectProgress"];
	    }
	}
	export class F10CoreThemes {
	    boardTypes?: any[];
	    themes?: any[];
	    history?: any[];
	    selectedBoardReasons?: any[];
	    popularLeaders?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10CoreThemes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.boardTypes = source["boardTypes"];
	        this.themes = source["themes"];
	        this.history = source["history"];
	        this.selectedBoardReasons = source["selectedBoardReasons"];
	        this.popularLeaders = source["popularLeaders"];
	    }
	}
	export class F10EquityStructure {
	    latest?: any[];
	    history?: any[];
	    composition?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10EquityStructure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latest = source["latest"];
	        this.history = source["history"];
	        this.composition = source["composition"];
	    }
	}
	export class F10IndustryCompareMetrics {
	    valuation?: any[];
	    performance?: any[];
	    growth?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10IndustryCompareMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valuation = source["valuation"];
	        this.performance = source["performance"];
	        this.growth = source["growth"];
	    }
	}
	export class F10MainIndicators {
	    latest?: any[];
	    yearly?: any[];
	    quarterly?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10MainIndicators(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latest = source["latest"];
	        this.yearly = source["yearly"];
	        this.quarterly = source["quarterly"];
	    }
	}
	export class F10Management {
	    managementList?: any[];
	    salaryDetails?: any[];
	    holdingChanges?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10Management(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.managementList = source["managementList"];
	        this.salaryDetails = source["salaryDetails"];
	        this.holdingChanges = source["holdingChanges"];
	    }
	}
	export class F10OperationsRequired {
	    latestIndicators?: Record<string, any>;
	    latestIndicatorsExtra?: Record<string, any>;
	    latestIndicatorsQuote?: Record<string, any>;
	    eventReminders?: any[];
	    news?: any[];
	    announcements?: any[];
	    shareholderAnalysis?: any[];
	    dragonTigerList?: any[];
	    blockTrades?: any[];
	    marginTrading?: any[];
	    mainIndicators?: any[];
	    sectorTags?: any[];
	    coreThemes?: any[];
	    institutionForecast?: any[];
	    forecastChart?: any[];
	    reportSummary?: any[];
	    researchReports?: any[];
	    forecastRevisionTrack?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10OperationsRequired(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latestIndicators = source["latestIndicators"];
	        this.latestIndicatorsExtra = source["latestIndicatorsExtra"];
	        this.latestIndicatorsQuote = source["latestIndicatorsQuote"];
	        this.eventReminders = source["eventReminders"];
	        this.news = source["news"];
	        this.announcements = source["announcements"];
	        this.shareholderAnalysis = source["shareholderAnalysis"];
	        this.dragonTigerList = source["dragonTigerList"];
	        this.blockTrades = source["blockTrades"];
	        this.marginTrading = source["marginTrading"];
	        this.mainIndicators = source["mainIndicators"];
	        this.sectorTags = source["sectorTags"];
	        this.coreThemes = source["coreThemes"];
	        this.institutionForecast = source["institutionForecast"];
	        this.forecastChart = source["forecastChart"];
	        this.reportSummary = source["reportSummary"];
	        this.researchReports = source["researchReports"];
	        this.forecastRevisionTrack = source["forecastRevisionTrack"];
	    }
	}
	export class F10ValuationTrend {
	    source?: string;
	    range?: string;
	    requestedRange?: string;
	    fallback?: boolean;
	    dateType?: number;
	    labels?: Record<string, string>;
	    pe?: any[];
	    pb?: any[];
	    ps?: any[];
	    pcf?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10ValuationTrend(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.range = source["range"];
	        this.requestedRange = source["requestedRange"];
	        this.fallback = source["fallback"];
	        this.dateType = source["dateType"];
	        this.labels = source["labels"];
	        this.pe = source["pe"];
	        this.pb = source["pb"];
	        this.ps = source["ps"];
	        this.pcf = source["pcf"];
	    }
	}
	export class F10RelatedStocks {
	    industryRankings?: any[];
	    conceptRelations?: any[];
	
	    static createFrom(source: any = {}) {
	        return new F10RelatedStocks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.industryRankings = source["industryRankings"];
	        this.conceptRelations = source["conceptRelations"];
	    }
	}
	export class StockValuation {
	    price?: number;
	    peTtm?: number;
	    pb?: number;
	    totalMarketCap?: number;
	    floatMarketCap?: number;
	    turnoverRate?: number;
	    amplitude?: number;
	    totalShares?: number;
	    floatShares?: number;
	
	    static createFrom(source: any = {}) {
	        return new StockValuation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.price = source["price"];
	        this.peTtm = source["peTtm"];
	        this.pb = source["pb"];
	        this.totalMarketCap = source["totalMarketCap"];
	        this.floatMarketCap = source["floatMarketCap"];
	        this.turnoverRate = source["turnoverRate"];
	        this.amplitude = source["amplitude"];
	        this.totalShares = source["totalShares"];
	        this.floatShares = source["floatShares"];
	    }
	}
	export class StockBuyback {
	    records?: any[];
	    latest?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new StockBuyback(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = source["records"];
	        this.latest = source["latest"];
	    }
	}
	export class ShareholderChanges {
	    records?: any[];
	    latest?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ShareholderChanges(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = source["records"];
	        this.latest = source["latest"];
	    }
	}
	export class LockupRelease {
	    records?: any[];
	    latest?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new LockupRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = source["records"];
	        this.latest = source["latest"];
	    }
	}
	export class ShareholderNumbers {
	    records?: any[];
	    latest?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ShareholderNumbers(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = source["records"];
	        this.latest = source["latest"];
	    }
	}
	export class StockPeer {
	    symbol: string;
	    name: string;
	    market?: string;
	
	    static createFrom(source: any = {}) {
	        return new StockPeer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.market = source["market"];
	    }
	}
	export class IndustryCompare {
	    industry?: string;
	    peers?: StockPeer[];
	
	    static createFrom(source: any = {}) {
	        return new IndustryCompare(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.industry = source["industry"];
	        this.peers = this.convertValues(source["peers"], StockPeer);
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
	export class InstitutionalHoldings {
	    topHolders?: any[];
	    controller?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new InstitutionalHoldings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.topHolders = source["topHolders"];
	        this.controller = source["controller"];
	    }
	}
	export class FundFlowSeries {
	    fields?: string[];
	    lines?: string[][];
	    labels?: Record<string, string>;
	    latest?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new FundFlowSeries(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fields = source["fields"];
	        this.lines = source["lines"];
	        this.labels = source["labels"];
	        this.latest = source["latest"];
	    }
	}
	export class PerformanceEvents {
	    forecast?: any[];
	    express?: any[];
	    schedule?: any[];
	
	    static createFrom(source: any = {}) {
	        return new PerformanceEvents(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.forecast = source["forecast"];
	        this.express = source["express"];
	        this.schedule = source["schedule"];
	    }
	}
	export class FinancialStatements {
	    income?: any[];
	    balance?: any[];
	    cashflow?: any[];
	
	    static createFrom(source: any = {}) {
	        return new FinancialStatements(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.income = source["income"];
	        this.balance = source["balance"];
	        this.cashflow = source["cashflow"];
	    }
	}
	export class F10Overview {
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
	
	    static createFrom(source: any = {}) {
	        return new F10Overview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.updatedAt = source["updatedAt"];
	        this.source = source["source"];
	        this.company = source["company"];
	        this.financials = this.convertValues(source["financials"], FinancialStatements);
	        this.performance = this.convertValues(source["performance"], PerformanceEvents);
	        this.fundFlow = this.convertValues(source["fundFlow"], FundFlowSeries);
	        this.institutions = this.convertValues(source["institutions"], InstitutionalHoldings);
	        this.industry = this.convertValues(source["industry"], IndustryCompare);
	        this.bonus = this.convertValues(source["bonus"], BonusFinancing);
	        this.business = this.convertValues(source["business"], BusinessAnalysis);
	        this.shareholders = this.convertValues(source["shareholders"], ShareholderNumbers);
	        this.pledge = this.convertValues(source["pledge"], EquityPledge);
	        this.lockup = this.convertValues(source["lockup"], LockupRelease);
	        this.holderChange = this.convertValues(source["holderChange"], ShareholderChanges);
	        this.buyback = this.convertValues(source["buyback"], StockBuyback);
	        this.valuation = this.convertValues(source["valuation"], StockValuation);
	        this.operations = this.convertValues(source["operations"], F10OperationsRequired);
	        this.coreThemes = this.convertValues(source["coreThemes"], F10CoreThemes);
	        this.industryMetrics = this.convertValues(source["industryMetrics"], F10IndustryCompareMetrics);
	        this.mainIndicators = this.convertValues(source["mainIndicators"], F10MainIndicators);
	        this.management = this.convertValues(source["management"], F10Management);
	        this.capitalOperation = this.convertValues(source["capitalOperation"], F10CapitalOperation);
	        this.equityStructure = this.convertValues(source["equityStructure"], F10EquityStructure);
	        this.relatedStocks = this.convertValues(source["relatedStocks"], F10RelatedStocks);
	        this.valuationTrend = this.convertValues(source["valuationTrend"], F10ValuationTrend);
	        this.errors = source["errors"];
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
	
	
	
	export class MarketIndex {
	    code: string;
	    name: string;
	    price: number;
	    change: number;
	    changePercent: number;
	    volume: number;
	    amount: number;
	
	    static createFrom(source: any = {}) {
	        return new MarketIndex(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.change = source["change"];
	        this.changePercent = source["changePercent"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
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
	
	export class StockMoveItem {
	    rank: number;
	    code: string;
	    name: string;
	    price: number;
	    changePercent: number;
	    speed: number;
	    turnoverRate: number;
	    volume: number;
	    amount: number;
	    mainNetInflow: number;
	    mainNetInflowRatio: number;
	    high: number;
	    low: number;
	    open: number;
	    preClose: number;
	    updateTime?: string;
	
	    static createFrom(source: any = {}) {
	        return new StockMoveItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.changePercent = source["changePercent"];
	        this.speed = source["speed"];
	        this.turnoverRate = source["turnoverRate"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	        this.mainNetInflow = source["mainNetInflow"];
	        this.mainNetInflowRatio = source["mainNetInflowRatio"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.open = source["open"];
	        this.preClose = source["preClose"];
	        this.updateTime = source["updateTime"];
	    }
	}
	export class StockMoveList {
	    moveType: string;
	    items: StockMoveItem[];
	    total?: number;
	    updateTime?: string;
	
	    static createFrom(source: any = {}) {
	        return new StockMoveList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.moveType = source["moveType"];
	        this.items = this.convertValues(source["items"], StockMoveItem);
	        this.total = source["total"];
	        this.updateTime = source["updateTime"];
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
	export class MarketStatus {
	    status: string;
	    statusText: string;
	    isTradeDay: boolean;
	    holidayName: string;
	
	    static createFrom(source: any = {}) {
	        return new MarketStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.isTradeDay = source["isTradeDay"];
	        this.holidayName = source["holidayName"];
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
