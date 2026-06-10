package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/opentoys/agentsdk/modules/dag"
	"github.com/opentoys/agentsdk/skill"
	"github.com/opentoys/agentsdk/tool"
	"github.com/opentoys/agentsdk/types"
)

type Agent struct {
	client    types.OpenAIChatClient
	messages  []types.ChatCompletionMessage
	cfg       *types.Config
	tools     map[string]types.Tool
	subAgents map[string]*Agent // 子代理实例映射
}

func New(cfg types.Config) *Agent {
	a := &Agent{
		cfg:       &cfg,
		messages:  cfg.History,
		tools:     make(map[string]types.Tool),
		subAgents: make(map[string]*Agent),
	}
	for _, v := range cfg.Tools {
		a.tools[v.Function.Name] = v
	}
	// 构建子代理并注册为 Tool
	for _, sa := range cfg.SubAgents {
		a.tools[sa.Name] = a.buildSubAgentTool(sa)
	}
	return a
}

func (s *Agent) Run(ctx context.Context, in string) (out string, e error) {
	return s.loadSkill(ctx, in, s.cfg.SkillsFS)
}

func (s *Agent) loadSkill(ctx context.Context, in string, dir fs.FS) (out string, e error) {
	packages, err := skill.ParseSkillPackages(dir)
	if err != nil {
		return
	}
	switch len(packages) {
	case 0:
		return s.runWithSkill(ctx, in, nil)
	case 1:
		return s.runWithSkill(ctx, in, packages[0])
	}
	skills := make(map[string]*skill.SkillPackage, len(packages))
	for _, pkg := range packages {
		if pkg != nil {
			skills[pkg.Meta.Name] = pkg
		}
	}
	// --- STEP 2: SKILL SELECTION ---
	// If skill is explicitly specified via --skill flag, use it directly
	// Otherwise, ask LLM to select the best skill
	name, e := s.selectSkill(ctx, in, skills)
	if e != nil {
		e = fmt.Errorf("failed during skill selection: %w", err)
		return
	}

	sk, ok := skills[name]
	if !ok {
		e = fmt.Errorf("skill '%s' not found.", name)
		return
	}
	return s.runWithSkill(ctx, in, sk)
}

func (a *Agent) selectSkill(ctx context.Context, in string, skills map[string]*skill.SkillPackage) (out string, e error) {
	var sb strings.Builder
	sb.WriteString("User Request: ")
	sb.WriteString(in)
	sb.WriteString("\n\n")
	sb.WriteString("Available Skills:\n")
	for name, skill := range skills {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, skill.Meta.Description))
	}
	sb.WriteString("\nSelection Guidelines:\n")
	sb.WriteString("- For pure mathematical calculations (arithmetic, trigonometry, logarithms, etc.), ALWAYS prefer 'calculator-skill' over spreadsheet skills\n")
	sb.WriteString("- Only choose spreadsheet skills (xlsx, csv) when the user needs to create/read/modify spreadsheet FILES\n")
	sb.WriteString("- Function names that happen to exist in Excel do NOT make it a spreadsheet task\n")
	sb.WriteString("\nBased on the user request and guidelines above, which single skill is the most appropriate to use?")
	sb.WriteString("\n\nIMPORTANT: You MUST select exactly one skill from the above list, even if the request seems simple. Respond with ONLY the skill name, nothing else. Do not explain your choice or answer the question directly.")

	skillPrompt := skill.SkillsToPrompt(skills, a.cfg.Tools)
	// Use a temporary message history for skill selection
	resp, e := a.cfg.ChatClient.CreateChatCompletion(ctx, types.ChatCompletionRequest{
		Messages: []types.ChatCompletionMessage{
			{
				Role:    types.ChatMessageRoleSystem,
				Content: skillPrompt,
			},
			{
				Role:    types.ChatMessageRoleUser,
				Content: sb.String(),
			},
		},
	})
	if e != nil {
		e = fmt.Errorf("select skill error: %v", e)
		return
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.Trim(content, "'\"")

	// Extract just the skill name if there's extra text
	// Look for skill names in the content
	return extractSkillName(content, skills), nil
}

