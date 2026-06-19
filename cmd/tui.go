package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
	"github.com/AutoXMate/AutoXmate/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	Long:  `Opens a full-screen interactive TUI for browsing, searching, installing, and running tools.`,
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
			return fmt.Errorf("no tools cached; run 'autoxmate sync' first")
		}

		app := tui.NewApp(tools, cache)
		return app.Run()
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
