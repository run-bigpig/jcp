package hottrend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// ToutiaoFetcher 头条热榜获取器
type ToutiaoFetcher struct {
	client *http.Client
}

// NewToutiaoFetcher 创建头条热榜获取器
func NewToutiaoFetcher() *ToutiaoFetcher {
	return &ToutiaoFetcher{
		client: proxy.GetManager().GetClientWithTimeout(10 * time.Second),
	}
}

func (f *ToutiaoFetcher) Platform() string   { return "toutiao" }
func (f *ToutiaoFetcher) PlatformCN() string { return "头条热榜" }

// toutiaoResponse 头条API响应结构
type toutiaoResponse struct {
	Data []struct {
		Title    string `json:"Title"`
		HotValue string `json:"HotValue"`
		ClusterID string `json:"ClusterIdStr"`
	} `json:"data"`
}

// Fetch 获取头条热榜数据
func (f *ToutiaoFetcher) Fetch() ([]HotItem, error) {
	url := "https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc"

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

	var result toutiaoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var items []HotItem
	for i, item := range result.Data {
		itemURL := fmt.Sprintf("https://www.toutiao.com/trending/%s/", item.ClusterID)
		items = append(items, HotItem{
			ID:       fmt.Sprintf("toutiao_%s", item.ClusterID),
			Title:    item.Title,
			URL:      itemURL,
			Rank:     i + 1,
			Platform: "toutiao",
			Extra:    item.HotValue,
		})
	}
	return items, nil
}
