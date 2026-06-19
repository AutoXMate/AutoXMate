package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
	"github.com/AutoXMate/AutoXmate/site"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync tool definitions from AutoXMate site",
	Long: `Download the latest tool definitions from the AutoXMate site
and cache them locally for offline use.

The data comes from https://autoxmate.github.io/api/v1/commands.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		force, _ := cmd.Flags().GetBool("force")

		cache, err := core.OpenCache()
		if err != nil {
			return err
		}
		defer cache.Close()

		// Check if we need to sync
		if !force {
			count, err := cache.ToolCount()
			if err == nil && count > 0 {
				lastSync, err := cache.GetLastSync()
				if err == nil && !lastSync.IsZero() {
					fmt.Printf("Last sync: %s (%d tools cached)\n", lastSync.Format("2006-01-02 15:04:05"), count)
					fmt.Println("Use --force to sync again")
					return nil
				}
			}
		}

		client := site.NewClient(url)
		count, err := site.Sync(client, cache)
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		fmt.Printf("Successfully synced %d tools\n", count)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringP("url", "u", "", "Alternate API URL")
	syncCmd.Flags().BoolP("force", "f", false, "Force re-sync even if cached")
}
