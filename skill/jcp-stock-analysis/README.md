# JCP Stock Analysis Skill - Quick Start

This is a self-contained OpenClaw skill for AI-powered Chinese A-share stock analysis.

## What This Skill Does

Analyzes Chinese stocks using three expert AI agents:
- **技术分析师** - Technical analysis (K-line, indicators)
- **基本面分析师** - Fundamental analysis (financials, valuation)
- **风控专家** - Risk assessment

## Prerequisites

1. **LLM API Key** - OpenAI-compatible API (OpenAI, NVIDIA, DeepSeek, Kimi, GLM, etc.)
2. **macOS/Linux** - The binary is compiled for macOS (Darwin x64)
3. **Go (optional)** - Only needed if you want to rebuild from source

## Quick Start (5 minutes)

### 1. Configure LLM API

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

### 2. Analyze a Stock

```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "stockCode": "600519",
    "query": "分析投资价值"
  }'
```

### 3. Wait 4-5 Minutes

The three agents work in parallel and will return comprehensive analysis.

## Starting the API Server

**Option 1: Use the start script (recommended)**
```bash
cd scripts
./start.sh
```

**Option 2: Direct binary execution**
```bash
cd scripts
./jcp-api
```

**Option 3: Custom port**
```bash
PORT=9090 ./jcp-api
```

The server will print:
```
Starting JCP API on port 8080
```

## Example Stock Codes

| Code | Name |
|------|------|
| 600519 | 贵州茅台 |
| 000001 | 平安银行 |
| 600036 | 招商银行 |
| 000858 | 五粮液 |
| 600000 | 浦发银行 |

## Example Queries

```
分析 600519 的投资价值
```

```
请从技术面和基本面分析 000001
```

```
评估 600036 的风险因素
```

```
全面分析 000858，包括技术面、基本面和风险
```

## Troubleshooting

### "AI not configured" error

Solution: Configure the LLM API first (see step 1 above).

### Analysis times out

Solution: The system uses a 300-second timeout per agent. This is optimized for models like Kimi/GLM. For faster models (GPT-3.5-turbo), you can reduce the timeout by modifying the source code.

### Port already in use

Solution: Use a different port:
```bash
PORT=9090 ./jcp-api
```

## System Requirements

- **Storage:** ~70MB for the binary
- **Memory:** ~500MB RAM (during analysis)
- **Network:** Internet connection to LLM API
- **OS:** macOS (Darwin x64) or Linux

## Supported LLM Providers

| Provider | baseUrl Example |
|----------|-----------------|
| OpenAI | `https://api.openai.com/v1` |
| NVIDIA | `https://integrate.api.nvidia.com/v1/` |
| DeepSeek | `https://api.deepseek.com/v1` |
| Moonshot (Kimi) | Use NVIDIA or DeepSeek proxy |
| GLM | `https://open.bigmodel.cn/api/paas/v4/` |

## Advanced Usage

- **Customize agents:** See `references/AGENT_CUSTOMIZATION.md`
- **Adjust timeouts:** See `references/TIMEOUT.md`
- **Complete API reference:** See `references/API_REFERENCE.md`

## Configuration Files

The API stores configuration in `~/.jcp-api/config.json`. You can edit this file directly or use the `/configure` API endpoint.

## Performance

- **Typical response time:** 4-5 minutes with Kimi/GLM models
- **Success rate:** 100% with current settings
- **Agents:** 3 parallel agents

## Support

For issues or questions, refer to the detailed documentation in the `references/` directory.

## License

This skill is based on the JCP (韭菜盘) project. See main project LICENSE for details.
