package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/proxy"
)

const (
	eastmoneyReportAPI = "https://reportapi.eastmoney.com/report/list"
)

// ResearchReport 个股研报数据结构
type ResearchReport struct {
	Title              string `json:"title"`              // 研报标题
	StockName          string `json:"stockName"`          // 股票名称
	StockCode          string `json:"stockCode"`          // 股票代码
	OrgSName           string `json:"orgSName"`           // 券商简称
	PublishDate        string `json:"publishDate"`        // 发布日期
	PredictThisYearEps string `json:"predictThisYearEps"` // 今年预测EPS
	PredictThisYearPe  string `json:"predictThisYearPe"`  // 今年预测PE
	PredictNextYearEps string `json:"predictNextYearEps"` // 明年预测EPS
	PredictNextYearPe  string `json:"predictNextYearPe"`  // 明年预测PE
	IndvInduName       string `json:"indvInduName"`       // 行业名称
	EmRatingName       string `json:"emRatingName"`       // 评级名称
	Researcher         string `json:"researcher"`         // 研究员
	EncodeUrl          string `json:"encodeUrl"`          // 报告链接编码
	InfoCode           string `json:"infoCode"`           // 研报唯一标识码
}

// ReportContentResponse 研报内容响应
type ReportContentResponse struct {
	Content string `json:"content"` // 研报正文内容
	PDFUrl  string `json:"pdfUrl"`  // PDF下载链接
}

// ResearchReportResponse API 响应结构
type ResearchReportResponse struct {
	Data       []ResearchReport `json:"data"`
	TotalPage  int              `json:"TotalPage"`
	TotalCount int              `json:"TotalCount"`
}

// ResearchReportService 研报服务
type ResearchReportService struct {
	client *http.Client
}

// NewResearchReportService 创建研报服务
func NewResearchReportService() *ResearchReportService {
	return &ResearchReportService{
		client: proxy.GetManager().GetClientWithTimeout(15 * time.Second),
	}
}

// GetResearchReports 获取个股研报
// stockCode: 股票代码 (如 "000001"，支持带前缀如 "sz000001")
// pageSize: 每页数量
// pageNo: 页码
func (s *ResearchReportService) GetResearchReports(stockCode string, pageSize, pageNo int) (*ResearchReportResponse, error) {
	// 去除股票代码前缀
	code := strings.TrimPrefix(stockCode, "sz")
	code = strings.TrimPrefix(code, "sh")

	// 构建请求URL
	url := fmt.Sprintf("%s?industryCode=*&pageSize=%d&industry=*&rating=*&ratingChange=*&beginTime=2020-01-01&endTime=%d-01-01&pageNo=%d&fields=&qType=0&orgCode=&code=%s&rcode=",
		eastmoneyReportAPI, pageSize, time.Now().Year()+1, pageNo, code)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result ResearchReportResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}

// FormatReportsToText 将研报数据格式化为文本
func (s *ResearchReportService) FormatReportsToText(reports []ResearchReport) string {
	if len(reports) == 0 {
		return "暂无研报数据"
	}

	var sb strings.Builder
	for i, r := range reports {
		sb.WriteString(fmt.Sprintf("%d. 【%s】%s\n", i+1, r.EmRatingName, r.Title))
		sb.WriteString(fmt.Sprintf("   券商: %s | 研究员: %s\n", r.OrgSName, r.Researcher))
		sb.WriteString(fmt.Sprintf("   发布日期: %s | 行业: %s\n", r.PublishDate, r.IndvInduName))
		if r.PredictThisYearEps != "" || r.PredictThisYearPe != "" {
			sb.WriteString(fmt.Sprintf("   预测EPS: %s | 预测PE: %s\n", r.PredictThisYearEps, r.PredictThisYearPe))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// GetReportPDFUrl 根据 infoCode 生成研报 PDF 下载链接
func (s *ResearchReportService) GetReportPDFUrl(infoCode string) string {
	if infoCode == "" {
		return ""
	}
	return fmt.Sprintf("https://pdf.dfcfw.com/pdf/H3_%s_1.pdf", infoCode)
}

// GetReportContent 获取研报正文内容
// infoCode: 研报唯一标识码
func (s *ResearchReportService) GetReportContent(infoCode string) (*ReportContentResponse, error) {
	if infoCode == "" {
		return nil, fmt.Errorf("infoCode 不能为空")
	}

	// 从东方财富研报详情页面获取内容
	url := fmt.Sprintf("https://data.eastmoney.com/report/zw_stock.jshtml?infocode=%s", infoCode)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 提取正文内容
	content := s.extractReportContent(string(body))
	pdfUrl := s.GetReportPDFUrl(infoCode)

	return &ReportContentResponse{
		Content: content,
		PDFUrl:  pdfUrl,
	}, nil
}

// extractReportContent 从 HTML 中提取研报正文
func (s *ResearchReportService) extractReportContent(html string) string {
	// 提取 ctx-content 区域的内容
	startTag := `class="ctx-content"`
	startIdx := strings.Index(html, startTag)
	if startIdx == -1 {
		return "无法获取研报正文内容"
	}

	// 找到内容开始位置
	contentStart := strings.Index(html[startIdx:], ">")
	if contentStart == -1 {
		return "无法解析研报内容"
	}
	startIdx += contentStart + 1

	// 找到结束标签
	endIdx := strings.Index(html[startIdx:], "</div>")
	if endIdx == -1 {
		return "无法解析研报内容"
	}

	content := html[startIdx : startIdx+endIdx]

	// 清理 HTML 标签
	content = s.cleanHTML(content)

	return strings.TrimSpace(content)
}

// cleanHTML 清理 HTML 标签，保留纯文本
func (s *ResearchReportService) cleanHTML(html string) string {
	// 替换常见标签为换行
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br />", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n")
	html = strings.ReplaceAll(html, "</div>", "\n")

	// 移除所有 HTML 标签
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// 清理多余空白
	text := result.String()
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")

	return text
}
