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
	Prompt   string              `json:"-"`
	Type     string              `json:"type"`
	Function *FunctionDefinition `json:"function,omitempty"`
	Exec     Runner              `json:"-"`
}

type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Strict      bool           `json:"strict,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}
