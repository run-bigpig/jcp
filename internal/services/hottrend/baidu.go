package hottrend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// BaiduFetcher 百度热搜获取器
type BaiduFetcher struct {
	client *http.Client
}

// NewBaiduFetcher 创建百度热搜获取器
func NewBaiduFetcher() *BaiduFetcher {
	return &BaiduFetcher{
		client: proxy.GetManager().GetClientWithTimeout(10 * time.Second),
	}
}

func (f *BaiduFetcher) Platform() string   { return "baidu" }
func (f *BaiduFetcher) PlatformCN() string { return "百度热搜" }

// baiduResponse 百度API响应结构
type baiduResponse struct {
	Data struct {
		Cards []struct {
			Content []struct {
				Content []struct {
					Word  string `json:"word"`
					URL   string `json:"url"`
					Index int    `json:"index"`
				} `json:"content"`
			} `json:"content"`
		} `json:"cards"`
	} `json:"data"`
}

// Fetch 获取百度热搜数据
func (f *BaiduFetcher) Fetch() ([]HotItem, error) {
	url := "https://top.baidu.com/api/board?platform=wise&tab=realtime"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result baiduResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var items []HotItem
	rank := 1
	for _, card := range result.Data.Cards {
		for _, contentGroup := range card.Content {
			for _, item := range contentGroup.Content {
				if item.Word == "" {
					continue
				}
				items = append(items, HotItem{
					ID:       fmt.Sprintf("baidu_%d", rank),
					Title:    item.Word,
					URL:      item.URL,
					Rank:     rank,
					Platform: "baidu",
				})
				rank++
				if rank > 50 {
					return items, nil
				}
			}
		}
	}
	return items, nil
}
