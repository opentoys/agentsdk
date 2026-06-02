# Agent SDK

AI Agent 运行时框架，通过可插拔的 **Skill（技能）** 和 **MCP 工具协议** 实现领域能力的动态扩展。

## 目录结构

```
agentsdk/
├── agent.go                  # 核心入口：Agent 主逻辑（发现→选择→执行、模式分发）
├── planer.go                 # Plan 系统：多 Skill 计划生成、校验、自动修复、多步执行
├── go.mod / go.sum           # 模块定义与依赖
├── readme.md                 # 本文档
├── Makefile                  # 构建脚本
├── types/                    # 核心类型定义
│   ├── openai.go             # OpenAI Chat API 类型（请求/响应/消息/Tool）
│   ├── mcp.go                # MCP 相关类型（Server/Session 接口）
│   ├── client.go             # 通用处理器类型
│   └── log.go                # Logger 接口定义
├── memory/                   # 对话记忆摘要系统
│   └── summarize.go          # LLM 驱动的多级对话压缩/摘要
├── prompt/                   # Prompt 模板管理
│   ├── memory.go             # 记忆摘要三级 Prompt 模板
│   └── skill.go              # 技能相关 Prompt
├── skill/                    # 技能包解析与工具生成
│   ├── parser.go             # SKILL.md / skill.md 解析（Claude + OpenAI 双格式）
│   └── tool_gen.go           # 从技能定义自动生成 Tool 定义
├── tool/                     # 内置工具集
│   ├── bash.go               # Bash shell 执行工具（含安全检查 + 超时）
│   ├── http.go               # HTTP 请求工具（curl 兼容）
│   ├── read.go               # 本地文件/目录读取工具
│   └── search.go             # Tavily 网络搜索工具
├── vfs/                      # 虚拟文件系统（实现 fs.FS 接口）
│   ├── mem.go                # 内存文件系统 MemFS
│   ├── zip.go                # ZIP 文件系统 ZipFS + CreateZip/ParseZip 工具
│   └── os.go                 # ZIP 导出/合并等 OS 辅助方法
├── modules/                  # 可插拔功能模块
│   ├── aichat/               # OpenAI / Anthropic Chat 客户端
│   │   ├── openai.go         # HTTP 直连 OpenAI 兼容 API（Option 模式）
│   │   └── anthropic.go      # Anthropic API 客户端
│   ├── mcp/                  # MCP 协议客户端
│   │   ├── client.go         # 多服务器连接管理、工具获取、调用（含重试）
│   │   ├── config.go         # Server 配置结构 & JSON 加载
│   │   └── sdk.go            # Go MCP SDK 封装（降级至 go1.23）
│   ├── log/                  # 日志模块
│   │   └── log.go            # 默认 Logger 实现
│   ├── yaml/                 # YAML 解析模块
│   │   └── simple.go         # 简化 YAML 操作封装
│   └── stdlib/               # 标准库扩展
│       ├── jsonx/            # JSON 序列化/反序列化扩展
│       └── httpx/            # HTTP CSRF 防护等扩展 fork go1.26
├── examples/
│   ├── cli/                  # CLI 单 Skill 示例
│   │   └── main.go
│   └── multi/                # 多 Agent 协作示例
│       └── main.go
```

---

## 核心架构

### 设计理念

Agent 不硬编码任何领域知识，而是通过三层工具体系 + 可插拔模块获取能力。支持 **Direct（单 Skill）** 和 **Plan（多 Skill 计划）** 两种执行模式：

