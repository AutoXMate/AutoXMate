package core

import (
	"testing"
)

func TestMatchFlagToParam_ByFlag(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "target", TemplateKey: "target", Type: "string"},
		},
	}

	key, ok := matchFlagToParam(&tool, "-sV")
	if !ok {
		t.Fatal("expected match for '-sV'")
	}
	if key != "flag-s" {
		t.Errorf("got key %q, want %q", key, "flag-s")
	}
}

func TestMatchFlagToParam_ByAlias(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Parameters: []Parameter{
			{Name: "flag-A", TemplateKey: "flag-A", Type: "boolean", Aliases: []string{"-A"}},
		},
	}

	key, ok := matchFlagToParam(&tool, "-A")
	if !ok {
		t.Fatal("expected match for '-A'")
	}
	if key != "flag-A" {
		t.Errorf("got key %q, want %q", key, "flag-A")
	}
}

func TestMatchFlagToParam_NoMatch(t *testing.T) {
	tool := ToolDefinition{
		Name: "nmap",
		Parameters: []Parameter{
			{Name: "flag-s", Flag: "-sV"},
		},
	}

	_, ok := matchFlagToParam(&tool, "--unknown")
	if ok {
		t.Fatal("expected no match for '--unknown'")
	}
}

func TestMatchFlagToParam_NameFallback(t *testing.T) {
	tool := ToolDefinition{
		Name: "test",
		Parameters: []Parameter{
			{Name: "output", Flag: "-o"},
		},
	}

	key, ok := matchFlagToParam(&tool, "-o")
	if !ok {
		t.Fatal("expected match for '-o'")
	}
	if key != "output" {
		t.Errorf("got key %q, want %q", key, "output")
	}
}

func TestSmartRenderWithParams_UsesBuildCommand(t *testing.T) {
	toolDef := &ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "target", TemplateKey: "target", Type: "string", PositionalOrder: 1, Flag: ""},
		},
	}

	filter := QueryFilter{
		Params: map[string]string{
			"flag-s": "true",
			"target": "10.0.0.1",
		},
	}

	cmd := smartRenderWithDefaults("nmap", "nmap", toolDef, filter, "", nil)
	if cmd != "nmap -sV 10.0.0.1" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV 10.0.0.1")
	}
}

func TestSmartRenderWithParams_EmptyParamsFallsBack(t *testing.T) {
	// When Params is empty/nil, should fall back to heuristic rendering
	toolDef := &ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {flag-s} {target}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
			{Name: "target", TemplateKey: "target", Type: "string", Aliases: []string{"scanme.nmap.org"}},
		},
	}

	filter := QueryFilter{
		Target: "10.0.0.1",
		Flags:  []string{"-sV"},
	}

	cmd := smartRenderWithDefaults("nmap", "nmap", toolDef, filter, "", nil)
	// Heuristic: just appends tool name + flags + target
	if cmd != "nmap -sV 10.0.0.1" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV 10.0.0.1")
	}
}

func TestSmartRenderWithParams_NilToolDefFallsBack(t *testing.T) {
	// When toolDef is nil even with Params, fall back to heuristic
	filter := QueryFilter{
		Target: "10.0.0.1",
		Flags:  []string{"-sV"},
		Params: map[string]string{"flag-s": "true", "target": "10.0.0.1"},
	}

	cmd := smartRenderWithDefaults("nmap", "nmap", nil, filter, "", nil)
	if cmd != "nmap -sV 10.0.0.1" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV 10.0.0.1")
	}
}

