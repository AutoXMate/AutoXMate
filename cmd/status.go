package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

var statusCmd = &cobra.Command{
	Use:   "status [tool-id]",
	Short: "Check tool installation status",
	Long:  `Check which tools are installed on the system.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")

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

		if len(args) == 1 {
			// Check single tool
			toolID := args[0]
			tool := core.FindToolByID(tools, toolID)
			if tool == nil {
				tool = core.FindToolByName(tools, toolID)
			}
			if tool == nil {
				return fmt.Errorf("tool not found: %s", toolID)
			}

			status := core.DetectTool(*tool)
			printToolStatus(*tool, status)
			return nil
		}

		// Check all tools (or all if --all)
		statuses := core.DetectAllTools(tools)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "  ID\tNAME\tSTATUS\tVERSION\tLOCATION")

		count := 0
		for _, t := range tools {
			s := statuses[t.ID]
			if !all && !s.OnPath && !s.DockerImage && s.PackageManager == "" {
				continue
			}
			count++
			statusStr := "✓"
			loc := s.PathLocation
			if s.PathLocation == "" && s.DockerImage {
				statusStr = "✓ docker"
				loc = "docker"
			} else if s.PathLocation == "" && s.PackageManager != "" {
				statusStr = "✓ " + s.PackageManager
				loc = s.PackageManager
			} else if s.PathLocation == "" {
				statusStr = "✗"
				loc = "not installed"
			}

			ver := s.Version
			if len(ver) > 30 {
				ver = ver[:27] + "..."
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", t.ID, t.Name, statusStr, ver, loc)
		}
		w.Flush()

		if !all {
			fmt.Printf("\n  %d installed tools (run with --all to see all %d tools)\n", count, len(tools))
		}

		return nil
	},
}

func printToolStatus(tool core.ToolDefinition, status core.InstallStatus) {
	fmt.Printf("\n  Tool: %s (%s)\n", tool.Name, tool.ID)
	fmt.Printf("  Description: %s\n", tool.Description)
	fmt.Printf("  Namespace: %s\n", tool.Namespace)
	fmt.Printf("  Risk Level: %s\n", tool.RiskLevel)
	fmt.Printf("  Trust Level: %s\n", tool.TrustLevel)
	fmt.Println()
	fmt.Printf("  Installed: ")
	if status.OnPath {
		fmt.Printf("✓ (on PATH at %s)\n", status.PathLocation)
	} else if status.DockerImage {
		fmt.Printf("✓ (Docker image)\n")
	} else if status.PackageManager != "" {
		fmt.Printf("✓ (%s)\n", status.PackageManager)
	} else {
		fmt.Printf("✗ (not installed)\n")
	}

	if status.Version != "" {
		fmt.Printf("  Version: %s\n", status.Version)
	}

	if len(tool.Install) > 0 {
		fmt.Println()
		fmt.Println("  Install methods:")
		data, _ := json.MarshalIndent(tool.Install, "    ", "  ")
		fmt.Println("  " + string(data))
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolP("all", "a", false, "Show all tools, including not installed")
}
