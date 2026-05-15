package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/opentoys/agentsdk/types"
)

func DefineTavilySearch() types.Tool {
	return types.Tool{
		Type: types.ToolTypeFunction,
		Function: &types.FunctionDefinition{
			Name:        "tavily_search",
			Description: "Performs a web search using the Tavily API for the given query and returns a summary of results.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query.",
					},
				},
				"required": []string{"query"},
			},
		},
		Exec: func(ctx context.Context, in string) (out string, e error) {
			var params struct {
				Query string `json:"query"`
			}
			if e = json.Unmarshal([]byte(in), &params); e != nil {
				e = fmt.Errorf("failed to unmarshal tavily_search arguments: %w (cleaned args: %s)", e, in)
				return
			}
			return TavilySearch(ctx, params.Query)
		},
		Prompt: `**tavily_search(query)**: Perform web search using the Tavily API.
  Use when you need to search the web for current information.
  `,
	}
}

// TavilySearch performs a web search using the Tavily API.
func TavilySearch(ctx context.Context, query string) (string, error) {
	return TavilySearchWithLimit(ctx, query, 20)
}

// TavilySearchWithLimit performs a web search using the Tavily API with a custom result limit.
func TavilySearchWithLimit(ctx context.Context, query string, maxResults int) (string, error) {
	return TavilySearchWithLimitAndURL(ctx, query, maxResults, "https://api.tavily.com/search")
}

// TavilySearchWithLimitAndURL performs a web search using the Tavily API with a custom result limit and URL (for testing).
func TavilySearchWithLimitAndURL(ctx context.Context, query string, maxResults int, apiURL string) (string, error) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("TAVILY_API_KEY environment variable is not set")
	}

	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 100 {
		maxResults = 100
	}

	requestBody, err := json.Marshal(map[string]any{
		"query":          query,
		"search_depth":   "basic",
		"max_results":    maxResults,
		"include_images": true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform Tavily search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Tavily API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
		Images []string `json:"images"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Tavily response: %w", err)
	}

	var sb bytes.Buffer
	for _, item := range result.Results {
		sb.WriteString(fmt.Sprintf("Title: %s\nURL: %s\nContent: %s\n\n", item.Title, item.URL, item.Content))
	}

	if len(result.Images) > 0 {
		sb.WriteString("\nRelevant Images:\n")
		for _, imgURL := range result.Images {
			sb.WriteString(fmt.Sprintf("- Image URL: %s\n", imgURL))
		}
		sb.WriteString("\n")
	}

	if sb.Len() == 0 {
		return "No results found.", nil
	}

	return sb.String(), nil
}
