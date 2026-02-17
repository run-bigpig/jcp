# Timeout Parameter Optimization Guide

## Overview

The JCP Stock Analysis API uses timeout parameters to control agent execution time. Proper timeout settings are critical for balancing response quality and system responsiveness.

## Current Configuration

As of this version, the system uses the following timeout settings (defined in `internal/meeting/service.go`):

```go
const (
    MeetingTimeout       = 10 * time.Minute  // Entire meeting timeout
    AgentTimeout         = 300 * time.Second  // Individual agent timeout
    ModeratorTimeout     = 60 * time.Second
    ModelCreationTimeout = 10 * time.Second
)
```

## Performance Testing Results

### Test with moonshotai/kimi-k2.5 (NVIDIA API)

| Timeout Setting | Success Rate | Avg Response Time | Notes |
|-----------------|--------------|-------------------|-------|
| 90 seconds | 33% (1/3 agents) | 1m30s | Too short for slow models |
| 300 seconds | 100% (3/3 agents) | 4m44s | Optimal for Kimi/GLM models |

### Model Response Time Estimates

| Model Type | Typical Agent Response Time | Recommended Timeout |
|------------|----------------------------|---------------------|
| Fast (GPT-3.5-turbo) | 30-60 seconds | 120 seconds |
| Medium (GPT-4) | 60-120 seconds | 180 seconds |
| Slow (Kimi, GLM-4) | 90-150 seconds | 300 seconds |

## Changing Timeout Settings

### Step 1: Modify Source Code

Edit `internal/meeting/service.go`:

```go
const (
    MeetingTimeout       = 10 * time.Minute  // Increase if needed
    AgentTimeout         = 300 * time.Second // Adjust per model
    // ...
)
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

## Timeout Trade-offs

### Shorter Timeout (120-180 seconds)
- ✅ Faster responses
- ✅ Lower latency
- ❌ May timeout on slow models
- ❌ Incomplete analysis (some agents may fail)

### Longer Timeout (300-600 seconds)
- ✅ Higher success rate
- ✅ Complete multi-agent analysis
- ✅ Better for complex queries
- ❌ Longer wait time
- ❌ Higher API costs (longer token usage)

## Best Practices

### 1. Match Timeout to Model Speed

- **Fast models:** 120-180 seconds
- **Medium models:** 180-240 seconds
- **Slow models:** 300-600 seconds

### 2. Consider Query Complexity

- **Simple queries:** Can use shorter timeouts
- **Complex analysis:** Longer timeouts recommended

### 3. Monitor Agent Times

Check logs for agent completion times to fine-tune timeout:

```
INFO Meeting: all agents done, got 3 responses
```

If agent warnings appear:
```
WARN Meeting: agent 1 timeout
WARN Meeting: agent 3 timeout
```

Increase `AgentTimeout`.

### 4. Use Meeting Timeout Safely

`MeetingTimeout` should be at least 3x `AgentTimeout` for 3-agent parallel execution.

Calculation:
```
MeetingTimeout >= AgentTimeout × Number of Agents
```

Example: 3 agents × 300s = 900s minimum MeetingTimeout (use 15 minutes)

## Troubleshooting

### Frequent Timeouts

**Symptoms:**
- Agent timeout warnings in logs
- Only 1/3 or 2/3 agents complete

**Solutions:**
1. Increase `AgentTimeout` by 50-100%
2. Switch to faster model
3. Simplify query complexity

### Very Slow Responses

**Symptoms:**
- Analysis takes >10 minutes
- All agents timeout

**Solutions:**
1. Check API endpoint latency
2. Try different model
3. Reduce number of agents

## Advanced: Dynamic Timeout Adjustment

For more advanced use cases, you can implement dynamic timeouts based on model and query complexity. This requires modifying the agent initialization code in `internal/meeting/service.go`.

Example concept:
```go
func getTimeoutForModel(modelName string) time.Duration {
    switch modelName {
    case "gpt-3.5-turbo":
        return 120 * time.Second
    case "gpt-4":
        return 180 * time.Second
    case "moonshotai/kimi-k2.5":
        return 300 * time.Second
    default:
        return 300 * time.Second
    }
}
```

Note: This is advanced customization requiring Go development knowledge.

## Contact & Support

For timeout-related issues or questions, refer to the JCP project documentation or check logs for detailed error messages.
