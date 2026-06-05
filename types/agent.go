package types

import (
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
}
