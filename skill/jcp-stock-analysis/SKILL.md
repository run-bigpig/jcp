---
name: jcp-stock-analysis
description: AI-driven intelligent A-share stock analysis system with multi-agent collaboration. Analyze Chinese stocks using multiple expert perspectives (technical, fundamental, risk) to provide comprehensive investment insights. Requires an LLM API key (OpenAI-compatible) to function. Use when you need: (1) stock analysis and investment recommendations, (2) multi-perspective evaluation with expert agents, (3) risk assessment for Chinese A-share stocks
---

# JCP Stock Analysis Skill

AI-powered stock analysis system for Chinese A-share markets. Uses three specialized agents (technical analyst, fundamental analyst, risk expert) to provide comprehensive investment insights.

## Quick Start

### 1. Start the API Server

```bash
./scripts/jcp-api
```

The server starts on port `8080` by default. Override with `PORT` environment variable:

```bash
PORT=9090 ./scripts/jcp-api
```

### 2. Configure LLM API

Before analyzing stocks, configure an LLM API provider:

```bash
curl -X POST http://localhost:8080/configure \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "baseUrl": "https://integrate.api.nvidia.com/v1/",
    "apiKey": "your-api-key-here",
    "modelName": "moonshotai/kimi-k2.5"
  }'
```

**Supported providers:**
- `openai` - OpenAI-compatible APIs (including NVIDIA, various Chinese providers)
- `gemini` - Google Gemini
- `vertexai` - Google Vertex AI

**Important:** For slow models (like Kimi, GLM), increase timeout. The system is configured with 300-second timeout per agent (see [TIMEOUT.md](references/TIMEOUT.md) for optimization guide).

### 3. Analyze a Stock

```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "stockCode": "600519",
    "query": "分析投资价值，包括技术面、基本面和风险因素"
  }'
```

**Response format:**
```json
{
  "success": true,
  "results": [
    {
      "agentId": "1",
      "agentName": "技术分析师",
      "role": "技术分析",
      "content": "技术面分析内容...",
      "round": 0,
      "msgType": ""
    },
    {
      "agentId": "2",
      "agentName": "基本面分析师",
      "role": "基本面分析",
      "content": "基本面分析内容...",
      "round": 0,
      "msgType": ""
    },
    {
      "agentId": "3",
      "agentName": "风控专家",
      "role": "风险管理",
      "content": "风险分析内容...",
      "round": 0,
      "msgType": ""
    }
  ]
}
```

### 4. Check Status

```bash
curl http://localhost:8080/status
```

Response:
```json
{
  "success": true,
  "configured": true,
  "provider": "openai",
  "modelName": "moonshotai/kimi-k2.5"
}
```

## How It Works

### Three-Agent System

| Agent | expertise | Focus Areas |
|-------|-----------|-------------|
| **技术分析师** | Technical Analysis | K-line patterns, technical indicators, support/resistance levels |
| **基本面分析师** | Fundamental Analysis | Financial reports, valuation, operating performance |
| **风控专家** | Risk Management | Risk assessment, position sizing, risk factors |

### Analysis Process

1. User submits analysis request (stock code + question)
2. System spawns 3 agents with stock and query context
3. Each agent analyzes from their perspective (parallel execution)
4. Responses are collected and returned as single JSON

### Performance Characteristics

- **Timeout per agent:** 300 seconds (configurable in meeting/service.go)
- **Total meeting timeout:** 10 minutes
- **Success rate:** 100% with current 300s timeout (tested with moonshotai/kimi-k2.5)
- **Typical response time:** 4-5 minutes for complete 3-agent analysis

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/status` | GET | Check AI configuration status |
| `/configure` | POST | Configure LLM API |
| `/analyze` | POST | Analyze stock |

## Configuration

### AI Provider Configuration

**Request body for `/configure`:**
```json
{
  "provider": "openai",
  "baseUrl": "https://api.openai.com/v1",
  "apiKey": "sk-...",
  "modelName": "gpt-4"
}
```

**Fields:**
- `provider`: `"openai"`, `"gemini"`, `"vertexai"`
- `baseUrl`: API endpoint URL
- `apiKey`: Your API key
- `modelName`: Model name

### Configuration Storage

Configuration files are stored in `~/.jcp-api/config.json`. The system persists API settings across runs.

## Usage Examples

### Example 1: Basic Analysis

**User request:**
```
分析 600519 贵州茅台
```

**OpenClaw translates to:**
```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"stockCode": "600519", "query": "分析投资价值"}'
```

### Example 2: Specific Question

**User request:**
```
请从技术面和基本面分析 000001 平安银行
```

**OpenClaw translates to:**
```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"stockCode": "000001", "query": "请从技术面和基本面分析投资价值"}'
```

### Example 3: Risk-Focused Analysis

**User request:**
```
评估 600036 招商银行的风险因素
```

**OpenClaw translates to:**
```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"stockCode": "600036", "query": "评估风险因素，包括市场风险、操作风险和信用风险"}'
```

## Troubleshooting

### Issue: "AI not configured" error

**Solution:** Configure LLM API first using `/configure` endpoint.

### Issue: Agent timeout errors

**Solution:** For slow model APIs (like Kimi, GLM), the system already uses 300-second timeout. For further optimization, see [references/TIMEOUT.md](references/TIMEOUT.md).

### Issue: Port already in use

**Solution:**
```bash
PORT=9090 ./scripts/jcp-api
```

## Technical Details

### Architecture

- **Language:** Go 1.24+
- **Web Framework:** Echo (labstack/echo/v4)
- **AI SDK:** Google ADK (Agent Development Kit)
- **Concurrency:** Parallel agent execution

### Key Files

- `main.go` - HTTP API server
- `internal/meeting/service.go` - Multi-agent orchestration
- `internal/agent/container.go` - Agent management
- Timeout settings in `internal/meeting/service.go`

### Dependencies

All dependencies are embedded in the compiled `jcp-api` binary. No external dependencies required to run the service.

## For Advanced Users

See [references/](references/) for detailed documentation:
- `TIMEOUT.md` - Timeout parameter optimization guide
- `AGENT_CUSTOMIZATION.md` - Customizing agent roles and prompts
- `API_REFERENCE.md` - Complete API reference

## Notes

- This is a simplified API version of the JCP project, optimized for OpenClaw integration
- The binary includes all Go dependencies - no need to install Go to run
- Stock market data source uses embedded databases in the binary
- For feature requests or issues, report to the JCP GitHub repo
