package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

var runCmd = &cobra.Command{
	Use:   "run <tool-id> [params...]",
	Short: "Execute a tool with parameters",
	Long: `Execute a tool from the knowledge base with parameters.
Parameters are passed as key=value pairs.

Examples:
  autoxmate run nmap target=scanme.nmap.org
  autoxmate run nmap target=scanme.nmap.org flag-i=10.0.0.0/8
  autoxmate run sqlmap target=http://example.com
  autoxmate run curl url=https://api.example.com/data
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		toolID := args[0]
		params := make(map[string]string)

		for _, arg := range args[1:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				params[parts[0]] = parts[1]
			}
		}

		interactive, _ := cmd.Flags().GetBool("interactive")
		raw, _ := cmd.Flags().GetBool("raw")

		cache, err := core.OpenCache()
		if err != nil {
			return err
		}
		defer cache.Close()

		tools, err := cache.LoadTools()
		if err != nil {
			return fmt.Errorf("load tools from cache: %w", err)
		}

		if len(tools) == 0 {
			return fmt.Errorf("no tools in cache; run 'autoxmate sync' first")
		}

		tool := core.FindToolByID(tools, toolID)
		if tool == nil {
			tool = core.FindToolByName(tools, toolID)
		}
		if tool == nil {
			return fmt.Errorf("tool not found: %s", toolID)
		}

		if interactive {
			params = interactivePrompt(tool)
		}

		opts := core.ExecOptions{
			Params: params,
			Quiet:  raw,
		}

		output, err := core.ExecuteTool(*tool, params, opts)
		if err != nil {
			return err
		}

		if raw {
			fmt.Print(output)
		} else {
			fmt.Printf("\nOutput:\n%s\n", output)
		}

		return nil
	},
}

func interactivePrompt(tool *core.ToolDefinition) map[string]string {
	params := make(map[string]string)
	fmt.Printf("\n  Enter parameters for %s:\n", tool.Name)

	for _, p := range tool.Parameters {
		if !p.Required && p.DefaultValue == nil {
			continue // Skip optional params without defaults
		}

		defaultStr := ""
		if p.DefaultValue != nil {
			defaultStr = fmt.Sprintf(" [%v]", p.DefaultValue)
		}
		req := ""
		if p.Required {
			req = " (required)"
		}

		fmt.Printf("  %s%s%s: ", p.Name, req, defaultStr)
		var val string
		fmt.Scanln(&val)

		if val == "" && p.DefaultValue != nil {
			val = fmt.Sprintf("%v", p.DefaultValue)
		}

		if val != "" {
			key := p.TemplateKey
			if key == "" {
				key = p.Name
			}
			params[key] = val
		}
	}

	return params
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolP("interactive", "i", false, "Interactive parameter prompt")
	runCmd.Flags().BoolP("raw", "r", false, "Raw output (no formatting)")
}
