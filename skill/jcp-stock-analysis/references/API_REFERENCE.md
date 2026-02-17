# JCP Stock Analysis API - Complete Reference

## Base URL

Default: `http://localhost:8080`

Override with `PORT` environment variable:
```bash
PORT=9090 ./scripts/jcp-api
```

## Health Check

### `GET /health`

Check if the API server is running.

**Response:**
```json
{
  "status": "ok"
}
```

**Status Code:** 200

---

## Get Configuration Status

### `GET /status`

Get current AI configuration status.

**Response (configured):**
```json
{
  "success": true,
  "configured": true,
  "provider": "openai",
  "modelName": "moonshotai/kimi-k2.5"
}
```

**Response (not configured):**
```json
{
  "success": true,
  "configured": false,
  "provider": "",
  "modelName": ""
}
```

**Status Code:** 200

---

## Configure AI Provider

### `POST /configure`

Configure the LLM API provider for stock analysis.

**Request Headers:**
```
Content-Type: application/json
```

**Request Body:**
```json
{
  "provider": "openai",
  "baseUrl": "https://integrate.api.nvidia.com/v1/",
  "apiKey": "nvapi-xxxxx",
  "modelName": "moonshotai/kimi-k2.5"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | Yes | `"openai"`, `"gemini"`, or `"vertexai"` |
| `baseUrl` | string | Yes | API endpoint URL |
| `apiKey` | string | Yes | Your API key |
| `modelName` | string | Yes | Model name to use |

**Response (success):**
```json
{
  "success": true
}
```

**Response (error):**
```json
{
  "success": false
}
```

**Status Codes:**
- 200: Configuration saved successfully
- 400: Invalid request

**Example - OpenAI:** `https://api.openai.com/v1`
**Example - NVIDIA:** `https://integrate.api.nvidia.com/v1/`
**Example - Chinese Provider:** `https://your-provider.com/v1`

---

## Analyze Stock

### `POST /analyze`

Analyze a stock using three expert agents (technical, fundamental, risk).

**Request Headers:**
```
Content-Type: application/json
```

