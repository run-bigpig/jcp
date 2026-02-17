# Agent Customization Guide

## Overview

The JCP Stock Analysis API uses a three-agent system for stock analysis. This guide explains how to customize agent roles, prompts, and behavior.

## Default Agent Configuration

The default agents are defined in `cmd/api/main.go`:

```go
agents := []models.AgentConfig{
    {
        ID:          "1",
        Name:        "技术分析师",
        Role:        "技术分析",
        Instruction: "从技术角度分析",
    },
    {
        ID:          "2",
        Name:        "基本面分析师",
        Role:        "基本面分析",
        Instruction: "从基本面角度分析",
    },
    {
        ID:          "3",
        Name:        "风控专家",
        Role:        "风险管理",
        Instruction: "从风险角度分析",
    },
}
```

## Agent Fields

| Field | Type | Description |
|-------|------|-------------|
| `ID` | string | Unique identifier (1, 2, 3, etc.) |
| `Name` | string | Display name shown in responses |
| `Role` | string | Role description or title |
| `Instruction` | string | System prompt for the agent |

## Customizing Agents

### Step 1: Modify Agent Configuration

Edit `cmd/api/main.go` in the `Analyze` function to change agent definitions:

```go
agents := []models.AgentConfig{
    {
        ID:          "1",
        Name: "量化分析师",
        Role: "量化交易",
        Instruction: "从量化角度分析，关注数学模型和统计指标",
    },
    {
        ID:          "2",
        Name: "行业专家",
        Role: "行业研究",
        Instruction: "从行业发展、竞争格局和政策环境角度分析",
    },
    {
        ID:          "3",
        Name: "宏观经济分析师",
        Role: "宏观经济",
        Instruction: "从宏观经济和政策周期角度分析",
    },
}
```

### Step 2: Recompile

```bash
cd /path/to/jcp
go build -o skill/jcp-stock-analysis/scripts/jcp-api cmd/api/main.go
```

### Step 3: Restart Server

```bash
./scripts/jcp-api
```

## Agent Prompting Best Practices

### 1. Be Specific About Perspective

Bad:
```go
Instruction: "分析股票"
```

Good:
```go
Instruction: "从技术角度分析，重点关注K线形态、技术指标和支撑位"
```

### 2. Define Output Format

```go
Instruction: "从基本面分析财务状况。按以下格式输出：\n1. 盈利能力分析\n2. 偿债能力分析\n3. 成长性分析\n4. 投资建议"
```

### 3. Avoid Overlapping Responsibilities

Overlapping:
- Agent 1: "分析技术面和基本面"
- Agent 2: "分析基本面和风险"

Better separation:
- Agent 1: "从技术角度分析"
- Agent 2: "从基本面角度分析"
- Agent 3: "从风险角度分析"

### 4. Consider Model Capabilities

Different LLM models have different strengths. Adjust prompts accordingly:

**For detailed models (GPT-4, Claude):**
```go
Instruction: "进行深入的技术分析，MACD、KDJ、RSI 等所有常用指标"
```

**For faster models (GPT-3.5-turbo):**
```go
Instruction: "快速评估技术面，关注关键指标和趋势"
```

## Custom Agent Examples

### Example 1: Value Investing Focus

```go
agents := []models.AgentConfig{
    {
        ID:          "1",
        Name: "价值分析师",
        Role: "价值投资",
        Instruction: "从价值投资角度分析，关注市盈率、市净率、股息率等估值指标",
    },
    {
        ID:          "2",
        Name: "成长分析师",
        Role: "成长性",
        Instruction: "从成长性角度分析，关注营收增长、利润增长和市场份额",
    },
    {
        ID:          "3",
        Name: "安全边际分析师",
        Role: "安全边际",
        Instruction: "评估安全边际，分析下行风险和最坏情景",
    },
}
```

### Example 2: Sector-Specific Analysis

```go
agents := []models.AgentConfig{
    {
        ID:          "1",
        Name: "金融业专家",
        Role: "银行业",
        Instruction: "针对银行股，分析息差变化、资产质量和监管政策影响",
    },
    {
        ID:          "2",
        Name: "信贷周期分析师",
        Role: "信贷周期",
        Instruction: "分析信贷周期对银行业绩的影响",
    },
    {
        ID:          "3",
        Name: "利率市场化专家",
        Role: "利率影响",
        Instruction: "评估利率市场化对银行收入结构的影响",
    },
}
```

### Example 3: Simplified Analysis (2 Agents)