```
                    ┌──────────────┐
                    │   agent.go   │  ◄── 入口 / 协调者
                    │   (Agent)    │
                    └──────┬───────┘
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
   ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
   │  skill/      │ │   tool/      │ │ modules/mcp/ │
   │  parser +    │ │  BaseTools   │ │ 外部协议工具  │
   │  tool_gen    │ │              │ │              │
   └──────────────┘ └──────────────┘ └──────────────┘
          │                │                │
     Skill Package    Bash/Http/Read   远程服务器工具
     (技能脚本)       /Search          (SSE/stdio)

   ┌──────────────────────────────────────────────────┐
   │ planer.go                                        │
   │ Plan → Validate → Auto-fix → Execute Step by Step│
   └──────────────────────────────────────────────────┘

### 三层工具体系

| 层级             | 来源                   | 说明                                                    | 示例                        |
| ---------------- | ---------------------- | ------------------------------------------------------- | --------------------------- |
| **BaseTools**    | `tool/` 包内置         | 通用基础能力，注册到 `Config.BaseTools`                 | bash, http, read, search    |
| **Script Tools** | 技能包 `scripts/` 目录 | 技能专属的 `.py/.ts/.js/.sh` 脚本，自动或手动定义为工具 | run_query.py, run_deploy.sh |
| **MCP Tools**    | 外部 MCP 服务器        | 通过 Model Context Protocol 连接的远程服务工具          | 组件查询, 数据库操作        |

### 执行流程

#### ModeDirect（默认，单 Skill）

```
用户输入 userPrompt
        │
        ▼
┌─ 1. discoverSkills(SkillsFS) ──────────────────────┐
│  扫描 fs.FS → ParseSkillPackages → []SkillPackage    │
└──────────────────────────────────────────────────────┘
        │
        ▼
┌─ 2. selectSkill(userPrompt, skills) ─────────────────┐
│  构建 prompt → LLM 选择最合适的技能                     │
│  extractSkillName 从 AI 回答中提取技能名                │
└──────────────────────────────────────────────────────┘
        │
        ▼
┌─ 3. executeSkillWithTools() ──────────────────────────┐
│  构建 system message → 进入工具调用循环 (最多20轮)        │
│                                                        │
│  每轮循环:                                              │
│  ├─ LLM 决定调用哪个工具                                │
│  ├─ 含 "__" → mcpSessions.CallTool (MCP 工具)           │
│  ├─ 在 BaseTools 中 → tool.Exec() (内置工具)             │
│  ├─ 在 scriptMap 中 → tool.Bash() (脚本工具)             │
│  ├─ 结果追加到 messages                                 │
│  └─ 无 tool_calls → 返回最终文本                        │
└──────────────────────────────────────────────────────┘
```

#### ModePlan（多 Skill 计划）

```
用户输入 userPrompt
        │
        ▼
┌─ 1. discoverSkills() ────────────────────────────────┐
│  扫描 fs.FS → 获取全部可用 Skill                       │
└──────────────────────────────────────────────────────┘
        │
        ▼
┌─ 2. createPlan() ────────────────────────────────────┐
│  LLM 生成结构化 JSON 计划 (goal + steps)                │
│      ↓                                                │
│  ValidatePlan() ─ 9 项规则校验                          │
│      │                ↓ Invalid                        │
│      │           tryAutoFixPlan()                       │
│      │           ├─ formatValidateErrors()              │
│      │           ├─ fixPlan() → LLM 修复               │
│      │           └─ 循环 (上限 PlanMaxRetries 次)       │
│      ↓ Valid                                           │
│  plan.Status = "created"                               │
└──────────────────────────────────────────────────────┘
        │
        ▼
┌─ 3. executePlan() ───────────────────────────────────┐
│  for each step:                                        │
│    ├─ 检查依赖 → 失败则 skip                           │
│    ├─ 累加前步结果 → prevContext                        │
│    ├─ executePlanStep() → 独立消息上下文 + 工具循环     │
│    ├─ 记录 step.Result / step.Error                    │
│    └─ 确定 plan.Status (completed / partial / failed)  │
└──────────────────────────────────────────────────────┘
```

---

## 快速开始

### 基本用法

```go
import (
    "os"
    "github.com/opentoys/agentsdk"
    "github.com/opentoys/agentsdk/modules/aichat"
    "github.com/opentoys/agentsdk/modules/log"
    "github.com/opertoys/agentsdk/tool"
    "github.com/opertoys/agentsdk/types"
)

// 1. 创建 Chat Client（使用 Option 模式配置）
chatClient := aichat.NewOpenAI(
    aichat.WithOpenAIKey(os.Getenv("OPENAI_API_KEY")),
    aichat.WithOpenAIBase(os.Getenv("OPENAI_API_BASE")),
    aichat.WithOpenAIModel(os.Getenv("OPENAI_API_MODEL")),
)

