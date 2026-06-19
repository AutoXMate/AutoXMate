package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/AutoXMate/AutoXmate/core"
)

func domainColor(domain string) string {
	colors := map[string]string{
		"security":  colRed,
		"network":   colBlue,
		"system":    colGreen,
		"text":      colOrange,
		"dev":       colPurple,
		"container": colCyan,
	}
	if c, ok := colors[domain]; ok {
		return c
	}
	return colText
}

func domainCounts(tools []core.ToolDefinition) map[string]int {
	counts := make(map[string]int)
	for _, t := range tools {
		d := domainName(t)
		counts[d]++
	}
	return counts
}

func domainName(t core.ToolDefinition) string {
	if t.Namespace == "" {
		return "other"
	}
	for i := 0; i < len(t.Namespace); i++ {
		if t.Namespace[i] == ':' {
			return t.Namespace[:i]
		}
	}
	return t.Namespace
}

func toolsForDomain(tools []core.ToolDefinition, domain string) []core.ToolDefinition {
	if domain == "" || domain == "all" {
		return tools
	}
	var filtered []core.ToolDefinition
	for _, t := range tools {
		if domainName(t) == domain {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// --- Command Entry Views ---

func (m *Model) commandView() string {
	out := m.renderOutput(m.commandOutputHeight())
	ac := ""
	if m.completing && len(m.completeResults) > 0 {
		ac = "\n" + m.autocompleteView()
	}
	inp := "\n" + m.inputView()
	status := "\n" + m.statusBarView()
	toast := m.toastView()
	return toast + out + ac + inp + status
}

func (m *Model) renderOutput(height int) string {
	var b strings.Builder

	if len(m.output) == 0 {
		b.WriteString(outputLineStyle.Render("  Type 'help' for available commands"))
		output := b.String()
		if lines := strings.Count(output, "\n"); lines < height {
			output += strings.Repeat("\n", height-lines)
		}
		return output
	}

	n := len(m.output)

	if m.outputOff > n {
		m.outputOff = n
	}

	contentLines := height

	scrollHintLine := ""
	if m.outputOff > 0 {
		scrollHintLine = scrollHintStyle.Render(fmt.Sprintf("  ↑ scrolled (PgDn to see more, -%d lines)", m.outputOff))
		contentLines--
	}

	endRow := n - m.outputOff
	if endRow < 0 {
		endRow = 0
	}
	startRow := endRow - contentLines
	if startRow < 0 {
		startRow = 0
	}

	// First pass: merge code blocks and handle collapsible entries
	type renderedLine struct {
		content string
	}
	var rendered []renderedLine

	inCodeBlock := false
	var codeLang string
	var codeLines []string

	flushCodeBlock := func() {
		if len(codeLines) > 0 {
			highlighted := renderHighlightedLines(codeLang, strings.Join(codeLines, "\n"))
			for _, hlLine := range highlighted {
				rendered = append(rendered, renderedLine{content: hlLine})
			}
			codeLines = nil
			codeLang = ""
		}
	}

	for i := startRow; i < endRow; i++ {
		entry := m.output[i]
		line := entry.Content
		trimmed := strings.TrimSpace(line)

		// Collapsible entry check
		if entry.Hidden != nil {
			flushCodeBlock()
			toggleText := fmt.Sprintf("  ▶ %d more lines (click to expand)", len(entry.Hidden))
			rendered = append(rendered, renderedLine{content: collapsibleStyle.Render(toggleText)})
			continue
		}

		if entry.Kind == "code" || strings.HasPrefix(trimmed, "```") {
			if strings.HasPrefix(trimmed, "```") && !inCodeBlock {
				inCodeBlock = true
				codeLang = strings.TrimSpace(trimmed[3:])
				continue
			} else if strings.HasPrefix(trimmed, "```") && inCodeBlock {
				flushCodeBlock()
				inCodeBlock = false
				continue
			} else if inCodeBlock {
				codeLines = append(codeLines, line)
				continue
			}
		} else if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		flushCodeBlock()
		rendered = append(rendered, renderedLine{content: m.renderLine(entry, line)})
	}
	flushCodeBlock()

	// Generate scrollbar
	visibleCount := len(rendered)
	scrollbar := m.renderScrollbar(n, visibleCount, startRow)

	// Render scroll hint + each line with scrollbar suffix
	if scrollHintLine != "" {
		b.WriteString(scrollHintLine)
		if len(scrollbar) > 0 {
			b.WriteString(" " + scrollbar[0])
		}
		b.WriteString("\n")
		if len(scrollbar) > 0 {
			scrollbar = scrollbar[1:]
		}
	}

	for i, rl := range rendered {
		b.WriteString(rl.content)
		if i < len(scrollbar) {
			b.WriteString(scrollbar[i])
		}
		b.WriteString("\n")
	}

	output := b.String()
	if lines := strings.Count(output, "\n"); lines < height {
		output += strings.Repeat("\n", height-lines)
	}
	return output
}

func (m *Model) renderLine(entry scrollbackEntry, line string) string {
	switch entry.Kind {
	case "prompt":
		return promptLineStyle.Render(line)
	case "error":
		return errorLineStyle.Render(line)
	case "system":
		return successLineStyle.Render(line)
	case "diff":
		return m.renderDiffLine(line)
	case "markdown":
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			return mdHeaderStyle.Render(line)
		}
		return outputLineStyle.Render(line)
	default:
		if strings.Contains(line, "\x1b[") {
			return line
		}
		return outputLineStyle.Render(line)
	}
}

func (m *Model) renderScrollbar(totalLines, visibleLines, startRow int) []string {
	if visibleLines <= 0 || totalLines <= 0 {
		return nil
	}

	bar := make([]string, visibleLines)
	if visibleLines >= totalLines {
		// Everything visible, no scrollbar needed
		for i := range bar {
			bar[i] = scrollbarTrackStyle.Render(" ")
		}
		return bar
	}

	thumbSize := max(1, int(float64(visibleLines)/float64(totalLines)*float64(visibleLines)))
	if thumbSize > visibleLines {
		thumbSize = visibleLines
	}

	// Where the thumb starts in the scrollbar
	thumbPos := int(float64(startRow) / float64(totalLines-visibleLines) * float64(visibleLines-thumbSize))
	if thumbPos > visibleLines-thumbSize {
		thumbPos = visibleLines - thumbSize
	}
	if thumbPos < 0 {
		thumbPos = 0
	}

	for i := 0; i < visibleLines; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			bar[i] = scrollbarThumbStyle.Render(" █")
		} else {
			bar[i] = scrollbarTrackStyle.Render(" │")
		}
	}
	return bar
}

