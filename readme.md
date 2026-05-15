# Agent SDK

AI Agent 运行时框架，通过可插拔的 **Skill（技能）** 和 **MCP 工具协议** 实现领域能力的动态扩展。

## 目录结构

```
agentsdk/
├── agent.go                  # 核心入口：Agent 主逻辑（发现→选择→执行）
├── go.mod / go.sum           # 模块定义与依赖
├── readme.md                 # 本文档
├── mcp/                      # MCP 客户端封装层
│   ├── client.go             # 多服务器连接管理、工具获取、工具调用（含重试）
│   └── config.go             # 配置结构定义 & JSON 加载
├── memory/                   # 对话记忆摘要系统
│   └── summarize.go          # LLM 驱动的多级对话压缩/摘要
├── skill/                    # 技能包解析与工具生成
│   ├── parser.go             # SKILL.md / skill.md 解析（Claude + OpenAI 双格式）
│   └── tool_gen.go           # 从技能定义自动生成 OpenAI Tool 定义
├── tool/                     # 内置工具集
│   ├── types.go              # Tool 核心类型定义（含 Prompt 字段）
│   ├── bash.go               # Bash shell 执行工具（含安全检查 + 超时）
│   ├── read.go               # 本地文件/目录读取工具
│   ├── search.go             # Tavily 网络搜索工具
├── vfs/                      # 虚拟文件系统（实现 fs.FS 接口）
│   ├── mem.go                # 内存文件系统 MemFS
│   └── zip.go                # ZIP 文件系统 ZipFS
├── examples/cli/             # CLI 示例程序
│   ├── main.go               # 完整使用示例（含 VFS + 记忆持久化）
│   └── xxx.json              # 示例历史消息 JSON
└── modules/officalmcp/       # Go MCP SDK (第三方库，内嵌, 用于剔除部分依赖降低 go version)
```

---

## 核心架构

### 设计理念

Agent 不硬编码任何领域知识，而是通过三层工具体系获取能力：

```
                    ┌──────────────┐
                    │   agent.go   │  ◄── 入口 / 协调者
                    │   (Agent)    │
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
 ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐
 │  skill/      │  │   tool/      │  │     mcp/         │
 │  parser +    │  │  BaseTools   │  │  外部协议工具     │
 │  tool_gen    │  │              │  │                  │
 └──────────────┘  └──────────────┘  └──────────────────┘
       │                  │                  │
  Skill Package        Bash/Http/Read      远程服务器工具
  (技能脚本)         /Search/Curl         (SSE/stdio)
```

### 三层工具体系

| 层级             | 来源                   | 说明                                                    | 示例                         |
| ---------------- | ---------------------- | ------------------------------------------------------- | ---------------------------- |
| **BaseTools**    | `tool/` 包内置         | 通用基础能力，注册到 `Config.BaseTools`                 | bash, http, read, search     |
| **Script Tools** | 技能包 `scripts/` 目录 | 技能专属的 `.py/.ts/.js/.sh` 脚本，自动或手动定义为工具 | run_query.py, run_deploy.sh  |
| **MCP Tools**    | 外部 MCP 服务器        | 通过 Model Context Protocol 连接的远程服务工具          | TDesign 组件查询, 数据库操作 |

### 执行流程

```
用户输入 userPrompt
        │
        ▼
┌─ 1. discoverSkills(SkillsDir) ──────────────────────┐
│  扫描 fs.FS → ParseSkillPackages → []SkillPackage     │
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
│  ├─ 含 "__" → mcpClient.CallTool (MCP 工具)            │
│  ├─ 在 BaseTools 中 → tool.Exec() (内置工具)            │
│  ├─ 在 scriptMap 中 → tool.Bash() (脚本工具)            │
│  ├─ 结果追加到 messages                                 │
│  └─ 无 tool_calls → 返回最终文本                        │
└──────────────────────────────────────────────────────┘
```

---

## 快速开始

### 基本用法

```go
// 使用内存文件系统加载技能
mem := vfs.NewMem()
mem.WriteFile("my-skill/SKILL.md", []byte("..."))

cfg := &agentsdk.Config{
    SkillsDir: mem,
    APIKey:    os.Getenv("OPENAI_API_KEY"),
    APIBase:   os.Getenv("OPENAI_API_BASE"),
    Model:     os.Getenv("OPENAI_API_MODEL"),
    Debug:     true,
    BaseTools: map[string]*tool.Tool{
        "bash": tool.DefineBashTool(),
    },
}

agent, err := agentsdk.New(cfg)
if err != nil {
    panic(err)
}

resp, err := agent.Run(context.Background(), os.Getenv("INPUT"))
if err != nil {
    panic(err)
}
fmt.Println(resp)
```

