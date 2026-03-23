package openclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/run-bigpig/jcp/internal/meeting"
	"github.com/run-bigpig/jcp/internal/models"
)

// AnalyzeRequest 分析请求
type AnalyzeRequest struct {
	StockCode string `json:"stockCode"` // 股票代码
	Query     string `json:"query"`     // 分析问题
}

// AnalyzeResponse 分析响应
type AnalyzeResponse struct {
	Success bool   `json:"success"`
	Summary string `json:"summary,omitempty"` // 最终总结
	Error   string `json:"error,omitempty"`   // 错误信息
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, AnalyzeResponse{Error: "method not allowed"})
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, AnalyzeResponse{Error: "invalid request body"})
		return
	}

	if req.StockCode == "" || req.Query == "" {
		writeJSON(w, http.StatusBadRequest, AnalyzeResponse{Error: "stockCode and query required"})
		return
	}

	// 获取股票实时数据
	stock, err := s.stockResolver(req.StockCode)
	if err != nil || stock == nil {
		log.Error("获取股票数据失败: %s, %v", req.StockCode, err)
		writeJSON(w, http.StatusBadRequest, AnalyzeResponse{Error: "failed to get stock data"})
		return
	}

	// 获取 AI 配置
	aiConfig := s.aiResolver("")
	if aiConfig == nil {
		writeJSON(w, http.StatusServiceUnavailable, AnalyzeResponse{Error: "AI not configured"})
		return
	}

	// 获取全部专家
	agents := s.resolveAgents()
	if len(agents) == 0 {
		writeJSON(w, http.StatusBadRequest, AnalyzeResponse{Error: "no agents available"})
		return
	}

	chatReq := meeting.ChatRequest{
		Stock:     *stock,
		Agents:    agents,
		AllAgents: agents,
		Query:     req.Query,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// 使用现有的 RunSmartMeetingWithCallback 方法
	var summary string
	_, err = s.meetingService.RunSmartMeetingWithCallback(ctx, aiConfig, chatReq, func(resp meeting.ChatResponse) {
		if resp.MsgType == "summary" {
			summary = resp.Content
		}
	}, nil)
	if err != nil {
		log.Error("分析失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, AnalyzeResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, AnalyzeResponse{Success: true, Summary: summary})
}

// resolveAgents 获取全部可用专家配置
func (s *Server) resolveAgents() []models.AgentConfig {
	var configs []models.AgentConfig
	for _, a := range s.agentContainer.GetAllAgents() {
		configs = append(configs, models.AgentConfig{
			ID: a.GetID(), Name: a.GetName(),
			Role: a.GetRole(), Instruction: a.GetInstruction(),
		})
	}
	return configs
}
