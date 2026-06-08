package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/opentoys/agentsdk/skill"
	"github.com/opentoys/agentsdk/types"
)

type Agent struct {
	client   types.OpenAIChatClient
	messages []types.ChatCompletionMessage
	cfg      *types.Config
	tools    map[string]types.Tool
}

func New(cfg types.Config) *Agent {
	a := &Agent{cfg: &cfg, messages: cfg.History}
	a.tools = make(map[string]types.Tool)
	for _, v := range cfg.Tools {
		a.tools[v.Function.Name] = v
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
	var tools = s.cfg.Tools
	var scripts map[string]string
	var cwd string
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
		sk.BaseTools = append(sk.BaseTools, s.cfg.Tools...)
		tools, scripts = skill.GenerateToolDefinitions(sk)
		cwd = sk.Path
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
				toolOutput, e = s.execTool(ctx, tc, scripts, cwd)
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

func (s *Agent) execTool(ctx context.Context, toolCall types.ToolCall, scripts map[string]string, skillPath string) (out string, e error) {
	// Set workdir if skillPath is available
	if skillPath != "" {
		os.Setenv("WORKDIR", skillPath)
		defer os.Unsetenv("WORKDIR")
	}
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
			var cmd = scriptexec[ext]
			if cmd == "" { // 如果没有匹配的后缀，则直接执行
				cmd = scriptPath
			} else {
				cmd = fmt.Sprintf("%s %s", cmd, scriptPath)
			}

			// Add arguments if provided
			if len(params.Args) > 0 {
				for _, arg := range params.Args {
					cmd += fmt.Sprintf(" %s", shellQuote(arg))
				}
			}

			out, e = bash(ctx, cmd)
			if e != nil {
				return "", fmt.Errorf("tool execution failed for %s: %w", toolCall.Function.Name, e)
			}
			return
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