func (m *Model) renderDiffLine(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "+++"):
		return diffHeaderStyle.Render(line)
	case strings.HasPrefix(trimmed, "@@"):
		return diffMetaStyle.Render(line)
	case strings.HasPrefix(trimmed, "-"):
		return diffRemovedStyle.Render(line)
	case strings.HasPrefix(trimmed, "+"):
		return diffAddedStyle.Render(line)
	default:
		return outputLineStyle.Render(line)
	}
}

func (m *Model) inputView() string {
	if m.running {
		prompt := "⟳ "
		if m.isShellMode() {
			prompt = "$ "
		}
		return inputLineStyle.Render(prompt + m.runLabel)
	}

	shellMode := m.isShellMode()
	promptChar := "> "
	promptStyle := promptCharStyle
	if shellMode {
		promptChar = "$ "
		promptStyle = shellPromptStyle
	}
	prompt := promptStyle.Render(promptChar + " ")

	cursorStyled := inputCursorStyle.Render(" ")

	// Cycling placeholder when input is empty
	if m.input == "" {
		ph := ""
		if m.currentScreen == screenHome || m.currentScreen == screenLoading {
			ph = placeholders[m.placeholderIdx%len(placeholders)]
		} else {
			ph = placeholders[m.placeholderIdx%len(placeholders)]
		}
		return inputLineStyle.Render(prompt + placeholderStyle.Render(ph) + cursorStyled)
	}

	var inputContent string
	if !strings.Contains(m.input, "\n") {
		input := m.input
		pos := m.inputCursor
		if pos >= len(input) {
			input += cursorStyled
		} else {
			input = input[:pos] + cursorStyled + input[pos:]
		}
		inputContent = prompt + input
	} else {
		// Multi-line rendering
		lines := strings.Split(m.input, "\n")
		maxLines := m.inputDisplayLines()
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		var b strings.Builder
		cursorRow := 0
		cursorCol := 0
		offset := 0
		for i, line := range lines {
			if i > 0 {
				offset++ // for the \n
			}
			lineLen := len(line)
			if offset+lineLen <= m.inputCursor || (i == len(lines)-1 && m.inputCursor >= offset+lineLen) {
				if i == len(lines)-1 {
					cursorRow = i
					cursorCol = m.inputCursor - offset
				}
			} else if m.inputCursor >= offset && m.inputCursor <= offset+lineLen {
				cursorRow = i
				cursorCol = m.inputCursor - offset
			}
			offset += lineLen
		}

		for i, line := range lines {
			if i == cursorRow {
				if cursorCol >= len(line) {
					b.WriteString(line + cursorStyled)
				} else {
					b.WriteString(line[:cursorCol] + cursorStyled + line[cursorCol:])
				}
			} else {
				b.WriteString(line)
			}
			if i < len(lines)-1 {
				b.WriteString("\n")
			}
		}
		inputContent = prompt + b.String()
	}

	// Counter badge on the right
	lineCount := 1
	if strings.Contains(m.input, "\n") {
		lineCount = strings.Count(m.input, "\n") + 1
	}
	charCount := utf8.RuneCountInString(m.input)
	counter := inputCounterStyle.Render(fmt.Sprintf("[%d chars · %d line]", charCount, lineCount))
	if lineCount > 1 {
		counter = inputCounterStyle.Render(fmt.Sprintf("[%d chars · %d lines]", charCount, lineCount))
	}

	return inputLineStyle.Render(inputContent + "  " + counter)
}

