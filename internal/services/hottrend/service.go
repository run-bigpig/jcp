package hottrend

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HotTrendService 舆情热点聚合服务
type HotTrendService struct {
	fetchers map[string]Fetcher
	cache    *FileCache
}

// NewHotTrendService 创建舆情热点服务
func NewHotTrendService(dataDir string) (*HotTrendService, error) {
	// 获取缓存目录
	cacheDir, err := getCacheDir(dataDir)
	if err != nil {
		return nil, err
	}

	// 创建文件缓存，TTL 5分钟
	cache, err := NewFileCache(cacheDir, 5*time.Minute)
	if err != nil {
		return nil, err
	}

	// 注册所有 fetcher
	fetchers := map[string]Fetcher{
		"weibo":    NewWeiboFetcher(),
		"zhihu":    NewZhihuFetcher(),
		"bilibili": NewBilibiliFetcher(),
		"baidu":    NewBaiduFetcher(),
		"douyin":   NewDouyinFetcher(),
		"toutiao":  NewToutiaoFetcher(),
	}

	return &HotTrendService{
		fetchers: fetchers,
		cache:    cache,
	}, nil
}

// getCacheDir 获取缓存目录
func getCacheDir(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) != "" {
		return filepath.Join(dataDir, "cache", "hottrend"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".jcp", "cache", "hottrend"), nil
}

// GetPlatforms 获取支持的平台列表
func (s *HotTrendService) GetPlatforms() []PlatformInfo {
	return SupportedPlatforms
}

// GetHotTrend 获取单个平台的热点数据
func (s *HotTrendService) GetHotTrend(platform string) HotTrendResult {
	fetcher, ok := s.fetchers[platform]
	if !ok {
		return HotTrendResult{
			Platform: platform,
			Error:    "不支持的平台",
		}
	}

	// 先检查缓存
	if items, ok := s.cache.Get(platform); ok {
		return HotTrendResult{
			Platform:   platform,
			PlatformCN: fetcher.PlatformCN(),
			Items:      items,
			UpdatedAt:  time.Now(),
			FromCache:  true,
		}
	}

	// 从网络获取
	items, err := fetcher.Fetch()
	if err != nil {
		return HotTrendResult{
			Platform:   platform,
			PlatformCN: fetcher.PlatformCN(),
			Error:      err.Error(),
		}
	}

	// 写入缓存
	_ = s.cache.Set(platform, items)

	return HotTrendResult{
		Platform:   platform,
		PlatformCN: fetcher.PlatformCN(),
		Items:      items,
		UpdatedAt:  time.Now(),
		FromCache:  false,
	}
}

// GetAllHotTrends 并发获取所有平台的热点数据
func (s *HotTrendService) GetAllHotTrends() []HotTrendResult {
	platforms := make([]string, 0, len(s.fetchers))
	for p := range s.fetchers {
		platforms = append(platforms, p)
	}
	return s.GetHotTrends(platforms)
}

// GetHotTrends 并发获取指定平台的热点数据
func (s *HotTrendService) GetHotTrends(platforms []string) []HotTrendResult {
	var wg sync.WaitGroup
	results := make([]HotTrendResult, len(platforms))

	for i, platform := range platforms {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			results[idx] = s.GetHotTrend(p)
		}(i, platform)
	}

	wg.Wait()
	return results
}
