package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/opentoys/agentsdk/skill"
	"github.com/opentoys/agentsdk/types"
)

// ---------------------------------------------------------------------------
// Run Mode
// ---------------------------------------------------------------------------

// RunMode defines how the agent processes user input.
type RunMode string

const (
	// ModeDirect selects one skill and executes it immediately (default, backward compatible).
	ModeDirect RunMode = "direct"
	// ModePlan creates a multi-step plan before executing skills in sequence.
	ModePlan RunMode = "plan"
	// ModeAuto lets the agent decide: plan if multi-skill is needed, otherwise direct.
	ModeAuto RunMode = "auto"
)

// ---------------------------------------------------------------------------
// Plan types
// ---------------------------------------------------------------------------

// PlanStepStatus tracks the execution state of a plan step.
type PlanStepStatus string

const (
	StepPending   PlanStepStatus = "pending"
	StepRunning   PlanStepStatus = "running"
	StepCompleted PlanStepStatus = "completed"
	StepFailed    PlanStepStatus = "failed"
	StepSkipped   PlanStepStatus = "skipped"
)

// PlanStep represents a single step in the execution plan.
type PlanStep struct {
	ID           string         `json:"id"`
	Description  string         `json:"description"`
	Skill        string         `json:"skill"`        // skill name to use for this step
	Input        string         `json:"input"`        // sub-prompt fed to the skill
	Dependencies []string       `json:"dependencies"` // IDs of steps this depends on
	Status       PlanStepStatus `json:"status"`
	Result       string         `json:"result"`
	Error        string         `json:"error,omitempty"`
}

// Plan represents a complete execution plan with ordered steps.
type Plan struct {
	Goal    string     `json:"goal"`
	Thought string     `json:"thought,omitempty"` // LLM reasoning about the plan
	Steps   []PlanStep `json:"steps"`
	Status  string     `json:"status"`
}

// ---------------------------------------------------------------------------
// Plan validation
// ---------------------------------------------------------------------------

// PlanValidateError describes a single validation failure.
type PlanValidateError struct {
	Type    string `json:"type"`    // error category
	Message string `json:"message"` // human-readable description
	StepID  string `json:"step_id"` // which step this error relates to, empty if plan-level
}

// PlanValidate holds the result of validating a plan.
type PlanValidate struct {
	Valid  bool                `json:"valid"`
	Errors []PlanValidateError `json:"errors,omitempty"`
}