func (m *Model) autocompleteView() string {
	if !m.completing || len(m.completeResults) == 0 {
		return ""
	}

	var b strings.Builder
	prefix := m.input[m.completeStart:]
	acPrefix := acActiveStyle.Render(" " + prefix)

	maxResults := 10
	if len(m.completeResults) < maxResults {
		maxResults = len(m.completeResults)
	}

	for i := 0; i < maxResults; i++ {
		item := m.completeResults[i]
		if i == m.completeCursor {
			b.WriteString(acActiveStyle.Render("▸ " + item))
			b.WriteString("  ")
			b.WriteString(acItemStyle.Render("(Tab to select)"))
		} else {
			b.WriteString(acItemStyle.Render("  " + item))
		}
		b.WriteString("\n")
	}

	b.WriteString(acItemStyle.Render(fmt.Sprintf("  [%d/%d] ↑↓ navigate", m.completeCursor+1, len(m.completeResults))))

	content := acPrefix + "\n" + b.String()
	return acBoxStyle.Render(content)
}

func (m *Model) toastView() string {
	if len(m.toasts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range m.toasts {
		elapsed := time.Since(t.timestamp)
		if elapsed > 4*time.Second {
			continue
		}
		icon := "•"
		var style lipgloss.Style
		switch t.kind {
		case "success":
			icon = "✓"
			style = toastSuccessStyle
		case "error":
			icon = "✗"
			style = toastErrorStyle
		default:
			icon = "i"
			style = toastInfoStyle
		}
		b.WriteString(style.Render(fmt.Sprintf(" %s %s ", icon, t.message)))
		b.WriteString("\n")
	}
	return b.String()
}

// --- Markdown Rendering ---

func renderMarkdown(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "### "):
			lines = append(lines, mdHeaderStyle.Render("  "+trimmed[4:]))
		case strings.HasPrefix(trimmed, "## "):
			lines = append(lines, mdHeaderStyle.Render("  "+trimmed[3:]))
		case strings.HasPrefix(trimmed, "# "):
			lines = append(lines, mdHeaderStyle.Render("  "+trimmed[2:]))
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			bullet := trimmed[2:]
			lines = append(lines, outputLineStyle.Render("  • "+bullet))
		case strings.HasPrefix(trimmed, "```"):
			// Code block start/end — skip delimiter
			continue
		default:
			// Inline bold: **text**
			rendered := renderInlineMarkdown(trimmed)
			lines = append(lines, outputLineStyle.Render("  "+rendered))
		}
	}
	return lines
}

func renderInlineMarkdown(text string) string {
	// Handle **bold**
	var result strings.Builder
	for i := 0; i < len(text); i++ {
		if i+1 < len(text) && text[i] == '*' && text[i+1] == '*' {
			// Find closing **
			j := i + 2
			for j < len(text)-1 {
				if text[j] == '*' && text[j+1] == '*' {
					boldText := text[i+2 : j]
					result.WriteString(mdBoldStyle.Render(boldText))
					i = j + 1
					goto next
				}
				j++
			}
			// No closing **, emit literal
			result.WriteByte(text[i])
		} else if i+1 < len(text) && text[i] == '`' {
			// Inline code
			j := i + 1
			for j < len(text) && text[j] != '`' {
				j++
			}
			if j > i+1 && j < len(text) {
				code := text[i+1 : j]
				result.WriteString(mdCodeStyle.Render(code))
				i = j
			} else {
				result.WriteByte(text[i])
			}
		} else {
			result.WriteByte(text[i])
		}
	next:
	}
	return result.String()
}

