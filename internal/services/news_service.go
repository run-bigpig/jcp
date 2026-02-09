package services

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

// Telegraph 快讯数据结构
type Telegraph struct {
	Time    string `json:"time"`
	Content string `json:"content"`
	URL     string `json:"url"`
}

// NewsService 资讯服务
type NewsService struct {
	client *http.Client

	// 缓存
	telegraphs    []Telegraph
	lastFetchTime time.Time
	mu            sync.RWMutex
}

// NewNewsService 创建资讯服务
func NewNewsService() *NewsService {
	return &NewsService{
		client:     proxy.GetManager().GetClientWithTimeout(10 * time.Second),
		telegraphs: make([]Telegraph, 0),
	}
}

// GetTelegraphList 获取财联社快讯列表
func (s *NewsService) GetTelegraphList() ([]Telegraph, error) {
	// 检查缓存，30秒内不重复请求
	s.mu.RLock()
	if time.Since(s.lastFetchTime) < 30*time.Second && len(s.telegraphs) > 0 {
		result := make([]Telegraph, len(s.telegraphs))
		copy(result, s.telegraphs)
		s.mu.RUnlock()
		return result, nil
	}
	s.mu.RUnlock()

	// 请求财联社快讯页面
	req, err := http.NewRequest("GET", "https://www.cls.cn/telegraph", nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头，模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析 HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	telegraphs := make([]Telegraph, 0, 20)

	// 解析快讯内容 - 查找包含 telegraph-content-box 的父级元素
	// 父级元素同时包含内容和 subject-bottom-box（含详情链接）
	doc.Find("div.telegraph-content-box").Each(func(i int, sel *goquery.Selection) {
		if i >= 20 {
			return
		}

		// 获取时间
		timeStr := sel.Find("span.telegraph-time-box").Text()
		timeStr = strings.TrimSpace(timeStr)

		// 获取内容 - 内容在 span > div 结构中
		content := sel.Find("span > div").Text()
		content = strings.TrimSpace(content)
		content = cleanContent(content)

		// 获取详情链接 - 链接在父级的兄弟元素 subject-bottom-box 中
		url := ""
		parent := sel.Parent()
		if href, exists := parent.Find("div.subject-bottom-box a[href^='/detail/']").Attr("href"); exists {
			url = "https://www.cls.cn" + href
		}

		if content != "" {
			telegraphs = append(telegraphs, Telegraph{
				Time:    timeStr,
				Content: content,
				URL:     url,
			})
		}
	})

	// 更新缓存
	s.mu.Lock()
	s.telegraphs = telegraphs
	s.lastFetchTime = time.Now()
	s.mu.Unlock()

	return telegraphs, nil
}

// GetLatestTelegraph 获取最新一条快讯
func (s *NewsService) GetLatestTelegraph() *Telegraph {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.telegraphs) > 0 {
		return &s.telegraphs[0]
	}
	return nil
}

// cleanContent 清理内容中的多余空白字符
func cleanContent(s string) string {
	// 替换多个空白字符为单个空格
	s = strings.Join(strings.Fields(s), " ")
	// 移除特殊字符
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}
