package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

type MCPRequest struct {
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI integration",
	Long: `Start a Model Context Protocol (MCP) server on stdio.
This allows AI models (Claude, etc.) to discover and execute
AutoMate tools programmatically.

The MCP server implements the standard protocol:
  - tools/list      - List available tools
  - tools/call      - Execute a tool
  - resources/list  - List tool definitions
  - resources/read  - Read tool definition
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cache, err := core.OpenCache()
		if err != nil {
			return err
		}
		defer cache.Close()

		tools, err := cache.LoadTools()
		if err != nil {
			return fmt.Errorf("load tools: %w", err)
		}

		if len(tools) == 0 {
			fmt.Fprintf(os.Stderr, "No tools cached. Run 'autoxmate sync' first.\n")
			return nil
		}

		fmt.Fprintf(os.Stderr, "AutoXMate MCP server starting (%d tools)...\n", len(tools))
		fmt.Fprintf(os.Stderr, "Listening on stdio for MCP protocol messages.\n")

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			var req MCPRequest
			if err := json.Unmarshal([]byte(line), &req); err != nil {
				continue
			}

			resp := handleMCPRequest(req, tools)
			data, _ := json.Marshal(resp)
			fmt.Println(string(data))
		}

		return scanner.Err()
	},
}

func handleMCPRequest(req MCPRequest, tools []core.ToolDefinition) MCPResponse {
	switch req.Method {
	case "tools/list":
		return handleToolsList(req, tools)
	case "tools/call":
		return handleToolsCall(req, tools)
	case "resources/list":
		return handleResourcesList(req, tools)
	case "resources/read":
		return handleResourcesRead(req, tools)
	default:
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

type MCPToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func handleToolsList(req MCPRequest, tools []core.ToolDefinition) MCPResponse {
	var result []MCPToolSpec

	for _, t := range tools {
		if t.Name == "" {
			continue
		}

		spec := MCPToolSpec{
			Name:        t.ID,
			Description: t.Description,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			},
		}

		props := spec.InputSchema["properties"].(map[string]interface{})
		for _, p := range t.Parameters {
			prop := map[string]interface{}{
				"type":        p.Type,
				"description": p.Description,
			}
			if p.DefaultValue != nil {
				prop["default"] = p.DefaultValue
			}
			props[p.Name] = prop
		}

		result = append(result, spec)
	}

	return MCPResponse{ID: req.ID, Result: result}
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

func handleToolsCall(req MCPRequest, tools []core.ToolDefinition) MCPResponse {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{Code: -32602, Message: "Invalid params: " + err.Error()},
		}
	}

	tool := core.FindToolByID(tools, params.Name)
	if tool == nil {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{Code: -32602, Message: "Tool not found: " + params.Name},
		}
	}

	// Convert arguments to string map
	strArgs := make(map[string]string)
	for k, v := range params.Arguments {
		strArgs[k] = fmt.Sprintf("%v", v)
	}

	output, err := core.ExecuteTool(*tool, strArgs, core.ExecOptions{Quiet: true})
	if err != nil {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		ID: req.ID,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": output},
			},
		},
	}
}

type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

func handleResourcesList(req MCPRequest, tools []core.ToolDefinition) MCPResponse {
	var resources []MCPResource
	for _, t := range tools {
		resources = append(resources, MCPResource{
			URI:         fmt.Sprintf("autoxmate://tools/%s", t.ID),
			Name:        t.Name,
			Description: t.Description,
			MimeType:    "application/json",
		})
	}
	return MCPResponse{ID: req.ID, Result: resources}
}

func handleResourcesRead(req MCPRequest, tools []core.ToolDefinition) MCPResponse {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{Code: -32602, Message: "Invalid params"},
		}
	}

	id := strings.TrimPrefix(params.URI, "autoxmate://tools/")
	tool := core.FindToolByID(tools, id)
	if tool == nil {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{Code: -32602, Message: "Resource not found"},
		}
	}

	data, _ := json.MarshalIndent(tool, "", "  ")
	return MCPResponse{
		ID: req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]string{
				{"uri": params.URI, "mimeType": "application/json", "text": string(data)},
			},
		},
	}
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
