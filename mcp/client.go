package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sashabaranov/go-openai"
)

// Client manages connections to multiple MCP servers.
type Client struct {
	sessions   map[string]*mcp.ClientSession
	config     *Config
	maxRetries int
}

// NewClient creates a new MCP client and connects to the servers defined in the config.
func NewClient(ctx context.Context, config *Config) (*Client, error) {
	maxRetries := config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // Default to 3 retries
	}

	c := &Client{
		sessions:   make(map[string]*mcp.ClientSession),
		config:     config,
		maxRetries: maxRetries,
	}

	for name, server := range config.MCPServers {
		if err := c.connectToServer(ctx, name, server); err != nil {
			// Log error but continue connecting to other servers
			fmt.Fprintf(os.Stderr, "Failed to connect to MCP server %s: %v\n", name, err)
		}
	}

	return c, nil
}

func (c *Client) connectToServer(ctx context.Context, name string, server MCPServer) error {
	var transport mcp.Transport

	if server.Type == "sse" {
		sseTransport := &mcp.SSEClientTransport{
			Endpoint: server.URL,
		}
		if len(server.Headers) > 0 {
			sseTransport.HTTPClient = &http.Client{
				Transport: &headerTransport{
					Transport: http.DefaultTransport,
					Headers:   server.Headers,
				},
			}
		}
		transport = sseTransport
	} else {
		// Default to stdio
		cmd := exec.Command(server.Command, server.Args...)
		cmd.Env = os.Environ()
		for k, v := range server.Env {
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

	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.sessions[name] = session
	return nil
}

type headerTransport struct {
	Transport http.RoundTripper
	Headers   map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}
	return t.Transport.RoundTrip(req)
}

// Close closes all connections.
func (c *Client) Close() error {
	var errs []error
	for _, session := range c.sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to close some connections: %v", errs)
	}
	return nil
}

// GetTools fetches tools from all connected servers and converts them to OpenAI tools.
func (c *Client) GetTools(ctx context.Context) ([]openai.Tool, error) {
	var allTools []openai.Tool

	for serverName, session := range c.sessions {
		listToolsResult, err := session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list tools from server %s: %v\n", serverName, err)
			continue
		}

		for _, tool := range listToolsResult.Tools {
			openaiTool := openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        fmt.Sprintf("%s__%s", serverName, tool.Name),
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
			allTools = append(allTools, openaiTool)
		}
	}

	return allTools, nil
}

// CallTool calls a tool on the appropriate server with retry and reconnection support.
// The tool name is expected to be in the format "serverName__toolName".
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	serverName, toolName, err := parseToolName(name)
	if err != nil {
		return nil, err
	}

	server, ok := c.config.MCPServers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found in config", serverName)
	}

	for i := 0; i < c.maxRetries; i++ {
		session, ok := c.sessions[serverName]
		if !ok {
			return nil, fmt.Errorf("server %s session not found", serverName)
		}

		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		})

		if err == nil {
			return result, nil
		}

		// Check if it's a connection-related error
		if c.isConnectionError(err) {
			log.Printf("Connection error detected for server %s, attempting reconnection (%d/%d): %v",
				serverName, i+1, c.maxRetries, err)

			// Close the old session
			if session != nil {
				session.Close()
			}

			// Wait with exponential backoff before reconnecting
			if i < c.maxRetries-1 {
				backoff := time.Second * time.Duration(i+1)
				log.Printf("Waiting %v before reconnecting...", backoff)
				time.Sleep(backoff)

				// Attempt to reconnect
				if reconnectErr := c.connectToServer(ctx, serverName, server); reconnectErr != nil {
					log.Printf("Reconnection failed: %v", reconnectErr)
					continue
				}
				log.Printf("Reconnection successful for server %s", serverName)
				continue
			}
		}

		// For non-connection errors, return immediately
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	return nil, fmt.Errorf("failed to call tool after %d retries", c.maxRetries)
}

// isConnectionError checks if an error is related to connection issues
func (c *Client) isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "connection closed") ||
		strings.Contains(errMsg, "EOF") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "connection reset")
}

func parseToolName(name string) (string, string, error) {
	parts := strings.Split(name, "__")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tool name format: %s", name)
	}
	return parts[0], parts[1], nil
}
