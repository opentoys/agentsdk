package agentsdk

import (
	"context"
	"strings"

	"github.com/opentoys/agentsdk/skill"
	"github.com/opentoys/agentsdk/tool"
)

var scriptexec = map[string]string{
	".py":    "python3",
	".js":    "node",
	".tengo": "tengo",
}

var bash = func(ctx context.Context, cmd string) (string, error) {
	return tool.Bash(cmd)
}

func RegisterExec(ext, exec string) {
	scriptexec[ext] = exec
}

func RegisterBash(f func(context.Context, string) (string, error)) {
	bash = f
}

var basetoolinput = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"input": map[string]any{
			"type":        "string",
			"description": "传递给子代理的自然语言任务描述",
		},
	},
	"required": []string{"input"},
}

func defaultMap(s ...map[string]any) map[string]any {
	if len(s) == 0 {
		return nil
	}
	for _, v := range s {
		if v != nil {
			return v
		}
	}
	return s[len(s)-1]
}

// shellQuote quotes a string for safe shell execution
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Simple quoting - wrap in single quotes and escape existing single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// getAvailableSkillNames returns a slice of available skill names for error messages
func getAvailableSkillNames(skills map[string]*skill.SkillPackage) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	return names
}

// extractSkillName extracts the skill name from AI response content
func extractSkillName(content string, skills map[string]*skill.SkillPackage) string {
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
