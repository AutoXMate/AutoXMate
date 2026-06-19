package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "autoxmate",
	Short: "AutoXMate - Security tool automation CLI",
	Long: `AutoXMate is a CLI tool that syncs with the AutoXMate knowledge site,
detects installed tools, installs missing ones, and executes
tools with parameterized commands.

  autoxmate list        List all available tools
  autoxmate status      Check which tools are installed
  autoxmate sync        Sync tool definitions from AutoXMate site
  autoxmate install     Install tools
  autoxmate run         Execute a tool with parameters
  autoxmate search      Search tools by name/capability/domain
  autoxmate query       Structured/free-text query for exact commands
  autoxmate exec        Open an interactive terminal session
  autoxmate mcp         Start MCP server for AI integration
  autoxmate tui         Launch the interactive TUI
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
}
