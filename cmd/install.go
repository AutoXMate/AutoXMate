package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

var installCmd = &cobra.Command{
	Use:   "install [tool-id]",
	Short: "Install tools",
	Long: `Install one or more tools. If no tool ID is provided,
shows interactive prompt. Use --all to install all missing tools.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		method, _ := cmd.Flags().GetString("method")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

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

		opts := core.InstallOptions{
			Method: method,
			DryRun: dryRun,
		}

		if all {
			fmt.Println("Installing all missing tools...")
			installed, err := core.InstallAllTools(tools, opts)
			if err != nil {
				return err
			}
			fmt.Printf("\nInstalled %d tools\n", installed)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("specify a tool ID to install, or use --all")
		}

		toolID := args[0]
		tool := core.FindToolByID(tools, toolID)
		if tool == nil {
			tool = core.FindToolByName(tools, toolID)
		}
		if tool == nil {
			return fmt.Errorf("tool not found: %s", toolID)
		}

		return core.InstallTool(*tool, opts)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().BoolP("all", "a", false, "Install all missing tools")
	installCmd.Flags().StringP("method", "m", "", "Force specific install method (apt, brew, pip, go, git, etc.)")
	installCmd.Flags().Bool("dry-run", false, "Show what would be installed without installing")
}
