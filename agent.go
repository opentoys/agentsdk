package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/opentoys/agentsdk/mcp"
	"github.com/opentoys/agentsdk/skill"
	"github.com/opentoys/agentsdk/tool"
	openai "github.com/sashabaranov/go-openai"
)

const xcontextid = "x-context-id"

func GetContextID(ctx context.Context) (id string) {
	id, _ = ctx.Value(xcontextid).(string)
	return
}

func SetContextID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, xcontextid, id)
}

// OpenAIChatClient interface for dependency injection and testing
type OpenAIChatClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// Agent manages the skill discovery, selection, and execution process.
type Agent struct {
	client    OpenAIChatClient
	cfg       *Config
	messages  []openai.ChatCompletionMessage // Stores the conversation history
	mcpClient *mcp.Client
}

// Config holds all the necessary configuration for the runner.
type Config struct {
	APIKey        string
	APIBase       string
	Model         string
	SkillsDir     string
	MCPServers    map[string]mcp.MCPServer
	MCPMaxRetries int
	Debug         bool
	History       []openai.ChatCompletionMessage
	BaseTools     map[string]*tool.Tool
}

// New creates and initializes a new Agent.
func New(cfg *Config) (a *Agent, e error) {
	if cfg.APIKey == "" {
		return nil, errors.New("API key is not set")
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o" // Default model
	}

	mcpClient, e := mcp.NewClient(context.Background(), &mcp.Config{
		MCPServers: cfg.MCPServers,
		MaxRetries: cfg.MCPMaxRetries,
	})
	if e != nil {
		return
	}

	openaiConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.APIBase != "" {
		openaiConfig.BaseURL = cfg.APIBase
	}
	client := openai.NewClientWithConfig(openaiConfig)

	return &Agent{
		client:    client,
		cfg:       cfg,
		messages:  cfg.History, // Initialize empty message history
		mcpClient: mcpClient,
	}, nil
}

// Run executes the main skill selection and execution logic for a single turn.
func (a *Agent) Run(ctx context.Context, userPrompt string) (resp string, e error) {
	selectedSkill, err := a.selectAndPrepareSkill(ctx, userPrompt)
	if err != nil {
		return "", err
	}
	selectedSkill.BaseTools = tool.Tools(a.cfg.BaseTools).Base()
	resp, e = a.executeSkillWithTools(ctx, userPrompt, selectedSkill)
	return
}

// Chat reuse all configurations, just reset context message
func (a *Agent) Chat(ctx context.Context, prompt string) (content string, e error) {
	req := openai.ChatCompletionRequest{
		Model: a.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0,
	}
	a.debugPrintRequest(req)
	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	a.debugPrintResponse(resp)
	content = resp.Choices[0].Message.Content
	return
}

// Messages reuse all configurations, just reset context message
func (a *Agent) Messages() (lst []openai.ChatCompletionMessage) {
	return append(lst, a.messages...)
}

// NewChat reuse all configurations, just reset context message
func (a *Agent) NewChat(history []openai.ChatCompletionMessage) (n *Agent) {
	n = &Agent{
		client:    a.client,
		cfg:       a.cfg,
		messages:  append([]openai.ChatCompletionMessage{}, history...),
		mcpClient: a.mcpClient,
	}
	return
}

// selectAndPrepareSkill discovers and selects the appropriate skill.
func (a *Agent) selectAndPrepareSkill(ctx context.Context, userPrompt string) (*skill.SkillPackage, error) {
	availableSkills, err := a.discoverSkills(a.cfg.SkillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	if len(availableSkills) == 0 {
		return nil, errors.New("no valid skills found")
	}
	// --- STEP 2: SKILL SELECTION ---
	var selectedSkillName string

	// If skill is explicitly specified via --skill flag, use it directly
	// Otherwise, ask LLM to select the best skill
	selectedSkillName, err = a.selectSkill(ctx, userPrompt, availableSkills)
	if err != nil {
		return nil, fmt.Errorf("failed during skill selection: %w", err)
	}

	selectedSkill, ok := availableSkills[selectedSkillName]
	if !ok {
		return nil, fmt.Errorf("skill '%s' not found. Available skills: %v", selectedSkillName, getAvailableSkillNames(availableSkills))
	}
	return &selectedSkill, nil
}

// getAvailableSkillNames returns a slice of available skill names for error messages
func getAvailableSkillNames(skills map[string]skill.SkillPackage) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	return names
}

func (a *Agent) discoverSkills(skillsRoot string) (map[string]skill.SkillPackage, error) {
	packages, err := skill.ParseSkillPackages(skillsRoot)
	if err != nil {
		return nil, err
	}

	skills := make(map[string]skill.SkillPackage, len(packages))
	for _, pkg := range packages {
		if pkg != nil {
			skills[pkg.Meta.Name] = *pkg
		}
	}

	return skills, nil
}

