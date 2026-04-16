## Agent SDK

A minimized agent sdk that supports mcp and skills calls.

### 使用方式

```go
    rcfg := &agentsdk.Config{
        SkillsDir:  os.Getenv("SKILL_DIRS"), // 指定 skills 文件夹，由于最小化，并不通用。尽可能符合 openai skill 调用方式
        MCPServers: nil,
        APIKey:     os.Getenv("OPENAI_API_KEY"),
        APIBase:    os.Getenv("OPENAI_API_BASE"),
        Model:      os.Getenv("OPENAI_API_MODE"),
        Debug:      true,
        BaseTools: map[string]*tool.Tool{  // 自定义工具函数
			"bash": tool.DefineBashTool(),
		},
    }

    agent, e := agentsdk.New(rcfg)
    if e != nil {
        panic(e)
    }

    resp, e := agent.Run(context.Background(), os.Getenv("INPUT"))
    if e != nil {
        panic(e)
    }
    fmt.Println(resp)
```

### 参考项目

1. [goskills](https://github.com/smallnest/goskills)
2. [anna](https://github.com/vaayne/anna)
