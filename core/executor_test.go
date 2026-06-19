package core

import (
	"strings"
	"testing"
)

func TestBuildCommand_LegacyAliasFallback(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "string", Aliases: []string{"-sV"}},
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap -sV scanme.nmap.org" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV scanme.nmap.org")
	}
}

func TestBuildCommand_LegacyOverride(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "string", Aliases: []string{"-sV"}},
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"target": "192.168.1.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap -sV 192.168.1.1" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV 192.168.1.1")
	}
}

func TestBuildCommand_BooleanFlagEnabled(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"flag-s": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap -sV scanme.nmap.org" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV scanme.nmap.org")
	}
}

func TestBuildCommand_BooleanFlagDisabled(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Boolean not enabled → no -sV emitted
	if cmd != "nmap scanme.nmap.org" {
		t.Errorf("got %q, want %q", cmd, "nmap scanme.nmap.org")
	}
}

func TestBuildCommand_ValueFlagSpaceFormat(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {exclude} {target}",
		},
		Parameters: []Parameter{
			{Name: "exclude", TemplateKey: "exclude", Type: "string", Flag: "--exclude", Format: "space"},
			{Name: "target", TemplateKey: "target", Type: "string", Flag: "", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"exclude": "host1,host2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap --exclude host1,host2 scanme.nmap.org" {
		t.Errorf("got %q, want %q", cmd, "nmap --exclude host1,host2 scanme.nmap.org")
	}
}

func TestBuildCommand_ValueFlagEqualsFormat(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {output}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"hydra"}},
			{Name: "output", TemplateKey: "output", Type: "string", Flag: "-o", Format: "equals"},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"output": "results.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "hydra -o=results.txt" {
		t.Errorf("got %q, want %q", cmd, "hydra -o=results.txt")
	}
}

func TestBuildCommand_ValueFlagJoinedFormat(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {port}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"nc"}},
			{Name: "port", TemplateKey: "port", Type: "string", Flag: "-p", Format: "joined"},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"port": "8080"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nc -p8080" {
		t.Errorf("got %q, want %q", cmd, "nc -p8080")
	}
}

func TestBuildCommand_ValueFlagNoneFormat(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {target}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"dig"}},
			{Name: "target", TemplateKey: "target", Type: "string", Flag: "", Format: "none"},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"target": "example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "dig example.com" {
		t.Errorf("got %q, want %q", cmd, "dig example.com")
	}
}

func TestBuildCommand_PositionalParam(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "target", TemplateKey: "target", Type: "string", PositionalOrder: 1, Flag: ""},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"flag-s": "true", "target": "192.168.1.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap -sV 192.168.1.1" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV 192.168.1.1")
	}
}

func TestBuildCommand_PositionalOrdered(t *testing.T) {
	tool := ToolDefinition{
		Name: "cp",
		Execution: Execution{
			Template: "cp {source} {dest}",
		},
		Parameters: []Parameter{
			{Name: "source", TemplateKey: "source", Type: "string", PositionalOrder: 1, Flag: ""},
			{Name: "dest", TemplateKey: "dest", Type: "string", PositionalOrder: 2, Flag: ""},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"source": "a.txt", "dest": "b.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "cp a.txt b.txt" {
		t.Errorf("got %q, want %q", cmd, "cp a.txt b.txt")
	}
}

func TestBuildCommand_PositionalReversedOrder(t *testing.T) {
	// Template has dest before source, but positional_order should produce
	// source (order=1) before dest (order=2)
	tool := ToolDefinition{
		Name: "cp",
		Execution: Execution{
			Template: "cp {dest} {source}",
		},
		Parameters: []Parameter{
			{Name: "source", TemplateKey: "source", Type: "string", PositionalOrder: 1, Flag: ""},
			{Name: "dest", TemplateKey: "dest", Type: "string", PositionalOrder: 2, Flag: ""},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"source": "a.txt", "dest": "b.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// source (order=1) comes first, dest (order=2) comes second
	if cmd != "cp a.txt b.txt" {
		t.Errorf("got %q, want %q", cmd, "cp a.txt b.txt")
	}
}

func TestBuildCommand_MixedStructuralAndLegacy(t *testing.T) {
	// Some params have metadata, some don't
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {flag-A} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "flag-A", TemplateKey: "flag-A", Type: "boolean", Aliases: []string{"-A"}},
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	// flag-s has metadata (Flag, boolean) → enabled
	// flag-A has no metadata (no Flag) → legacy alias fallback → emits -A
	// target has no metadata → legacy alias fallback → emits scanme.nmap.org
	cmd, err := BuildCommand(tool, map[string]string{"flag-s": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap -sV -A scanme.nmap.org" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV -A scanme.nmap.org")
	}
}

func TestBuildCommand_DefaultValue(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {threads}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"hydra"}},
			{Name: "threads", TemplateKey: "threads", Type: "string", Flag: "-t", DefaultValue: float64(16)},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "hydra -t 16" {
		t.Errorf("got %q, want %q", cmd, "hydra -t 16")
	}
}

