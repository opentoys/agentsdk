package mcp

// Basic implementation, in order to rely only on the standard library
/*

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/opentoys/agentsdk/types"
)

func Clienter(ctx context.Context, config *Server) (cs types.ClientSessioner, e error) {
	var transport mcp.Transport

	if config.Type == "sse" {
		sseTransport := &mcp.SSEClientTransport{
			Endpoint: config.URL,
		}
		if len(config.Headers) > 0 {
			sseTransport.HTTPClient = &http.Client{
				Transport: &headerTransport{
					Transport: http.DefaultTransport,
					Headers:   config.Headers,
				},
			}
		}
		transport = sseTransport
	} else {
		// Default to stdio
		cmd := exec.Command(config.Command, config.Args...)
		cmd.Env = os.Environ()
		for k, v := range config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// Capture stderr for debugging
		cmd.Stderr = os.Stderr

		transport = &mcp.CommandTransport{
			Command: cmd,
		}
	}

	mcpClient := mcp.NewClient(&mcp.Implementation{
		Name:    "goskills",
		Version: "0.1.0",
	}, nil)

	session, e := mcpClient.Connect(ctx, transport, nil)
	if e != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", e)
	}
	return &Session{cs: session}, nil
}

type Session struct {
	cs *mcp.ClientSession
}

func (s *Session) ListTools(ctx context.Context) (tools []types.Tool, e error) {
	resp, e := s.cs.ListTools(ctx, &mcp.ListToolsParams{})
	if e != nil {
		return
	}
	for _, tool := range resp.Tools {
		tools = append(tools, types.Tool{
			Type: types.ToolTypeFunction,
			Function: &types.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return
}

func (s *Session) CallTool(ctx context.Context, name string, args map[string]any) (rw any, e error) {
	rw, e = s.cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	return
}

func (s *Session) Close() (e error) {
	return s.cs.Close()
}

*/