// ValidatePlan checks the plan against structural and logical rules.
func (p *Plan) ValidatePlan(skills map[string]skill.SkillPackage) PlanValidate {
	var errs []PlanValidateError

	// 1. Must have at least one step
	if len(p.Steps) == 0 {
		errs = append(errs, PlanValidateError{
			Type:    "empty_plan",
			Message: "plan has no steps",
		})
		return PlanValidate{Valid: false, Errors: errs}
	}

	// 2. Must have a goal
	if strings.TrimSpace(p.Goal) == "" {
		errs = append(errs, PlanValidateError{
			Type:    "missing_goal",
			Message: "plan goal is empty",
		})
	}

	// Collect all step IDs for fast lookup
	idSet := make(map[string]bool)
	dupCheck := make(map[string]bool)

	for _, step := range p.Steps {
		// 3. Duplicate step IDs
		if dupCheck[step.ID] {
			errs = append(errs, PlanValidateError{
				Type:    "duplicate_id",
				Message: fmt.Sprintf("duplicate step id '%s'", step.ID),
				StepID:  step.ID,
			})
		}
		dupCheck[step.ID] = true
		idSet[step.ID] = true

		// 4. Step ID must not be empty
		if strings.TrimSpace(step.ID) == "" {
			errs = append(errs, PlanValidateError{
				Type:    "empty_id",
				Message: "step has empty id",
			})
		}

		// 5. Description must not be empty
		if strings.TrimSpace(step.Description) == "" {
			errs = append(errs, PlanValidateError{
				Type:    "empty_description",
				Message: fmt.Sprintf("step '%s' has empty description", step.ID),
				StepID:  step.ID,
			})
		}

		// 6. Skill must be valid
		if step.Skill != "" && step.Skill != "none" {
			if _, ok := skills[step.Skill]; !ok {
				errs = append(errs, PlanValidateError{
					Type:    "unknown_skill",
					Message: fmt.Sprintf("step '%s' references unknown skill '%s'", step.ID, step.Skill),
					StepID:  step.ID,
				})
			}
		}

		// 7. Self-dependency check
		for _, depID := range step.Dependencies {
			if depID == step.ID {
				errs = append(errs, PlanValidateError{
					Type:    "self_dependency",
					Message: fmt.Sprintf("step '%s' depends on itself", step.ID),
					StepID:  step.ID,
				})
			}
		}
	}

	// 8. Dependency IDs must reference existing steps
	if len(idSet) > 0 {
		for _, step := range p.Steps {
			for _, depID := range step.Dependencies {
				if !idSet[depID] {
					errs = append(errs, PlanValidateError{
						Type:    "invalid_dependency",
						Message: fmt.Sprintf("step '%s' depends on non-existent step '%s'", step.ID, depID),
						StepID:  step.ID,
					})
				}
			}
		}
	}

	// 9. Circular dependency detection (DFS-based)
	if circErrs := detectCircularDeps(p.Steps, idSet); len(circErrs) > 0 {
		errs = append(errs, circErrs...)
	}

	if len(errs) > 0 {
		return PlanValidate{Valid: false, Errors: errs}
	}
	return PlanValidate{Valid: true}
}

// detectCircularDeps uses DFS to find circular dependencies among steps.
func detectCircularDeps(steps []PlanStep, idSet map[string]bool) []PlanValidateError {
	var errs []PlanValidateError

	// Build adjacency list
	graph := make(map[string][]string)
	for _, step := range steps {
		graph[step.ID] = step.Dependencies
	}

	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		stack[node] = true

		for _, neighbor := range graph[node] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if stack[neighbor] {
				// Found a cycle
				errs = append(errs, PlanValidateError{
					Type:    "circular_dependency",
					Message: fmt.Sprintf("circular dependency detected involving step '%s'", neighbor),
					StepID:  neighbor,
				})
				return true
			}
		}
		stack[node] = false
		return false
	}

	for id := range idSet {
		if !visited[id] {
			dfs(id)
		}
	}

	return errs
}

// formatValidateErrors formats validation errors for LLM consumption.
func formatValidateErrors(errs []PlanValidateError) string {
	var sb strings.Builder
	sb.WriteString("The plan has the following validation errors:\n\n")
	for i, e := range errs {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, e.Type, e.Message))
	}
	sb.WriteString("\nPlease fix ALL errors above and return a corrected JSON plan.")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Auto-fix loop
// ---------------------------------------------------------------------------

// planMaxRetries returns the configured max retries with a default fallback.
func (a *Agent) planMaxRetries() int {
	if a.cfg.PlanMaxRetries <= 0 {
		return 3
	}
	return a.cfg.PlanMaxRetries
}

// tryAutoFixPlan validates the plan and, if invalid, asks the LLM to fix it.
// It retries up to PlanMaxRetries times.
func (a *Agent) tryAutoFixPlan(ctx context.Context, userPrompt string, plan *Plan, skills map[string]skill.SkillPackage) (*Plan, error) {
	for attempt := 0; attempt <= a.planMaxRetries(); attempt++ {
		v := plan.ValidatePlan(skills)
		if v.Valid {
			a.printf(ctx, "[Plan] Validation passed (attempt %d)\n", attempt)
			return plan, nil
		}

		if attempt >= a.planMaxRetries() {
			return nil, fmt.Errorf("plan validation failed after %d auto-fix attempts: %s",
				a.planMaxRetries(), formatValidateErrors(v.Errors))
		}

		a.printf(ctx, "[Plan] Validation failed (attempt %d/%d):\n", attempt+1, a.planMaxRetries())
		for _, e := range v.Errors {
			a.printf(ctx, "  - [%s] %s\n", e.Type, e.Message)
		}

		fixed, err := a.fixPlan(ctx, userPrompt, plan, v.Errors, skills)
		if err != nil {
			return nil, fmt.Errorf("auto-fix failed at attempt %d: %w", attempt+1, err)
		}
		plan = fixed
	}
	return plan, nil
}