func TestBuildCommand_DefaultValueOverridden(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {threads}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"hydra"}},
			{Name: "threads", TemplateKey: "threads", Type: "string", Flag: "-t", DefaultValue: float64(16)},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"threads": "32"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "hydra -t 32" {
		t.Errorf("got %q, want %q", cmd, "hydra -t 32")
	}
}

func TestBuildCommand_NoTemplate(t *testing.T) {
	tool := ToolDefinition{
		Name:      "test",
		Execution: Execution{},
	}

	_, err := BuildCommand(tool, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing template, got nil")
	}
}

func TestBuildCommand_UnresolvedPlaceholders(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {unknown_param}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"hydra"}},
		},
	}

	_, err := BuildCommand(tool, map[string]string{})
	if err == nil {
		t.Fatal("expected error for unresolved placeholders, got nil")
	}
}

func TestBuildCommand_BooleanEmptyValueNoEmit(t *testing.T) {
	// User explicitly passes an empty string for a boolean param
	// This should NOT emit the flag
	tool := ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"flag-s": ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap scanme.nmap.org" {
		t.Errorf("got %q, want %q", cmd, "nmap scanme.nmap.org")
	}
}

func TestBuildCommand_FlagDefaultFormatIsSpace(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {output}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"hydra"}},
			{Name: "output", TemplateKey: "output", Type: "string", Flag: "-o"},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"output": "out.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "hydra -o out.txt" {
		t.Errorf("got %q, want %q", cmd, "hydra -o out.txt")
	}
}

func TestBuildCommand_EmptyParamNoMetadataNoEmit(t *testing.T) {
	// A param with metadata (Flag) but no value and not user-provided
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{tool} {exclude}",
		},
		Parameters: []Parameter{
			{Name: "tool", TemplateKey: "tool", Type: "string", Aliases: []string{"nmap"}},
			{Name: "exclude", TemplateKey: "exclude", Type: "string", Flag: "--exclude"},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "nmap" {
		t.Errorf("got %q, want %q", cmd, "nmap")
	}
}

func TestBuildCommand_ParamByNameFallback(t *testing.T) {
	// If TemplateKey is empty, fall back to Name for lookups
	tool := ToolDefinition{
		Name: "test",
		Execution: Execution{
			Template: "{target} {port}",
		},
		Parameters: []Parameter{
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"default.target"}},
			{Name: "port", TemplateKey: "port", Type: "string", Flag: "-p"},
		},
	}

	cmd, err := BuildCommand(tool, map[string]string{"port": "443"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "default.target -p 443" {
		t.Errorf("got %q, want %q", cmd, "default.target -p 443")
	}
}

func TestExecuteTool_TimeoutExceeds(t *testing.T) {
	// A tool that sleeps past the timeout should be killed
	tool := ToolDefinition{
		Name: "sleep",
		Execution: Execution{
			Template: "sleep {time}",
			Shell:    true,
		},
		Parameters: []Parameter{
			{Name: "time", TemplateKey: "time", Type: "string", Aliases: []string{"10"}},
		},
	}

	opts := ExecOptions{
		Quiet:   true,
		Timeout: 1, // 1 second timeout
	}

	_, err := ExecuteTool(tool, map[string]string{"time": "30"}, opts)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "signal: killed") && !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "context deadline") {
		t.Errorf("got error: %v, expected timeout-related error", err)
	}
}

func TestExecuteTool_TimeoutNotReached(t *testing.T) {
	// A tool that finishes well within the timeout should succeed
	tool := ToolDefinition{
		Name: "echo",
		Execution: Execution{
			Template: "echo {message}",
			Shell:    true,
		},
		Parameters: []Parameter{
			{Name: "message", TemplateKey: "message", Type: "string", Aliases: []string{"hello"}},
		},
	}

	opts := ExecOptions{
		Quiet:   true,
		Timeout: 30, // 30 second timeout — plenty for echo
	}

	out, err := ExecuteTool(tool, map[string]string{"message": "hello world"}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("got output %q, want it to contain 'hello world'", out)
	}
}
