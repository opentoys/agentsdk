package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	toon "github.com/toon-format/toon-go"
)

var envs = make(map[string]string)

func init() {
	for _, env := range os.Environ() {
		kv := strings.Split(env, "=")
		envs[kv[0]] = kv[1]
	}
}

func DefineHttpRequest() *Tool {
	return &Tool{
		Define: openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "http",
				Description: "Curl-compatible http request tool, does not rely on bash",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "curl requests full command",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		Exec: func(in string) (out string, e error) {
			var params struct {
				Command string `json:"command"`
			}
			if e = json.Unmarshal([]byte(in), &params); e != nil {
				e = fmt.Errorf("failed to unmarshal bash arguments: %w (cleaned args: %s)", e, in)
				return
			}
			for k, v := range envs {
				params.Command = strings.ReplaceAll(params.Command, "$"+k, v)
			}
			return HttpRequest(params.Command)
		},
		Prompt: `**http(curls)**: Curl-compatible http request tool, does not rely on bash.
  Use for Used to obtain http api information.
`,
	}
}

func HttpRequest(cmd string) (rw string, e error) {
	curl := CurlParse(cmd)
	var timeout = time.Minute
	if curl.Timeout > 0 {
		timeout = time.Second * time.Duration(curl.Timeout)
	}
	var ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, curl.Method, curl.URL, bytes.NewBufferString(curl.Body))
	if e != nil {
		return
	}
	for k, v := range curl.Headers {
		req.Header.Set(k, v)
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return
	}
	defer resp.Body.Close()
	buf, e := io.ReadAll(resp.Body)
	if e != nil {
		return
	}
	if len(buf) > 10 && strings.Contains(resp.Header.Get("Content-Type"), "json") {
		var jsondata map[string]any
		if e = json.Unmarshal(buf, &jsondata); e != nil {
			return
		}
		rw, e = toon.MarshalString(jsondata)
		return
	}
	rw = string(buf)
	return
}
