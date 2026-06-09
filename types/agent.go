package types

import (
	"context"
	"io/fs"
)

// Config holds all the necessary configuration for the runner.
type Config struct {
	SkillsFS    fs.FS
	Debug       Logger
	ChatClient  OpenAIChatClient
	McpSessions ClientSessioner
	History     []ChatCompletionMessage // Defining historical messages
	Tools       []Tool                  // Custom tool collection
	SubAgents   []SubAgentConfig        // 声明式子代理配置列表
}

// SubAgentConfig 声明式子代理配置
// 框架自动管理子代理生命周期，将其转换为父代理可调用的 Tool
type SubAgentConfig struct {
	Name         string // 子代理名称，同时作为生成 Tool 的 function name
	Description  string // 子代理能力描述，用于 LLM 理解何时调用该子代理
	SystemPrompt string // 子代理的系统提示词（可选，覆盖 skill 自身的 prompt）
	SkillsFS     fs.FS  // 子代理的技能文件系统（可选，为 nil 时复用父代理的 SkillsFS）
	Tools        []Tool // 子代理的专属工具（可选）
	McpSessions  ClientSessioner
	Parameters   map[string]any
}

type ToolConfig struct {
	Name         string // 子代理名称，同时作为生成 Tool 的 function name
	Description  string // 子代理能力描述，用于 LLM 理解何时调用该子代理
	SystemPrompt string // 子代理的系统提示词（可选，覆盖 skill 自身的 prompt）
	Parameters   map[string]any
	Exec         func(ctx context.Context, in string) (out string, e error)
}

// PlanStep 表示计划中的一个步骤
// 用于 RunWithPlan 模式下 LLM 生成的执行计划
type PlanStep struct {
	Name  string   `json:"name"`  // 步骤名称（对应 SubAgentConfig.Name 或工具名）
	Input string   `json:"input"` // 传递给该步骤的自然语言输入
	After []string `json:"after"` // 前置依赖步骤名称列表（DAG 模式）
}