### 使用 ZIP 加载技能

```go
zipFS := vfs.NewZip()
r, _ := vfs.ZipReadFile("skills.zip")
zipFS.Add("my-skill", r)

cfg := &agentsdk.Config{
    SkillsDir: zipFS,
    // ...
}
```

## API 参考

### Agent

#### Config 结构体

| 字段            | 类型                             | 必填 | 默认值 | 说明                                |
| --------------- | -------------------------------- | ---- | ------ | ----------------------------------- |
| `APIKey`        | string                           | 是   | -      | OpenAI API Key                      |
| `APIBase`       | string                           | 是   | -      | OpenAI API Base URL                 |
| `Model`         | string                           | 否   | gpt-4o | 模型名称                            |
| `SkillsDir`     | `fs.FS`                          | 否   | nil    | 技能文件系统（支持任意 fs.FS 实现） |
| `MCPServers`    | `map[string]mcp.MCPServer`       | 否   | nil    | MCP 服务器配置                      |
| `MCPMaxRetries` | int                              | 否   | 3      | MCP 工具调用最大重试次数            |
| `Debug`         | bool                             | 否   | false  | 是否打印调试信息                    |
| `History`       | `[]openai.ChatCompletionMessage` | 否   | nil    | 初始消息历史                        |
| `BaseTools`     | `map[string]*tool.Tool`          | 否   | nil    | 自定义基础工具集合                  |
| `AllowSkills`   | `[]string`                       | 否   | nil    | 技能白名单（空为不限制）            |

#### 方法

| 方法       | 签名                                                         | 说明                                    |
| ---------- | ------------------------------------------------------------ | --------------------------------------- |
| `New`      | `New(cfg *Config) (*Agent, error)`                           | 创建并初始化 Agent（含 MCP 连接）       |
| `Run`      | `(a *Agent) Run(ctx, prompt string) (string, error)`         | **主入口**：执行完整的技能选择+执行流程 |
| `Chat`     | `(a *Agent) Chat(ctx, prompt string) (string, error)`        | 纯聊天模式（无技能/工具）               |
| `Messages` | `(a *Agent) Messages() []ChatCompletionMessage`              | 获取当前消息历史                        |
| `NewChat`  | `(a *Agent) NewChat(history []ChatCompletionMessage) *Agent` | 复用配置创建新会话实例                  |

### Skill 系统

### VFS 虚拟文件系统 (`vfs/`)

技能加载通过 `fs.FS` 接口抽象，内置两种虚拟文件系统实现：

#### MemFS — 内存文件系统

适用于动态构建技能内容或测试场景。

```go
mem := vfs.NewMem("skills")       // 可选参数指定根目录名
mem.WriteFile("calc/SKILL.md", []byte(skillContent))
mem.WriteFile("calc/scripts/run.py", []byte(script))

cfg := &agentsdk.Config{
    SkillsDir: mem,  // 直接作为 fs.FS 使用
}
```

| 方法        | 签名                                                    | 说明                     |
| ----------- | ------------------------------------------------------- | ------------------------ |
| `NewMem`    | `NewMem(names ...string) *MemFS`                        | 创建内存文件系统         |
| `Open`      | `(m *MemFS) Open(name string) (fs.File, error)`         | 打开文件/目录            |
| `ReadDir`   | `(m *MemFS) ReadDir(dir string) ([]fs.DirEntry, error)` | 读取目录                 |
| `WriteFile` | `(m *MemFS) WriteFile(name string, data []byte) error`  | 写入文件（自动创建目录） |
| `Remove`    | `(m *MemFS) Remove(name string) error`                  | 删除文件                 |
| `Export`    | `(m *MemFS) Export(buf io.Writer) error`                | 导出 zip 文件            |
| `Merge`     | `(m *MemFS) Merge(prefix string, sub fs.FS) (e error)`  | 合并 FS                  |

#### ZipFS — ZIP 文件系统

从本地文件或 URL 加载 ZIP 压缩包中的技能。

```go
zipFS := vfs.NewZip()
r, _ := vfs.ZipReadFile("skills.zip")     // 从本地文件读取
r, _ := vfs.ZipReadURL("https://...")     // 从 URL 读取
zipFS.Add("my-skill", r)                  // 注册到虚拟文件系统

cfg := &agentsdk.Config{
    SkillsDir: zipFS,
}
```

