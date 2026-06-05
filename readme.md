# Agent SDK

<p align="center">
  <strong>AI Agent 运行时框架</strong> — 通过可插拔的 <em>Skill（技能）</em>与 <em>MCP 工具协议</em>实现领域能力的动态扩展
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
  - [三层工具体系](#三层工具体系)
  - [执行流程](#执行流程)
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
- [设计模式](#设计模式)
- [参考项目](#参考项目)

---

## 特性

- **零外部依赖** — 仅使用 Go 标准库，无第三方依赖
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
    // 1. 创建 Chat Client（Option 模式）
    chatClient := aichat.New(
        aichat.WithKey(os.Getenv("OPENAI_API_KEY")),
        aichat.WithBase(os.Getenv("OPENAI_API_BASE")),
        aichat.WithModel(os.Getenv("OPENAI_API_MODEL")),
    )

    // 2. 准备技能文件系统
    skillsFS := os.DirFS("/path/to/skills")

    // 3. 创建 Agent
    agent := agentsdk.New(types.Config{
        SkillsFS:   skillsFS,
        Debug:      &log.DefaultLog{},
        ChatClient: chatClient,
        BaseTools: map[string]types.Tool{
            "bash": tool.DefineBashTool(),
            "http": tool.DefineHTTPTool(),
        },
    })

    // 4. 执行
    resp, err := agent.Run(context.Background(), "帮我分析这个数据")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp)
}
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

### 使用 ZIP 加载技能

```go
zipFS := vfs.NewZip()
r, _ := vfs.ZipReadFile("skills.zip")
zipFS.Add("my-skill", r)

agent := agentsdk.New(types.Config{
    SkillsFS:   zipFS,
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

Agent 不硬编码任何领域知识，而是通过三层工具体系 + 可插拔模块获取能力：

```
                    ┌──────────────┐
                    │   agent.go   │  ◄── 入口 / 协调者
                    │   (Agent)    │
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
 ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐
 │  skill/      │  │   tool/      │  │   modules/mcp/   │
 │  parser +    │  │  BaseTools   │  │  外部协议工具     │
 │  tool_gen    │  │              │  │                  │
 └──────────────┘  └──────────────┘  └──────────────────┘
       │                  │                  │
  Skill Package        Bash/Http/Read      远程服务器工具
  (技能脚本)           /Search/Curl         (SSE/stdio)
```

### 三层工具体系

| 层级             | 来源                   | 说明                                                    | 示例                        |
| ---------------- | ---------------------- | ------------------------------------------------------- | --------------------------- |
| **BaseTools**    | `tool/` 包内置         | 通用基础能力，注册到 `Config.BaseTools`                 | bash, http, read, search    |
| **Script Tools** | 技能包 `scripts/` 目录 | 技能专属的 `.py/.ts/.js/.sh` 脚本，自动或手动定义为工具 | run_query.py, run_deploy.sh |
| **MCP Tools**    | 外部 MCP 服务器        | 通过 Model Context Protocol 连接的远程服务工具          | 组件查询, 数据库操作        |

### 执行流程

```
用户输入 userPrompt
        │
        ▼
┌─ 1. discoverSkills(SkillsFS) ────────────────────────┐
│  扫描 fs.FS → ParseSkillPackages → []SkillPackage      │
│  ┌─ 0 个技能: 返回错误                                  │
│  ├─ 1 个技能: 直接执行                                  │
│  └─ N 个技能: 进入选择流程                              │
└────────────────────────────────────────────────────────┘
        │
        ▼
┌─ 2. selectSkill(userPrompt, skills) ──────────────────┐
│  构建 prompt → LLM 选择最合适的技能                     │
│  extractSkillName 从 AI 回答中提取技能名                │
└────────────────────────────────────────────────────────┘
        │
        ▼
┌─ 3. runWithSkill() ───────────────────────────────────┐
│  构建 system message → 进入工具调用循环 (最多 20 轮)     │
│                                                        │
│  每轮循环:                                              │
│  ├─ LLM 决定调用哪个工具                                │
│  ├─ 含 "__" → McpSessions.CallTool (MCP 工具)           │
│  ├─ 在 BaseTools 中 → tool.Exec() (内置工具)             │
│  ├─ 在 scriptMap 中 → bash() (脚本工具)                  │
│  ├─ 结果追加到 messages                                 │
│  └─ 无 tool_calls → 返回最终文本                        │
└────────────────────────────────────────────────────────┘
```

---

## API 参考

### Agent

#### Config 结构体

| 字段          | 类型                            | 必填   | 默认值 | 说明                         |
| ------------- | ------------------------------- | ------ | ------ | ---------------------------- |
| `SkillsFS`    | `fs.FS`                         | **是** | -      | 技能文件系统                  |
| `ChatClient`  | `types.OpenAIChatClient`        | **是** | -      | LLM 聊天客户端                |
| `Debug`       | `types.Logger`                  | 否     | nil    | 日志接口实现                  |
| `McpSessions` | `types.ClientSessioner`         | 否     | nil    | MCP 会话管理器                |
| `History`     | `[]types.ChatCompletionMessage` | 否     | nil    | 初始消息历史                  |
| `BaseTools`   | `map[string]types.Tool`         | 否     | nil    | 自定义基础工具集合            |

#### 核心方法

| 方法            | 签名                                                | 说明                                     |
| --------------- | --------------------------------------------------- | ---------------------------------------- |
| `New`           | `New(cfg types.Config) *Agent`                      | 创建并初始化 Agent                       |
| `Run`           | `(a *Agent) Run(ctx, prompt string) (string, error)` | **主入口**：执行完整的技能选择+执行流程   |

#### 注册函数

| 函数            | 签名                                                          | 说明                           |
| --------------- | ------------------------------------------------------------- | ------------------------------ |
| `RegisterExec`  | `RegisterExec(ext, exec string)`                              | 注册脚本执行器（如 `.py` → `python3`） |
| `RegisterBash`  | `RegisterBash(f func(context.Context, string) (string, error))` | 覆盖默认的 Bash 执行函数       |

默认脚本执行器映射：

| 扩展名   | 执行命令    |
| -------- | ----------- |
| `.py`    | `python3`   |
| `.js`    | `node`      |
| `.tengo` | `tengo`     |

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

| 字段           | 类型           | 说明                   |
| -------------- | -------------- | ---------------------- |
| `Name`         | string         | 技能名称（唯一标识）   |
| `Description`  | string         | 技能描述               |
| `AllowedTools` | []string       | 允许使用的工具列表     |
| `Model`        | string         | 推荐使用的模型         |
| `Author`       | string         | 作者                   |
| `Version`      | string         | 版本号                 |
| `License`      | string         | 许可证                 |
| `Tools`        | []ToolDefinition | 自定义工具定义       |

---

### Tool 系统

#### Tool 结构体

```go
type Tool struct {
    Type     string                    // 工具类型，默认 "function"
    Function *FunctionDefinition       // 函数定义（名称、描述、参数 Schema）
    Exec     func(ctx context.Context, in string) (string, error) // 执行函数
    Prompt   string                    // 可选：附加提示信息
}
```

#### FunctionDefinition 结构体

```go
type FunctionDefinition struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Strict      bool   `json:"strict,omitempty"`
    Parameters  any    `json:"parameters"`
}
```

#### 内置工具

| 工具名          | 创建函数                        | 功能说明              | 安全特性                     |
| --------------- | ------------------------------- | --------------------- | ---------------------------- |
| `bash`          | `DefineBashTool()`              | Shell 命令执行        | 危险命令拦截 + 2 分钟超时    |
| `http_request`  | `DefineHTTPTool()`              | HTTP 请求（curl兼容） | JSON 自动美化输出            |
| `read_local`    | `DefineReadLocal(fsys fs.FS)`   | 文件/目录读取         | 基于 fs.FS，支持任意文件系统 |
| `tavily_search` | `DefineTavilySearch()`          | Tavily AI 搜索        | 默认上限 20 条               |

#### Bash 工具

支持环境变量 `$WORKDIR` 作为命令的工作根目录。

**危险命令拦截列表**：`rm -rf /`、`rm -rf /*`、`> /dev/sd`、`> /dev/null`、`mkfs`、`dd if=`

```go
result, err := tool.Bash("ls -la")                              // 默认 2 分钟超时
result, err = tool.BashWithTimeout("sleep 5", 10*time.Second)   // 自定义超时
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

agent := agentsdk.New(types.Config{
    SkillsFS: mem,
})

// 导出为 ZIP
mem.Export(zipWriter)

// 合并其他 FS
mem.Merge("prefix/", otherFS)
```

| 方法        | 签名                                                   | 说明                     |
| ----------- | ------------------------------------------------------ | ------------------------ |
| `NewMem`    | `NewMem(names ...string) *MemFS`                       | 创建内存文件系统         |
| `WriteFile` | `(m *MemFS) WriteFile(name string, data []byte) error` | 写入文件（自动创建目录） |
| `Remove`    | `(m *MemFS) Remove(name string) error`                 | 删除文件                 |
| `Export`    | `(m *MemFS) Export(buf io.Writer) error`               | 导出为 ZIP               |
| `Merge`     | `(m *MemFS) Merge(prefix string, sub fs.FS) error`     | 合并其他 FS 到指定前缀   |

#### ZipFS — ZIP 文件系统

从本地文件或远程 URL 加载 ZIP 压缩包中的技能。

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

#### 使用 `os.DirFS` 加载本地目录

```go
agent := agentsdk.New(types.Config{
    SkillsFS: os.DirFS("/path/to/skills"),
})
```

---

### Memory 系统

LLM 驱动的多级对话摘要系统，用于控制上下文长度。

#### 三级摘要策略

| 策略             | 触发条件        | 行为         |
| ---------------- | --------------- | ------------ |
| **Leaf**         | 单个对话片段    | 增量摘要     |
| **Condensed D1** | 多个 leaf 合并  | 压缩为单节点 |
| **Condensed D2** | 多 session 合并 | 更高层级压缩 |

#### Prompt 模板

| 模板常量               | 说明                  |
| ---------------------- | --------------------- |
| `LeafPolicyNormal`     | Leaf 节点普通摘要策略 |
| `LeafPolicyAggressive` | Leaf 节点激进压缩策略 |
| `LeafPromptTemplate`   | Leaf 摘要 Prompt 模板 |
| `CondensedD1Prompt`    | D1 级压缩 Prompt 模板 |
| `CondensedD2Prompt`    | D2 级压缩 Prompt 模板 |

#### 自动升级策略

```
normal policy → 检查 token 是否超预算 (150%)
    ↓ 超出
aggressive policy → 再检查
    ↓ 还超出
deterministicFallback → 截断到目标长度
```

#### 使用示例

```go
summarizer := &memory.LLMSummarizer{
    Generate: func(ctx context.Context, prompt string) (string, error) {
        return llmClient.Generate(prompt)
    },
}

opts := memory.SummarizeOptions{
    TargetTokens: 4000,
    IsCondenced: true,
    Depth:        1,
}

summary, err := memory.BuildSummarize(ctx, summarizer, messages, opts)
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

resp, err := client.CreateChatCompletion(ctx, types.ChatCompletionRequest{
    Messages:    []types.ChatCompletionMessage{{Role: "user", Content: "Hello"}},
    Temperature: 0,
    Tools:       []types.Tool{...},
})
```

#### 配置选项

| 选项         | 说明                     | 默认值                         |
| ------------ | ------------------------ | ------------------------------ |
| `WithKey`    | API Key                  | 无（必填）                     |
| `WithBase`   | API Base URL             | `https://api.deepseek.com/v1`  |
| `WithModel`  | 默认模型                 | `deepseek-v4-flash`            |

---

### mcp — MCP 协议客户端

> 路径：`modules/mcp/`

支持 stdio / SSE 两种传输方式，内建连接重试与指数退避机制。

```go
// 从 JSON 文件加载配置
config, _ := mcp.LoadConfig("servers.json")

// 创建客户端（自动连接所有配置的服务器）
client := mcp.NewClient(ctx, config)
defer client.Close()

// 列出所有服务器的工具（自动添加 serverName__ 前缀避免冲突）
tools, _ := client.ListTools(ctx)

// 调用工具（含自动重试）
result, _ := client.CallTool(ctx, "serverName__toolName", map[string]any{"arg": "value"})
```

#### MCPServer 配置

| 字段      | 类型              | 说明                           |
| --------- | ----------------- | ------------------------------ |
| `Type`    | string            | 传输方式：`"stdio"` 或 `"sse"` |
| `Command` | string            | 启动命令（stdio 模式）         |
| `Args`    | []string          | 命令参数（stdio 模式）         |
| `URL`     | string            | 服务地址（sse 模式）           |
| `Headers` | map[string]string | 自定义请求头（sse 模式）       |

---

### log — 日志模块

> 路径：`modules/log/`

实现 `types.Logger` 接口，用于 Agent 调试输出。

```go
agent := agentsdk.New(types.Config{
    Debug: &log.DefaultLog{},
    // ...
})
```

---

## 设计模式

| 模式         | 应用位置                              | 说明                             |
| ------------ | ------------------------------------- | -------------------------------- |
| **Strategy** | `OpenAIChatClient`, `Summarizer` 接口 | 可替换 LLM 后端和摘要引擎        |
| **Plugin**   | Skill Package + Modules               | 技能作为插件热发现、模块按需加载 |
| **Pipeline** | 发现 → 选择 → 执行                    | Agent 的核心流水线               |
| **Adapter**  | MCP Client, aichat                    | 将外部格式适配为统一的 Tool 接口 |
| **Option**   | aichat.New                            | 函数选项模式，灵活配置客户端     |

---

## 参考项目

- [goskills](https://github.com/smallnest/goskills) — Reference skills implementation
- [anna](https://github.com/vaayne/anna) — Reference memory implementation
- [go-mcp](https://github.com/modelcontextprotocol/go-sdk) — Downgrading MCP to Go 1.23

---

## License

[MIT](LICENSE)