// 2. 创建 Agent
agent, err := agentsdk.New(agentsdk.Config{
    SkillsDir: "/path/to/skills",   // 本地技能目录
    Debug:     &log.DefaultLog{},    // 可选：开启调试日志
    ChatClient: chatClient,          // 注入 LLM 客户端
    BaseTools: map[string]types.Tool{
        "bash": tool.DefineBashTool(),
        "http": tool.DefineHTTPTool(),
    },
})
if err != nil {
    panic(err)
}

// 3. 执行
resp, err := agent.Run(context.Background(), "帮我分析这个数据")
if err != nil {
    panic(err)
}
fmt.Println(resp)
```

### 使用 VFS 内存文件系统加载技能

```go
import "github.com/opertoys/agentsdk/vfs"

mem := vfs.NewMem()
mem.WriteFile("my-skill/SKILL.md", []byte(skillContent))
mem.WriteFile("my-skill/scripts/run.py", []byte(scriptContent))

agent, _ := agentsdk.New(agentsdk.Config{
    SkillsFS:   mem,          // 直接传入 fs.FS
    ChatClient: chatClient,
})
```

### 使用 ZIP 文件加载技能

```go
zipFS := vfs.NewZip()
r, _ := vfs.ZipReadFile("skills.zip")
zipFS.Add("my-skill", r)

agent, _ := agentsdk.New(agentsdk.Config{
    SkillsFS:   zipFS,
    ChatClient: chatClient,
})
```

### 使用 MCP 外部工具

```go
import "github.com/opertoys/agentsdk/modules/mcp"

// 加载 MCP 配置
config, _ := mcp.LoadConfig("mcp_config.json")

// 创建 MCP 客户端
mcpClient := mcp.NewClient(ctx, config)

agent, _ := agentsdk.New(agentsdk.Config{
    ChatClient:  chatClient,
    McpSessions: mcpClient,    // 注入 MCP 会话管理器
    SkillsFS:   os.DirFS("./skills"),
})
defer mcpClient.Close()
```

### Plan 模式 — 多 Skill 调用与计划执行

当任务需要多个 Skill 协作完成时，启用 Plan 模式：

```go
// ModePlan: 始终走计划流程
agent, _ := agentsdk.New(agentsdk.Config{
    Mode:           agentsdk.ModePlan,
    PlanMaxRetries: 5,              // 校验失败自动修复上限 (默认 3)
    ChatClient:     chatClient,
    SkillsDir:      "./skills",
})

// Run 内部自动: 生成计划 → 校验 → auto-fix → 按步骤执行
resp, _ := agent.Run(ctx, "先分析数据趋势，再生成Excel报表，最后发邮件通知")
```

#### 显式 Plan / Exec 分步控制

```go
// 只生成计划（不执行），用户可审查修改
plan, _ := agent.CreatePlan(ctx, "抓取网页 → 翻译成中文 → 生成PDF")

// 检查校验结果
v := plan.ValidatePlan(skills)
if !v.Valid {
    for _, e := range v.Errors {
        log.Printf("[%s] %s", e.Type, e.Message)
    }
}

// 手动修改计划
plan.Steps = append(plan.Steps, agentsdk.PlanStep{
    ID:          "step_4",
    Description: "发送完成通知",
    Skill:       "email-skill",
    Input:       "通知用户任务已完成",
    Dependencies: []string{"step_3"},
})

