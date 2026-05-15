package types

import (
	"context"
)

const (
	ToolTypeFunction = "function"
)

type ClientSessioner interface {
	GetTools(ctx context.Context) ([]Tool, error)
	CallTool(context.Context, string, map[string]any) (any, error)
}

type Tool struct {
	Prompt   string                                                     `json:"-"`
	Type     string                                                     `json:"type"`
	Function *FunctionDefinition                                        `json:"function,omitempty"`
	Exec     func(ctx context.Context, in string) (out string, e error) `json:"-"`
}

type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Strict      bool   `json:"strict,omitempty"`
	Parameters  any    `json:"parameters"`
}