| 方法          | 签名                                               | 说明                    |
| ------------- | -------------------------------------------------- | ----------------------- |
| `NewZip`      | `NewZip() *ZipFS`                                  | 创建 ZIP 文件系统       |
| `ZipReadFile` | `ZipReadFile(name string) (*zip.Reader, error)`    | 从本地文件读取 ZIP      |
| `ZipReadURL`  | `ZipReadURL(url string) (*zip.Reader, error)`      | 从 URL 下载并读取 ZIP   |
| `ZipCreate`   | `ZipCreate(content map[string]string) *zip.Reader` | 从内存 map 创建 ZIP     |
| `Add`         | `(z *ZipFS) Add(name string, data *zip.Reader)`    | 注册 ZIP 到虚拟文件系统 |

#### 使用 `os.DirFS` 加载本地目录

由于 `SkillsDir` 类型为 `fs.FS`，也可直接使用 Go 标准库加载本地目录：

```go
cfg := &agentsdk.Config{
    SkillsDir: os.DirFS("/path/to/skills"),
}
```

### Tool 系统

#### Tool 结构体

```go
type Tool struct {
    Define openai.Tool                          // OpenAI Tool 定义
    Exec   func(in string) (out string, e error) // 工具执行函数
    Prompt string                               // 工具描述 prompt（用于技能选择上下文）
}
```

#### 内置工具一览

| 工具名          | 创建函数                      | 功能说明            | 安全特性                     |
| --------------- | ----------------------------- | ------------------- | ---------------------------- |
| `bash`          | `DefineBashTool()`            | Shell 命令执行      | 危险命令拦截 + 2分钟超时     |
| `http_request`  | `DefineHttpRequest()`         | curl 兼容 HTTP 请求 | JSON 自动美化输出            |
| `read_local`    | `DefineReadLocal(fsys fs.FS)` | 文件/目录读取       | 基于 fs.FS，支持任意文件系统 |
| `tavily_search` | `DefineTavilySearch()`        | Tavily AI 搜索      | 默认上限 20 条               |

##### Bash 工具

支持环境变量 `$WORKDIR` 作为命令的工作根目录。

**危险命令拦截列表**: `rm -rf /`, `rm -rf /*`, `> /dev/sd`, `> /dev/null`, `mkfs`, `dd if=`

```go
result, err := tool.Bash("ls -la")           // 默认 2 分钟超时
result, err = tool.BashWithTimeout("sleep 5", 10*time.Second)  // 自定义超时
```

#### MCPServer 配置

#### 两种传输方式

| 方式             | 配置                      | 场景                                               |
| ---------------- | ------------------------- | -------------------------------------------------- |
| **stdio** (默认) | 设置 `Command` + `Args`   | 子进程通信（如 npx 启动的 MCP server）             |
| **SSE**          | 设置 `Type="sse"` + `URL` | HTTP 长连接（远程 MCP server），支持自定义 Headers |

#### 重试机制

- MCP 工具调用失败时检测是否为连接错误
- 连接错误时自动重连 + 指数退避等待
- 最多重试 `MaxRetries` 次（默认 3 次）

---

### Memory 系统 (`memory/`)

LLM 驱动的多级对话摘要系统，用于控制上下文长度。

#### 三级摘要策略

| 策略             | 常量                         | 触发条件        | 行为         |
| ---------------- | ---------------------------- | --------------- | ------------ |
| **Leaf**         | `IsCondensed=false`          | 单个对话片段    | 增量摘要     |
| **Condensed D1** | `IsCondensed=true, Depth<=1` | 多个 leaf 合并  | 压缩为单节点 |
| **Condensed D2** | `IsCondensed=true, Depth>1`  | 多 session 合并 | 更高层级压缩 |

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
        // 调用 LLM 进行摘要
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

| 模式         | 应用位置                              | 说明                               |
| ------------ | ------------------------------------- | ---------------------------------- |
| **Strategy** | `OpenAIChatClient`, `Summarizer` 接口 | 可替换 LLM 后端和摘要引擎          |
| **Plugin**   | Skill Package                         | 技能作为插件热发现、动态加载       |
| **Pipeline** | 发现 → 选择 → 执行                    | Agent 的核心流水线                 |
| **Adapter**  | MCP Client, curl.go                   | 将外部格式适配为统一的 OpenAI Tool |

## 参考项目

- [goskills](https://github.com/smallnest/goskills) `Reference skills implementation`
- [anna](https://github.com/vaayne/anna) `Reference memory implementation`
- [go-mcp](https://github.com/modelcontextprotocol/go-sdk) `Downgrading mcp to go1.23`
