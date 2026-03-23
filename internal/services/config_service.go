package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/run-bigpig/jcp/internal/embed"
	"github.com/run-bigpig/jcp/internal/models"
)

// ConfigService 配置服务
type ConfigService struct {
	configPath    string
	watchlistPath string
	config        *models.AppConfig
	watchlist     []models.Stock
	mu            sync.RWMutex
}

// NewConfigService 创建配置服务
func NewConfigService(dataDir string) (*ConfigService, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	cs := &ConfigService{
		configPath:    filepath.Join(dataDir, "config.json"),
		watchlistPath: filepath.Join(dataDir, "watchlist.json"),
	}

	if err := cs.loadConfig(); err != nil {
		return nil, err
	}
	if err := cs.loadWatchlist(); err != nil {
		return nil, err
	}

	return cs, nil
}

// loadConfig 加载配置
func (cs *ConfigService) loadConfig() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	data, err := os.ReadFile(cs.configPath)
	if os.IsNotExist(err) {
		cs.config = cs.defaultConfig()
		return cs.saveConfigLocked()
	}
	if err != nil {
		return err
	}

	var config models.AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	raw := string(data)
	// 兼容旧配置：缺少该字段时默认开启，便于排障观察 Agent 请求/响应链路。
	if !strings.Contains(raw, "\"verboseAgentIO\"") {
		config.VerboseAgentIO = true
	}
	// 兼容旧配置：缺少该字段时默认开启，保持历史行为（二轮复议默认开启）。
	if !strings.Contains(raw, "\"enableSecondReview\"") {
		config.EnableSecondReview = true
	}
	cs.normalizeConfig(&config)
	cs.config = &config
	return nil
}

// defaultConfig 默认配置
func (cs *ConfigService) defaultConfig() *models.AppConfig {
	return &models.AppConfig{
		Theme:               "military",
		AIConfigs:           []models.AIConfig{},
		DefaultAIID:         "",
		AIRetryCount:        3,
		VerboseAgentIO:      true,
		AgentSelectionStyle: models.AgentSelectionBalanced,
		EnableSecondReview:  true,
		Proxy: models.ProxyConfig{
			Mode: models.ProxyModeNone,
		},
		Memory: models.MemoryConfig{
			Enabled:           true,
			MaxRecentRounds:   3,
			MaxKeyFacts:       20,
			MaxSummaryLength:  300,
			CompressThreshold: 5,
		},
	}
}

// saveConfigLocked 保存配置(需要已持有锁)
func (cs *ConfigService) saveConfigLocked() error {
	data, err := json.MarshalIndent(cs.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.configPath, data, 0644)
}

// GetConfig 获取配置
func (cs *ConfigService) GetConfig() *models.AppConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.config
}

// UpdateConfig 更新配置
func (cs *ConfigService) UpdateConfig(config *models.AppConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.normalizeConfig(config)
	cs.config = config
	return cs.saveConfigLocked()
}

func (cs *ConfigService) normalizeConfig(config *models.AppConfig) {
	if config == nil {
		return
	}
	if config.AIRetryCount < 1 {
		config.AIRetryCount = 3
	}
	if config.AIRetryCount > 5 {
		config.AIRetryCount = 5
	}
	switch config.AgentSelectionStyle {
	case models.AgentSelectionBalanced, models.AgentSelectionConservative, models.AgentSelectionAggressive:
	default:
		config.AgentSelectionStyle = models.AgentSelectionBalanced
	}
	for i := range config.AIConfigs {
		if config.AIConfigs[i].Timeout <= 0 {
			config.AIConfigs[i].Timeout = 60
		}
		if config.AIConfigs[i].Timeout < 5 {
			config.AIConfigs[i].Timeout = 5
		}
		if config.AIConfigs[i].Timeout > 3600 {
			config.AIConfigs[i].Timeout = 3600
		}
	}
}

