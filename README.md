# 韭菜盘 (JCP AI)

> AI 驱动的智能股票分析系统 - 多 Agent 协作，让投资决策更智能

[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61dafb.svg)](https://reactjs.org)
[![Wails](https://img.shields.io/badge/Wails-v2-red.svg)](https://wails.io)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-0.2.0-orange.svg)](https://github.com/run-bigpig/jcp/releases)

## 项目简介

韭菜盘是一款基于 Wails 框架开发的跨平台桌面应用，集成了多个 AI 大模型（OpenAI、Google Gemini、DeepSeek、Kimi、GLM 等 OpenAI 兼容接口），通过多 Agent 协作讨论的方式，为用户提供专业的股票分析和投资建议。

### 核心特性

- **多 Agent 智库** - 多个 AI 专家角色协作讨论，提供多维度分析视角
- **策略管理系统** - 灵活的策略配置，支持多 Agent 组合与独立 AI 配置
- **智能记忆系统** - 按股票隔离的长期记忆，AI 能记住历史讨论和关键结论
- **提示词增强** - AI 驱动的提示词优化，提升 Agent 响应质量
- **实时行情** - 股票实时数据、K线图表、盘口深度一应俱全
- **热点舆情** - 聚合百度、抖音、B站、头条等平台热点趋势
- **研报服务** - 专业研究报告查询和智能分析
- **MCP 扩展** - 支持 Model Context Protocol，可扩展更多工具能力
- **布局持久化** - 自动保存窗口和面板布局，下次启动自动恢复

## 技术栈

| 层级 | 技术 |
|------|------|
| **框架** | Wails v2 (Go + Web 混合桌面应用) |
| **后端** | Go 1.24 |
| **前端** | React 18 + TypeScript + Vite |
| **UI** | TailwindCSS + Lucide Icons |
| **图表** | Recharts |
| **AI** | OpenAI / Gemini / DeepSeek / Kimi / GLM 等 |
| **分词** | GSE (纯 Go 实现，无 CGO 依赖) |

## 功能展示

### 主界面
- 左侧：自选股列表与市场指数
- 中间：K线图表（支持分时/日K/周K/月K）
- 右侧：AI 智库讨论室

![主界面展示](image/1.png)

![功能展示](image/2.png)

### 核心功能模块

| 模块 | 功能描述 |
|------|----------|
| 📈 **股票行情** | 实时行情数据、多周期K线、盘口深度 |
| ⭐ **自选管理** | 添加/删除自选股、实时监控 |
| 🤖 **AI 智库** | 多 Agent 协作分析、智能问答 |
| 🎯 **策略管理** | 策略配置、Agent 组合、独立 AI 配置 |
| 🔥 **热点舆情** | 百度/抖音/B站/头条热点聚合 |
| 📊 **研报服务** | 专业研报查询与分析 |
| 💬 **会议室** | Agent 多轮讨论、MCP 工具调用 |
| 🧠 **记忆系统** | 按股票隔离的长期记忆、历史摘要、关键事实提取 |
| ✨ **提示词增强** | AI 驱动的提示词优化 |
| 🔌 **连接测试** | AI 配置连通性验证 |

## 快速开始

### 环境要求

- Go 1.24+
- Node.js 18+
- Wails CLI v2

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 克隆项目

```bash
git clone https://github.com/run-bigpig/jcp.git
cd jcp
```

### 安装依赖

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 下载 Go 依赖
go mod download
```

### 开发模式运行

```bash
wails dev
```

### 构建发布版本

```bash
# 构建当前平台
wails build

# 构建 Windows 版本
wails build -platform windows/amd64

# 构建 macOS 版本
wails build -platform darwin/amd64

# 构建 Linux 版本
wails build -platform linux/amd64
```

## 配置说明

首次运行时，需要在设置中配置 AI 模型的 API Key：

1. 点击右上角设置图标
2. 选择 AI 模型提供商（OpenAI / Gemini）
3. 填入对应的 API Key
4. 保存配置

配置文件存储在 `data/config.json`。

## 项目结构

```
ccjc/
├── main.go                 # 应用入口
├── app.go                  # 后端核心逻辑
├── wails.json              # Wails 配置
├── frontend/               # 前端项目
│   ├── src/
│   │   ├── components/     # React 组件
│   │   ├── services/       # 服务层
│   │   └── hooks/          # 自定义 Hooks
│   └── package.json
├── internal/               # Go 后端模块
│   ├── adk/                # AI 开发工具包
│   ├── services/           # 业务服务（策略管理等）
│   ├── models/             # 数据模型
│   ├── agent/              # Agent 系统
│   └── meeting/            # 会议室系统
└── data/                   # 数据存储
    ├── config.json         # 应用配置
    ├── strategies.json     # 策略配置
    └── watchlist.json      # 自选股列表
```

## AI Agent 系统

项目内置多个专家 Agent，各司其职：

| Agent | 角色 | 职责 |
|-------|------|------|
| 技术分析师 | 图表专家 | K线形态、技术指标分析 |
| 基本面分析师 | 财务专家 | 财报解读、估值分析 |
| 情绪分析师 | 舆情专家 | 市场情绪、热点追踪 |
| 风控专家 | 风险管理 | 风险评估、仓位建议 |

Agent 配置通过策略管理系统进行，支持：
- 创建多个策略，每个策略包含不同的 Agent 组合
- 为每个 Agent 或策略配置独立的 AI 模型
- 使用提示词增强功能优化 Agent 表现

## 记忆系统

项目实现了按股票隔离的智能记忆系统，让 AI 能够"记住"历史讨论：

### 核心能力

| 功能 | 说明 |
|------|------|
| **股票隔离** | 每只股票独立记忆空间，互不干扰 |
| **关键事实提取** | 自动提取讨论中的重要事实、观点、决策 |
| **历史摘要** | LLM 自动生成历史讨论摘要 |
| **相关性检索** | 基于 TF-IDF 的关键词匹配，召回相关历史 |
| **自动压缩** | 超过阈值自动压缩旧记忆，控制上下文长度 |

### 记忆结构

- **KeyFacts**: 关键事实列表（事实/观点/决策）
- **RecentRounds**: 最近 N 轮讨论详情
- **Summary**: AI 生成的历史摘要

记忆数据存储在 `data/memory/` 目录下，按股票代码分文件存储。

## MCP 扩展

支持 Model Context Protocol，可扩展以下工具：

- 股票实时行情查询
- K线数据获取
- 盘口深度数据
- 新闻资讯搜索
- 研报查询
- 热点舆情获取

## 开发指南

### 添加新的 AI 工具

1. 在 `internal/adk/tools/` 下创建工具文件
2. 实现 `Tool` 接口
3. 在 `registry.go` 中注册工具

### 添加新的 Agent

1. 编辑 `data/agents.json`
2. 配置 Agent 的名称、角色、系统提示词
3. 重启应用生效

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 提交 Pull Request

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 社区

- [LINUX DO](https://linux.do/) - 真诚、友善、团结、专业，共建你我引以为荣之社区

## 致谢

- [Wails](https://wails.io/) - 优秀的 Go 桌面应用框架
- [React](https://reactjs.org/) - 前端 UI 框架
- [TailwindCSS](https://tailwindcss.com/) - CSS 框架
- [Recharts](https://recharts.org/) - 图表库
- [GSE](https://github.com/go-ego/gse) - 高性能中文分词库

---

## OpenClaw Skill 集成

### 简介

韭菜盘 (JCP AI) 现已支持作为 OpenClaw Skill 使用。通过简化版 HTTP API，OpenClaw 助手可以直接调用 JCP 的核心分析功能，实现智能股票分析。

### 快速开始

#### 1. 获取 Skill

从 `feature/openclaw-skill` 分支获取完整 skill：

```bash
git clone https://github.com/NKingpp/jcp.git
cd jcp
git checkout feature/openclaw-skill
```

或直接下载 skill 目录：
```
skill/jcp-stock-analysis/
```

#### 2. 启动 API 服务

```bash
cd skill/jcp-stock-analysis/scripts
./start.sh
```

服务将在 `http://localhost:8080` 启动。

#### 3. 配置 LLM API

```bash
curl -X POST http://localhost:8080/configure \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "baseUrl": "https://integrate.api.nvidia.com/v1/",
    "apiKey": "your-api-key",
    "modelName": "moonshotai/kimi-k2.5"
  }'
```

#### 4. 在 OpenClaw 中使用

配置完成后，OpenClaw 助手可以直接调用：

```
分析 600519 贵州茅台的投资价值
```

或更详细的查询：
```
请从技术面、基本面和风险三个角度分析 000001 平安银行
```

### Skill 架构

```
skill/jcp-stock-analysis/
├── SKILL.md                          # OpenClaw 主技能文件
├── README.md                          # 快速参考指南
├── scripts/
│   ├── jcp-api                        # 预编译二进制 (62MB, macOS x64)
│   └── start.sh                       # 便捷启动脚本
└── references/
    ├── API_REFERENCE.md              # 完整 API 文档
    ├── TIMEOUT.md                     # 超时参数优化指南
    └── AGENT_CUSTOMIZATION.md        # 代理自定义指南
```

### 核心特性

- **三专家系统**: 技术分析师、基本面分析师、风控专家并行分析
- **OpenAI 兼容**: 支持多种 LLM provider (OpenAI, NVIDIA, DeepSeek, Kimi, GLM 等)
- **独立运行**: 预编译二进制，无需 Go 安装即可使用
- **优化的超时**: 300秒单专家超时，100% 成功率
- **完整文档**: 包含 API 参考、使用指南、自定义教程

### API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/status` | GET | 获取配置状态 |
| `/configure` | POST | 配置 LLM API |
| `/analyze` | POST | 股票分析 |

详细 API 文档请参考 `skill/jcp-stock-analysis/references/API_REFERENCE.md`。

### 平台支持

**当前版本**:
- ✅ macOS x64 (Darwin amd64)
- ⏳ Linux x64 (需自行编译)
- ⏳ Windows (需交叉编译)

**其他平台编译说明**:

```bash
# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -o skill/jcp-stock-analysis/scripts/jcp-api-linux cmd/api/main.go

# Windows (amd64)
GOOS=windows GOARCH=amd64 go build -o skill/jcp-stock-analysis/scripts/jcp-api.exe cmd/api/main.go
```

### 性能指标

| 指标 | 数值 |
|------|------|
| 二进制大小 | 62 MB (macOS x64) |
| 内存占用 | ~500MB (运行时) |
| 响应时间 | 4-5 分钟 (完整三专家分析) |
| 专家完成率 | 3/3 (100%) |
| 超时设置 | 300秒 per agent |

### 自定义配置

#### 修改超时参数

编辑 `internal/meeting/service.go`:

```go
const (
    AgentTimeout = 300 * time.Second  // 单专家超时
    MeetingTimeout = 10 * time.Minute  // 整个会议超时
)
```

#### 自定义专家角色

编辑 `cmd/api/main.go`:

```go
agents := []models.AgentConfig{
    {
        ID: "1",
        Name: "量化分析师",
        Role: "量化交易",
        Instruction: "从量化角度分析，关注数学模型和统计指标",
    },
    // ... 更多自定义角色
}
```

详细指南请参考：
- `skill/jcp-stock-analysis/references/TIMEOUT.md` - 超时优化
- `skill/jcp-stock-analysis/references/AGENT_CUSTOMIZATION.md` - 代理自定义

### 故障排除

#### 问题: "AI not configured" 错误

**解决方案**: 先调用 `/configure` 端点配置 LLM API。

#### 问题: Agent 超时

**解决方案**: 当前已配置 300秒超时，适用于 Kimi/GLM 等慢速模型。如使用 GPT-3.5-turbo 等快速模型，可减少超时至 120-180秒。

#### 问题: 端口被占用

**解决方案**: 使用不同的端口:

```bash
PORT=9090 ./jcp-api
```

### 开源贡献

OpenClaw Skill 部分的开发遵循敏捷开发原则，欢迎贡献：

1. Fork 项目
2. 创建特性分支 `git checkout -b feature/your-improvement`
3. 提交更改
4. 推送到 `feature/openclaw-skill` 分支或新建分支
5. 提交 Pull Request

### 配置存储

API 配置持久化存储在:
```
~/.jcp-api/config.json
```

### 相关资源

- [OpenClaw 文档](https://docs.openclaw.ai)
- [JCP 原始项目](https://github.com/run-bigpig/jcp)
- [ClawHub - OpenClaw Skills](https://www.clawhub.com)

### 版本说明

OpenClaw Skill 遵循 JCP 项目的版本管理:
- 当前版本: 0.2.0
- 特性分支: `feature/openclaw-skill`
- 推荐总是使用最新的 feature 分支

### 许可证

OpenClaw Skill 部分遵循主项目的 MIT 许可证。
