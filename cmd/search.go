package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search tools by name, capability, or domain",
	Long: `Search the tool database using keywords.
Searches across tool names, descriptions, capabilities, and namespaces.

Examples:
  autoxmate search nmap
  autoxmate search vulnerability
  autoxmate search --capability network.scan
  autoxmate search --domain security
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) > 0 {
			query = args[0]
		}
		capability, _ := cmd.Flags().GetString("capability")
		domain, _ := cmd.Flags().GetString("domain")
		format, _ := cmd.Flags().GetString("format")

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

		var results []core.ToolDefinition

		if capability != "" {
			results = core.FilterByCapability(tools, capability)
		} else if domain != "" {
			results = core.FilterByDomain(tools, domain)
		} else {
			results = core.SearchTools(tools, query)
		}

		if format == "json" {
			data, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		displayQuery := query
		if displayQuery == "" && capability != "" {
			displayQuery = "capability:" + capability
		} else if displayQuery == "" && domain != "" {
			displayQuery = "domain:" + domain
		}
		fmt.Printf("\n  %d results for \"%s\"\n\n", len(results), displayQuery)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "  ID\tNAME\tDESCRIPTION")

		for _, t := range results {
			desc := t.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\n", t.ID, t.Name, desc)
		}
		w.Flush()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringP("capability", "c", "", "Search by capability")
	searchCmd.Flags().StringP("domain", "d", "", "Search by domain")
	searchCmd.Flags().StringP("format", "f", "table", "Output format (table, json)")
}
