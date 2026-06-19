package core

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
)

// Token categories for free-text parsing
var (
	ipRe     = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(?:/\d{1,2})?\b`)
	urlRe    = regexp.MustCompile(`https?://\S+`)
	domainRe = regexp.MustCompile(`\b(network|web|ad|security|container|system|cloud|wireless|mobile|social|physical)\b`)
	portRe   = regexp.MustCompile(`\bport\s+(\d+(?:-\d+)?)\b`)
	flagRe   = regexp.MustCompile(`(?:^|\s)((?:--[a-zA-Z0-9][a-zA-Z0-9-]+(?:\s+\S+)?)|(?:-[a-zA-Z0-9][a-zA-Z0-9]*))`)
)

// knownToolNames is populated from the queries index at runtime
var knownToolNames []string

// knownActionKeywords maps free-text keywords to action names
// Populated from the action taxonomy via queries index
var knownActionKeywords map[string][]string

// ---------------------------------------------------------------------------
// Query Parsing
// ---------------------------------------------------------------------------

// isStructuredQuery returns true if the input looks like a structured query
func isStructuredQuery(s string) bool {
	return strings.Contains(s, "(") && strings.Contains(s, ")")
}

// ParseQuery parses a structured query string into a QueryFilter
// Format: domain(network):action(port-scan):tool(nmap):flags(-sV):target(10.0.0.0/24)
func ParseQuery(query string) QueryFilter {
	f := QueryFilter{Source: "structured"}

	if !isStructuredQuery(query) {
		return f
	}

	// Split by "):" but not inside values
	parts := splitQueryParts(query)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Split on first '('
		parenIdx := strings.IndexByte(part, '(')
		if parenIdx < 0 || !strings.HasSuffix(part, ")") {
			continue
		}
		key := strings.TrimSpace(part[:parenIdx])
		valuesStr := part[parenIdx+1 : len(part)-1]

		// Parameter fields keep their full value (don't split on /)
		paramFields := map[string]bool{"target": true, "port": true, "user": true, "pass": true, "password": true, "hash": true, "wordlist": true, "output": true}

		var values []string
		if paramFields[strings.ToLower(key)] {
			values = []string{strings.TrimSpace(valuesStr)}
		} else {
			// Split values by "/" for filter fields
			values = strings.Split(valuesStr, "/")
			for i := range values {
				values[i] = strings.TrimSpace(values[i])
			}
		}

		setQueryField(&f, key, values)
	}

	return f
}