func (s *Agent) runWithSkill(ctx context.Context, in string, sk *skill.SkillPackage) (out string, e error) {
	var resp types.ChatCompletionResponse
	var tools []types.Tool
	for _, v := range s.tools {
		tools = append(tools, v)
	}
	var scripts map[string]string
	if sk != nil {
		var skillBody strings.Builder
		skillBody.WriteString("## SELECTED SKILL\n")
		skillBody.WriteString(fmt.Sprintf("Name: %s\n", sk.Meta.Name))
		skillBody.WriteString(fmt.Sprintf("Description: %s\n", sk.Meta.Description))
		skillBody.WriteString(fmt.Sprintf("Version: %s\n\n", sk.Meta.Version))
		skillBody.WriteString("## SKILL INSTRUCTIONS\n")
		skillBody.WriteString(sk.Body)
		skillBody.WriteString("\n\n ## SKILL CONTEXT\n")
		skillBody.WriteString(fmt.Sprintf("Skill Root Path: %s\n", sk.Path))
		s.messages = append(s.messages, types.ChatCompletionMessage{
			Role:    types.ChatMessageRoleSystem,
			Content: skillBody.String(),
		})
		sk.BaseTools = append(sk.BaseTools, tools...)
		tools, scripts = skill.GenerateToolDefinitions(sk)
	}

	s.messages = append(s.messages, types.ChatCompletionMessage{
		Role:    types.ChatMessageRoleUser,
		Content: in,
	})

	// Add MCP tools if client is available
	if s.cfg.McpSessions != nil {
		mcpTools, err := s.cfg.McpSessions.ListTools(ctx)
		if err != nil {
			return
		} else {
			tools = append(tools, mcpTools...)
		}
	}

	for range 20 { // Limit to 20 iterations to prevent infinite loops
		if resp, e = s.cfg.ChatClient.CreateChatCompletion(ctx, types.ChatCompletionRequest{
			Messages: s.messages, // Use agent's messages
			Tools:    tools,
		}); e != nil {
			e = fmt.Errorf("ChatCompletion error: %w", e)
			return
		}

		msg := resp.Choices[0].Message
		s.messages = append(s.messages, msg) // Append LLM's response

		if len(msg.ToolCalls) == 0 {
			out = msg.Content
			return
		}

		for _, tc := range msg.ToolCalls {
			var toolOutput string
			// Check if it is an MCP tool
			if s.cfg.McpSessions != nil && strings.Contains(tc.Function.Name, "__") {
				// Clean arguments for MCP tools too
				cleanedArgs := cleanToolArguments(tc.Function.Arguments)

				var args map[string]any
				if err := json.Unmarshal([]byte(cleanedArgs), &args); err != nil {
					toolOutput = fmt.Sprintf("Error unmarshalling arguments: %v (cleaned args: %s)", err, cleanedArgs)
				} else {
					var result any
					result, err = s.cfg.McpSessions.CallTool(ctx, tc.Function.Name, args)
					if err == nil {
						// Convert result to string/JSON
						resBytes, _ := json.Marshal(result)
						toolOutput = string(resBytes)
					}
				}
			} else {
				toolOutput, e = s.execTool(ctx, tc, scripts)
			}

			if e != nil {
				// Provide detailed error information to help LLM understand what went wrong
				s.messages = append(s.messages, types.ChatCompletionMessage{
					Role:       types.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content: fmt.Sprintf(`Tool execution failed: %s
Error details: %v
Tool name: %s
Arguments: %s

You can try:
1. Retry with different parameters
2. Use a different tool to fix it
3. Modify your approach`,
						tc.Function.Name, e, tc.Function.Name, tc.Function.Arguments),
				})
			} else {
				s.messages = append(s.messages, types.ChatCompletionMessage{
					Role:       types.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    toolOutput,
				})
			}
		}
	}
	return "", errors.New("exceeded maximum tool call iterations")
}

func (s *Agent) execTool(ctx context.Context, toolCall types.ToolCall, scripts map[string]string) (out string, e error) {
	// Clean the arguments before parsing
	cleanedArgs := cleanToolArguments(toolCall.Function.Arguments)

	var exec, ok = s.tools[toolCall.Function.Name]
	if !ok {
		// Handle custom script tools from scriptMap
		if scriptPath, ok := scripts[toolCall.Function.Name]; ok {
			var params struct {
				Args []string `json:"args"`
			}
			if cleanedArgs != "" {
				if err := json.Unmarshal([]byte(cleanedArgs), &params); err != nil {
					return "", fmt.Errorf("failed to unmarshal script arguments: %w (cleaned args: %s)", err, cleanedArgs)
				}
			}

			// Build command based on script extension
			var ext = filepath.Ext(scriptPath)
			var runner = scriptexec[ext]
			var cmd = scriptPath
			if runner == nil {
				runner = tool.BashWithContext
				if rt, ok := scriptruntime[ext]; ok {
					cmd = rt + " " + cmd
				}
			}

			// Add arguments if provided
			if len(params.Args) > 0 {
				for _, arg := range params.Args {
					cmd += fmt.Sprintf(" %s", shellQuote(arg))
				}
			}

			out, e = runner(ctx, cmd)
			if e != nil {
				return "", fmt.Errorf("tool execution failed for %s: %w", toolCall.Function.Name, e)
			}
			return out, e
		}
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
	out, e = exec.Exec(ctx, cleanedArgs)
	if e != nil {
		return "", fmt.Errorf("tool execution failed for %s: %w", toolCall.Function.Name, e)
	}
	return
}

func WarpSkill(sk *skill.SkillPackage, a *Agent) types.Tool {
	return types.Tool{
		Type: types.ToolTypeFunction,
		Function: &types.FunctionDefinition{
			Name:        sk.Meta.Name,
			Description: sk.Meta.Description,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"input": map[string]any{
						"type":        "string",
						"description": "Any natural language description that requires current tools, the more formal the better",
					},
				},
				"required": []string{"input"},
			},
		},
		Exec: func(ctx context.Context, in string) (out string, e error) {
			var params struct {
				Input string `json:"input"`
			}
			if e = json.Unmarshal([]byte(in), &params); e != nil {
				e = fmt.Errorf("failed to unmarshal bash arguments: %w (cleaned args: %s)", e, in)
				return
			}
			return a.runWithSkill(ctx, in, sk)
		},
	}
}