// fixPlan asks the LLM to produce a corrected plan based on validation errors.
func (a *Agent) fixPlan(ctx context.Context, userPrompt string, plan *Plan, errs []PlanValidateError, skills map[string]skill.SkillPackage) (*Plan, error) {
	originalJSON, _ := json.Marshal(plan)

	systemPrompt := fmt.Sprintf(`You are a plan fixer. Your ONLY job is to fix validation errors in a plan.
%s

## Fix Rules
1. Output ONLY valid JSON - no markdown, no explanation.
2. Preserve the original plan structure as much as possible.
3. If a step references an unknown skill, either use a valid skill from the list or set it to "none".
4. If dependency IDs are wrong, correct them to reference existing step IDs.
5. If there are duplicate IDs, rename them to be unique.
6. Remove circular dependencies by restructuring the steps.
7. Ensure all steps have valid IDs, descriptions, and skill references.

Return the corrected plan in JSON format following the schema exactly.`, buildPlanSystemPrompt(skills, a.cfg.BaseTools))

	userMsg := fmt.Sprintf("User request:\n%s\n\nOriginal plan (with errors):\n%s\n\n%s",
		userPrompt, string(originalJSON), formatValidateErrors(errs))

	result, err := a.callPlanLLM(ctx, systemPrompt, userMsg, "plan_fix_response")
	if err != nil {
		return nil, fmt.Errorf("fix plan LLM call failed: %w", err)
	}

	var fixed Plan
	if err := json.Unmarshal([]byte(result.Content), &fixed); err != nil {
		return nil, fmt.Errorf("failed to parse fixed plan JSON: %w\nRaw: %s", err, result.Content)
	}

	return &fixed, nil
}

// ---------------------------------------------------------------------------
// LLM call with response_format fallback
// ---------------------------------------------------------------------------

// planLLMResponse carries the result of a plan-generating LLM call.
type planLLMResponse struct {
	Content string
	Usage   types.Usage
}

// callPlanLLM sends a plan-related LLM request with automatic response_format fallback.
// Tries json_schema first; if the provider doesn't support it, falls back to json_object.
func (a *Agent) callPlanLLM(ctx context.Context, systemPrompt, userPrompt, schemaName string) (*planLLMResponse, error) {
	schema := planResponseSchema()

	// --- attempt 1: json_schema (strict structured output) ---
	req := types.ChatCompletionRequest{
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: types.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0,
		ResponseFormat: &types.ChatCompletionResponseFormat{
			Type: "json_schema",
			JSONSchema: &types.ChatCompletionResponseFormatJSONSchema{
				Name:   schemaName,
				Schema: jsonMarshaler{schema: schema},
				Strict: true,
			},
		},
	}

	a.debugPrintRequest(ctx, req)
	resp, err := a.cfg.ChatClient.CreateChatCompletion(ctx, req)
	if err == nil {
		a.debugPrintResponse(ctx, resp)
		a.addUsage(resp.Usage)
		return &planLLMResponse{
			Content: strings.TrimSpace(resp.Choices[0].Message.Content),
			Usage:   resp.Usage,
		}, nil
	}

	// --- json_schema unavailable? fall back to json_object ---
	if !isResponseFormatUnavailable(err) {
		return nil, err
	}

	a.printf(ctx, "[Plan] json_schema unavailable, falling back to json_object\n")

	// Prepend JSON-only instruction to system prompt
	systemPrompt = "Output ONLY valid JSON. No markdown wrapping, no code fences, no explanation.\n\n" + systemPrompt

	req2 := types.ChatCompletionRequest{
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: types.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0,
		ResponseFormat: &types.ChatCompletionResponseFormat{
			Type: "json_object",
		},
	}

	a.debugPrintRequest(ctx, req2)
	resp2, err := a.cfg.ChatClient.CreateChatCompletion(ctx, req2)
	if err != nil {
		return nil, err
	}
	a.debugPrintResponse(ctx, resp2)
	a.addUsage(resp2.Usage)

	content := strings.TrimSpace(resp2.Choices[0].Message.Content)
	// Strip markdown fences if LLM still wrapped JSON
	content = extractJSONContent(content)

	return &planLLMResponse{Content: content, Usage: resp2.Usage}, nil
}