// 执行修改后的计划
result, _ := agent.ExecutePlan(ctx, plan)
fmt.Println(result)
```

#### ModeAuto — 智能选择

```go
agent, _ := agentsdk.New(agentsdk.Config{
    Mode: agentsdk.ModeAuto,  // LLM 自动判断: 复杂任务走 Plan，简单任务走 Direct
    // ...
})
```

## API 参考

### Agent

#### Config 结构体

| 字段             | 类型                            | 必填   | 默认值     | 说明                                    |
| ---------------- | ------------------------------- | ------ | ---------- | --------------------------------------- |
| `SkillsDir`      | string                          | 否     | ""         | 技能目录路径（与 SkillsFS 二选一）        |
| `SkillsFS`       | `fs.FS`                         | 否     | nil        | 技能文件系统（优先于 SkillsDir）          |
| `Debug`          | `types.Logger`                  | 否     | nil        | 日志接口实现                            |
| `ChatClient`     | `types.OpenAIChatClient`        | **是** | -          | LLM 聊天客户端（必填）                   |
| `McpSessions`    | `types.ClientSessioner`         | 否     | nil        | MCP 会话管理器                          |
| `History`        | `[]types.ChatCompletionMessage` | 否     | nil        | 初始消息历史                            |
| `BaseTools`      | `map[string]types.Tool`         | 否     | nil        | 自定义基础工具集合                      |
| `Mode`           | `RunMode`                       | 否     | ModeDirect | 执行模式: direct / plan / auto          |
| `PlanMaxRetries` | int                             | 否     | 3          | Plan 校验失败时的自动修复次数上限        |

#### 方法

| 方法         | 签名                                                         | 说明                                               |
| ------------ | ------------------------------------------------------------ | -------------------------------------------------- |
| `New`        | `New(cfg Config) (*Agent, error)`                            | 创建并初始化 Agent                                 |
| `Run`        | `(a *Agent) Run(ctx, prompt string) (string, error)`         | **主入口**：根据 Mode 分发到 Direct 或 Plan 流程   |
| `CreatePlan` | `(a *Agent) CreatePlan(ctx, prompt string) (*Plan, error)`   | 生成执行计划（含校验+自动修复），不执行            |
| `ExecutePlan`| `(a *Agent) ExecutePlan(ctx, plan *Plan) (string, error)`    | 执行已有计划，每步独立上下文，前步结果传给后步     |
| `Messages`   | `(a *Agent) Messages() []ChatCompletionMessage`              | 获取当前消息历史                                   |
| `Usage`      | `(a *Agent) Usage() Usage`                                   | 获取 Token 用量统计                                |
| `NewChat`    | `(a *Agent) NewChat(history []ChatCompletionMessage) *Agent` | 复用配置创建新会话实例                             |

### Modules 模块系统

#### aichat — OpenAI Chat 客户端 (`modules/aichat/`)

基于 HTTP 直连 OpenAI 兼容 API 的轻量级客户端，采用 **函数选项模式** 配置。

```go
client := aichat.NewOpenAI(
    aichat.WithOpenAIKey("sk-xxx"),
    aichat.WithOpenAIBase("https://api.openai.com/v1"),
    aichat.WithOpenAIModel("gpt-4o"),
)

resp, err := client.CreateChatCompletion(ctx, types.ChatCompletionRequest{
    Messages:    []types.ChatCompletionMessage{{Role: "user", Content: "Hello"}},
    Temperature: 0,
    Tools:       []types.Tool{...},
})
```

#### mcp — MCP 协议客户端 (`modules/mcp/`)

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

##### MCPServer 配置结构

| 字段      | 类型              | 说明                           |
| --------- | ----------------- | ------------------------------ |
| `Type`    | string            | 传输方式：`"stdio"` 或 `"sse"` |
| `Command` | string            | 启动命令（stdio 模式）         |
| `Args`    | []string          | 命令参数（stdio 模式）         |
| `URL`     | string            | 服务地址（sse 模式）           |
| `Headers` | map[string]string | 自定义请求头（sse 模式）       |

#### log — 日志模块 (`modules/log/`)

实现 `types.Logger` 接口，用于 Agent 调试输出。

```go
agent, _ := agentsdk.New(agentsdk.Config{
    Debug: &log.DefaultLog{},
    // ...
})
```

### Plan 系统 (`planer.go`)

多 Skill 协作的计划生成、校验、自动修复与执行系统。

#### RunMode — 执行模式

| 常量        | 值       | 说明                                |
| ----------- | -------- | ----------------------------------- |
| `ModeDirect`| `direct` | 默认：选一个 Skill 立即执行          |
| `ModePlan`  | `plan`   | 始终生成计划 → 校验 → 多步执行       |
| `ModeAuto`  | `auto`   | LLM 自动判断：复杂走 Plan，简单走 Direct |

#### Plan 核心类型

```go
type Plan struct {
    Goal    string     // 总体目标
    Thought string     // LLM 推理说明
    Steps   []PlanStep // 步骤列表
    Status  string     // created / running / completed / partial / failed
}

