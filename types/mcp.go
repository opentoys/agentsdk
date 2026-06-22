package types

import (
	"context"
)

const (
	ToolTypeFunction = "function"
)

type ClientSessioner interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(context.Context, string, map[string]any) (any, error)
	Close() error
}

type Tool struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Description string         `json:"description,omitempty"`
	Strict      bool           `json:"strict,omitempty"`
	Parameters  map[string]any `json:"parameters"`
	Prompt      string         `json:"-"`
	Exec        Runner         `json:"-"`
}