func NewTool(cfg types.ToolConfig) (tool types.Tool) {
	return types.Tool{
		Type: types.ToolTypeFunction,
		Function: &types.FunctionDefinition{
			Name:        cfg.Name,
			Description: cfg.Description,
			Parameters:  defaultMap(cfg.Parameters, basetoolinput),
		},
		Exec: func(ctx context.Context, in string) (out string, e error) {
			var params struct {
				Input string `json:"input"`
			}
			if e = json.Unmarshal([]byte(in), &params); e != nil {
				e = fmt.Errorf("failed to unmarshal sub-agent arguments: %w (args: %s)", e, in)
				return
			}
			return cfg.Exec(ctx, params.Input)
		},
	}
}

// buildSubAgentTool 根据声明式配置创建子代理并包装为 Tool
// 子代理复用父代理的 ChatClient，拥有独立的消息历史和工具集
func (s *Agent) buildSubAgentTool(sa types.SubAgentConfig) types.Tool {
	// 构建子代理配置
	cfg := types.Config{
		ChatClient:  s.cfg.ChatClient,
		Debug:       s.cfg.Debug,
		Tools:       sa.Tools,
		SkillsFS:    sa.SkillsFS,
		McpSessions: sa.McpSessions,
	}
	// 如果子代理指定了 SkillsFS 则使用它，否则复用父代理的
	if cfg.SkillsFS == nil {
		cfg.SkillsFS = s.cfg.SkillsFS
	}
	if cfg.Tools == nil {
		cfg.Tools = s.cfg.Tools
	}
	if cfg.McpSessions == nil {
		cfg.McpSessions = s.cfg.McpSessions
	}

	// 创建子代理实例
	sub := New(cfg)

	// 包装为 Tool
	return types.Tool{
		Type: types.ToolTypeFunction,
		Function: &types.FunctionDefinition{
			Name:        sa.Name,
			Description: sa.Description,
			Parameters:  defaultMap(sa.Parameters, basetoolinput),
		},
		Exec: func(ctx context.Context, in string) (out string, e error) {
			var params struct {
				Input string `json:"input"`
			}
			if e = json.Unmarshal([]byte(in), &params); e != nil {
				e = fmt.Errorf("failed to unmarshal sub-agent arguments: %w (args: %s)", e, in)
				return
			}
			// 如果配置了 SystemPrompt，在子代理消息中注入
			if sa.SystemPrompt != "" && len(sub.messages) == 0 {
				sub.messages = append(sub.messages, types.ChatCompletionMessage{
					Role:    types.ChatMessageRoleSystem,
					Content: sa.SystemPrompt,
				})
			}
			return sub.Run(ctx, params.Input)
		},
	}
}

// RunWithPlan 让 LLM 生成多步骤执行计划，然后按计划调度子代理执行
// 支持顺序执行和 DAG 并行执行两种模式，由 LLM 根据任务复杂度自动选择
func (s *Agent) RunWithPlan(ctx context.Context, input string) (string, error) {
	// 如果没有配置子代理，退化为普通 Run
	if len(s.subAgents) == 0 {
		return s.Run(ctx, input)
	}

	// 调用 LLM 生成计划
	steps, e := s.generatePlan(ctx, input)
	if e != nil {
		return "", fmt.Errorf("生成计划失败: %w", e)
	}

	// 如果计划为空，退化为普通 Run
	if len(steps) == 0 {
		return s.Run(ctx, input)
	}

	// 如果只有一个步骤，直接执行
	if len(steps) == 1 {
		return s.executePlanStepByName(ctx, steps[0].Name, steps[0].Input)
	}

	// 多步骤：构建 DAG 并执行
	g := planStepsToGraph(steps)
	e = g.Run(ctx, func(ctx context.Context, name, prompt string) error {
		// 展开前序步骤的结果引用
		prompt = expandStepInput(ctx, prompt)
		out, e := s.executePlanStepByName(ctx, name, prompt)
		if e != nil {
			return e
		}
		dag.SetResultKV(ctx, name, out)
		return nil
	})
	if e != nil {
		return "", fmt.Errorf("计划执行失败: %w", e)
	}

	// 返回最后一个步骤的结果作为最终输出
	results := dag.GetResult(ctx)
	lastStep := steps[len(steps)-1]
	if finalOut, ok := results[lastStep.Name]; ok {
		return finalOut, nil
	}
	// 如果找不到最后步骤，汇总所有结果
	var sb strings.Builder
	for k, v := range results {
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", k, v))
	}
	return sb.String(), nil
}

