package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

var queryCmd = &cobra.Command{
	Use:   "query <query-string>",
	Short: "Query the tool database for exact commands",
	Long: `Query the tool database using a structured query language and get exact commands.

Structured query format:
  domain(<value>[/<value>]):action(<value>[/<value>]):tool(<value>):flags(<flags>):target(<target>):port(<port>)

Categories:
  domain     - Security domain: network, web, ad, security, container, system
  action     - Task/operation: port-scan, sql-injection, bruteforce, user-enum, etc.
  tool       - Tool name: nmap, sqlmap, hydra, netexec, etc.
  flags      - Tool flags: -sV, -sC, -p-, -A, --batch, etc.
  target     - Target IP, CIDR, URL, or domain
  port       - Port number or range
  phase      - Testing phase: recon, enumeration, exploitation, post-exploitation
  service    - Target service: SMB, LDAP, HTTP, MSSQL, Kerberos
  risk       - Risk level: low, medium, high, critical
  protocol   - Protocol: tcp, udp, smb, dns, ldap

You can also use / for OR within a category:
  domain(network/web):action(port-scan/sql-injection)

Free-text queries are also parsed:
  "scan ports 10.10.10.0/24 with nmap -sV"

Examples:
  autoxmate query "domain(network):action(port-scan):tool(nmap):flags(-sV -sC):target(10.10.10.0/24)"
  autoxmate query "domain(ad):action(user-enum):target(10.10.10.100)"
  autoxmate query "scan ports 10.10.10.0/24"
  autoxmate query --json "domain(network):action(port-scan)"
  autoxmate query --run "domain(network):action(port-scan):target(scanme.nmap.org)"
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		queryStr := args[0]
		format, _ := cmd.Flags().GetString("format")
		shouldRun, _ := cmd.Flags().GetBool("run")
		showExplain, _ := cmd.Flags().GetBool("explain")

		localPath, _ := cmd.Flags().GetString("local")

		var tools []core.ToolDefinition
		var queriesIndex *core.QueriesIndex

		if localPath != "" {
			var err error
			tools, err = core.ParseCommandsFile(localPath + "/commands.json")
			if err != nil {
				return fmt.Errorf("load local commands: %w", err)
			}
			if len(tools) == 0 {
				return fmt.Errorf("no tools found in %s", localPath)
			}
			queriesIndex, _ = core.LoadQueriesIndex(localPath + "/queries.json")
		} else {
			cache, err := core.OpenCache()
			if err != nil {
				return err
			}
			defer cache.Close()

			tools, err = cache.LoadTools()
			if err != nil {
				return fmt.Errorf("load tools from cache: %w", err)
			}
			if len(tools) == 0 {
				return fmt.Errorf("no tools in cache; run 'autoxmate sync' first")
			}

			// Try to load queries index from cache
			queriesIndex, _ = cache.LoadQueriesIndex()
		}

		// Fall back to local file if not found from cache
		if queriesIndex == nil {
			if path, err := core.FindQueriesIndexFile(); err == nil {
				queriesIndex, _ = core.LoadQueriesIndex(path)
			}
		}

		// Parse query
		var filter core.QueryFilter
		if isStructured(queryStr) {
			filter = core.ParseQuery(queryStr)
		} else {
			filter = core.ParseFreeText(queryStr, queriesIndex, tools)
		}

		// Execute query
		results := core.ExecuteQuery(filter, tools, queriesIndex)

		if format == "json" {
			if results == nil {
				fmt.Println("[]")
				return nil
			}
			data, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		if len(results) == 0 {
			fmt.Printf("\n  No matches found for query.\n")
			fmt.Printf("  Try: autoxmate search %s\n", queryStr)
			return nil
		}

		result := results[0]

		fmt.Println()
		fmt.Printf("  ┌─ %s (%s) ─", result.ToolName, result.Namespace)
		confPct := int(result.Confidence * 100)
		fmt.Printf("─ [%d%%] ─┐\n", confPct)
		fmt.Printf("  │ %s\n", result.Command)
		fmt.Println("  │")
		if showExplain || result.Explanation != "" {
			fmt.Printf("  │ %s\n", result.Explanation)
			fmt.Println("  │")
		}
		if result.Phase != "" {
			fmt.Printf("  │ Phase: %s", result.Phase)
			if result.RiskLevel != "" {
				fmt.Printf(" | Risk: %s", result.RiskLevel)
			}
			fmt.Println()
			fmt.Println("  │")
		}
		fmt.Printf("  │ %s\n", result.Description)
		fmt.Println("  └" + strings.Repeat("─", 50) + "┘")
		fmt.Println()

		if shouldRun {
			fmt.Printf("  Running: %s\n", result.Command)
			return core.RunShellCommand(result.Command, "")
		}

		return nil
	},
}

func isStructured(s string) bool {
	return strings.Contains(s, "(") && strings.Contains(s, ")")
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringP("format", "f", "table", "Output format (table, json)")
	queryCmd.Flags().BoolP("run", "r", false, "Execute the resulting command")
	queryCmd.Flags().BoolP("explain", "e", false, "Show match explanation")
	queryCmd.Flags().String("local", "", "Path to local api/v1 directory (for testing)")
}
