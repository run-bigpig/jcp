package hottrend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// WeiboFetcher 微博热搜获取器
type WeiboFetcher struct {
	client *http.Client
}

// NewWeiboFetcher 创建微博热搜获取器
func NewWeiboFetcher() *WeiboFetcher {
	return &WeiboFetcher{
		client: proxy.GetManager().GetClientWithTimeout(10 * time.Second),
	}
}

func (f *WeiboFetcher) Platform() string   { return "weibo" }
func (f *WeiboFetcher) PlatformCN() string { return "微博热搜" }

// weiboResponse 微博API响应结构
type weiboResponse struct {
	OK   int `json:"ok"`
	Data struct {
		Realtime []struct {
			Word    string `json:"word"`
			Note    string `json:"note"`
			Num     int    `json:"num"`
			Rank    int    `json:"rank"`
			Realpos int    `json:"realpos"`
		} `json:"realtime"`
	} `json:"data"`
}

// Fetch 获取微博热搜数据
func (f *WeiboFetcher) Fetch() ([]HotItem, error) {
	url := "https://weibo.com/ajax/side/hotSearch"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://weibo.com/")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result weiboResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.OK != 1 {
		return nil, fmt.Errorf("weibo api error: ok=%d", result.OK)
	}

	var items []HotItem
	for i, item := range result.Data.Realtime {
		if item.Word == "" {
			continue
		}
		rank := i + 1
		// 构建微博搜索URL
		searchURL := fmt.Sprintf("https://s.weibo.com/weibo?q=%s", item.Word)
		items = append(items, HotItem{
			ID:       fmt.Sprintf("weibo_%d", rank),
			Title:    item.Word,
			URL:      searchURL,
			HotScore: item.Num,
			Rank:     rank,
			Platform: "weibo",
		})
		if rank >= 50 {
			break
		}
	}
	return items, nil
}
