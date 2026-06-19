package core

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func logTo(opts InstallOptions, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if opts.Output != nil {
		fmt.Fprintln(opts.Output, msg)
	} else {
		fmt.Println(msg)
	}
}

// InstallTool installs a tool using its defined install methods
func InstallTool(tool ToolDefinition, opts InstallOptions) error {
	if len(tool.Install) == 0 {
		return fmt.Errorf("no install methods defined for %s", tool.Name)
	}

	logTo(opts, "Installing %s...", tool.Name)

	for _, inst := range tool.Install {
		if opts.Method != "" && inst.Method != opts.Method {
			continue
		}

		logTo(opts, "  Method: %s", inst.Method)

		if err := runInstallMethod(inst, opts); err != nil {
			logTo(opts, "  ! Failed: %v", err)
			if opts.Method != "" {
				return err
			}
			continue
		}

		logTo(opts, "  ✓ %s installed successfully", tool.Name)
		return nil
	}

	return fmt.Errorf("all install methods failed for %s", tool.Name)
}

// InstallAllTools installs all tools that are not already installed
func InstallAllTools(tools []ToolDefinition, opts InstallOptions) (int, error) {
	var installed int
	for _, t := range tools {
		status := DetectTool(t)
		if status.OnPath || status.DockerImage || status.PackageManager != "" {
			logTo(opts, "  • %s already installed", t.Name)
			continue
		}
		if err := InstallTool(t, opts); err != nil {
			logTo(opts, "  ✗ %s: %v", t.Name, err)
			continue
		}
		installed++
	}
	return installed, nil
}

type InstallOptions struct {
	Method string // If set, only use this install method
	DryRun bool
	Yes    bool // Auto-confirm
	Output io.Writer // If set, redirect command output here instead of stdout
}

func runInstallMethod(inst Install, opts InstallOptions) error {
	for _, cmdStr := range inst.Commands {
		if cmdStr == "" {
			continue
		}

		logTo(opts, "    → %s", cmdStr)

		var cmd *exec.Cmd
		if strings.Contains(cmdStr, "&&") || strings.Contains(cmdStr, "|") || strings.Contains(cmdStr, ";") {
			cmd = exec.Command("sh", "-c", cmdStr)
		} else {
			parts := strings.Fields(cmdStr)
			if len(parts) == 0 {
				continue
			}
			cmd = exec.Command(parts[0], parts[1:]...)
		}

		if opts.Output != nil {
			cmd.Stdout = opts.Output
			cmd.Stderr = opts.Output
		} else {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
	}
	return nil
}