func (m *Model) statusBarView() string {
	installed := 0
	for _, s := range m.statuses {
		if s.OnPath || s.DockerImage || s.PackageManager != "" {
			installed++
		}
	}

	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(colTextMuted)).Render("│")

	var hints string
	if m.currentScreen == screenHome {
		hints = ""
	} else {
		hints = fmt.Sprintf("  ? keys  %s  ↑↓ hist  %s  Ctrl+S stash  %s  Alt+S restore  %s  Ctrl+O browse  %s  Ctrl+R hist  %s  q quit", sep, sep, sep, sep, sep, sep)
	}
	stats := fmt.Sprintf(" %d tools  %s  %d installed  %s  v%s  ", len(m.tools), sep, installed, sep, appVersion)

	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		statusBarLeft.Render(hints),
		statusBarRight.Render(stats),
	)
}

// --- Which-Key Overlay ---

func (m *Model) whichKeyView() string {
	content := strings.Builder{}

	content.WriteString(dialogTitle.Render(" Which-Key — AutoMate Keybindings"))
	content.WriteString("\n\n")

	bindings := []struct {
		key, desc string
	}{
		{"General", ""},
		{"  ?", "Toggle this keybinding reference"},
		{"  q, Ctrl+C", "Quit AutoXMate"},
		{"", ""},
		{"Navigation", ""},
		{"  ↑/↓", "Command history (previous/next)"},
		{"  PgUp/PgDn", "Scroll output area"},
		{"  Home/End", "Jump to start/end of input"},
		{"  ←/→", "Move cursor within input"},
		{"", ""},
		{"Commands", ""},
		{"  Tab", "Insert @ and start tool autocomplete"},
		{"  @", "Tool mention autocomplete"},
		{"  /", "Command autocomplete"},
		{"  Alt+Enter", "Insert newline (multi-line input)"},
		{"", ""},
		{"Overlays", ""},
		{"  Ctrl+O", "Open tool browser (fuzzy search + install status)"},
		{"  Ctrl+P", "Open command palette"},
		{"  Ctrl+R", "Reverse history search"},
		{"  Ctrl+X B", "Toggle domain sidebar"},
		{"Input", ""},
		{"  Ctrl+S", "Stash current input (save for later)"},
		{"  Alt+S", "Restore last stashed input"},
		{"  Tab (shell)", "Complete filenames in shell (!) mode"},
		{"  Ctrl+L", "Clear output"},
		{"", ""},
		{"Confirm Dialog", ""},
		{"  y / Enter", "Confirm (yes)"},
		{"  n / Esc", "Cancel (no)"},
	}

	for _, b := range bindings {
		if b.key == "" && b.desc == "" {
			content.WriteString("\n")
			continue
		}
		if b.desc == "" {
			// Section header
			content.WriteString(dialogSection.Render(b.key))
			content.WriteString("\n")
		} else {
			content.WriteString(fmt.Sprintf("  %s  %s\n",
				dialogKey.Render(b.key),
				dialogDesc.Render(b.desc),
			))
		}
	}

	content.WriteString("\n")
	content.WriteString(dialogHint.Render("  Press any key to close"))

	return dialogOverlay.Width(56).Render(content.String())
}

// --- Confirm Dialog ---

func (m *Model) confirmView() string {
	content := strings.Builder{}

	content.WriteString(confirmPromptStyle.Render(m.confirmMessage))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("  %s    %s",
		confirmYesStyle.Render("[ y ] Yes"),
		confirmNoStyle.Render("[ n ] No"),
	))
	content.WriteString("\n\n")
	content.WriteString(dialogHint.Render("  y/enter: confirm  •  n/esc: cancel"))

	return dialogOverlay.Width(50).Render(content.String())
}

// --- Tool Browser (Ctrl+O) ---