// splitQueryParts splits "domain(network):action(port-scan):tool(nmap)" into parts
func splitQueryParts(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ':':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

// setQueryField sets a field on QueryFilter based on the key name
func setQueryField(f *QueryFilter, key string, values []string) {
	switch strings.ToLower(key) {
	case "domain":
		mergeValues(&f.Domains, values)
	case "action":
		mergeValues(&f.Actions, values)
	case "tool":
		mergeValues(&f.Tools, values)
	case "flags":
		mergeValues(&f.Flags, values)
		f.RawFlags = strings.Join(values, " ")
	case "phase":
		if len(values) > 0 {
			f.Phase = values[0]
		}
	case "service":
		mergeValues(&f.Services, values)
	case "technique":
		mergeValues(&f.Techniques, values)
	case "mitre":
		mergeValues(&f.MitreIDs, values)
	case "risk":
		if len(values) > 0 {
			f.Risk = values[0]
		}
	case "platform":
		mergeValues(&f.Platforms, values)
	case "protocol":
		mergeValues(&f.Protocols, values)
	case "port":
		if len(values) > 0 {
			f.Port = values[0]
		}
	case "target":
		if len(values) > 0 {
			f.Target = values[0]
		}
	case "user":
		if len(values) > 0 {
			f.Username = values[0]
		}
	case "pass", "password":
		if len(values) > 0 {
			f.Password = values[0]
		}
	case "hash":
		if len(values) > 0 {
			f.Hash = values[0]
		}
	case "wordlist":
		if len(values) > 0 {
			f.Wordlist = values[0]
		}
	case "output":
		if len(values) > 0 {
			f.Output = values[0]
		}
	}
}

func mergeValues(dst *[]string, values []string) {
	for _, v := range values {
		found := false
		for _, d := range *dst {
			if d == v {
				found = true
				break
			}
		}
		if !found {
			*dst = append(*dst, v)
		}
	}
}

// ---------------------------------------------------------------------------
// Free-text / Keyword Parsing
// ---------------------------------------------------------------------------

// ParseFreeText parses a natural-language query into a QueryFilter
func ParseFreeText(input string, queriesIndex *QueriesIndex, tools ...[]ToolDefinition) QueryFilter {
	f := QueryFilter{Source: "free-text"}
	inputLower := strings.ToLower(input)

	// Extract target (IP/CIDR)
	if m := ipRe.FindString(input); m != "" {
		f.Target = m
	}

	// Extract target (URL)
	if m := urlRe.FindString(input); m != "" {
		f.Target = m
	}

	// Extract port
	if m := portRe.FindStringSubmatch(input); len(m) > 1 {
		f.Port = m[1]
	}

	// Extract tool name from known tools (before flag extraction to avoid stealing flags)
	toolMatched := false

	// Collect tool names from available sources
	type toolNameEntry struct {
		name string
		id   string
	}
	var toolNames []toolNameEntry

	// From queries index (preferred)
	if queriesIndex != nil {
		for _, td := range queriesIndex.Tools {
			toolNames = append(toolNames, toolNameEntry{td.Name, td.ID})
		}
	}

	// Fallback: from tools slice directly (when no index available)
	if len(toolNames) == 0 && len(tools) > 0 {
		for _, t := range tools[0] {
			toolNames = append(toolNames, toolNameEntry{t.Name, t.ID})
		}
	}

	// Sort tool names by length descending to match most specific first
	for i := 0; i < len(toolNames); i++ {
		for j := i + 1; j < len(toolNames); j++ {
			if len(toolNames[j].name) > len(toolNames[i].name) {
				toolNames[i], toolNames[j] = toolNames[j], toolNames[i]
			}
		}
	}
	for _, entry := range toolNames {
		nameLower := strings.ToLower(entry.name)
		if strings.Contains(inputLower, nameLower) {
			mergeValues(&f.Tools, []string{entry.name})
			toolMatched = true
			break
		}
	}

	// If a tool was matched, extract everything after the tool name as raw args
	if toolMatched && len(f.Tools) > 0 {
		toolName := f.Tools[0]
		tl := strings.ToLower(toolName)
		if idx := strings.Index(inputLower, tl); idx >= 0 {
			afterTool := strings.TrimSpace(input[idx+len(toolName):])
			// Remove target from afterTool (already extracted)
			if f.Target != "" {
				afterTool = strings.Replace(afterTool, f.Target, "", 1)
			}
			afterTool = strings.TrimSpace(afterTool)

			// Extract individual flags from the remaining text
			flagTokens := strings.Fields(afterTool)
			for _, ft := range flagTokens {
				if strings.HasPrefix(ft, "-") {
					mergeValues(&f.Flags, []string{ft})
				}
			}
			if len(flagTokens) > 0 {
				f.RawFlags = strings.Join(flagTokens, " ")
			}
		}
	} else {
		// Extract flags from anywhere
		if m := flagRe.FindAllString(input, -1); len(m) > 0 {
			for _, fl := range m {
				fl = strings.TrimSpace(fl)
				mergeValues(&f.Flags, []string{fl})
			}
			f.RawFlags = strings.Join(f.Flags, " ")
		}
	}

	// Cross-reference flags with matched tool parameters to populate Params
	if toolMatched && len(f.Tools) > 0 && len(tools) > 0 {
		toolName := f.Tools[0]
		for _, toolDef := range tools[0] {
			if strings.EqualFold(toolDef.Name, toolName) || strings.EqualFold(toolDef.ID, toolName) {
				f.Params = make(map[string]string)

				// Match extracted flags to tool parameters
				for _, flagToken := range f.Flags {
					if key, ok := matchFlagToParam(&toolDef, flagToken); ok {
						for _, p := range toolDef.Parameters {
							paramKey := p.TemplateKey
							if paramKey == "" {
								paramKey = p.Name
							}
							if paramKey == key && p.Type == "boolean" {
								f.Params[key] = "true"
								break
							}
						}
					}
				}

				// Map known filter fields to parameter keys only when
				// the tool definition uses placeholders in its template
				if toolDef.Execution.Template != toolDef.Name &&
					strings.Contains(toolDef.Execution.Template, "{") {
					if f.Target != "" {
						f.Params["target"] = f.Target
					}
					if f.Port != "" {
						f.Params["port"] = f.Port
					}
					if f.Username != "" {
						f.Params["user"] = f.Username
					}
					if f.Password != "" {
						f.Params["pass"] = f.Password
					}
					if f.Hash != "" {
						f.Params["hash"] = f.Hash
					}
					if f.Wordlist != "" {
						f.Params["wordlist"] = f.Wordlist
					}
					if f.Output != "" {
						f.Params["output"] = f.Output
					}
				}
				break
			}
		}
	}

	// Extract domain
	if m := domainRe.FindString(inputLower); m != "" {
		mergeValues(&f.Domains, []string{m})
	}

	// Match action keywords from taxonomy
	if queriesIndex != nil {
		for kw, actions := range queriesIndex.Keywords {
			if strings.Contains(inputLower, strings.ToLower(kw)) {
				mergeValues(&f.Actions, actions)
			}
		}
	}

	// If we have flags but no action, infer from common flag patterns
	if len(f.Flags) > 0 && len(f.Actions) == 0 {
		for _, fl := range f.Flags {
			switch {
			case strings.Contains(fl, "-sV") || strings.Contains(fl, "-sC") || strings.Contains(fl, "-p-"):
				mergeValues(&f.Actions, []string{"port-scan"})
			case strings.Contains(fl, "-O"):
				mergeValues(&f.Actions, []string{"os-detect"})
			case strings.Contains(fl, "--batch") || strings.Contains(fl, "--level"):
				mergeValues(&f.Actions, []string{"sql-injection"})
			case strings.Contains(fl, "-w") || strings.Contains(fl, "-u"):
				mergeValues(&f.Actions, []string{"directory-brute"})
			case strings.Contains(fl, "-l") || strings.Contains(fl, "-P"):
				mergeValues(&f.Actions, []string{"bruteforce"})
			}
		}
	}

	// Fallback: scan for other known action-like words
	actionWords := map[string]string{
		"scan":          "port-scan",
		"discover":      "service-detect",
		"os detection":  "os-detect",
		"vuln":          "vuln-scan",
		"vulnerability": "vuln-scan",
		"sql":           "sql-injection",
		"sqli":          "sql-injection",
		"xss":           "xss",
		"directory":     "directory-brute",
		"dirbust":       "directory-brute",
		"subdomain":     "subdomain-enum",
		"dns":           "dns-recon",
		"user enum":     "user-enum",
		"brute":         "bruteforce",
		"bruteforce":    "bruteforce",
		"spray":         "password-spray",
		"password":      "bruteforce",
		"crack":         "hash-crack",
		"hash":          "hash-crack",
		"asrep":         "asreproast",
		"kerberoast":    "kerberoast",
		"rid":           "rid-brute",
		"credential":    "credential-dump",
		"lateral":       "lateral-movement",
		"smb":           "smb-enum",
		"ldap":          "ldap-enum",
		"relay":         "relay",
		"mitm":          "mitm",
		"osint":         "osint",
		"payload":       "payload-gen",
		"shell":         "reverse-shell",
		"sniff":         "sniff",
		"forensic":      "forensics",
	}

	for word, action := range actionWords {
		if strings.Contains(inputLower, word) {
			mergeValues(&f.Actions, []string{action})
		}
	}

	return f
}

// ---------------------------------------------------------------------------
// Query Execution
// ---------------------------------------------------------------------------

// ExecuteQuery runs a query against the tool database and returns ranked results
func ExecuteQuery(filter QueryFilter, tools []ToolDefinition, queriesIndex *QueriesIndex) []QueryResult {
	if queriesIndex != nil && len(filter.Tools) == 0 && len(filter.Actions) > 0 {
		return executeWithIndex(filter, tools, queriesIndex)
	}
	return executeDirect(filter, tools)
}

func executeWithIndex(filter QueryFilter, tools []ToolDefinition, queriesIndex *QueriesIndex) []QueryResult {
	// Start with all tools from action matches
	actionToolIDs := make(map[string]float64) // tool_id -> score contribution

	// Track which action matched each tool for rendering
	toolActions := make(map[string]string) // tool_id -> action name

	for _, action := range filter.Actions {
		actionEntry, ok := queriesIndex.Actions[action]
		if !ok {
			continue
		}
		baseScore := 0.4 / float64(len(filter.Actions))
		for _, ref := range actionEntry.Tools {
			conf := 1.0
			if ref.Confidence == "partial" {
				conf = 0.7
			} else if ref.Confidence == "inferred" {
				conf = 0.5
			}
			if _, exists := actionToolIDs[ref.ToolID]; !exists {
				toolActions[ref.ToolID] = action
			}
			actionToolIDs[ref.ToolID] += baseScore * conf
		}
	}

	// Domain filter
	if len(filter.Domains) > 0 {
		domainTools := make(map[string]bool)
		domainLower := make(map[string]bool)
		for _, d := range filter.Domains {
			domainLower[strings.ToLower(d)] = true
		}
		for tid := range actionToolIDs {
			td, ok := queriesIndex.Tools[tid]
			if !ok {
				continue
			}
			if domainLower[strings.ToLower(td.Domain)] {
				domainTools[tid] = true
			}
		}
		if len(domainTools) > 0 {
			for tid := range actionToolIDs {
				if !domainTools[tid] {
					delete(actionToolIDs, tid)
				}
			}
		}
	}

	// Phase filter
	if filter.Phase != "" {
		phaseLower := strings.ToLower(filter.Phase)
		for tid := range actionToolIDs {
			td, ok := queriesIndex.Tools[tid]
			if !ok || strings.ToLower(td.Phase) != phaseLower {
				delete(actionToolIDs, tid)
			}
		}
	}

	// Service filter
	if len(filter.Services) > 0 {
		svcLower := make(map[string]bool)
		for _, s := range filter.Services {
			svcLower[strings.ToLower(s)] = true
		}
		for tid := range actionToolIDs {
			td, ok := queriesIndex.Tools[tid]
			if !ok {
				continue
			}
			match := false
			for _, s := range td.Services {
				if svcLower[strings.ToLower(s)] {
					match = true
					break
				}
			}
			if !match {
				delete(actionToolIDs, tid)
			}
		}
	}

	// Risk filter
	if filter.Risk != "" {
		riskLower := strings.ToLower(filter.Risk)
		for tid := range actionToolIDs {
			td, ok := queriesIndex.Tools[tid]
			if !ok || strings.ToLower(td.RiskLevel) != riskLower {
				delete(actionToolIDs, tid)
			}
		}
	}

	// Technique filter
	if len(filter.Techniques) > 0 {
		techLower := make(map[string]bool)
		for _, t := range filter.Techniques {
			techLower[strings.ToLower(t)] = true
		}
		for tid := range actionToolIDs {
			td, ok := queriesIndex.Tools[tid]
			if !ok {
				continue
			}
			match := false
			for _, t := range td.Techniques {
				if techLower[strings.ToLower(t)] {
					match = true
					break
				}
			}
			if !match {
				delete(actionToolIDs, tid)
			}
		}
	}

	// Build result set
	if len(actionToolIDs) == 0 {
		return nil
	}

	// Add tool-specific filter constraints
	if len(filter.Tools) > 0 {
		toolLower := make(map[string]bool)
		for _, t := range filter.Tools {
			toolLower[strings.ToLower(t)] = true
		}
		for tid := range actionToolIDs {
			td, ok := queriesIndex.Tools[tid]
			if !ok {
				delete(actionToolIDs, tid)
				continue
			}
			if !toolLower[strings.ToLower(td.Name)] && !toolLower[strings.ToLower(td.ID)] {
				delete(actionToolIDs, tid)
			}
		}
	}

	// Convert to results
	var results []QueryResult
	for tid := range actionToolIDs {
		td := queriesIndex.Tools[tid]
		toolName := td.Name
		domain := td.Domain
		if domain == "" {
			domain = domainFromNamespace(td.Namespace)
		}
		score := actionToolIDs[tid]

		// Bonus for exact tool name match
		if len(filter.Tools) > 0 {
			for _, ft := range filter.Tools {
				if strings.EqualFold(td.Name, ft) || strings.EqualFold(td.ID, ft) {
					score += 0.3
					break
				}
			}
		}

		// Bonus for flag match
		if len(filter.Flags) > 0 {
			flagBonus := 0.1 * math.Min(float64(len(filter.Flags))/3.0, 1.0)
			score += flagBonus
		}

		// Find the matching tool definition for rendering
		toolDef := FindToolByID(tools, tid)
		if toolDef == nil {
			toolDef = findToolByName(tools, td.Name)
		}

		// Look up action tool_defaults for smart command generation
		matchedAction := toolActions[tid]
		var toolDefaults *ToolDefault
		var actionDefaultFlags string
		if matchedAction != "" {
			if entry, ok := queriesIndex.Actions[matchedAction]; ok {
				actionDefaultFlags = entry.DefaultFlags
				if tdef, ok := entry.ToolDefaults[tid]; ok {
					toolDefaults = &tdef
				} else if tdef2, ok := entry.ToolDefaults[td.Name]; ok {
					toolDefaults = &tdef2
				}
			}
		}

		cmd := smartRenderWithDefaults(
			toolName, td.ID, toolDef, filter,
			actionDefaultFlags, toolDefaults,
		)

		desc := td.Description
		if desc == "" && toolDef != nil {
			desc = toolDef.Description
		}

		results = append(results, QueryResult{
			ToolID:      tid,
			ToolName:    td.Name,
			Namespace:   td.Namespace,
			Domain:      domain,
			Command:     cmd,
			Confidence:  math.Min(score, 1.0),
			Phase:       td.Phase,
			RiskLevel:   td.RiskLevel,
			Description: desc,
			Explanation: buildExplanation(filter, td),
		})
	}

	// Sort by confidence descending
	sortResults(results)

	// Return single best match
	if len(results) > 0 {
		return results[:1]
	}
	return nil
}

func executeDirect(filter QueryFilter, tools []ToolDefinition) []QueryResult {
	var candidates []ToolDefinition

	// Tool filter
	if len(filter.Tools) > 0 {
		toolLower := make(map[string]bool)
		for _, t := range filter.Tools {
			toolLower[strings.ToLower(t)] = true
		}
		for _, t := range tools {
			if toolLower[strings.ToLower(t.Name)] || toolLower[strings.ToLower(t.ID)] {
				candidates = append(candidates, t)
			}
		}
	}

	// Domain filter
	if len(filter.Domains) > 0 && len(candidates) == 0 {
		for _, d := range filter.Domains {
			candidates = append(candidates, FilterByDomain(tools, d)...)
		}
		candidates = deduplicateTools(candidates)
	}

	// Action filter via capabilities
	if len(filter.Actions) > 0 && len(candidates) == 0 {
		actionCaps := make(map[string]bool)
		for _, a := range filter.Actions {
			actionCaps[a] = true
		}
		for _, t := range tools {
			for _, cap := range t.Capabilities {
				for ac := range actionCaps {
					if strings.Contains(strings.ToLower(cap), strings.ToLower(ac)) {
						candidates = append(candidates, t)
						break
					}
				}
			}
		}
		candidates = deduplicateTools(candidates)
	}

	// No filters: use all tools
	if len(candidates) == 0 {
		candidates = tools
	}

	// Score each candidate
	var results []QueryResult
	for _, t := range candidates {
		score := 0.0

		// Tool name match
		if len(filter.Tools) > 0 {
			for _, ft := range filter.Tools {
				if strings.EqualFold(t.Name, ft) || strings.EqualFold(t.ID, ft) {
					score += 0.4
					break
				}
			}
		}

		// Domain match
		if len(filter.Domains) > 0 {
			td := domainFromNamespace(t.Namespace)
			for _, fd := range filter.Domains {
				if strings.EqualFold(td, fd) {
					score += 0.2
					break
				}
			}
		}

		// Action/capability match
		if len(filter.Actions) > 0 {
			capScore := 0.0
			for _, a := range filter.Actions {
				aLower := strings.ToLower(a)
				for _, cap := range t.Capabilities {
					if strings.Contains(strings.ToLower(cap), aLower) {
						capScore += 0.15
						break
					}
				}
			}
			score += math.Min(capScore, 0.4)
		}

		// Phase match
		if filter.Phase != "" && strings.EqualFold(t.Phase, filter.Phase) {
			score += 0.1
		}

		// Flag match (for rendering)
		if len(filter.Flags) > 0 {
			score += 0.05
		}

		cmd := renderDirectCommand(filter, t)

		explanation := buildDirectExplanation(filter, t)

		results = append(results, QueryResult{
			ToolID:      t.ID,
			ToolName:    t.Name,
			Namespace:   t.Namespace,
			Domain:      domainFromNamespace(t.Namespace),
			Command:     cmd,
			Confidence:  math.Min(score, 1.0),
			Phase:       t.Phase,
			RiskLevel:   t.RiskLevel,
			Description: t.Description,
			Explanation: explanation,
		})
	}

	sortResults(results)

	if len(results) > 0 {
		return results[:1]
	}
	return nil
}

// ---------------------------------------------------------------------------
// Command Rendering
// ---------------------------------------------------------------------------

// smartRenderWithDefaults builds a command using:
// 1. Action-level default flags (from taxonomy)
// 2. Per-tool defaults from the action taxonomy
// 3. Query filter flags (override)
// 4. Target, port, credentials from filter
func smartRenderWithDefaults(
	toolName, toolID string,
	toolDef *ToolDefinition,
	filter QueryFilter,
	actionDefaultFlags string,
	toolDefaults *ToolDefault,
) string {
	// If structured params are available, use BuildCommand with metadata
	if toolDef != nil && len(filter.Params) > 0 {
		cmd, err := BuildCommand(*toolDef, filter.Params)
		if err == nil {
			return cmd
		}
	}

	// Determine flags to use:
	// Priority: filter flags > tool_defaults flags
	// Action default flags only apply when tool_defaults exist for this tool
	var flags []string

	if len(filter.Flags) > 0 || filter.RawFlags != "" {
		// User specified flags: use those
		if filter.RawFlags != "" {
			flags = append(flags, strings.Fields(filter.RawFlags)...)
		} else {
			flags = append(flags, filter.Flags...)
		}
	} else if toolDefaults != nil && toolDefaults.Flags != "" {
		// Tool-specific defaults from taxonomy
		flags = append(flags, strings.Fields(toolDefaults.Flags)...)
	}
	// NOTE: actionDefaultFlags are NOT applied automatically - they only serve
	// as a hint when no other flags are available and we have tool defaults.
	// This prevents wrong flags (e.g., -sV -sC on ping).

	// Target (use tool_defaults target_param if available)
	target := filter.Target
	if target == "" && toolDefaults != nil && toolDefaults.TargetParam != "" {
		target = toolDefaults.TargetParam
	}

	// Build command parts
	var parts []string
	parts = append(parts, toolName)

	// Substitute template variables like {target} in flags
	hadTargetPlaceholder := false
	flagsStr := strings.Join(flags, " ")
	if strings.Contains(flagsStr, "{target}") || strings.Contains(flagsStr, "{ip}") || strings.Contains(flagsStr, "{host}") {
		hadTargetPlaceholder = true
	}
	flagsStr = strings.ReplaceAll(flagsStr, "{target}", target)
	flagsStr = strings.ReplaceAll(flagsStr, "{ip}", target)
	flagsStr = strings.ReplaceAll(flagsStr, "{host}", target)
	flagsStr = strings.ReplaceAll(flagsStr, "{user}", filter.Username)
	flagsStr = strings.ReplaceAll(flagsStr, "{pass}", filter.Password)
	flagsStr = strings.ReplaceAll(flagsStr, "{port}", filter.Port)
	flagsStr = strings.ReplaceAll(flagsStr, "{domain}", "")
	flagsStr = strings.ReplaceAll(flagsStr, "{lhost}", target)
	flagsStr = strings.ReplaceAll(flagsStr, "{lport}", filter.Port)

	if strings.TrimSpace(flagsStr) != "" {
		parts = append(parts, strings.TrimSpace(flagsStr))
	}

	// Only append target if it wasn't already embedded in flags via {target}
	if target != "" && !hadTargetPlaceholder {
		parts = append(parts, target)
	}

	// Credentials are inserted after target as tool-specific format
	if filter.Username != "" && filter.Password != "" && toolName != "" {
		// Most tools use -u username -p password or similar
		if !strings.Contains(strings.Join(flags, " "), "-u") {
			parts = append(parts, "-u", filter.Username)
		}
		if !strings.Contains(strings.Join(flags, " "), "-p") {
			parts = append(parts, "-p", filter.Password)
		}
	} else if filter.Username != "" {
		if !strings.Contains(strings.Join(flags, " "), "-u") {
			parts = append(parts, "-u", filter.Username)
		}
	}

	// Port
	if filter.Port != "" {
		parts = append(parts, "-p", filter.Port)
	}

	// Hash
	if filter.Hash != "" {
		if !strings.Contains(strings.Join(flags, " "), "-H") {
			parts = append(parts, "-H", filter.Hash)
		}
	}

	return strings.Join(parts, " ")
}

func renderQueryCommand(filter QueryFilter, toolDef *ToolDefinition, td QueryToolDef) string {
	return smartRenderWithDefaults(td.Name, td.ID, toolDef, filter, "", nil)
}

func renderDirectCommand(filter QueryFilter, tool ToolDefinition) string {
	return smartRenderWithDefaults(tool.Name, tool.ID, &tool, filter, "", nil)
}

// ---------------------------------------------------------------------------
// Explanations
// ---------------------------------------------------------------------------

func buildExplanation(filter QueryFilter, td QueryToolDef) string {
	var parts []string
	if len(filter.Actions) > 0 {
		parts = append(parts, fmt.Sprintf("action '%s'", strings.Join(filter.Actions, "/")))
	}
	if len(filter.Domains) > 0 {
		parts = append(parts, fmt.Sprintf("domain '%s'", strings.Join(filter.Domains, "/")))
	}
	if len(filter.Tools) > 0 {
		parts = append(parts, fmt.Sprintf("tool '%s'", strings.Join(filter.Tools, "/")))
	}
	if filter.Phase != "" {
		parts = append(parts, fmt.Sprintf("phase '%s'", filter.Phase))
	}
	if len(parts) == 0 {
		return "Best match from available tools"
	}
	return fmt.Sprintf("Matched by %s", strings.Join(parts, ", "))
}

func buildDirectExplanation(filter QueryFilter, t ToolDefinition) string {
	var parts []string
	if len(filter.Actions) > 0 {
		matchedCaps := []string{}
		for _, a := range filter.Actions {
			a = strings.ToLower(a)
			for _, cap := range t.Capabilities {
				if strings.Contains(strings.ToLower(cap), a) {
					matchedCaps = append(matchedCaps, cap)
				}
			}
		}
		if len(matchedCaps) > 0 {
			parts = append(parts, fmt.Sprintf("capabilities [%s]", strings.Join(matchedCaps, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("action '%s' (keyword)", strings.Join(filter.Actions, "/")))
		}
	}
	if len(filter.Domains) > 0 {
		parts = append(parts, fmt.Sprintf("domain '%s'", strings.Join(filter.Domains, "/")))
	}
	if len(parts) == 0 {
		return "Direct tool match"
	}
	return fmt.Sprintf("Matched by %s", strings.Join(parts, ", "))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func domainFromNamespace(ns string) string {
	if idx := strings.IndexByte(ns, ':'); idx > 0 {
		return ns[:idx]
	}
	return ns
}

func findToolByName(tools []ToolDefinition, name string) *ToolDefinition {
	for i, t := range tools {
		if t.Name == name {
			return &tools[i]
		}
	}
	return nil
}

func deduplicateTools(tools []ToolDefinition) []ToolDefinition {
	seen := make(map[string]bool)
	var out []ToolDefinition
	for _, t := range tools {
		if !seen[t.ID] {
			seen[t.ID] = true
			out = append(out, t)
		}
	}
	return out
}

func sortResults(results []QueryResult) {
	// Simple bubble sort (small n)
	n := len(results)
	for i := 0; i < n; i++ {
		swapped := false
		for j := 0; j < n-i-1; j++ {
			if results[j].Confidence < results[j+1].Confidence {
				results[j], results[j+1] = results[j+1], results[j]
				swapped = true
			}
		}
		if !swapped {
			break
		}
	}
}

// matchFlagToParam tries to match a flag token to a tool parameter by Flag or alias.
// Returns the template_key (or name) of the matched parameter.
func matchFlagToParam(toolDef *ToolDefinition, flagToken string) (string, bool) {
	for _, p := range toolDef.Parameters {
		if p.Flag == flagToken {
			key := p.TemplateKey
			if key == "" {
				key = p.Name
			}
			return key, true
		}
		for _, alias := range p.Aliases {
			aliasFlag := strings.Fields(alias)[0]
			if aliasFlag == flagToken {
				key := p.TemplateKey
				if key == "" {
					key = p.Name
				}
				return key, true
			}
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// QueriesIndex Loading
// ---------------------------------------------------------------------------

// LoadQueriesIndex reads a queries.json file and returns a QueriesIndex
func LoadQueriesIndex(path string) (*QueriesIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read queries index: %w", err)
	}
	var idx QueriesIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse queries index: %w", err)
	}
	return &idx, nil
}

// DefaultQueriesIndexPaths returns likely locations for queries.json
func DefaultQueriesIndexPaths() []string {
	configDir, _ := os.UserConfigDir()
	homeDir, _ := os.UserHomeDir()
	return []string{
		configDir + "/autoxmate/queries.json",
		homeDir + "/.autoxmate/queries.json",
		"queries.json",
		"api/v1/queries.json",
		"public/api/v1/queries.json",
	}
}

// FindQueriesIndexFile searches for queries.json in common locations
func FindQueriesIndexFile() (string, error) {
	for _, p := range DefaultQueriesIndexPaths() {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("queries.json not found")
}
