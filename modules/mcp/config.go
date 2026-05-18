package mcp

import "github.com/opentoys/agentsdk/types"

// Config represents the structure of the ~/.claude.json file.
type Config struct {
	Servers    map[string]Server `json:"mcpServers"`
	MaxRetries int               `json:"maxRetries,omitempty"` // Default retry count for tool calls
	Connecter
	types.Logger
}

// MCPServer represents a single MCP server configuration.
type Server struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type,omitempty"`    // "stdio" (default) or "sse"
	URL         string            `json:"url,omitempty"`     // For SSE
	Headers     map[string]string `json:"headers,omitempty"` // For SSE
}
