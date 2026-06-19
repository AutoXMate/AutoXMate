package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseCommandsFile reads a commands.json file and returns ToolDefinitions
func ParseCommandsFile(path string) ([]ToolDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read commands file: %w", err)
	}

	var tools []ToolDefinition
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("parse commands file: %w", err)
	}

	return tools, nil
}

// DefaultCommandsPaths returns likely locations for commands.json
func DefaultCommandsPaths() []string {
	configDir, _ := os.UserConfigDir()
	homeDir, _ := os.UserHomeDir()

	return []string{
		filepath.Join(configDir, "autoxmate", "commands.json"),
		filepath.Join(homeDir, ".autoxmate", "commands.json"),
		"commands.json",
		"api/v1/commands.json",
		"public/api/v1/commands.json",
	}
}

// FindCommandsFile searches for commands.json in common locations
func FindCommandsFile() (string, error) {
	for _, p := range DefaultCommandsPaths() {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("commands.json not found in any standard location; run 'autoxmate sync'")
}

// FindToolByID finds a tool by its ID
func FindToolByID(tools []ToolDefinition, id string) *ToolDefinition {
	for i, t := range tools {
		if t.ID == id {
			return &tools[i]
		}
	}
	return nil
}

// FindToolByName finds a tool by its name
func FindToolByName(tools []ToolDefinition, name string) *ToolDefinition {
	for i, t := range tools {
		if t.Name == name {
			return &tools[i]
		}
	}
	return nil
}

// SearchTools searches tools by name, description, capability, or namespace
func SearchTools(tools []ToolDefinition, query string) []ToolDefinition {
	var results []ToolDefinition
	for _, t := range tools {
		if contains(t.Name, query) || contains(t.Description, query) ||
			contains(t.Namespace, query) || contains(t.ID, query) {
			results = append(results, t)
		}
	}
	return results
}

// FilterByDomain filters tools by domain (first part of namespace)
func FilterByDomain(tools []ToolDefinition, domain string) []ToolDefinition {
	var results []ToolDefinition
	for _, t := range tools {
		if t.Namespace != "" {
			d := t.Namespace
			for i := 0; i < len(d); i++ {
				if d[i] == ':' {
					d = d[:i]
					break
				}
			}
			if d == domain {
				results = append(results, t)
			}
		}
	}
	return results
}

// FilterByAction filters tools by action name (matched against capabilities)
func FilterByAction(tools []ToolDefinition, action string) []ToolDefinition {
	var results []ToolDefinition
	aLower := strings.ToLower(action)
	for _, t := range tools {
		for _, cap := range t.Capabilities {
			if contains(cap, aLower) {
				results = append(results, t)
				break
			}
		}
	}
	return results
}

// FilterByCapability filters tools by capability
func FilterByCapability(tools []ToolDefinition, capability string) []ToolDefinition {
	var results []ToolDefinition
	for _, t := range tools {
		for _, c := range t.Capabilities {
			if contains(c, capability) {
				results = append(results, t)
				break
			}
		}
	}
	return results
}

// Domains returns unique domains from tools
func Domains(tools []ToolDefinition) []string {
	seen := make(map[string]bool)
	var domains []string
	for _, t := range tools {
		if t.Namespace != "" {
			d := t.Namespace
			for i := 0; i < len(d); i++ {
				if d[i] == ':' {
					d = d[:i]
					break
				}
			}
			if !seen[d] {
				seen[d] = true
				domains = append(domains, d)
			}
		}
	}
	return domains
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		findSubstring(toLower(s), toLower(substr))
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			b[i] = s[i] + 32
		} else {
			b[i] = s[i]
		}
	}
	return string(b)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