type PlanStep struct {
    ID           string         // 唯一标识，如 "step_1"
    Description  string         // 步骤描述
    Skill        string         // 使用的 Skill 名称 ("none" 表示无 Skill)
    Input        string         // 给该 Skill 的子提示
    Dependencies []string       // 依赖的步骤 ID 列表
    Status       PlanStepStatus // pending / running / completed / failed / skipped
    Result       string         // 执行结果
    Error        string         // 错误信息
}
```

#### Plan 校验 (`ValidatePlan`)

对生成的计划执行 9 项规则校验：

| 规则编号 | 类型                  | 说明                                     |
| -------- | --------------------- | ---------------------------------------- |
| 1        | `empty_plan`          | 计划至少包含 1 个 step                   |
| 2        | `missing_goal`        | goal 不能为空                            |
| 3        | `duplicate_id`        | step ID 不可重复                         |
| 4        | `empty_id`            | step ID 不能为空                         |
| 5        | `empty_description`   | step description 不能为空                |
| 6        | `unknown_skill`       | 引用的 Skill 必须在可用列表中             |
| 7        | `self_dependency`     | step 不能依赖自身                        |
| 8        | `invalid_dependency`  | dependency ID 必须引用存在的 step        |
| 9        | `circular_dependency` | DFS 检测环形依赖链（A→B→A）              |

```go
v := plan.ValidatePlan(skills)
if !v.Valid {
    for _, e := range v.Errors {
        fmt.Printf("[%s] step=%s: %s\n", e.Type, e.StepID, e.Message)
    }
}
```

#### 自动修复 (`tryAutoFixPlan`)

```
ValidatePlan() → Valid? ────→ ✅ 返回
       │ Invalid
       ▼
formatValidateErrors() → 错误报告
       │
       ▼
fixPlan() → LLM 生成修正后的 JSON Plan
       │
       ▼
  重试 (上限 PlanMaxRetries)
```

- 校验失败后自动将原始 Plan + 错误详情发送给 LLM 修复
- 修复结果再次校验，直到通过或达到上限
- 默认最多重试 3 次，可通过 `Config.PlanMaxRetries` 配置

### Skill 系统 (`skill/`)

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

支持 Claude Code 和 OpenAI 双格式解析。

### VFS 虚拟文件系统 (`vfs/`)

技能加载通过 `fs.FS` 接口抽象，内置多种虚拟文件系统实现：

#### MemFS — 内存文件系统

适用于动态构建技能内容或测试场景。

```go
mem := vfs.NewMem()                    // 创建内存 FS
mem.WriteFile("calc/SKILL.md", data)   // 写入文件（自动创建目录）

agent, _ := agentsdk.New(agentsdk.Config{
    SkillsFS: mem,
})

// 导出为 ZIP
mem.Export(zipWriter)

