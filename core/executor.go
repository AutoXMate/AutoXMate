package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type ExecOptions struct {
	Params  map[string]string
	Quiet   bool
	Timeout int // seconds
}

// renderFlag renders a flag with its value in the specified format
func renderFlag(flag, value, format string) string {
	switch format {
	case "equals":
		return flag + "=" + value
	case "joined":
		return flag + value
	case "none":
		return value
	default: // "space"
		return flag + " " + value
	}
}

// BuildCommand renders the execution template with the given parameters.
// When parameter metadata (Flag, Format) is available, it uses format-aware
// rendering. Falls back to legacy alias-based rendering when metadata is absent.
func BuildCommand(tool ToolDefinition, params map[string]string) (string, error) {
	tmpl := tool.Execution.Template
	if tmpl == "" {
		return "", fmt.Errorf("no execution template defined for %s", tool.Name)
	}

	// Build parameter lookup by template_key or name
	paramLookup := make(map[string]*Parameter)
	for i, p := range tool.Parameters {
		key := p.TemplateKey
		if key == "" {
			key = p.Name
		}
		paramLookup[key] = &tool.Parameters[i]
	}

	// Build value map from defaults + alias convention
	values := make(map[string]string)
	for _, p := range tool.Parameters {
		key := p.TemplateKey
		if key == "" {
			key = p.Name
		}
		if p.DefaultValue != nil {
			values[key] = fmt.Sprintf("%v", p.DefaultValue)
		}
		// Legacy alias convention: pre-fill empty for params with aliases
		if len(p.Aliases) > 0 && p.TemplateKey != "" {
			if _, exists := values[key]; !exists {
				values[key] = ""
			}
		}
	}
	// Override with user-provided params
	for k, v := range params {
		values[k] = v
	}

	// Track positional args collected during template processing
	type positionalArg struct {
		order int
		value string
	}
	var positionals []positionalArg

	var result strings.Builder
	i := 0
	for i < len(tmpl) {
		if tmpl[i] == '{' {
			j := i + 1
			for j < len(tmpl) && tmpl[j] != '}' {
				j++
			}
			if j < len(tmpl) {
				key := tmpl[i+1 : j]
				val, inValues := values[key]
				param, hasParam := paramLookup[key]

				if hasParam && param.Flag != "" {
					// Structural rendering with metadata
					_, userProvided := params[key]

					if param.PositionalOrder > 0 && val != "" {
						positionals = append(positionals, positionalArg{param.PositionalOrder, val})
					} else if param.Type == "boolean" {
						if val == "true" || (userProvided && val != "") {
							result.WriteString(param.Flag)
						}
					} else if val != "" {
						format := param.Format
						if format == "" {
							format = "space"
						}
						result.WriteString(renderFlag(param.Flag, val, format))
					}
					// Empty value with metadata: write nothing
				} else {
					// Legacy rendering (no metadata) or positional-only param
					if hasParam && param.PositionalOrder > 0 && val != "" {
						positionals = append(positionals, positionalArg{param.PositionalOrder, val})
					} else if inValues && val == "" {
						for _, p := range tool.Parameters {
							if (p.TemplateKey == key || p.Name == key) && len(p.Aliases) > 0 {
								result.WriteString(p.Aliases[0])
								break
							}
						}
					} else if inValues {
						result.WriteString(val)
					} else {
						result.WriteString(tmpl[i : j+1])
					}
				}
				i = j + 1
			} else {
				result.WriteByte(tmpl[i])
				i++
			}
		} else {
			result.WriteByte(tmpl[i])
			i++
		}
	}

	cmd := result.String()

	// Append positional args at end, sorted by order
	if len(positionals) > 0 {
		sort.Slice(positionals, func(a, b int) bool {
			return positionals[a].order < positionals[b].order
		})
		for _, p := range positionals {
			cmd += " " + p.value
		}
	}

	cmd = strings.TrimSpace(cmd)

	// Collapse multiple spaces from omitted template tokens
	for strings.Contains(cmd, "  ") {
		cmd = strings.ReplaceAll(cmd, "  ", " ")
	}

	// Validate no unresolved placeholders remain
	if strings.Contains(cmd, "{") && strings.Contains(cmd, "}") {
		return "", fmt.Errorf("unresolved parameters in template: %s", cmd)
	}

	return cmd, nil
}

// ExecuteTool runs a tool with the given parameters
func ExecuteTool(tool ToolDefinition, params map[string]string, opts ExecOptions) (string, error) {
	// Build command
	cmdStr, err := BuildCommand(tool, params)
	if err != nil {
		return "", fmt.Errorf("build command: %w", err)
	}

	if !opts.Quiet {
		fmt.Printf("Running: %s\n", cmdStr)
	}

	// Create command
	var cmd *exec.Cmd
	if tool.Execution.Shell {
		cmd = exec.CommandContext(context.Background(), "sh", "-c", cmdStr)
	} else {
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			return "", fmt.Errorf("empty command")
		}
		cmd = exec.CommandContext(context.Background(), parts[0], parts[1:]...)
	}

	// Set working directory
	if tool.Execution.Workdir != "" {
		cmd.Dir = tool.Execution.Workdir
	}

	// Set environment variables
	if len(tool.Execution.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range tool.Execution.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Apply timeout
	if opts.Timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.Timeout)*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)
		cmd.Dir = tool.Execution.Workdir
		if len(tool.Execution.Env) > 0 {
			cmd.Env = os.Environ()
			for k, v := range tool.Execution.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}
	}

	// Run and capture output
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		return output, fmt.Errorf("execution failed: %w\nOutput: %s", err, output)
	}

	return output, nil
}

// RunTerminal opens an interactive shell with AutoMate context
func RunTerminal(workdir string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if workdir != "" {
		cmd.Dir = workdir
	}

	fmt.Printf("AutoXMate terminal session. Type 'exit' to return.\n")
	return cmd.Run()
}

// RunShellCommand runs a single shell command
func RunShellCommand(cmdStr, workdir string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if workdir != "" {
		cmd.Dir = workdir
	}
	return cmd.Run()
}
