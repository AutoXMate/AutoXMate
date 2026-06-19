package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tools",
	Long:  `List all tools from the AutoXMate knowledge base with optional filters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		domain, _ := cmd.Flags().GetString("domain")
		capability, _ := cmd.Flags().GetString("capability")
		format, _ := cmd.Flags().GetString("format")
		installed, _ := cmd.Flags().GetBool("installed")

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

		// Apply filters
		if domain != "" {
			tools = core.FilterByDomain(tools, domain)
		}
		if capability != "" {
			tools = core.FilterByCapability(tools, capability)
		}

		// Filter by installed status
		if installed {
			statuses := core.DetectAllTools(tools)
			var filtered []core.ToolDefinition
			for _, t := range tools {
				if s, ok := statuses[t.ID]; ok && (s.OnPath || s.DockerImage || s.PackageManager != "") {
					filtered = append(filtered, t)
				}
			}
			tools = filtered
		}

		if format == "json" {
			data, _ := json.MarshalIndent(tools, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("\n  %d tools\n\n", len(tools))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "  ID\tNAME\tDOMAIN\tDESCRIPTION")

		for _, t := range tools {
			domain := t.Namespace
			for i := 0; i < len(domain); i++ {
				if domain[i] == ':' {
					domain = domain[:i]
					break
				}
			}
			desc := t.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", t.ID, t.Name, domain, desc)
		}
		w.Flush()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringP("domain", "d", "", "Filter by domain")
	listCmd.Flags().StringP("capability", "c", "", "Filter by capability")
	listCmd.Flags().StringP("format", "f", "table", "Output format (table, json)")
	listCmd.Flags().BoolP("installed", "i", false, "Show only installed tools")
}