**Request Body:**
```json
{
  "stockCode": "600519",
  "query": "请分析这只股票的投资价值"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `stockCode` | string | Yes | Stock code (e.g., "600519", "000001") |
| `query` | string | Yes | Analysis question or topic |

**Response (success):**
```json
{
  "success": true,
  "results": [
    {
      "agentId": "1",
      "agentName": "技术分析师",
      "role": "技术分析",
      "content": "**基本面**：白酒绝对龙头，品牌护城河深厚...\n\n**技术面**：长期趋势稳健...",
      "round": 0,
      "msgType": ""
    },
    {
      "agentId": "2",
      "agentName": "基本面分析师",
      "role": "基本面分析",
      "content": "基本面：绝对龙头，品牌护城河深厚，现金流充沛...",
      "round": 0,
      "msgType": ""
    },
    {
      "agentId": "3",
      "agentName": "风控专家",
      "role": "风险管理",
      "content": "基本面稳健，品牌护城河深厚，现金流充裕...",
      "round": 0,
      "msgType": ""
    }
  ]
}
```

**Response (error - AI not configured):**
```json
{
  "success": false,
  "message": "AI not configured"
}
```

**Response (error - analysis failed):**
```json
{
  "success": false,
  "message": "analysis failed: <error details>"
}
```

**Response Fields:**
- `success`: Whether analysis succeeded
- `results`: Array of agent responses (on success)
- `message`: Error message (on failure)

**Agent Response Fields:**
- `agentId`: Agent identifier (1, 2, or 3)
- `agentName`: Agent display name
- `role`: Agent role description
- `content`: Analysis text (may contain markdown)
- `round`: Round number (always 0 in current version)
- `msgType`: Message type (always empty in current version)

**Status Codes:**
- 200: Analysis completed successfully
- 400: Invalid request or AI not configured
- 500: Analysis error

**Typical Response Time:** 4-5 minutes (300-second timeout per agent)

---

## Stock Code Format

### Shanghai Stock Exchange
- Code: `600xxx`, `601xxx`, `603xxx`, `688xxx` (STAR)
- Example: `600519` (贵州茅台)

### Shenzhen Stock Exchange
- Main Board: `000xxx`, `001xxx`
- SME: `002xxx`
- ChiNext: `300xxx`
- Example: `000001` (平安银行)

### Beijing Stock Exchange (BSE)
- Code: `8xxxxx`, `4xxxxx`
- Not currently supported

---

## Query Examples

### Basic Analysis
```json
{
  "stockCode": "600519",
  "query": "分析投资价值"
}
```

### Technical Focus
```json
{
  "stockCode": "600036",
  "query": "从技术面分析，关注K线形态和支撑位"
}
```

### Fundamental Focus
```json
{
  "stockCode": "000001",
  "query": "分析财务状况和估值水平"
}
```

### Risk Assessment
```json
{
  "stockCode": "002594",
  "query": "评估主要风险因素"
}
```

### Comprehensive Analysis
```json
{
  "stockCode": "600519",
  "query": "请全面分析技术面、基本面和风险因素，给出投资建议"
}
```

---

## Error Handling

### Common Errors

**Error: "AI not configured"**
- Cause: LLM API not configured
- Solution: Call `/configure` endpoint first

**Error: "invalid request"**
- Cause: Invalid JSON or missing required fields
- Solution: Check request body format

**Error: "analysis failed"**
- Cause: LLM API error or network issue
- Solution: Check API key, network connection, and logs

### Rate Limiting

The API does not implement rate limiting. However, your LLM provider (OpenAI, etc.) may have rate limits. Monitor your API usage accordingly.

---

## CORS

The API enables CORS for all origins by default. This allows browser-based applications to make cross-origin requests.

If you need to restrict CORS, modify the CORS middleware in `cmd/api/main.go`:

```go
e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins: []string{"https://your-domain.com"},
    AllowMethods: []string{GET, POST, PUT, DELETE},
    AllowHeaders: []string{Origin, Content-Type},
}))
```

---

## Configuration Storage

API configuration is persisted to `~/.jcp-api/config.json`. You don't need to reconfigure after server restarts.

To clear configuration:
```bash
rm ~/.jcp-api/config.json
```

---

## Logging

The API logs to stdout with Echo's default logger. Enable more verbose logging by modifying the logger configuration in `cmd/api/main.go`.

Important log messages:
- `INFO Meeting: model created successfully` - LLM initialized
- `WARN Meeting: agent X timeout` - Agent timed out
- `INFO Meeting: all agents done, got N responses` - Analysis complete

---

## Security Considerations

1. **API Key Storage:** API keys are stored in plain text in `~/.jcp-api/config.json`. Ensure proper file permissions.

2. **Network Exposure:** By default, the API binds to `0.0.0.0:8080`, making it accessible from other machines. Use firewall rules to restrict access if needed.

3. **HTTPS:** The API uses HTTP only. For production use, consider using a reverse proxy (nginx, Caddy) with SSL/TLS termination.

---

## Version Information

Current version corresponds to JCP project commit. Check `jcp-api` binary for version details using:

```bash
./scripts/jcp-api --version
```

(If version flag is implemented)

---

## Testing the API

### Using curl

```bash
# Health check
curl http://localhost:8080/health

# Check status
curl http://localhost:8080/status

# Configure AI
curl -X POST http://localhost:8080/configure \
  -H "Content-Type: application/json" \
  -d '{"provider":"openai","baseUrl":"https://api.openai.com/v1","apiKey":"sk-xxx","modelName":"gpt-4"}'

# Analyze stock
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"stockCode":"600519","query":"分析投资价值"}'
```

### Using Python

```python
import requests

# Analyze stock
response = requests.post(
    "http://localhost:8080/analyze",
    json={"stockCode": "600519", "query": "分析投资价值"}
)
data = response.json()

for agent in data["results"]:
    print(f"{agent['agentName']}: {agent['content']}")
```

### Using JavaScript (Node.js)

```javascript
const axios = require('axios');

// Analyze stock
axios.post('http://localhost:8080/analyze', {
  stockCode: '600519',
  query: '分析投资价值'
})
.then(response => {
  response.data.results.forEach(agent => {
    console.log(`${agent.agentName}: ${agent.content}`);
  });
});
```

---

For additional questions or issues, refer to other documentation files or the JCP project repository.