func (m *Model) browserView() string {
	content := strings.Builder{}

	prompt := " Search: "
	if m.browserQuery == "" {
		prompt += "█"
	} else {
		prompt += m.browserQuery + "█"
	}
	content.WriteString(browserInputStyle.Render(prompt))
	content.WriteString("\n\n")

	if len(m.browserResults) == 0 {
		content.WriteString(dialogHint.Render("  No matching tools found."))
		content.WriteString("\n")
	} else {
		// Group by domain
		grouped := make(map[string][]core.ToolDefinition)
		var domains []string
		for _, t := range m.browserResults {
			d := domainName(t)
			if _, ok := grouped[d]; !ok {
				domains = append(domains, d)
			}
			grouped[d] = append(grouped[d], t)
		}
		sort.Strings(domains)

		total := len(m.browserResults)
		content.WriteString(dialogSection.Render(fmt.Sprintf(" %d tools", total)))
		content.WriteString("\n")

		maxShow := 15
		shown := 0
		done := false

		for _, d := range domains {
			if done {
				break
			}
			tools := grouped[d]
			dc := domainColor(d)

			// Category header
			if m.browserQuery == "" {
				content.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color(colTextMuted)).
					Padding(0, 1).
					Render(fmt.Sprintf("  ── %s (%d) ──", d, len(tools))))
				content.WriteString("\n")
			}

			for _, t := range tools {
				if shown >= maxShow {
					done = true
					break
				}
				statusStr := " "
				if s, ok := m.statuses[t.ID]; ok {
					if s.OnPath {
						statusStr = "✓"
					} else if s.PackageManager != "" || s.DockerImage {
						statusStr = "~"
					} else {
						statusStr = "✗"
					}
				}

				statusColor := colText
				if statusStr == "✓" {
					statusColor = colGreen
				} else if statusStr == "✗" {
					statusColor = colRed
				}

				// Check if this is the cursor item
				isCursor := false
				globalIdx := 0
				for _, tt := range m.browserResults {
					if tt.ID == t.ID {
						break
					}
					globalIdx++
				}
				isCursor = globalIdx == m.browserCursor

				line := fmt.Sprintf(" %s %-24s %s",
					lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render(statusStr+" "),
					lipgloss.NewStyle().Foreground(lipgloss.Color(dc)).Render(t.Name),
					lipgloss.NewStyle().Foreground(lipgloss.Color(colTextMuted)).Render(truncateStr(t.Description, 40)),
				)

				if isCursor {
					content.WriteString(browserActiveStyle.Render(line))
				} else {
					content.WriteString(browserItemStyle.Render(line))
				}
				content.WriteString("\n")
				shown++
			}
		}

		if total > maxShow {
			content.WriteString(dialogHint.Render(fmt.Sprintf("  ... %d more (type to filter)", total-maxShow)))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(dialogHint.Render("  ↑↓ nav  •  enter: details  •  i: install  •  r: run  •  tab: quick-install  •  esc: close"))

	return dialogOverlay.Width(66).Render(content.String())
}

// --- History Search (Ctrl+R) ---