func (a *Agent) selectSkill(ctx context.Context, userPrompt string, skills map[string]skill.SkillPackage) (string, error) {
	var sb strings.Builder
	sb.WriteString("User Request: " + "" + userPrompt + "" + "\n\n")
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

	skillPrompt := skill.SkillsToPrompt(skills, a.cfg.BaseTools)

	// Use a temporary message history for skill selection
	selectionMessages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a skill selection assistant. Your ONLY job is to select the most appropriate skill from the available list. You must ALWAYS choose exactly one skill - never refuse to select or try to answer the question yourself.\n" + skillPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: sb.String(),
		},
	}

	req := openai.ChatCompletionRequest{
		Model:       a.cfg.Model,
		Messages:    selectionMessages,
		Temperature: 0,
	}

	a.debugPrintRequest(req)
	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	a.debugPrintResponse(resp)

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.Trim(content, "'\"")

	// Extract just the skill name if there's extra text
	// Look for skill names in the content
	skillName := extractSkillName(content, skills)

	return skillName, nil
}

// extractSkillName extracts the skill name from AI response content
func extractSkillName(content string, skills map[string]skill.SkillPackage) string {
	// First, check if the content is already a valid skill name
	if _, exists := skills[content]; exists {
		return content
	}

	// Convert content to lowercase for case-insensitive matching
	lowerContent := strings.ToLower(content)

	// Look for any skill name mentioned in the content
	for skillName := range skills {
		// Check exact match (case-insensitive)
		if strings.Contains(lowerContent, strings.ToLower(skillName)) {
			return skillName
		}
	}

	// If no skill name found, return the original content
	// This preserves the existing behavior when no skills match
	return content
}

// debugPrintRequest prints the LLM request in debug mode
func (a *Agent) debugPrintRequest(req openai.ChatCompletionRequest) {
	if !a.cfg.Debug {
		return
	}
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
	fmt.Fprintln(os.Stderr, "LLM Request:")
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
	jsonBytes, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling request to JSON: %v\n", err)
	} else {
		fmt.Fprintln(os.Stderr, string(jsonBytes))
	}
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
}

// debugPrintResponse prints the LLM response in debug mode
func (a *Agent) debugPrintResponse(resp openai.ChatCompletionResponse) {
	if !a.cfg.Debug {
		return
	}
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
	fmt.Fprintln(os.Stderr, "LLM Response:")
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
	jsonBytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling response to JSON: %v\n", err)
	} else {
		fmt.Fprintln(os.Stderr, string(jsonBytes))
	}
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
}

// executeSkillWithTools sets up the initial system prompt and starts the tool-use conversation.
func (a *Agent) executeSkillWithTools(ctx context.Context, userPrompt string, skill *skill.SkillPackage) (string, error) {
	// Prepare the system message once
	var skillBody strings.Builder
	skillBody.WriteString("## SELECTED SKILL\n")
	skillBody.WriteString(fmt.Sprintf("Name: %s\n", skill.Meta.Name))
	skillBody.WriteString(fmt.Sprintf("Description: %s\n", skill.Meta.Description))
	skillBody.WriteString(fmt.Sprintf("Version: %s\n\n", skill.Meta.Version))
	skillBody.WriteString("## SKILL INSTRUCTIONS\n")
	skillBody.WriteString(skill.Body)
	skillBody.WriteString("\n\n ## SKILL CONTEXT\n")
	skillBody.WriteString(fmt.Sprintf("Skill Root Path: %s\n", skill.Path))
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: skillBody.String(),
	})

	return a.continueSkillWithTools(ctx, userPrompt, skill)
}

// continueSkillWithTools continues a conversation with a new user prompt.
func (a *Agent) continueSkillWithTools(ctx context.Context, userPrompt string, skillp *skill.SkillPackage) (string, error) {
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userPrompt,
	})

	availableTools, scriptMap := skill.GenerateToolDefinitions(skillp)

	// Add MCP tools if client is available
	if a.mcpClient != nil {
		mcpTools, err := a.mcpClient.GetTools(ctx)
		if err != nil {
			return "", err
		} else {
			availableTools = append(availableTools, mcpTools...)
		}
	}

	var finalResponse strings.Builder

	for range 20 { // Limit to 20 iterations to prevent infinite loops
		req := openai.ChatCompletionRequest{
			Model:    a.cfg.Model,
			Messages: a.messages, // Use agent's messages
			Tools:    availableTools,
		}

		a.debugPrintRequest(req)
		resp, err := a.client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", fmt.Errorf("ChatCompletion error: %w", err)
		}
		a.debugPrintResponse(resp)

		msg := resp.Choices[0].Message
		a.messages = append(a.messages, msg) // Append LLM's response

		if msg.ToolCalls == nil {
			finalResponse.WriteString(msg.Content)
			return finalResponse.String(), nil
		}

		for _, tc := range msg.ToolCalls {
			var toolOutput string
			var err error

			// Check if it is an MCP tool
			if a.mcpClient != nil && strings.Contains(tc.Function.Name, "__") {
				// Clean arguments for MCP tools too
				cleanedArgs := cleanToolArguments(tc.Function.Arguments)

				var args map[string]any
				if err := json.Unmarshal([]byte(cleanedArgs), &args); err != nil {
					toolOutput = fmt.Sprintf("Error unmarshalling arguments: %v (cleaned args: %s)", err, cleanedArgs)
				} else {
					var result any
					result, err = a.mcpClient.CallTool(ctx, tc.Function.Name, args)
					if err == nil {
						// Convert result to string/JSON
						resBytes, _ := json.Marshal(result)
						toolOutput = string(resBytes)
					}
				}
			} else {
				toolOutput, err = a.executeToolCall(tc, scriptMap, skillp.Path)
			}

			if err != nil {
				// Provide detailed error information to help LLM understand what went wrong
				errorMsg := fmt.Sprintf("Tool execution failed: %s\nError details: %v\nTool name: %s\nArguments: %s\n\nYou can try:\n1. Retry with different parameters\n2. Use a different tool to fix it\n3. Modify your approach",
					tc.Function.Name, err, tc.Function.Name, tc.Function.Arguments)
				a.messages = append(a.messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    errorMsg,
				})
			} else {
				a.messages = append(a.messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    toolOutput,
				})
			}
		}
	}
	return "", errors.New("exceeded maximum tool call iterations")
}