```go
agents := []models.AgentConfig{
    {
        ID:          "1",
        Name: "综合分析师",
        Role: "综合分析",
        Instruction: "从技术面和基本面综合分析",
    },
    {
        ID:          "2",
        Name: "风控专家",
        Role: "风险管理",
        Instruction: "评估投资风险并给出建议",
    },
}
```

## Prompt Engineering Techniques

### Chain-of-Thought

Encourage step-by-step reasoning:

```go
Instruction: "从技术角度分析。请按以下步骤：1. 观察K线形态 2. 计算技术指标 3. 识别支撑压力位 4. 给出操作建议"
```

### Few-Shot Examples

While you can't include examples in the instruction directly, you can reference the query format:

```go
Instruction: "参考查询中的具体股票和市场情况进行分析"
```

### Role Adoption

Clear role definition helps the model stay in character:

```go
Instruction: "你是一位有10年经验的技术分析专家，擅长K线形态识别和趋势判断"
```

## Adding More Agents

You can increase the number of agents by adding more entries to the array:

```go
agents := []models.AgentConfig{
    {ID: "1", Name: "技术分析师", Role: "技术分析", Instruction: "..."},
    {ID: "2", Name: "基本面分析师", Role: "基本面分析", Instruction: "..."},
    {ID: "3", Name: "风控专家", Role: "风险管理", Instruction: "..."},
    {ID: "4", Name: "量化分析师", Role: "量化分析", Instruction: "..."},
    {ID: "5", Name: "行业专家", Role: "行业研究", Instruction: "..."},
}
```

**Considerations:**
- More agents = longer response time
- Timeout settings may need adjustment
- Ensure diverse perspectives to avoid redundancy

## Removing Agents

Reduce the array to fewer agents:

```go
agents := []models.AgentConfig{
    {ID: "1", Name: "综合分析师", Role: "综合分析", Instruction: "从技术面和基本面全面分析"},
}
```

**Benefits:**
- Faster response time
- Lower API costs
- Simpler to understand

**Drawbacks:**
- Less diverse perspectives
- May overlook certain aspects

## Advanced: Dynamic Agent Selection

For more sophisticated use cases, you can implement dynamic agent selection based on query type or stock sector.

This requires modifying the `Analyze` function in `cmd/api/main.go` to use conditional logic:

```go
var agents []models.AgentConfig

if strings.Contains(req.Query, "技术") || strings.Contains(req.Query, "K线") {
    agents = technicalAnalysisAgents
} else if strings.Contains(req.Query, "基本面") {
    agents = fundamentalAnalysisAgents
} else {
    agents = defaultAgents
}
```

Note: This is advanced customization requiring Go programming knowledge.

## Testing Agent Configuration

After modifying agents, test with various stocks and queries:

```bash
# Test with different stocks
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"stockCode":"600519","query":"从新角色角度分析"}'

# Test with different queries
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"stockCode":"000001","query":"评估行业风险"}'
```

Verify that:
1. All agents complete successfully (no timeouts)
2. Agent outputs are distinct and non-redundant
3. Responses are relevant to the query
4. Response time is acceptable

## Common Issues

### Issue: Agents give similar responses

**Cause:** Too much overlap in instructions

**Solution:** Clarify each agent's distinct perspective

### Issue: Agent timeouts

**Cause:** Complex prompts + slow model = timeout

**Solution:** Simplify prompts or increase timeout (see [TIMEOUT.md](TIMEOUT.md))

### Issue: Responses are too generic

**Cause:** Instructions too vague

**Solution:** Be more specific about expected output format and focus areas

### Issue: Agents ignore instructions

**Cause:** Instructions too long or complex

**Solution:** Keep instructions concise (100 characters or less recommended)

## Example: Production-Ready Configuration

```go
agents := []models.AgentConfig{
    {
        ID:          "1",
        Name: "技术分析专家",
        Role: "技术分析",
        Instruction: "专注K线形态、技术指标、支撑压力位，给出具体操作建议",
    },
    {
        ID:          "2",
        Name: "价值投资专家",
        Role: "价值分析",
        Instruction: "关注财务质量、估值水平、安全边际，评估长期投资价值",
    },
    {
        ID:          "3",
        Name: "风险控制专家",
        Role: "风险控制",
        Instruction: "识别主要风险点，评估影响程度，提出风险对冲建议",
    },
}
```

For more examples or inspiration, see the JCP project's main codebase for advanced agent configurations.
