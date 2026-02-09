package hottrend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// DouyinFetcher 抖音热点获取器
type DouyinFetcher struct {
	client *http.Client
}

// NewDouyinFetcher 创建抖音热点获取器
func NewDouyinFetcher() *DouyinFetcher {
	return &DouyinFetcher{
		client: proxy.GetManager().GetClientWithTimeout(10 * time.Second),
	}
}

func (f *DouyinFetcher) Platform() string   { return "douyin" }
func (f *DouyinFetcher) PlatformCN() string { return "抖音热点" }

// douyinResponse 抖音API响应结构
type douyinResponse struct {
	Data struct {
		WordList []struct {
			Word     string `json:"word"`
			HotValue int    `json:"hot_value"`
		} `json:"word_list"`
	} `json:"data"`
}

// Fetch 获取抖音热点数据
func (f *DouyinFetcher) Fetch() ([]HotItem, error) {
	url := "https://www.douyin.com/aweme/v1/web/hot/search/list/"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result douyinResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var items []HotItem
	for i, item := range result.Data.WordList {
		searchURL := fmt.Sprintf("https://www.douyin.com/search/%s", item.Word)
		items = append(items, HotItem{
			ID:       fmt.Sprintf("douyin_%d", i+1),
			Title:    item.Word,
			URL:      searchURL,
			HotScore: item.HotValue,
			Rank:     i + 1,
			Platform: "douyin",
		})
	}
	return items, nil
}