// cleanToolArguments cleans the tool arguments by removing code fences, fixing escape sequences,
// and trimming whitespace. This handles cases where LLM returns malformed JSON arguments.
func cleanToolArguments(args string) string {
	// Trim leading and trailing whitespace
	args = strings.TrimSpace(args)

	// Remove code fence patterns: ```json, ```, ``` etc.
	// This handles cases like:
	// ```json
	// {"key": "value"}
	// ```
	fencePatterns := []string{
		"```json", "```JSON",
		"```", // Must be after specific variants
	}

	for _, fence := range fencePatterns {
		// Remove fence from start
		if strings.HasPrefix(args, fence) {
			args = strings.TrimPrefix(args, fence)
			args = strings.TrimLeft(args, "\n\r\t ")
		}
		// Remove fence from end
		if strings.HasSuffix(args, fence) {
			args = strings.TrimSuffix(args, fence)
			args = strings.TrimRight(args, "\n\r\t ")
		}
	}

	// Handle JSON wrapped in single quotes: '{"key": "value"}' -> {"key": "value"}
	// But only if the entire string is wrapped (starts and ends with single quote)
	if len(args) >= 2 && strings.HasPrefix(args, "'") && strings.HasSuffix(args, "'") {
		args = args[1 : len(args)-1]
	}

	// Fix over-escaped quotes in the JSON structure.
	// Some LLMs return: {\"key\": \"value\"} instead of {"key": "value"}
	// We detect this by checking if the string starts with {\" instead of {"
	// This indicates the quotes in the JSON structure are escaped.
	if strings.HasPrefix(args, `{\"`) || strings.HasPrefix(args, `[\"`) {
		// Replace \" with " throughout
		args = strings.ReplaceAll(args, `\"`, `"`)
	}

	// Fix escaped single quotes: \' -> ' (sometimes LLMs escape single quotes unnecessarily)
	args = strings.ReplaceAll(args, `\'`, `'`)

	// Final trim to ensure clean output
	return strings.TrimSpace(args)
}

func (a *Agent) executeToolCall(toolCall openai.ToolCall, scriptMap map[string]string, skillPath string) (string, error) {
	var toolOutput string
	var err error
	// Set workdir if skillPath is available
	if skillPath != "" {
		os.Setenv("WORKDIR", skillPath)
		defer os.Unsetenv("WORKDIR")
	}
	// Clean the arguments before parsing
	cleanedArgs := cleanToolArguments(toolCall.Function.Arguments)

	var exec, ok = a.cfg.BaseTools[toolCall.Function.Name]
	if !ok {
		// Handle custom script tools from scriptMap
		if scriptPath, ok := scriptMap[toolCall.Function.Name]; ok {
			var params struct {
				Args []string `json:"args"`
			}
			if cleanedArgs != "" {
				if err := json.Unmarshal([]byte(cleanedArgs), &params); err != nil {
					return "", fmt.Errorf("failed to unmarshal script arguments: %w (cleaned args: %s)", err, cleanedArgs)
				}
			}

			// Build command based on script extension
			var cmd string
			if strings.HasSuffix(scriptPath, ".py") {
				cmd = fmt.Sprintf("python3 %s", scriptPath)
			} else if strings.HasSuffix(scriptPath, ".ts") || strings.HasSuffix(scriptPath, ".js") {
				cmd = fmt.Sprintf("npx tsx %s", scriptPath)
			} else {
				cmd = fmt.Sprintf("bash %s", scriptPath)
			}

			// Add arguments if provided
			if len(params.Args) > 0 {
				for _, arg := range params.Args {
					cmd += fmt.Sprintf(" %s", shellQuote(arg))
				}
			}

			toolOutput, err = tool.Bash(cmd)
			if err != nil {
				return "", fmt.Errorf("tool execution failed for %s: %w", toolCall.Function.Name, err)
			}
		}
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
	toolOutput, err = exec.Exec(cleanedArgs)
	if err != nil {
		return "", fmt.Errorf("tool execution failed for %s: %w", toolCall.Function.Name, err)
	}
	return toolOutput, nil
}

// shellQuote quotes a string for safe shell execution
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Simple quoting - wrap in single quotes and escape existing single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