func (m *Model) histSearchView() string {
	content := strings.Builder{}

	prompt := " Search history: "
	if m.histSearchQuery == "" {
		prompt += "█"
	} else {
		prompt += m.histSearchQuery + "█"
	}
	content.WriteString(browserInputStyle.Render(prompt))
	content.WriteString("\n\n")

	if len(m.histSearchResults) == 0 {
		content.WriteString(dialogHint.Render("  No matching history entries."))
		content.WriteString("\n")
	} else {
		count := len(m.histSearchResults)
		content.WriteString(dialogSection.Render(fmt.Sprintf(" %d matches", count)))
		content.WriteString("\n")

		maxShow := 10
		if maxShow > count {
			maxShow = count
		}

		for i := 0; i < maxShow; i++ {
			idx := m.histSearchResults[i]
			line := truncateStr(m.history[idx], 58)
			if i == m.histSearchCursor {
				content.WriteString(histActiveStyle.Render(fmt.Sprintf("  %s", line)))
			} else {
				content.WriteString(histItemStyle.Render(fmt.Sprintf("  %s", line)))
			}
			content.WriteString("\n")
		}

		if count > maxShow {
			content.WriteString(dialogHint.Render(fmt.Sprintf("  ... %d more", count-maxShow)))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(dialogHint.Render("  ↑↓ nav  •  enter: select  •  esc: close"))

	return dialogOverlay.Width(66).Render(content.String())
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// --- Sidebar View ---

func (m *Model) sidebarView() string {
	const contentWidth = 38
	counts := domainCounts(m.tools)
	domains := core.Domains(m.tools)

	var b strings.Builder

	// Header (openCode-style: bold white title)
	b.WriteString(sidePanelHeader.Render("AutoXMate"))
	b.WriteString("\n")

	// Domain sections
	for _, d := range domains {
		count := counts[d]
		domainTools := core.FilterByDomain(m.tools, d)
		domainInstalled := 0
		for _, t := range domainTools {
			if s, ok := m.statuses[t.Name]; ok && (s.OnPath || s.DockerImage || s.PackageManager != "") {
				domainInstalled++
			}
		}

		dc := domainColor(d)
		active := m.selectedDomain == d

		// Domain name line: "  name          3/5"
		namePad := d
		countStr := fmt.Sprintf("%d/%d", domainInstalled, count)
		// Right-align the count string
		totalW := contentWidth - 2 // 2 for leading spaces
		nameW := totalW - len(countStr)
		if nameW < 1 {
			nameW = 1
		}
		fullLine := "  " + fmt.Sprintf("%-*s%s", nameW, namePad, countStr)

		if active {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(colBlue)).
				Bold(true).
				Background(lipgloss.Color(sideBg)).
				Render("▸ " + fullLine[2:]))
		} else {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(dc)).
				Background(lipgloss.Color(sideBg)).
				Render(fullLine))
		}
		b.WriteString("\n")

		// Tool lines (max 5)
		showTools := domainTools
		if len(showTools) > 5 {
			showTools = showTools[:5]
		}
		for _, t := range showTools {
			s, hasStatus := m.statuses[t.Name]
			badge := "·"
			badgeStyle := sideBadgeMissing
			if hasStatus && (s.OnPath || s.DockerImage || s.PackageManager != "") {
				badge = "✓"
				badgeStyle = sideBadgeInstalled
			}
			toolW := contentWidth - 6 // 4 indent + 1 badge + 1 space
			name := t.Name
			if len(name) > toolW {
				name = name[:toolW]
			}
			toolLine := fmt.Sprintf("    %s %s", badgeStyle.Render(badge), sideToolName.Render(name))
			b.WriteString(toolLine)
			b.WriteString("\n")
		}

		if len(domainTools) > 5 {
			more := len(domainTools) - 5
			b.WriteString(sideBadgeMissing.Render(fmt.Sprintf("    … %d more", more)))
			b.WriteString("\n")
		}
	}

	// Footer: "  • AutoMate 0.1.0"
	footer := fmt.Sprintf(" %s AutoMate %s ",
		sideBadgeInstalled.Render("•"),
		appVersion,
	)
	b.WriteString(sideFooterStyle.Render(footer))
	b.WriteString("\n")

	return sidePanelStyle.Width(contentWidth).Render(b.String())
}

func (m *Model) overlaySidebarView() string {
	sidebar := m.sidebarView()
	sidebarW := 42
	leftW := m.width - sidebarW

	// Backdrop: full terminal height, dark overlay bg
	backdropLines := m.height
	left := strings.Repeat("\n", backdropLines)
	leftStyled := sideOverlayBg.Width(leftW).Render(left)

	// Sidebar: pad to full terminal height so background fills evenly
	sideLines := strings.Count(sidebar, "\n") + 1
	if sideLines < m.height {
		sidebar += strings.Repeat("\n", m.height-sideLines)
	}
	rightStyled := lipgloss.NewStyle().
		Background(lipgloss.Color(sideBg)).
		Width(sidebarW).
		Render(sidebar)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, rightStyled)
}

// --- Palette View ---

func (m *Model) paletteView() string {
	var b strings.Builder

	b.WriteString(paletteStyle.Render(""))
	content := strings.Builder{}

	prompt := " Search: "
	if m.paletteQuery == "" {
		prompt += "█"
	} else {
		prompt += m.paletteQuery + "█"
	}

	content.WriteString(paletteInputStyle.Render(prompt))
	content.WriteString("\n\n")

	if len(m.paletteResults) > 0 {
		for i, t := range m.paletteResults {
			line := fmt.Sprintf("  %s  %s", t.Name, t.ID)
			if i == m.paletteCursor {
				content.WriteString(paletteActiveStyle.Render(line))
			} else {
				content.WriteString(paletteItemStyle.Render(line))
			}
			content.WriteString("\n")
		}
	} else if m.paletteQuery != "" {
		content.WriteString(paletteHintStyle.Render("  No matching tools found."))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(paletteHintStyle.Render("  ↑↓ navigate  •  enter: details  •  esc: close"))
	content.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(paletteStyle.Render(content.String()))

	return b.String()
}