// loadWatchlist 加载自选股列表
func (cs *ConfigService) loadWatchlist() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	data, err := os.ReadFile(cs.watchlistPath)
	if os.IsNotExist(err) {
		// 文件不存在时，初始化为空列表
		cs.watchlist = []models.Stock{}
		return cs.saveWatchlistLocked()
	}
	if err != nil {
		return err
	}

	var watchlist []models.Stock
	if err := json.Unmarshal(data, &watchlist); err != nil {
		return err
	}

	cs.watchlist = watchlist
	return nil
}

// saveWatchlistLocked 保存自选股(需要已持有锁)
func (cs *ConfigService) saveWatchlistLocked() error {
	data, err := json.MarshalIndent(cs.watchlist, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.watchlistPath, data, 0644)
}

// GetWatchlist 获取自选股列表
func (cs *ConfigService) GetWatchlist() []models.Stock {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.watchlist
}

// AddToWatchlist 添加自选股
func (cs *ConfigService) AddToWatchlist(stock models.Stock) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, s := range cs.watchlist {
		if s.Symbol == stock.Symbol {
			return nil
		}
	}
	cs.watchlist = append(cs.watchlist, stock)
	return cs.saveWatchlistLocked()
}

// RemoveFromWatchlist 移除自选股
func (cs *ConfigService) RemoveFromWatchlist(symbol string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, s := range cs.watchlist {
		if s.Symbol == symbol {
			cs.watchlist = append(cs.watchlist[:i], cs.watchlist[i+1:]...)
			return cs.saveWatchlistLocked()
		}
	}
	return nil
}

// stockBasicData stock_basic.json 的数据结构
type stockBasicData struct {
	Data struct {
		Fields []string        `json:"fields"`
		Items  [][]interface{} `json:"items"`
	} `json:"data"`
}

// StockSearchResult 股票搜索结果
type StockSearchResult struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Industry string `json:"industry"`
	Market   string `json:"market"`
}

// SearchStocks 搜索股票
func (cs *ConfigService) SearchStocks(keyword string, limit int) []StockSearchResult {
	if keyword == "" {
		return []StockSearchResult{}
	}

	keyword = strings.ToUpper(keyword)

	// 使用嵌入的股票数据
	var basicData stockBasicData
	if err := json.Unmarshal(embed.StockBasicJSON, &basicData); err != nil {
		return []StockSearchResult{}
	}

	// 找到字段索引
	var symbolIdx, nameIdx, industryIdx, tsCodeIdx int = -1, -1, -1, -1
	for i, field := range basicData.Data.Fields {
		switch field {
		case "symbol":
			symbolIdx = i
		case "name":
			nameIdx = i
		case "industry":
			industryIdx = i
		case "ts_code":
			tsCodeIdx = i
		}
	}

	if symbolIdx < 0 || nameIdx < 0 {
		return []StockSearchResult{}
	}

	var results []StockSearchResult
	for _, item := range basicData.Data.Items {
		if len(results) >= limit {
			break
		}

		symbol, _ := item[symbolIdx].(string)
		name, _ := item[nameIdx].(string)

		// 匹配代码或名称
		upperSymbol := strings.ToUpper(symbol)
		upperName := strings.ToUpper(name)

		if strings.Contains(upperSymbol, keyword) || strings.Contains(upperName, keyword) {
			var industry, market, fullSymbol string
			if industryIdx >= 0 && industryIdx < len(item) {
				industry, _ = item[industryIdx].(string)
			}
			// 从 ts_code 获取市场前缀
			if tsCodeIdx >= 0 && tsCodeIdx < len(item) {
				tsCode, _ := item[tsCodeIdx].(string)
				if strings.HasSuffix(tsCode, ".SH") {
					market = "上海"
					fullSymbol = "sh" + symbol
				} else if strings.HasSuffix(tsCode, ".SZ") {
					market = "深圳"
					fullSymbol = "sz" + symbol
				}
			}
			if fullSymbol == "" {
				fullSymbol = symbol
			}

			results = append(results, StockSearchResult{
				Symbol:   fullSymbol,
				Name:     name,
				Industry: industry,
				Market:   market,
			})
		}
	}

	return results
}
