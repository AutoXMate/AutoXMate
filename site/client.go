package site

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/AutoXMate/AutoXmate/core"
)

const DefaultBaseURL = "https://autoxmate.github.io/api/v1"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchCommands downloads commands.json from the site
func (c *Client) FetchCommands() ([]core.ToolDefinition, error) {
	url := c.BaseURL + "/commands.json"
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch commands: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch commands: HTTP %d", resp.StatusCode)
	}

	var tools []core.ToolDefinition
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return nil, fmt.Errorf("decode commands: %w", err)
	}

	return tools, nil
}

// FetchQueriesIndex downloads queries.json from the site
func (c *Client) FetchQueriesIndex() (*core.QueriesIndex, error) {
	url := c.BaseURL + "/queries.json"
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch queries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch queries: HTTP %d", resp.StatusCode)
	}

	var idx core.QueriesIndex
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, fmt.Errorf("decode queries: %w", err)
	}

	return &idx, nil
}

// Sync downloads tool definitions and queries index, caching them locally
func Sync(client *Client, cache *core.Cache) (int, error) {
	fmt.Println("Syncing tool definitions from AutoXMate site...")

	tools, err := client.FetchCommands()
	if err != nil {
		return 0, fmt.Errorf("fetch from site: %w", err)
	}

	if err := cache.StoreTools(tools); err != nil {
		return 0, fmt.Errorf("cache tools: %w", err)
	}

	// Also fetch and cache queries index
	idx, err := client.FetchQueriesIndex()
	if err == nil {
		if err := cache.StoreQueriesIndex(idx); err != nil {
			fmt.Printf("  Warning: failed to cache queries index: %v\n", err)
		} else {
			fmt.Printf("  Cached %d actions, %d tools\n", len(idx.Actions), len(idx.Tools))
		}
	} else {
		fmt.Printf("  Warning: queries index not available: %v\n", err)
	}

	if err := cache.SetLastSync(time.Now()); err != nil {
		return 0, fmt.Errorf("update sync timestamp: %w", err)
	}

	return len(tools), nil
}
