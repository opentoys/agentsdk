package tool

import "github.com/sashabaranov/go-openai"

type Tool struct {
	Define openai.Tool
	Exec   func(in string) (out string, e error)
	Prompt string
}

type Tools map[string]*Tool

func (s Tools) Base() (tools []openai.Tool) {
	for _, v := range s {
		tools = append(tools, v.Define)
	}
	return
}
