package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/opentoys/agentsdk/types"
)

type Connecter = func(ctx context.Context, config Server) (cs types.ClientSessioner, e error)

// Client manages connections to multiple MCP servers.
type Client struct {
	sessions   map[string]types.ClientSessioner
	config     Config
	maxRetries int
	Connecter  Connecter
}

// NewClient creates a new MCP client and connects to the servers defined in the config.
func NewClient(ctx context.Context, config Config) *Client {
	maxRetries := config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // Default to 3 retries
	}
	c := &Client{
		sessions:   make(map[string]types.ClientSessioner),
		config:     config,
		maxRetries: maxRetries,
		Connecter:  config.Connecter,
	}
	for name, server := range config.Servers {
		session, e := c.Connecter(ctx, server)
		if e != nil {
			// Log error but continue connecting to other servers
			fmt.Fprintf(os.Stderr, "Failed to connect to MCP server %s: %v\n", name, e)
		}
		c.sessions[name] = session
	}
	return c
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

// ListTools fetches tools from all connected servers and converts them to OpenAI tools.
func (c *Client) ListTools(ctx context.Context) (allTools []types.Tool, e error) {
	for serverName, session := range c.sessions {
		tools, e := session.ListTools(ctx)
		if e != nil {
			fmt.Fprintf(os.Stderr, "Failed to list tools from server %s: %v\n", serverName, e)
			continue
		}
		for _, tool := range tools {
			tool.Function.Name = fmt.Sprintf("%s__%s", serverName, tool.Function.Name)
			allTools = append(allTools, tool)
		}
	}
	return
}

func (c *Client) printf(ctx context.Context, msg string, args ...any) {
	if c.config.Logger != nil {
		c.config.Logger.Debugf(ctx, msg, args...)
	}
}

// CallTool calls a tool on the appropriate server with retry and reconnection support.
// The tool name is expected to be in the format "serverName__toolName".
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	serverName, toolName, err := parseToolName(name)
	if err != nil {
		return nil, err
	}

	server, ok := c.config.Servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found in config", serverName)
	}

	for i := 0; i < c.maxRetries; i++ {
		session, ok := c.sessions[serverName]
		if !ok {
			return nil, fmt.Errorf("server %s session not found", serverName)
		}

		result, err := session.CallTool(ctx, toolName, args)

		if err == nil {
			return result, nil
		}

		// Check if it's a connection-related error
		if c.isConnectionError(err) {
			c.printf(ctx, "Connection error detected for server %s, attempting reconnection (%d/%d): %v",
				serverName, i+1, c.maxRetries, err)

			// Close the old session
			if session != nil {
				session.Close()
			}

			// Wait with exponential backoff before reconnecting
			if i < c.maxRetries-1 {
				backoff := time.Second * time.Duration(i+1)
				c.printf(ctx, "Waiting %v before reconnecting...", backoff)
				time.Sleep(backoff)

				// Attempt to reconnect
				if sc, reconnectErr := c.Connecter(ctx, server); reconnectErr != nil {
					c.printf(ctx, "Reconnection failed: %v", reconnectErr)
					continue
				} else {
					c.sessions[serverName] = sc
				}
				c.printf(ctx, "Reconnection successful for server %s", serverName)
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
