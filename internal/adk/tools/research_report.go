package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetResearchReportInput 研报查询输入参数
type GetResearchReportInput struct {
	Code         string `json:"code,omitempty" jsonschema:"股票代码，如 sz000001 或 000001"`
	Symbol       string `json:"symbol,omitempty" jsonschema:"兼容字段：股票代码"`
	StockCode    string `json:"stockCode,omitempty" jsonschema:"兼容字段：股票代码"`
	Ticker       string `json:"ticker,omitempty" jsonschema:"兼容字段：股票代码"`
	SecurityCode string `json:"securityCode,omitempty" jsonschema:"兼容字段：股票代码"`
	SecuCode     string `json:"secuCode,omitempty" jsonschema:"兼容字段：股票代码"`
	PageSize     int    `json:"pageSize,omitzero" jsonschema:"每页数量，默认10"`
	PageNo       int    `json:"pageNo,omitzero" jsonschema:"页码，默认1"`
}

// GetResearchReportOutput 研报查询输出
type GetResearchReportOutput struct {
	Data       string `json:"data" jsonschema:"研报数据"`
	TotalCount int    `json:"totalCount" jsonschema:"总数量"`
}

// createResearchReportTool 创建研报查询工具
func (r *Registry) createResearchReportTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetResearchReportInput) (GetResearchReportOutput, error) {
		code := resolveStockCodeFromCandidates(
			ctx,
			input.Code,
			input.Symbol,
			input.StockCode,
			input.Ticker,
			input.SecurityCode,
			input.SecuCode,
		)
		fmt.Printf("[Tool:get_research_report] 调用开始, code=%s, pageSize=%d, pageNo=%d\n",
			code, input.PageSize, input.PageNo)

		if code == "" {
			fmt.Println("[Tool:get_research_report] 错误: 未提供股票代码")
			return GetResearchReportOutput{Data: "请提供股票代码"}, nil
		}

		pageSize := input.PageSize
		if pageSize == 0 {
			pageSize = 10
		}
		pageNo := input.PageNo
		if pageNo == 0 {
			pageNo = 1
		}

		result, err := r.researchReportService.GetResearchReports(code, pageSize, pageNo)
		if err != nil {
			fmt.Printf("[Tool:get_research_report] 错误: %v\n", err)
			return GetResearchReportOutput{}, err
		}

		text := r.researchReportService.FormatReportsToText(result.Data)
		fmt.Printf("[Tool:get_research_report] 调用完成, 返回%d条研报\n", len(result.Data))

		return GetResearchReportOutput{
			Data:       text,
			TotalCount: result.TotalCount,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_research_report",
		Description: "获取个股研报列表，包括券商评级、研究员、预测EPS/PE等信息",
	}, handler)
}

// GetReportContentInput 研报内容查询输入参数
type GetReportContentInput struct {
	InfoCode string `json:"infoCode" jsonschema:"研报唯一标识码，从研报列表中获取"`
}

// GetReportContentOutput 研报内容查询输出
type GetReportContentOutput struct {
	Content string `json:"content" jsonschema:"研报正文内容"`
	PDFUrl  string `json:"pdfUrl" jsonschema:"PDF下载链接"`
}

// createReportContentTool 创建研报内容查询工具
func (r *Registry) createReportContentTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetReportContentInput) (GetReportContentOutput, error) {
		fmt.Printf("[Tool:get_report_content] 调用开始, infoCode=%s\n", input.InfoCode)

		if input.InfoCode == "" {
			fmt.Println("[Tool:get_report_content] 错误: 未提供 infoCode")
			return GetReportContentOutput{Content: "请提供研报的 infoCode"}, nil
		}

		result, err := r.researchReportService.GetReportContent(input.InfoCode)
		if err != nil {
			fmt.Printf("[Tool:get_report_content] 错误: %v\n", err)
			return GetReportContentOutput{}, err
		}

		fmt.Printf("[Tool:get_report_content] 调用完成, 内容长度=%d\n", len(result.Content))

		return GetReportContentOutput{
			Content: result.Content,
			PDFUrl:  result.PDFUrl,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_report_content",
		Description: "获取研报正文内容，需要先通过 get_research_report 获取研报列表中的 infoCode",
	}, handler)
}
