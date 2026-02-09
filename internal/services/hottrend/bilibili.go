package hottrend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// BilibiliFetcher B站热搜获取器
type BilibiliFetcher struct {
	client *http.Client
}

// NewBilibiliFetcher 创建B站热搜获取器
func NewBilibiliFetcher() *BilibiliFetcher {
	return &BilibiliFetcher{
		client: proxy.GetManager().GetClientWithTimeout(10 * time.Second),
	}
}

func (f *BilibiliFetcher) Platform() string   { return "bilibili" }
func (f *BilibiliFetcher) PlatformCN() string { return "B站热搜" }

// bilibiliResponse B站API响应结构
type bilibiliResponse struct {
	Code int `json:"code"`
	List []struct {
		Keyword   string `json:"keyword"`
		ShowName  string `json:"show_name"`
		HotID     int    `json:"hot_id"`
		GotoType  int    `json:"goto_type"`
		GotoValue string `json:"goto_value"`
	} `json:"list"`
}

// Fetch 获取B站热搜数据
func (f *BilibiliFetcher) Fetch() ([]HotItem, error) {
	url := "https://s.search.bilibili.com/main/hotword?limit=50"

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

	var result bilibiliResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var items []HotItem
	for i, item := range result.List {
		searchURL := fmt.Sprintf("https://search.bilibili.com/all?keyword=%s", item.Keyword)
		items = append(items, HotItem{
			ID:       fmt.Sprintf("bilibili_%d", item.HotID),
			Title:    item.ShowName,
			URL:      searchURL,
			Rank:     i + 1,
			Platform: "bilibili",
		})
	}
	return items, nil
}
