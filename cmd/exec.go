package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AutoXMate/AutoXmate/core"
)

var execCmd = &cobra.Command{
	Use:   "exec [commands...]",
	Short: "Run manual terminal commands",
	Long: `Run arbitrary shell commands with AutoMate context.
If no command is provided, opens an interactive shell.

Examples:
  autoxmate exec nmap -sV scanme.nmap.org
  autoxmate exec python3 -c "print('hello')"
  autoxmate exec              (opens interactive shell)
`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		workdir, _ := cmd.Flags().GetString("workdir")

		if len(args) == 0 {
			// No arguments - open interactive shell
			return core.RunTerminal(workdir)
		}

		// Build command string from args
		cmdStr := args[0]
		for _, arg := range args[1:] {
			cmdStr += " " + arg
		}

		fmt.Printf("AutoXMate exec: %s\n", cmdStr)

		// Use the executor to run the command
		return core.RunShellCommand(cmdStr, workdir)
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringP("workdir", "w", "", "Working directory")
}
