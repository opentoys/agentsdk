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

	"github.com/opentoys/agentsdk/types"
)

// DefineHTTPTool name: http
func DefineHTTPTool() types.Tool {
	envs := os.Environ()
	envmap := make(map[string]string)
	for _, v := range envs {
		lst := strings.Split(v, "=")
		envmap[lst[0]] = strings.Join(lst[1:], "=")
	}
	return types.Tool{
		Type: types.ToolTypeFunction,
		Function: &types.FunctionDefinition{
			Name:        "http",
			Description: "Run a http request. Use for: send a http reuqest.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"method": map[string]any{
						"type":        "string",
						"description": "The http request method.",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "The http request method.",
					},
					"header": map[string]any{
						"type":        "string",
						"description": `The http request header json string. Such as: {"content-type":"plian/text"}`,
					},
					"body": map[string]any{
						"type":        "string",
						"description": "The http request body.",
					},
					"timeout": map[string]any{
						"type":        "string",
						"description": "The http request for timeout. such as: 30s, 1m30s, 500ms etc.",
					},
				},
				"required": []string{"method", "url"},
			},
		},
		Exec: func(ctx context.Context, in string) (out string, e error) {
			var params httpParams
			for k, v := range envmap {
				in = strings.ReplaceAll(in, "$"+k, v)
				in = strings.ReplaceAll(in, "{{"+k+"}}", v)
				in = strings.ReplaceAll(in, "${"+k+"}", v)
			}
			if e = json.Unmarshal([]byte(in), &params); e != nil {
				e = fmt.Errorf("failed to unmarshal http arguments: %w (cleaned args: %s)", e, in)
				return
			}
			out, e = request(params)
			return
		},
		Prompt: `**http(method, url, header, body)**: Universal tool for executing http request:
- send get request.
- send get request with header.
- send post request with body.
- send post request with body and header.
`,
	}
}

type httpParams struct {
	Method  string `json:"method"`
	Url     string `json:"url"`
	Header  string `json:"header"`
	Body    string `json:"body"`
	Timeout string `json:"timeout"`
}

func request(in httpParams) (rw string, e error) {
	ts, e := time.ParseDuration(in.Timeout)
	if e != nil {
		ts = time.Second * 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), ts)
	defer cancel()

	req, e := http.NewRequestWithContext(ctx, in.Method, in.Url, bytes.NewReader([]byte(in.Body)))
	if e != nil {
		return
	}
	if in.Header != "" {
		var header map[string]string
		if e = json.Unmarshal([]byte(in.Header), &header); e != nil {
			return
		}
		for k, v := range header {
			req.Header.Add(k, v)
		}
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
	rw = string(buf)
	return
}