// 合并其他 FS
mem.Merge("prefix/", otherFS)
```

| 方法        | 签名                                                   | 说明                     |
| ----------- | ------------------------------------------------------ | ------------------------ |
| `NewMem`    | `NewMem() *MemFS`                                      | 创建内存文件系统         |
| `WriteFile` | `(m *MemFS) WriteFile(name string, data []byte) error` | 写入文件（自动创建目录） |
| `Export`    | `(m *MemFS) Export(buf io.Writer) error`               | 导出为 zip               |
| `Merge`     | `(m *MemFS) Merge(prefix string, sub fs.FS) error`     | 合并其他 FS 到指定前缀   |

#### ZipFS — ZIP 文件系统

从本地文件加载 ZIP 压缩包中的技能。

```go
zipFS := vfs.NewZip()
r, _ := vfs.ZipReadFile("skills.zip")
zipFS.Add("my-skill", r)
```

#### VFS 工具函数 (`vfs/os.go`)

| 方法        | 签名                                                    | 说明                |
| ----------- | ------------------------------------------------------- | ------------------- |
| `CreateZip` | `CreateZip(w io.Writer, files map[string][]byte) error` | 从内存创建 ZIP      |
| `ParseZip`  | `ParseZip(data []byte) (map[string][]byte, error)`      | 解析 ZIP 到内存 Map |

#### 使用 `os.DirFS` 加载本地目录

由于 `SkillsFS` 类型为 `fs.FS`，可直接使用 Go 标准库：

```go
agent, _ := agentsdk.New(agentsdk.Config{
    SkillsDir: "/path/to/skills",  // 内部转为 os.DirFS
})
```

### Tool 系统 (`tool/`)

#### Tool 结构体

```go
type Tool struct {
    Define FunctionDefinition                    // 函数定义（名称、参数 Schema）
    Exec   func(ctx context.Context, in string) (string, error) // 执行函数
}
```

#### 内置工具一览

| 工具名          | 创建函数                      | 功能说明              | 安全特性                     |
| --------------- | ----------------------------- | --------------------- | ---------------------------- |
| `bash`          | `DefineBashTool()`            | Shell 命令执行        | 危险命令拦截 + 2分钟超时     |
| `http_request`  | `DefineHTTPTool()`            | HTTP 请求（curl兼容） | JSON 自动美化输出            |
| `read_local`    | `DefineReadLocal(fsys fs.FS)` | 文件/目录读取         | 基于 fs.FS，支持任意文件系统 |
| `tavily_search` | `DefineTavilySearch()`        | Tavily AI 搜索        | 默认上限 20 条               |

##### Bash 工具

支持环境变量 `$WORKDIR` 作为命令的工作根目录。

**危险命令拦截列表**: `rm -rf /`, `rm -rf /*`, `> /dev/sd`, `> /dev/null`, `mkfs`, `dd if=`

```go
result, err := tool.Bash("ls -la")           // 默认 2 分钟超时
result, err = tool.BashWithTimeout("sleep 5", 10*time.Second)  // 自定义超时
```

### Memory 系统 (`memory/`)

LLM 驱动的多级对话摘要系统，用于控制上下文长度。

#### 三级摘要策略

| 策略             | 触发条件        | 行为         |
| ---------------- | --------------- | ------------ |
| **Leaf**         | 单个对话片段    | 增量摘要     |
| **Condensed D1** | 多个 leaf 合并  | 压缩为单节点 |
| **Condensed D2** | 多 session 合并 | 更高层级压缩 |

#### Prompt 模板 (`prompt/memory.go`)

提供预定义的三级摘要 Prompt 模板：

| 模板常量               | 说明                  |
| ---------------------- | --------------------- |
| `LeafPolicyNormal`     | Leaf 节点普通摘要策略 |
| `LeafPolicyAggressive` | Leaf 节点激进压缩策略 |
| `LeafPromptTemplate`   | Leaf 摘要 Prompt 模板 |
| `CondensedD1Prompt`    | D1 级压缩 Prompt 模板 |
| `CondensedD2Prompt`    | D2 级压缩 Prompt 模板 |

#### 自动升级策略

```
normal policy → 检查 token 是否超预算(150%)
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

## 设计模式总结

| 模式            | 应用位置                                   | 说明                              |
| --------------- | ------------------------------------------ | --------------------------------- |
| **Strategy**    | `OpenAIChatClient`, `Summarizer` 接口      | 可替换 LLM 后端和摘要引擎         |
| **Plugin**      | Skill Package + Modules                    | 技能作为插件热发现、模块按需加载  |
| **Pipeline**    | Direct: 发现→选择→执行 / Plan: 生成→校验→修复→执行 | Agent 的核心流水线               |
| **Adapter**     | MCP Client, aichat                         | 将外部格式适配为统一的 Tool 接口  |
| **Option**      | aichat.NewOpenAI                           | 函数选项模式，灵活配置客户端      |
| **Validator**   | Plan.ValidatePlan                          | 9 项规则校验，错误分类精确到 step |
| **Retry**       | tryAutoFixPlan                             | 校验失败 → LLM 修复 → 循环重试   |
| **Composite**   | Plan + PlanStep + Dependency               | 多步骤组合执行，支持依赖编排      |

## 参考项目

- [goskills](https://github.com/smallnest/goskills) `Reference skills implementation`
- [anna](https://github.com/vaayne/anna) `Reference memory implementation`
- [go-mcp](https://github.com/modelcontextprotocol/go-sdk) `Downgrading mcp to go1.23`