func (a *Agent) addUsage(u types.Usage) {
	a.usage.CompletionTokens += u.CompletionTokens
	a.usage.PromptTokens += u.PromptTokens
	a.usage.TotalTokens += u.TotalTokens
}

// isResponseFormatUnavailable checks if the error indicates the provider
// does not support json_schema response format.
func isResponseFormatUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "response_format type is unavailable") ||
		strings.Contains(msg, "response_format") && strings.Contains(msg, "unavailable")
}

// extractJSONContent strips markdown code fences and finds JSON content.
// Handles: ```json {...} ```, ``` {...} ```, plain JSON, and mixed text.
var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")

func extractJSONContent(raw string) string {
	raw = strings.TrimSpace(raw)

	// Try extracting from a code fence block first
	matches := jsonBlockRe.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// No fence: return as-is
	return raw
}

// planResponseSchema returns the JSON schema for plan generation/fix responses.
func planResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"goal":    map[string]any{"type": "string", "description": "Overall goal of the task"},
			"thought": map[string]any{"type": "string", "description": "Brief reasoning about why this plan is appropriate"},
			"steps": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":           map[string]any{"type": "string", "description": "Unique step identifier e.g. step_1"},
						"description":  map[string]any{"type": "string", "description": "What this step accomplishes"},
						"skill":        map[string]any{"type": "string", "description": "Skill name to invoke, or 'none' if no skill needed"},
						"input":        map[string]any{"type": "string", "description": "Specific prompt/input for this step"},
						"dependencies": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "IDs of steps that must complete first"},
					},
					"required": []string{"id", "description", "skill", "input"},
				},
			},
		},
		"required": []string{"goal", "steps"},
	}
}

// findStep locates a step by ID within the plan.
func (p *Plan) findStep(id string) *PlanStep {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			return &p.Steps[i]
		}
	}
	return nil
}