func TestSmartRenderWithParams_BuildCommandErrorFallsBack(t *testing.T) {
	// Params present but template has unknown keys → BuildCommand errors → fall back
	toolDef := &ToolDefinition{
		Name: "nmap",
		Execution: Execution{
			Template: "nmap {unknown}",
		},
		Parameters: []Parameter{
			{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
		},
	}

	filter := QueryFilter{
		Flags:  []string{"-sV"},
		Target: "10.0.0.1",
		Params: map[string]string{"flag-s": "true", "target": "10.0.0.1"},
	}

	cmd := smartRenderWithDefaults("nmap", "nmap", toolDef, filter, "", nil)
	// Should fall back to heuristic even though params exist
	if cmd == "" {
		t.Fatal("expected fallback to heuristic, got empty string")
	}
	// Verify it's heuristic output (tool name + flags + target)
	if cmd != "nmap -sV 10.0.0.1" {
		t.Errorf("got %q, want %q", cmd, "nmap -sV 10.0.0.1")
	}
}

func TestParseFreeText_PopulatesParams(t *testing.T) {
	tools := []ToolDefinition{
		{
			ID:   "nmap",
			Name: "nmap",
			Execution: Execution{
				Template: "nmap {flag-s} {target}",
			},
			Parameters: []Parameter{
				{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
				{Name: "target", TemplateKey: "target", Type: "string", PositionalOrder: 1, Flag: ""},
			},
		},
	}

	filter := ParseFreeText("scan network with nmap -sV 10.0.0.1", nil, tools)

	if len(filter.Params) == 0 {
		t.Fatal("expected Params to be populated")
	}

	// Check boolean flag matched
	if filter.Params["flag-s"] != "true" {
		t.Errorf("flag-s: got %q, want %q", filter.Params["flag-s"], "true")
	}

	// Check target matched
	if filter.Params["target"] != "10.0.0.1" {
		t.Errorf("target: got %q, want %q", filter.Params["target"], "10.0.0.1")
	}
}

func TestParseFreeText_ParamsWithPort(t *testing.T) {
	tools := []ToolDefinition{
		{
			ID:   "nmap",
			Name: "nmap",
			Execution: Execution{
				Template: "nmap {flag-s} -p {port} {target}",
			},
			Parameters: []Parameter{
				{Name: "flag-s", TemplateKey: "flag-s", Type: "boolean", Flag: "-sV"},
				{Name: "port", TemplateKey: "port", Type: "string", Flag: "-p", Format: "space"},
				{Name: "target", TemplateKey: "target", Type: "string"},
			},
		},
	}

	filter := ParseFreeText("nmap -sV 10.0.0.1 port 443", nil, tools)

	if len(filter.Params) == 0 {
		t.Fatal("expected Params to be populated")
	}

	if filter.Params["port"] != "443" {
		t.Errorf("port: got %q, want %q", filter.Params["port"], "443")
	}
	if filter.Params["target"] != "10.0.0.1" {
		t.Errorf("target: got %q, want %q", filter.Params["target"], "10.0.0.1")
	}
	if filter.Params["flag-s"] != "true" {
		t.Errorf("flag-s: got %q, want %q", filter.Params["flag-s"], "true")
	}
}

func TestParseFreeText_ParamsNoToolMatch(t *testing.T) {
	// No tool matched → Params should be empty
	tools := []ToolDefinition{
		{
			ID:   "nmap",
			Name: "nmap",
			Parameters: []Parameter{
				{Name: "flag-s", Flag: "-sV"},
			},
		},
	}

	filter := ParseFreeText("no tool here", nil, tools)

	if len(filter.Params) != 0 {
		t.Errorf("expected empty Params, got %v", filter.Params)
	}
}

// --- Structured query tests ---

func TestParseQuery_Basic(t *testing.T) {
	filter := ParseQuery("domain(network):action(port-scan):tool(nmap):flags(-sV -sC):target(10.10.10.0/24)")

	if len(filter.Domains) != 1 || filter.Domains[0] != "network" {
		t.Errorf("domains: got %v, want [network]", filter.Domains)
	}
	if len(filter.Actions) != 1 || filter.Actions[0] != "port-scan" {
		t.Errorf("actions: got %v, want [port-scan]", filter.Actions)
	}
	if len(filter.Tools) != 1 || filter.Tools[0] != "nmap" {
		t.Errorf("tools: got %v, want [nmap]", filter.Tools)
	}
	if len(filter.Flags) != 1 {
		t.Errorf("flags count: got %v, want 1 (space-separated stays as single value)", len(filter.Flags))
	}
	if filter.Target != "10.10.10.0/24" {
		t.Errorf("target: got %q, want %q", filter.Target, "10.10.10.0/24")
	}
}

func TestParseQuery_ORValues(t *testing.T) {
	filter := ParseQuery("domain(network/web):action(port-scan/sql-injection)")

	if len(filter.Domains) != 2 {
		t.Errorf("domains: got %v, want 2 values", filter.Domains)
	}
	if len(filter.Actions) != 2 {
		t.Errorf("actions: got %v, want 2 values", filter.Actions)
	}
}

func TestParseQuery_PortTarget(t *testing.T) {
	filter := ParseQuery("domain(network):port(443):target(10.10.10.1)")

	if filter.Port != "443" {
		t.Errorf("port: got %q, want %q", filter.Port, "443")
	}
	if filter.Target != "10.10.10.1" {
		t.Errorf("target: got %q, want %q", filter.Target, "10.10.10.1")
	}
}

func TestParseQuery_EmptyInput(t *testing.T) {
	filter := ParseQuery("")
	if filter.Source != "structured" {
		t.Errorf("expected source 'structured' (default for ParseQuery), got %q", filter.Source)
	}
}

func TestParseQuery_PhaseAndRisk(t *testing.T) {
	filter := ParseQuery("domain(ad):action(enumeration):phase(reconnaissance):risk(low)")

	if filter.Phase != "reconnaissance" {
		t.Errorf("phase: got %q, want %q", filter.Phase, "reconnaissance")
	}
	if filter.Risk != "low" {
		t.Errorf("risk: got %q, want %q", filter.Risk, "low")
	}
}
