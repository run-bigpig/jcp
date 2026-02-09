package hottrend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// ZhihuFetcher 知乎热榜获取器
type ZhihuFetcher struct {
	client *http.Client
}

// NewZhihuFetcher 创建知乎热榜获取器
func NewZhihuFetcher() *ZhihuFetcher {
	return &ZhihuFetcher{
		client: proxy.GetManager().GetClientWithTimeout(10 * time.Second),
	}
}

func (f *ZhihuFetcher) Platform() string   { return "zhihu" }
func (f *ZhihuFetcher) PlatformCN() string { return "知乎热榜" }

// zhihuResponse 知乎API响应结构
type zhihuResponse struct {
	Data []struct {
		Target struct {
			ID        int    `json:"id"`
			TitleArea struct {
				Text string `json:"text"`
			} `json:"title_area"`
			MetricsArea struct {
				Text string `json:"text"`
			} `json:"metrics_area"`
			Link struct {
				URL string `json:"url"`
			} `json:"link"`
		} `json:"target"`
	} `json:"data"`
}

// Fetch 获取知乎热榜数据
func (f *ZhihuFetcher) Fetch() ([]HotItem, error) {
	url := "https://www.zhihu.com/api/v3/feed/topstory/hot-list-web?limit=50&desktop=true"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result zhihuResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var items []HotItem
	for i, item := range result.Data {
		items = append(items, HotItem{
			ID:       fmt.Sprintf("zhihu_%d", item.Target.ID),
			Title:    item.Target.TitleArea.Text,
			URL:      item.Target.Link.URL,
			Rank:     i + 1,
			Platform: "zhihu",
			Extra:    item.Target.MetricsArea.Text,
		})
	}
	return items, nil
}