// generatePlan 调用 LLM 生成执行计划
// LLM 根据任务复杂度自动决定使用顺序还是 DAG 模式
func (s *Agent) generatePlan(ctx context.Context, input string) ([]types.PlanStep, error) {
	var sb strings.Builder
	sb.WriteString("你是一个任务规划助手。根据用户请求和可用的子代理列表，生成一个执行计划。\n\n")
	sb.WriteString("## 用户请求\n")
	sb.WriteString(input)
	sb.WriteString("\n\n## 可用子代理\n")
	for name, tool := range s.tools {
		if tool.Function != nil {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", name, tool.Function.Description))
		}
	}
	sb.WriteString("\n## 输出要求\n")
	sb.WriteString("请生成 JSON 格式的执行计划，严格遵守以下格式：\n")
	sb.WriteString(`{"setps": [{"name": "步骤名称", "input": "传递给该子代理的自然语言描述", "after": ["前置步骤名称"]}]}` + "\n\n")
	sb.WriteString("规则：\n")
	sb.WriteString("1. name 必须是上述可用子代理之一\n")
	sb.WriteString("2. input 是传递给子代理的自然语言任务描述，应包含用户原始请求和必要上下文\n")
	sb.WriteString("3. after 是可选的前置依赖步骤名称列表\n")
	sb.WriteString("4. 如果步骤之间没有依赖关系，after 设为空数组 []\n")
	sb.WriteString("5. 如果步骤之间有依赖关系，在 after 中指定必须先完成的步骤名称\n")
	sb.WriteString("6. 后续步骤的 input 中可以使用 {{result:前置步骤名称}} 引用前序步骤的输出结果\n")
	sb.WriteString("7. 根据任务复杂度决定步骤数量和依赖关系：简单任务 1-2 个顺序步骤，复杂任务多个步骤利用 after 构建依赖图\n")
	sb.WriteString("\n只输出 JSON，不要包含任何其他文字说明。")

	resp, e := s.cfg.ChatClient.CreateChatCompletion(ctx, types.ChatCompletionRequest{
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleUser, Content: sb.String()},
		},
	})
	if e != nil {
		return nil, fmt.Errorf("调用 LLM 生成计划失败: %w", e)
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	// 清理可能的 markdown 代码块包裹
	content = cleanToolArguments(content)

	// 解析 JSON，复用 "setps" 键名（与 dag.NewJson 保持一致）
	var plan struct {
		Steps []types.PlanStep `json:"setps"`
	}
	if e = json.Unmarshal([]byte(content), &plan); e != nil {
		return nil, fmt.Errorf("解析计划 JSON 失败: %w (content: %s)", e, content)
	}
	return plan.Steps, nil
}

// executePlanStepByName 根据步骤名称执行对应子代理
func (s *Agent) executePlanStepByName(ctx context.Context, name, input string) (string, error) {
	tool, ok := s.tools[name]
	if !ok {
		return "", fmt.Errorf("未找到子代理: %s", name)
	}
	args, _ := json.Marshal(map[string]string{"input": input})
	return tool.Exec(ctx, string(args))
}

// planStepsToGraph 将 PlanStep 列表转换为 DAG Graph
func planStepsToGraph(steps []types.PlanStep) *dag.Graph {
	g := dag.New()
	for _, step := range steps {
		g.AddNode(step.Name, step.Input)
	}
	for _, step := range steps {
		for _, dep := range step.After {
			g.AddEdge(dep, step.Name)
		}
	}
	return g
}

// expandStepInput 替换步骤输入中的结果引用模板
// {{result:stepName}} 会被替换为对应步骤的执行结果
func expandStepInput(ctx context.Context, input string) string {
	results := dag.GetResult(ctx)
	if results == nil {
		return input
	}
	for name, result := range results {
		input = strings.ReplaceAll(input, "{{result:"+name+"}}", result)
	}
	return input
}
