# Agent SDK

<p align="center">
  <strong>AI Agent 运行时框架</strong> — 通过可插拔的 <em>Skill（技能）</em>、<em>SubAgent（子代理）</em>与 <em>MCP 工具协议</em>实现领域能力的动态扩展
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/opentoys/agentsdk"><img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go" alt="Go Version"></a>
  <img src="https://img.shields.io/badge/dependencies-zero-brightgreen" alt="Zero Dependencies">
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="License">
</p>

---

## 目录

- [特性](#特性)
- [安装](#安装)
- [快速开始](#快速开始)
- [核心架构](#核心架构)
  - [设计理念](#设计理念)
  - [四层工具体系](#四层工具体系)
  - [执行流程](#执行流程)
- [SubAgent 子代理](#subagent-子代理)
  - [Agent-as-Tool 模式](#agent-as-tool-模式)
  - [Plan 编排模式](#plan-编排模式)
- [API 参考](#api-参考)
  - [Agent](#agent)
  - [Skill 系统](#skill-系统)
  - [Tool 系统](#tool-系统)
  - [VFS 虚拟文件系统](#vfs-虚拟文件系统)
  - [Memory 系统](#memory-系统)
- [模块系统](#模块系统)
  - [aichat — OpenAI Chat 客户端](#aichat--openai-chat-客户端)
  - [mcp — MCP 协议客户端](#mcp--mcp-协议客户端)
  - [log — 日志模块](#log--日志模块)
  - [dag — DAG 执行引擎](#dag--dag-执行引擎)
- [设计模式](#设计模式)
- [参考项目](#参考项目)

---

## 特性

- **零外部依赖** — 仅使用 Go 标准库，无第三方依赖
- **SubAgent 子代理** — 声明式配置子代理，LLM 自动编排多 Agent 协作
- **Plan 编排** — LLM 生成多步执行计划，支持顺序和 DAG 并行两种模式
- **Skill 插件化** — 技能以目录包形式组织，支持热发现与动态加载
- **多种文件系统** — 内置 `MemFS`、`ZipFS`，支持任意 `fs.FS` 实现
- **MCP 协议** — 支持 stdio/SSE 两种传输方式，连接外部工具服务器
- **内置工具集** — 提供 Bash、HTTP、文件读取、Web 搜索等基础工具
- **脚本工具** — 自动将技能包内的 `.py`/`.js`/`.sh` 脚本注册为工具
- **对话摘要** — LLM 驱动的多级对话压缩，自动控制上下文长度

---

## 安装

```bash
go get github.com/opentoys/agentsdk
```

要求 Go 1.23 或更高版本。

---

## 快速开始

### 基本用法

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/opentoys/agentsdk"
    "github.com/opentoys/agentsdk/modules/aichat"
    "github.com/opentoys/agentsdk/modules/log"
    "github.com/opentoys/agentsdk/tool"
    "github.com/opentoys/agentsdk/types"
)

func main() {
    chatClient := aichat.New(
        aichat.WithKey(os.Getenv("OPENAI_API_KEY")),
        aichat.WithBase(os.Getenv("OPENAI_API_BASE")),
        aichat.WithModel(os.Getenv("OPENAI_API_MODEL")),
    )

    skillsFS := os.DirFS("/path/to/skills")

    agent := agentsdk.New(types.Config{
        SkillsFS:   skillsFS,
        Debug:      &log.DefaultLog{},
        ChatClient: chatClient,
        Tools: []types.Tool{
            tool.DefineBashTool(),
            tool.DefineHTTPTool(),
        },
    })

    resp, err := agent.Run(context.Background(), "帮我分析这个数据")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp)
}
```

### 使用 SubAgent

```go
agent := agentsdk.New(types.Config{
    ChatClient: chatClient,
    SubAgents: []types.SubAgentConfig{
        {Name: "researcher", Description: "搜索和研究信息的子代理"},
        {Name: "writer",     Description: "撰写内容的子代理"},
    },
    Tools: []types.Tool{tool.DefineBashTool()},
})

// LLM 自动选择并调用子代理
agent.Run(ctx, "帮我调研 AI 趋势并写报告")
```

### 使用 Plan 编排

```go
agent := agentsdk.New(types.Config{
    ChatClient: chatClient,
    SubAgents: []types.SubAgentConfig{
        {Name: "researcher", Description: "搜索和研究信息"},
        {Name: "writer",     Description: "撰写内容"},
        {Name: "reviewer",   Description: "审查和改进内容"},
    },
    Tools: []types.Tool{tool.DefineHTTPTool()},
})

// LLM 生成计划 → 按步骤自动执行
agent.RunWithPlan(ctx, "调研 AI 趋势，写一份报告，并审查质量")
```

### 使用 VFS 内存文件系统

```go
mem := vfs.NewMem()
mem.WriteFile("my-skill/SKILL.md", []byte(skillContent))
mem.WriteFile("my-skill/scripts/run.py", []byte(scriptContent))

agent := agentsdk.New(types.Config{
    SkillsFS:   mem,
    ChatClient: chatClient,
})
```

### 使用 MCP 外部工具

```go
config, _ := mcp.LoadConfig("mcp_config.json")
mcpClient := mcp.NewClient(ctx, config)
defer mcpClient.Close()

agent := agentsdk.New(types.Config{
    ChatClient:  chatClient,
    McpSessions: mcpClient,
    SkillsFS:   os.DirFS("./skills"),
})
```

---

## 核心架构

### 设计理念

Agent 不硬编码任何领域知识，而是通过四层工具体系 + SubAgent + 可插拔模块获取能力：

```
                      ┌──────────────┐
                      │   Agent      │  ◄── 入口 / 协调者
                      │   Run/Plan   │
                      └──────┬───────┘
                             │
      ┌──────────┬───────────┼───────────┬──────────┐
      ▼          ▼           ▼           ▼          ▼
┌──────────┐ ┌────────┐ ┌────────┐ ┌─────────┐ ┌──────────┐
│ SubAgent │ │ skill/ │ │ tool/  │ │ MCP     │ │ Custom   │
│ 子代理   │ │ parser │ │ 内置   │ │ 远程    │ │ Tool     │
│ (声明式) │ │ +gen   │ │        │ │         │ │ (NewTool)│
└──────────┘ └────────┘ └────────┘ └─────────┘ └──────────┘
     │            │          │          │           │
  Agent实例    技能包      Bash/     MCP服务器    自定义
  (独立LLM)   (脚本)     HTTP/Read  (SSE/stdio)  Exec函数
```

### 四层工具体系

| 层级             | 来源                   | 说明                                                        | 示例                        |
| ---------------- | ---------------------- | ----------------------------------------------------------- | --------------------------- |
| **SubAgent**     | `Config.SubAgents`     | 声明式子代理配置，每个子代理拥有独立的 Agent 实例和消息历史 | researcher, writer, dealer  |
| **BaseTools**    | `Config.Tools`         | 通用基础能力，也可通过 `NewTool()` 自定义                   | bash, http, read, search    |
| **Script Tools** | 技能包 `scripts/` 目录 | 技能专属的 `.py/.ts/.js/.sh` 脚本，自动或手动定义为工具     | run_query.py, run_deploy.sh |
| **MCP Tools**    | 外部 MCP 服务器        | 通过 Model Context Protocol 连接的远程服务工具              | 组件查询, 数据库操作        |

### 执行流程

```
用户输入 userPrompt
        │
        ▼
┌─ Run() ──────────────────────────────────────────────┐
│  loadSkill(SkillsFS) → ParseSkillPackages              │
│  ├─ 0 个技能: runWithSkill(nil)                         │
│  ├─ 1 个技能: runWithSkill(skill)                       │
│  └─ N 个技能: selectSkill() → LLM 选择 → runWithSkill   │
└────────────────────────────────────────────────────────┘
        │
        ▼
┌─ runWithSkill() ─────────────────────────────────────┐
│  合并工具: SubAgent tools + Base tools + Script + MCP  │
│                                                        │
│  工具调用循环 (最多 20 轮):                              │
│  ├─ LLM 决定调用哪个工具                                │
│  ├─ SubAgent → tool.Exec() → 子 Agent.Run()            │
│  ├─ 含 "__" → McpSessions.CallTool (MCP)               │
│  ├─ 在 Tools 中 → tool.Exec() (内置/自定义)             │
│  ├─ 在 scriptMap 中 → bash() (脚本工具)                 │
│  └─ 无 tool_calls → 返回最终文本                        │
└────────────────────────────────────────────────────────┘
```

---

## SubAgent 子代理

SubAgent 是框架内置的多 Agent 协作机制。通过声明式配置，将多个子 Agent 注册为父 Agent 可调用的 Tool，由 LLM 自动编排调用。

### Agent-as-Tool 模式

最常用的模式：每个子代理是一个拥有独立消息历史和 SystemPrompt 的 Agent，被包装为 Tool 注册到父 Agent。LLM 在工具调用循环中自动决定何时调用哪个子代理。

```go
agent := agentsdk.New(types.Config{
    ChatClient: chatClient,
    SubAgents: []types.SubAgentConfig{
        {
            Name:         "dealer",
            Description:  "荷官，负责发牌和判定胜负",
            SystemPrompt: "你是德州扑克荷官...",
        },
        {
            Name:         "xiaoming",
            Description:  "松凶型玩家",
            SystemPrompt: "你是玩家小明，松凶型风格...",
        },
    },
})

// 一行启动，LLM 自动编排子代理间的交互
agent.Run(ctx, "开始一局德州扑克")
```

**工作原理**：

```
父 Agent 收到 "开始一局德州扑克"
  │
  ├─ LLM: "调用 dealer 发牌"
  │     └─ dealer 子代理执行 → 返回发牌结果
  ├─ LLM: "调用 xiaoming 做决策"
  │     └─ xiaoming 子代理执行 → 返回决策
  ├─ LLM: "调用 xiaohong 做决策"
  │     └─ xiaohong 子代理执行 → 返回决策
  └─ ... LLM 持续编排直到游戏结束
```

### Plan 编排模式

通过 `RunWithPlan()` 实现。LLM 先生成多步骤执行计划（顺序或 DAG），然后框架按计划依次调用子代理，支持步骤间传递中间结果。

```go
agent := agentsdk.New(types.Config{
    ChatClient: chatClient,
    SubAgents: []types.SubAgentConfig{
        {Name: "researcher", Description: "搜索和研究信息"},
        {Name: "writer",     Description: "撰写内容"},
        {Name: "reviewer",   Description: "审查内容"},
    },
    Tools: []types.Tool{tool.DefineHTTPTool()},
})

// LLM 自动生成计划并按步骤执行
agent.RunWithPlan(ctx, "调研 AI 趋势并写报告")
```

**LLM 生成的计划示例**：

```json
{
  "setps": [
    { "name": "researcher", "input": "调研 2024 年 AI 领域重大突破" },
    { "name": "writer", "input": "基于 {{result:researcher}} 撰写一份报告" },
    {
      "name": "reviewer",
      "input": "审查 {{result:writer}} 的质量和准确性",
      "after": ["writer"]
    }
  ]
}
```

- 步骤间通过 `{{result:步骤名}}` 引用前序步骤的输出
- `after` 字段定义依赖关系，无依赖的步骤可并行执行
- 无子代理时自动退化为普通 `Run()`

### SubAgentConfig 配置

| 字段           | 类型              | 必填   | 说明                                                 |
| -------------- | ----------------- | ------ | ---------------------------------------------------- |
| `Name`         | string            | **是** | 子代理名称，同时作为注册到父 Agent 的 Tool 名称      |
| `Description`  | string            | **是** | 能力描述，LLM 依据此描述决定何时调用该子代理         |
| `SystemPrompt` | string            | 否     | 系统提示词，定义子代理的角色和行为准则               |
| `SkillsFS`     | `fs.FS`           | 否     | 子代理专属技能文件系统，nil 时复用父 Agent 的        |
| `Tools`        | `[]Tool`          | 否     | 子代理专属工具集，nil 时复用父 Agent 的              |
| `McpSessions`  | `ClientSessioner` | 否     | 子代理专属 MCP 会话，nil 时复用父 Agent 的           |
| `Parameters`   | `map[string]any`  | 否     | 自定义 Tool 参数 Schema，nil 时使用默认的 input 字段 |

### PlanStep 结构

| 字段    | 类型     | 说明                             |
| ------- | -------- | -------------------------------- |
| `Name`  | string   | 步骤名称，对应子代理名或工具名   |
| `Input` | string   | 传递给该步骤的自然语言输入       |
| `After` | []string | 前置依赖步骤名称列表（DAG 模式） |

---

## API 参考

### Agent

#### Config 结构体

| 字段          | 类型                            | 必填   | 说明                 |
| ------------- | ------------------------------- | ------ | -------------------- |
| `ChatClient`  | `types.OpenAIChatClient`        | **是** | LLM 聊天客户端       |
| `SkillsFS`    | `fs.FS`                         | 否     | 技能文件系统         |
| `Debug`       | `types.Logger`                  | 否     | 日志接口实现         |
| `McpSessions` | `types.ClientSessioner`         | 否     | MCP 会话管理器       |
| `History`     | `[]types.ChatCompletionMessage` | 否     | 初始消息历史         |
| `Tools`       | `[]Tool`                        | 否     | 自定义工具集合       |
| `SubAgents`   | `[]SubAgentConfig`              | 否     | 声明式子代理配置列表 |

#### 核心方法

| 方法          | 签名                                                         | 说明                                                    |
| ------------- | ------------------------------------------------------------ | ------------------------------------------------------- |
| `New`         | `New(cfg types.Config) *Agent`                               | 创建并初始化 Agent（含子代理注册）                      |
| `Run`         | `(a *Agent) Run(ctx, prompt string) (string, error)`         | **主入口**：技能选择 + 工具调用循环，LLM 自动编排子代理 |
| `RunWithPlan` | `(a *Agent) RunWithPlan(ctx, prompt string) (string, error)` | **Plan 模式**：LLM 生成计划 → DAG/顺序执行子代理        |

#### 注册函数

| 函数           | 签名                                          | 说明                              |
| -------------- | --------------------------------------------- | --------------------------------- |
| `NewTool`      | `NewTool(cfg types.ToolConfig) Tool`          | 创建自定义 Tool（快捷构造）       |
| `WarpSkill`    | `WarpSkill(sk *SkillPackage, a *Agent) Tool`  | 将 SkillPackage 包装为 Tool       |
| `RegisterExec` | `RegisterExec(ext string, exec types.Runner)` | 注册脚本执行器（如 `.js` → goja） |

#### ToolConfig 结构体

用于 `NewTool()` 快捷创建自定义工具：

```go
t := agentsdk.NewTool(types.ToolConfig{
    Name:        "my_tool",
    Description: "工具描述",
    Exec: func(ctx context.Context, in string) (string, error) {
        return "result", nil
    },
})
```

| 字段          | 类型             | 必填   | 说明              |
| ------------- | ---------------- | ------ | ----------------- |
| `Name`        | string           | **是** | 工具名称          |
| `Description` | string           | **是** | 工具描述          |
| `Parameters`  | `map[string]any` | 否     | 自定义参数 Schema |
| `Exec`        | `types.Runner`   | **是** | 执行函数          |

默认脚本运行时映射：

| 扩展名 | 执行命令 |
| ------ | -------- |
| `.py`  | `python` |
| `.js`  | `node`   |
| `.php` | `php`    |
| `.rb`  | `ruby`   |

---

### Skill 系统

#### Skill Package 结构

每个技能是一个包含 `SKILL.md`（或 `skill.md`）的目录：

```
my-skill/
├── SKILL.md           # 技能元数据 + 指令
├── scripts/           # 可选：脚本工具
│   ├── run.py
│   └── deploy.sh
└── resources/         # 可选：资源文件
```

#### SKILL.md 格式

支持 Claude Code 和 OpenAI 双格式解析。使用 YAML frontmatter 定义元数据：

```markdown
---
name: my-skill
description: 描述技能的功能
allowedTools:
  - bash
  - http
---

# 技能指令

这里是技能的详细指令内容...
```

#### Skill 元数据字段

| 字段           | 类型             | 说明                 |
| -------------- | ---------------- | -------------------- |
| `Name`         | string           | 技能名称（唯一标识） |
| `Description`  | string           | 技能描述             |
| `AllowedTools` | []string         | 允许使用的工具列表   |
| `Model`        | string           | 推荐使用的模型       |
| `Author`       | string           | 作者                 |
| `Version`      | string           | 版本号               |
| `License`      | string           | 许可证               |
| `Tools`        | []ToolDefinition | 自定义工具定义       |

---

### Tool 系统

#### Tool 结构体

```go
type Tool struct {
    Prompt   string              // 可选：附加提示信息（不序列化到 API）
    Type     string              // 工具类型，默认 "function"
    Function *FunctionDefinition // 函数定义（名称、描述、参数 Schema）
    Exec     Runner              // 执行函数（type Runner = func(ctx, in string) (out string, e error)）
}
```

#### 内置工具

| 工具名          | 创建函数                      | 功能说明              | 安全特性                     |
| --------------- | ----------------------------- | --------------------- | ---------------------------- |
| `bash`          | `DefineBashTool()`            | Shell 命令执行        | 危险命令拦截 + 2 分钟超时    |
| `http_request`  | `DefineHTTPTool()`            | HTTP 请求（curl兼容） | JSON 自动美化输出            |
| `read_local`    | `DefineReadLocal(fsys fs.FS)` | 文件/目录读取         | 基于 fs.FS，支持任意文件系统 |
| `tavily_search` | `DefineTavilySearch()`        | Tavily AI 搜索        | 默认上限 20 条               |

#### Bash 工具

支持环境变量 `$WORKDIR` 作为命令的工作根目录。

**危险命令拦截列表**：`rm -rf /`、`rm -rf /*`、`> /dev/sd`、`> /dev/null`、`mkfs`、`dd if=`

```go
result, err := tool.Bash("ls -la")                              // 默认 2 分钟超时
result, err = tool.BashWithContext(ctx, "sleep 5")              // 自定义超时
```

---

### VFS 虚拟文件系统

技能加载通过 `fs.FS` 接口抽象，内置多种虚拟文件系统实现。

#### MemFS — 内存文件系统

适用于动态构建技能内容或测试场景。

```go
mem := vfs.NewMem()                    // 创建内存 FS
mem.WriteFile("calc/SKILL.md", data)   // 写入文件（自动创建目录）
mem.Remove("calc/old.md")              // 删除文件
mem.Export(zipWriter)                  // 导出为 ZIP
mem.Merge("prefix/", otherFS)          // 合并其他 FS
```

#### ZipFS — ZIP 文件系统

```go
zipFS := vfs.NewZip()
r, _ := vfs.ZipReadFile("skills.zip")     // 本地文件
r, _ := vfs.ZipReadURL("https://...")      // 远程 URL
zipFS.Add("my-skill", r)
```

#### 工具函数

| 函数          | 签名                                                    | 说明                |
| ------------- | ------------------------------------------------------- | ------------------- |
| `ZipReadFile` | `ZipReadFile(name string) (*zip.Reader, error)`         | 读取本地 ZIP 文件   |
| `ZipReadURL`  | `ZipReadURL(url string) (*zip.Reader, error)`           | 从 URL 读取 ZIP     |
| `ZipCreate`   | `ZipCreate(content map[string]string) *zip.Reader`      | 从内存创建 ZIP      |
| `CreateZip`   | `CreateZip(w io.Writer, files map[string][]byte) error` | 写入 ZIP 到 Writer  |
| `ParseZip`    | `ParseZip(data []byte) (map[string][]byte, error)`      | 解析 ZIP 到内存 Map |

---

### Memory 系统

LLM 驱动的多级对话摘要系统，用于控制上下文长度。

#### 三级摘要策略

| 策略             | 触发条件        | 行为         |
| ---------------- | --------------- | ------------ |
| **Leaf**         | 单个对话片段    | 增量摘要     |
| **Condensed D1** | 多个 leaf 合并  | 压缩为单节点 |
| **Condensed D2** | 多 session 合并 | 更高层级压缩 |

#### 自动升级策略

```
normal policy → 检查 token 是否超预算 (150%)
    ↓ 超出
aggressive policy → 再检查
    ↓ 还超出
deterministicFallback → 截断到目标长度
```

```go
summarizer := &memory.LLMSummarizer{
    Generate: func(ctx context.Context, prompt string) (string, error) {
        return llmClient.Generate(prompt)
    },
}
summary, err := memory.BuildSummarize(ctx, summarizer, messages, memory.SummarizeOptions{
    TargetTokens: 4000,
    IsCondenced: true,
    Depth:        1,
})
```

---

## 模块系统

### aichat — OpenAI Chat 客户端

> 路径：`modules/aichat/`

基于 HTTP 直连 OpenAI 兼容 API 的轻量级客户端，采用 **函数选项模式** 配置。

```go
client := aichat.New(
    aichat.WithKey("sk-xxx"),
    aichat.WithBase("https://api.openai.com/v1"),
    aichat.WithModel("gpt-4o"),
)
```

| 选项        | 说明     | 默认值                        |
| ----------- | -------- | ----------------------------- |
| `WithKey`   | API Key  | 无（必填）                    |
| `WithBase`  | Base URL | `https://api.deepseek.com/v1` |
| `WithModel` | 默认模型 | `deepseek-v4-flash`           |

---

### mcp — MCP 协议客户端

> 路径：`modules/mcp/`

支持 stdio / SSE 两种传输方式，内建连接重试与指数退避机制。

```go
config, _ := mcp.LoadConfig("servers.json")
client := mcp.NewClient(ctx, config)
defer client.Close()

// 工具名格式: serverName__toolName（自动添加前缀避免冲突）
tools, _ := client.ListTools(ctx)
result, _ := client.CallTool(ctx, "serverName__toolName", map[string]any{"arg": "value"})
```

---

### log — 日志模块

> 路径：`modules/log/`

实现 `types.Logger` 接口，用于 Agent 调试输出。

```go
agent := agentsdk.New(types.Config{
    Debug: &log.DefaultLog{},
})
```

---

### dag — DAG 执行引擎

> 路径：`modules/dag/`

有向无环图（DAG）执行引擎，用于 `RunWithPlan` 的计划编排。

```go
g := dag.New()
g.AddNode("step1", "输入描述1")
g.AddNode("step2", "输入描述2")
g.AddEdge("step1", "step2") // step2 依赖 step1

g.Run(ctx, func(ctx context.Context, name, prompt string) error {
    // 执行节点逻辑
    dag.SetResultKV(ctx, name, result) // 存储结果
    return nil
})

results := dag.GetResult(ctx) // map[string]string
```

也支持从 JSON 构建：

```go
g := dag.NewJson([]byte(`{"setps": [
    {"name": "research", "input": "调研AI趋势"},
    {"name": "write",    "input": "写报告", "after": ["research"]}
]}`))
```

---

## 设计模式

| 模式            | 应用位置                              | 说明                             |
| --------------- | ------------------------------------- | -------------------------------- |
| **Strategy**    | `OpenAIChatClient`, `Summarizer` 接口 | 可替换 LLM 后端和摘要引擎        |
| **Plugin**      | Skill Package + SubAgent              | 技能/子代理作为插件热发现和注册  |
| **Pipeline**    | 发现 → 选择 → 执行                    | Agent 的核心流水线               |
| **Adapter**     | MCP Client, aichat                    | 将外部格式适配为统一的 Tool 接口 |
| **Option**      | aichat.New                            | 函数选项模式，灵活配置客户端     |
| **Delegate**    | SubAgent (Agent-as-Tool)              | 父 Agent 委托子 Agent 处理子任务 |
| **Orchestrate** | RunWithPlan (DAG)                     | LLM 生成计划，框架按 DAG 编排    |

---

## 参考项目

- [goskills](https://github.com/smallnest/goskills) — Reference skills implementation
- [anna](https://github.com/vaayne/anna) — Reference memory implementation
- [go-mcp](https://github.com/modelcontextprotocol/go-sdk) — Downgrading MCP to Go 1.23

---

## License

[MIT](LICENSE)