// allDepsCompleted checks whether all dependencies of a step are completed.
func (p *Plan) allDepsCompleted(deps []string) bool {
	for _, depID := range deps {
		dep := p.findStep(depID)
		if dep == nil || dep.Status != StepCompleted {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Agent public plan API
// ---------------------------------------------------------------------------

// CreatePlan generates an execution plan without executing it.
// Users can review, modify the plan, then call ExecutePlan to run it.
func (a *Agent) CreatePlan(ctx context.Context, userPrompt string) (*Plan, error) {
	availableSkills, err := a.discoverSkills(a.cfg.SkillsFS)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	if len(availableSkills) == 0 {
		return nil, errors.New("no valid skills found")
	}
	return a.createPlan(ctx, userPrompt, availableSkills)
}

// ExecutePlan executes a previously created plan step by step.
// Each step runs with an isolated message context, receiving results
// from previous steps as context input.
func (a *Agent) ExecutePlan(ctx context.Context, plan *Plan) (string, error) {
	availableSkills, err := a.discoverSkills(a.cfg.SkillsFS)
	if err != nil {
		return "", fmt.Errorf("failed to discover skills: %w", err)
	}
	return a.executePlan(ctx, plan, availableSkills)
}

// runWithPlan creates and executes a plan internally.
func (a *Agent) runWithPlan(ctx context.Context, userPrompt string) (string, error) {
	plan, err := a.CreatePlan(ctx, userPrompt)
	if err != nil {
		return "", err
	}
	return a.ExecutePlan(ctx, plan)
}

// ---------------------------------------------------------------------------
// Plan generation
// ---------------------------------------------------------------------------

// createPlan generates an execution plan via LLM using structured JSON output.
// After generation, validates the plan and auto-fixes any errors up to PlanMaxRetries times.
func (a *Agent) createPlan(ctx context.Context, userPrompt string, skills map[string]skill.SkillPackage) (*Plan, error) {
	systemPrompt := buildPlanSystemPrompt(skills, a.cfg.BaseTools)
	userMsg := "User request:\n" + userPrompt

	result, err := a.callPlanLLM(ctx, systemPrompt, userMsg, "plan_response")
	if err != nil {
		return nil, fmt.Errorf("plan generation failed: %w", err)
	}

	plan := new(Plan)
	if err := json.Unmarshal([]byte(result.Content), plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w\nRaw: %s", err, result.Content)
	}

	// Validate and auto-fix if needed
	plan, err = a.tryAutoFixPlan(ctx, userPrompt, plan, skills)
	if err != nil {
		return nil, err
	}

	plan.Status = "created"
	return plan, nil
}

// ---------------------------------------------------------------------------
// Plan execution
// ---------------------------------------------------------------------------

// executePlan runs a multi-step plan, executing each step in sequence.
// Results from previous steps are passed as context to subsequent steps.
func (a *Agent) executePlan(ctx context.Context, plan *Plan, skills map[string]skill.SkillPackage) (string, error) {
	if len(plan.Steps) == 0 {
		return "", errors.New("plan has no steps to execute")
	}

	plan.Status = "running"

	var stepResults []string

	for i := range plan.Steps {
		step := &plan.Steps[i]
		step.Status = StepRunning
		a.printf(ctx, "[Plan] Executing step %s: %s (skill: %s)\n", step.ID, step.Description, step.Skill)

		if !plan.allDepsCompleted(step.Dependencies) {
			step.Status = StepSkipped
			step.Error = "dependency step failed or not completed"
			a.printf(ctx, "[Plan] Step %s skipped: dependencies not met\n", step.ID)
			continue
		}

		prevContext := ""
		if len(stepResults) > 0 {
			var ctxParts []string
			for idx, result := range stepResults {
				if idx < len(plan.Steps) {
					ctxParts = append(ctxParts, fmt.Sprintf("[Step %s result]\n%s", plan.Steps[idx].ID, result))
				} else {
					ctxParts = append(ctxParts, result)
				}
			}
			prevContext = strings.Join(ctxParts, "\n---\n")
		}

		result, err := a.executePlanStep(ctx, step, prevContext, skills)
		if err != nil {
			step.Status = StepFailed
			step.Error = err.Error()
			a.printf(ctx, "[Plan] Step %s failed: %v\n", step.ID, err)
			continue
		}

		step.Status = StepCompleted
		step.Result = result
		stepResults = append(stepResults, result)
		a.printf(ctx, "[Plan] Step %s completed\n", step.ID)
	}

	plan.Status = planOutcome(plan)
	return buildPlanSummary(plan), nil
}

func planOutcome(plan *Plan) string {
	allCompleted := true
	hasFailures := false
	for _, step := range plan.Steps {
		if step.Status != StepCompleted {
			allCompleted = false
		}
		if step.Status == StepFailed {
			hasFailures = true
		}
	}
	if allCompleted {
		return "completed"
	}
	if hasFailures {
		return "partial"
	}
	return "completed"
}

// executePlanStep executes a single plan step with its own isolated message context.
func (a *Agent) executePlanStep(ctx context.Context, step *PlanStep, prevContext string, skills map[string]skill.SkillPackage) (string, error) {
	sk, hasSkill := skills[step.Skill]
	var skillPkg *skill.SkillPackage
	if hasSkill {
		skillPkg = &sk
		for _, v := range a.cfg.BaseTools {
			skillPkg.BaseTools = append(skillPkg.BaseTools, v)
		}
	}

	systemContent := buildStepSystemPrompt(step, prevContext, planGoalContext{}, skillPkg)
	messages := []types.ChatCompletionMessage{
		{Role: types.ChatMessageRoleSystem, Content: systemContent},
		{Role: types.ChatMessageRoleUser, Content: step.Input},
	}

	availableTools, scriptMap := buildStepTools(skillPkg, a.cfg.BaseTools)

	if a.cfg.McpSessions != nil {
		mcpTools, err := a.cfg.McpSessions.ListTools(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list MCP tools: %w", err)
		}
		availableTools = append(availableTools, mcpTools...)
	}

	var finalResponse strings.Builder

	for range 20 {
		req := types.ChatCompletionRequest{
			Messages: messages,
			Tools:    availableTools,
		}

		a.debugPrintRequest(ctx, req)
		resp, err := a.cfg.ChatClient.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", fmt.Errorf("ChatCompletion error in step %s: %w", step.ID, err)
		}
		a.debugPrintResponse(ctx, resp)
		a.usage.CompletionTokens += resp.Usage.CompletionTokens
		a.usage.PromptTokens += resp.Usage.PromptTokens
		a.usage.TotalTokens += resp.Usage.TotalTokens

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if msg.ToolCalls == nil {
			finalResponse.WriteString(msg.Content)
			return finalResponse.String(), nil
		}

		for _, tc := range msg.ToolCalls {
			var toolOutput string
			var toolErr error

			if a.cfg.McpSessions != nil && strings.Contains(tc.Function.Name, "__") {
				cleanedArgs := cleanToolArguments(tc.Function.Arguments)
				var args map[string]any
				if err := json.Unmarshal([]byte(cleanedArgs), &args); err != nil {
					toolOutput = fmt.Sprintf("Error unmarshalling arguments: %v (cleaned args: %s)", err, cleanedArgs)
				} else {
					var result any
					result, toolErr = a.cfg.McpSessions.CallTool(ctx, tc.Function.Name, args)
					if toolErr == nil {
						resBytes, _ := json.Marshal(result)
						toolOutput = string(resBytes)
					}
				}
			} else {
				skillPath := ""
				if skillPkg != nil {
					skillPath = skillPkg.Path
				}
				toolOutput, toolErr = a.executeToolCall(ctx, tc, scriptMap, skillPath)
			}

			if toolErr != nil {
				errorMsg := fmt.Sprintf("Tool execution failed: %s\nError details: %v\nTool name: %s\nArguments: %s",
					tc.Function.Name, toolErr, tc.Function.Name, tc.Function.Arguments)
				messages = append(messages, types.ChatCompletionMessage{
					Role:       types.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    errorMsg,
				})
			} else {
				messages = append(messages, types.ChatCompletionMessage{
					Role:       types.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    toolOutput,
				})
			}
		}
	}

	return "", fmt.Errorf("step %s: exceeded maximum tool call iterations (20)", step.ID)
}

// buildStepTools assembles the tool list for a plan step.
func buildStepTools(skillPkg *skill.SkillPackage, baseTools map[string]types.Tool) ([]types.Tool, map[string]string) {
	var availableTools []types.Tool
	var scriptMap map[string]string
	if skillPkg != nil {
		availableTools, scriptMap = skill.GenerateToolDefinitions(skillPkg)
	} else {
		scriptMap = make(map[string]string)
		for _, t := range baseTools {
			availableTools = append(availableTools, t)
		}
	}
	return availableTools, scriptMap
}

// ---------------------------------------------------------------------------
// Prompt builders
// ---------------------------------------------------------------------------

type planGoalContext struct{}

// buildPlanSystemPrompt constructs the system prompt for plan generation.
func buildPlanSystemPrompt(skills map[string]skill.SkillPackage, baseTools map[string]types.Tool) string {
	var sb strings.Builder

	sb.WriteString("You are a planning assistant. Your job is to create a step-by-step execution plan.\n\n")

	sb.WriteString("## Available Skills\n\n")
	for _, sk := range skills {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", sk.Meta.Name, sk.Meta.Description))
	}

	if len(baseTools) > 0 {
		sb.WriteString("\n## Base Tools\n\n")
		for _, t := range baseTools {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Function.Name, t.Function.Description))
		}
	}

	sb.WriteString("\n## Planning Rules\n\n")
	sb.WriteString("1. Break complex tasks into sequential steps, each using ONE skill\n")
	sb.WriteString("2. Simple tasks that need only one skill should have a single step\n")
	sb.WriteString("3. Use 'dependencies' when a step needs the output of another step\n")
	sb.WriteString("4. Steps without dependencies can theoretically run in parallel\n")
	sb.WriteString("5. If no skill is needed for a step, set skill to 'none'\n")
	sb.WriteString("6. The 'input' field should be a clear, actionable sub-prompt for the skill\n")
	sb.WriteString("7. Do NOT include steps that the user didn't ask for\n\n")

	sb.WriteString("Generate a JSON plan following the schema exactly.")

	return sb.String()
}

