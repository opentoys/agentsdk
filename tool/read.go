package tool

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func DefineReadLocal(dirs ...fs.FS) *Tool {
	return &Tool{
		Define: openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "read",
				Description: "Provides file and folder reading functions",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "Use for when you need to read local information",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		Exec: func(in string) (out string, e error) {
			var params struct {
				Path string `json:"path"`
			}
			if e = json.Unmarshal([]byte(in), &params); e != nil {
				e = fmt.Errorf("failed to unmarshal bash arguments: %w (cleaned args: %s)", e, in)
				return
			}
			for _, v := range dirs {
				rw, _ := ReadFS(v, params.Path)
				if rw != "" {
					return rw, nil
				}
			}
			return ReadLocal(params.Path)
		},
		Prompt: `**read(path)**: Provides file and folder reading functions
  Use for when you need to read local information

`,
	}
}

func ReadFS(root fs.FS, path string) (rw string, e error) {
	f, e := fs.Stat(root, path)
	if e != nil {
		return "", e
	}
	if f.IsDir() {
		ents, e := fs.ReadDir(root, path)
		if e != nil {
			return "", e
		}
		var sw strings.Builder
		for _, v := range ents {
			if v.IsDir() {
				sw.WriteString(fmt.Sprintf("dir: %s\n", v.Name()))
			} else {
				f, _ := v.Info()
				sw.WriteString(fmt.Sprintf("file: %s	size: %d	modification: %s	\n", v.Name(), f.Size(), f.ModTime()))
			}
		}
		return sw.String(), nil
	}

	buf, e := fs.ReadFile(root, path)
	rw = string(buf)
	return
}

func ReadLocal(path string) (rw string, e error) {
	path, e = filepath.Abs(path)
	if e != nil {
		return
	}

	fo, e := os.Stat(path)
	if e != nil {
		return
	}
	if fo.IsDir() {
		ents, e := os.ReadDir(path)
		if e != nil {
			return "", e
		}
		var sw strings.Builder
		for _, v := range ents {
			if v.IsDir() {
				sw.WriteString(fmt.Sprintf("dir: %s\n", v.Name()))
			} else {
				f, _ := v.Info()
				sw.WriteString(fmt.Sprintf("file: %s	size: %d	modification: %s	\n", v.Name(), f.Size(), f.ModTime()))
			}
		}
		return sw.String(), nil
	}

	buf, e := os.ReadFile(path)
	rw = string(buf)
	return
}