// buildStepSystemPrompt constructs the system prompt for executing a single plan step.
func buildStepSystemPrompt(step *PlanStep, prevContext string, _ planGoalContext, sk *skill.SkillPackage) string {
	var sb strings.Builder

	if prevContext != "" {
		sb.WriteString("## Context from Previous Steps\n")
		sb.WriteString(prevContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Current Task\n")
	sb.WriteString(fmt.Sprintf("You are executing step '%s': %s\n\n", step.ID, step.Description))

	if sk != nil {
		sb.WriteString("## Skill Instructions\n")
		sb.WriteString(fmt.Sprintf("Skill: %s\n", sk.Meta.Name))
		sb.WriteString(fmt.Sprintf("Description: %s\n\n", sk.Meta.Description))
		sb.WriteString("### Instructions\n")
		sb.WriteString(sk.Body)
		if sk.Path != "" {
			sb.WriteString(fmt.Sprintf("\n\nSkill Root Path: %s\n", sk.Path))
		}
	}

	return sb.String()
}

// buildPlanSummary creates a human-readable summary of plan execution.
func buildPlanSummary(plan *Plan) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Plan: %s\n", plan.Goal))
	sb.WriteString(fmt.Sprintf("Status: %s\n\n", plan.Status))

	for _, step := range plan.Steps {
		icon := "✅"
		switch step.Status {
		case StepFailed:
			icon = "❌"
		case StepSkipped:
			icon = "⏭️"
		case StepPending:
			icon = "⏳"
		}
		sb.WriteString(fmt.Sprintf("%s **%s**: %s (%s)\n", icon, step.ID, step.Description, step.Skill))

		if step.Result != "" {
			sb.WriteString(fmt.Sprintf("   Result: %s\n", truncateStr(step.Result, 200)))
		}
		if step.Error != "" {
			sb.WriteString(fmt.Sprintf("   Error: %s\n", step.Error))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// truncateStr truncates a string to maxLen characters, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// jsonMarshaler wraps a schema map to implement encoding/json.Marshaler.
type jsonMarshaler struct {
	schema map[string]any
}

func (j jsonMarshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.schema)
}
