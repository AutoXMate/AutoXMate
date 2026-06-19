package tui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/AutoXMate/AutoXmate/core"
	"github.com/AutoXMate/AutoXmate/site"
)

const (
	appVersion = "0.1.0"
)

type screen int

const (
	screenLoading screen = iota
	screenHome
	screenCommand
)

type leaderTimeoutMsg struct{}
type toastDismissMsg struct{}
type spinnerTickMsg struct{}
type loadingDoneMsg struct{}
type streamOutputMsg struct {
	line string
	kind string // "text", "code", "diff", "error", "system", "prompt"
}
type cmdDoneMsg struct {
	lines []string
}

type scrollbackEntry struct {
	Kind    string // "text", "code", "diff", "error", "system", "prompt"
	Content string
	Data    interface{} // optional extra data (e.g., code language)
	Hidden  []scrollbackEntry // nil = not collapsed; non-nil = collapsed, these are the hidden lines
}

type sysInfo struct {
	hostname  string
	os        string
	kernel    string
	arch      string
	cpuCores  int
	memory    string
	uptime    string
	toolCount int
	buildVer  string
	timeStr   string
}

type Model struct {
	program    *tea.Program
	tools      []core.ToolDefinition
	toolsCache *core.Cache
	cursor     int
	filtered   []core.ToolDefinition
	selected   *core.ToolDefinition
	statuses   map[string]core.InstallStatus
	width      int
	height     int
	sysInfo    sysInfo

	currentScreen    screen
	leaderActive     bool
	sidebarMode      string   // "auto" or "hide"
	sidebarOpen      bool     // transient toggle state
	selectedDomain   string

	paletteVisible bool
	paletteQuery   string
	paletteResults []core.ToolDefinition
	paletteCursor  int

	// Command entry mode
	input       string
	inputCursor int
	history     []string
	historyIdx  int // -1 = new input, 0+ = browsing history
	output      []scrollbackEntry
	outputLines int // total lines added (for scroll calculations)
	outputOff   int // scroll offset from bottom

	// Running state
	running  bool
	runLabel string

	// Autocomplete
	completing      bool
	completeStart   int
	completeQuery   string
	completeResults []string
	completeCursor  int
	completeType    string // "tool" or "command"

	// Toasts
	toasts []toastMsg

	// Multi-line composing
	composing bool

	// Interactive overlays
	whichKeyVisible   bool
	confirmVisible    bool
	confirmMessage    string
	confirmYesLabel   string
	confirmNoLabel    string
	pendingCmd        func(bool) tea.Cmd

	// Tool browser (Ctrl+O)
	browserVisible bool
	browserQuery   string
	browserResults []core.ToolDefinition
	browserCursor  int

	// History search (Ctrl+R)
	histSearchVisible  bool
	histSearchQuery    string
	histSearchResults  []int  // indices into m.history
	histSearchCursor   int

	// Input stash (Ctrl+S / Alt+S)
	stash        []historyEntry
	pastedBuf    string // stores pasted text for paste placeholder
	pastedAt     int    // index in input where paste label starts (-1 = none)
	historyDraft string // saved input when browsing history (for restore)

	// Kill ring (Ctrl+K/U/Y)
	killRing string

	// Undo/redo (Ctrl+Z / Ctrl+Shift+Z)
	undoStack []string
	redoStack []string

	// Phase 7: Home screen + polish
	loadingTools      bool
	loadingComplete   bool
	loadingStartTime  time.Time
	homeContentOffset int
	placeholderIdx    int

	// Home screen content (captured for transition to command mode)
	homeContent string
}

type toastMsg struct {
	message   string
	kind      string // "success", "error", "info"
	timestamp time.Time
}

type historyEntry struct {
	Input     string `json:"input"`
	Timestamp int64  `json:"timestamp"`
}

const (
	maxHistoryEntries = 50
	maxStashEntries   = 50
	historyFile       = "history.jsonl"
	stashFile         = "stash.jsonl"
)

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	p := filepath.Join(dir, "autoxmate")
	os.MkdirAll(p, 0755)
	return p
}

type autocompleteItem struct {
	name string
	desc string
}

func loadHistory() []historyEntry {
	return loadJSONL(filepath.Join(cacheDir(), historyFile), maxHistoryEntries)
}

func saveHistory(entries []historyEntry) {
	saveJSONL(filepath.Join(cacheDir(), historyFile), entries)
}

func loadStash() []historyEntry {
	return loadJSONL(filepath.Join(cacheDir(), stashFile), maxStashEntries)
}

func saveStash(entries []historyEntry) {
	saveJSONL(filepath.Join(cacheDir(), stashFile), entries)
}

func loadJSONL(path string, max int) []historyEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []historyEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e historyEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	if len(entries) > max {
		entries = entries[len(entries)-max:]
	}
	return entries
}

func saveJSONL(path string, entries []historyEntry) {
	var b strings.Builder
	for _, e := range entries {
		data, _ := json.Marshal(e)
		b.Write(data)
		b.WriteByte('\n')
	}
	os.WriteFile(path, []byte(b.String()), 0644)
}

func appendJSONL(path string, e historyEntry, max int) {
	entries := loadJSONL(path, max-1)
	entries = append(entries, e)
	if len(entries) > max {
		entries = entries[len(entries)-max:]
	}
	saveJSONL(path, entries)
}

func NewApp(tools []core.ToolDefinition, cache *core.Cache) *Model {
	statuses := core.DetectAllTools(tools)
	m := &Model{
		tools:            tools,
		toolsCache:       cache,
		filtered:         tools,
		statuses:         statuses,
		currentScreen:    screenLoading,
		loadingTools:     true,
		loadingStartTime: time.Now(),
		sysInfo:          collectSysInfo(tools),
		historyIdx:       -1,
		outputLines:      0,
		outputOff:        0,
		pastedAt:         -1,
		sidebarMode:      "auto",
	}

	// Load persistent history
	histEntries := loadHistory()
	for _, e := range histEntries {
		m.history = append(m.history, e.Input)
	}
	m.stash = loadStash()

	return m
}

func (m *Model) Run() error {
	fmt.Fprint(os.Stderr, "\x1b]11;#0a0a0a\x07")
	defer fmt.Fprint(os.Stderr, "\x1b]111\x07")
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	_, err := p.Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.dismissOldToasts(),
		m.spinnerTick(),
		func() tea.Msg {
			time.Sleep(600 * time.Millisecond)
			return loadingDoneMsg{}
		},
	)
}

func (m *Model) dismissOldToasts() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		return toastDismissMsg{}
	})
}

func (m *Model) spinnerTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(_ time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func (m *Model) nextSpinnerFrame() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[time.Now().UnixMilli()/120%int64(len(frames))]
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case leaderTimeoutMsg:
		m.leaderActive = false
		return m, nil

	case spinnerTickMsg:
		m.placeholderIdx = int(time.Now().UnixMilli()/3000) % len(placeholders)
		return m, m.spinnerTick()

	case loadingDoneMsg:
		m.loadingTools = false
		m.loadingComplete = true
		if m.currentScreen == screenLoading {
			m.currentScreen = screenHome
		}
		return m, nil

	case streamOutputMsg:
		m.writeOutputKind(msg.kind, msg.line, nil)
		return m, nil

	case toastDismissMsg:
		var active []toastMsg
		for _, t := range m.toasts {
			if time.Since(t.timestamp) < 4*time.Second {
				active = append(active, t)
			}
		}
		m.toasts = active
		return m, m.dismissOldToasts()

	case cmdDoneMsg:
		m.running = false
		m.runLabel = ""
		hadSuccess := false
		hadError := false
		for _, line := range msg.lines {
			m.writeOutput(line)
			if strings.HasPrefix(line, "  ✓") {
				hadSuccess = true
			}
			if strings.HasPrefix(line, "  ✗") || strings.Contains(line, "Error:") {
				hadError = true
			}
		}
		if len(msg.lines) > 0 && msg.lines[0] != "" {
			firstLine := msg.lines[0]
			statusMsg := "Command completed"
			switch {
			case strings.Contains(firstLine, "installed"):
				if hadSuccess {
					m.addToast("Install succeeded", "success")
				} else if hadError {
					m.addToast("Install failed", "error")
				}
				statusMsg = "Install completed"
			case strings.Contains(firstLine, "Running"):
				if hadError {
					m.addToast("Command failed", "error")
				} else {
					m.addToast("Command completed", "success")
				}
				statusMsg = "Run completed"
			case strings.Contains(firstLine, "Sync"):
				if hadError {
					m.addToast("Sync failed", "error")
				} else {
					m.addToast("Sync complete", "success")
				}
				statusMsg = "Sync completed"
			}
			m.writeOutput("  " + statusMsg)
		}
		m.maybeCollapseOutput()
		m.outputOff = 0
		m.statuses = core.DetectAllTools(m.tools)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	outputHeight := m.commandOutputHeight()

	switch msg.Type {
	case tea.MouseWheelUp:
		if msg.Y < outputHeight {
			m.outputOff += 3
			if m.outputOff > len(m.output) {
				m.outputOff = len(m.output)
			}
		}
		return m, nil

	case tea.MouseWheelDown:
		if msg.Y < outputHeight {
			m.outputOff -= 3
			if m.outputOff < 0 {
				m.outputOff = 0
			}
		}
		return m, nil

	case tea.MouseLeft:
		// Check scrollbar click on rightmost 2 columns
		if msg.X >= m.width-2 && msg.Y < outputHeight {
			visibleLines := outputHeight
			totalLines := len(m.output)
			if visibleLines > 0 && totalLines > 0 {
				ratio := float64(msg.Y) / float64(visibleLines)
				m.outputOff = int(float64(totalLines) * ratio)
				if m.outputOff > totalLines {
					m.outputOff = totalLines
				}
				if m.outputOff < 0 {
					m.outputOff = 0
				}
			}
			return m, nil
		}

		// Check collapsible toggle click in output area
		if msg.Y < outputHeight {
			entryIdx := m.outputIndexAtScreenRow(msg.Y)
			if entryIdx >= 0 && entryIdx < len(m.output) {
				entry := &m.output[entryIdx]
				if entry.Hidden != nil {
					entry.Hidden = nil
					return m, nil
				}
			}
			return m, nil
		}

		// Input bar click (command mode only)
		if m.currentScreen == screenCommand {
			inputY, inputLines := m.inputBarY()
			if msg.Y >= inputY && msg.Y < inputY+inputLines {
				shellMode := m.isShellMode()
				promptChar := "> "
				promptStyle := promptCharStyle
				if shellMode {
					promptChar = "$ "
					promptStyle = shellPromptStyle
				}
				promptWidth := lipgloss.Width(promptStyle.Render(promptChar + " "))

				row := msg.Y - inputY
				relX := msg.X - 1 - promptWidth // 1 for inputLineStyle left padding
				if relX < 0 {
					relX = 0
				}
				if row > 0 {
					relX = msg.X - 1 // no prompt on subsequent lines
					if relX < 0 {
						relX = 0
					}
				}

				// Find byte offset for the clicked line
				lines := strings.Split(m.input, "\n")
				lineStart := 0
				for i := 0; i < row && i < len(lines); i++ {
					lineStart += len(lines[i]) + 1
				}
				line := ""
				if row < len(lines) {
					line = lines[row]
				}

				pos := 0
				col := 0
				for col < relX && pos < len(line) {
					r, size := utf8.DecodeRuneInString(line[pos:])
					rw := runewidth.RuneWidth(r)
					if col+rw > relX {
						break
					}
					pos += size
					col += rw
				}
				m.inputCursor = lineStart + pos
				return m, nil
			}
		}

		// Sidebar overlay: click outside → dismiss, click on sidebar → select domain
		if m.sidebarOverlay() {
			if msg.X >= m.width-42 {
				sidebarY := msg.Y
				domain := m.domainAtSidebarY(sidebarY)
				if domain != "" {
					m.setDomainFilter(domain)
				}
			} else {
				m.sidebarOpen = false
			}
			return m, nil
		}

		// Wide sidebar click (horizontal layout)
		if m.sidebarVisible() && m.width > 120 && msg.X >= m.width-42 {
			sidebarY := msg.Y
			domain := m.domainAtSidebarY(sidebarY)
			if domain != "" {
				m.setDomainFilter(domain)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) inputBarY() (int, int) {
	toastLines := m.toastLineCount()
	outputHeight := m.commandOutputHeight()

	acHeight := 0
	if m.completing && len(m.completeResults) > 0 {
		n := len(m.completeResults)
		if n > 10 {
			n = 10
		}
		acHeight = 1 + n + 4 // \n separator + border top + prefix + results + footer + border bottom
	}

	inputY := toastLines + outputHeight + acHeight + 1
	return inputY, m.inputDisplayLines()
}

func (m *Model) outputIndexAtScreenRow(screenY int) int {
	n := len(m.output)
	if n == 0 {
		return -1
	}

	if m.outputOff > n {
		m.outputOff = n
	}

	// Replicate renderOutput's line calculation
	startRow := n - m.outputOff
	if startRow < 0 {
		startRow = 0
	}

	// Account for the scroll hint line at top if scrolled
	scrollHintLines := 0
	if m.outputOff > 0 {
		scrollHintLines = 1
	}

	idx := startRow + (screenY - scrollHintLines)
	if idx < 0 || idx >= n {
		return -1
	}
	return idx
}

func (m *Model) writeOutput(line string) {
	m.writeOutputKind(detectOutputKind(line), line, nil)
}

func (m *Model) writeOutputKind(kind, line string, data interface{}) {
	m.output = append(m.output, scrollbackEntry{Kind: kind, Content: line, Data: data})
	m.outputLines++
}

func (m *Model) maybeCollapseOutput() {
	const maxVisible = 20
	var result []scrollbackEntry

	i := 0
	for i < len(m.output) {
		if m.output[i].Kind == "text" || m.output[i].Kind == "code" {
			runStart := i
			for i < len(m.output) && (m.output[i].Kind == "text" || m.output[i].Kind == "code") {
				i++
			}
			runLen := i - runStart

			if runLen > maxVisible {
				entry := m.output[runStart]
				entry.Hidden = make([]scrollbackEntry, runLen-1)
				copy(entry.Hidden, m.output[runStart+1:runStart+runLen])
				result = append(result, entry)
			} else {
				result = append(result, m.output[runStart:runStart+runLen]...)
			}
		} else {
			result = append(result, m.output[i])
			i++
		}
	}
	m.output = result
}

func detectOutputKind(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "> "):
		return "prompt"
	case strings.HasPrefix(trimmed, "  ✗") || strings.Contains(line, "Error:"):
		return "error"
	case strings.HasPrefix(trimmed, "  ✓"):
		return "system"
	case strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "+++"):
		return "diff"
	case strings.HasPrefix(trimmed, "@@") && strings.Contains(trimmed, "@@"):
		return "diff"
	case strings.HasPrefix(trimmed, "```"):
		return "code"
	case strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "###"):
		return "markdown"
	case strings.Contains(trimmed, "\x1b["):
		return "text" // ANSI escapes, pass through
	default:
		return "text"
	}
}

func (m *Model) addToast(message, kind string) {
	msg := toastMsg{message: message, kind: kind, timestamp: time.Now()}
	m.toasts = append(m.toasts, msg)
	if len(m.toasts) > 3 {
		m.toasts = m.toasts[len(m.toasts)-3:]
	}
}

func (m *Model) startAutocomplete(triggerIdx int) {
	m.completing = true
	m.completeStart = triggerIdx
	m.completeQuery = ""
	m.completeResults = nil
	m.completeCursor = 0
	m.updateAutocomplete()
}

func (m *Model) updateAutocomplete() {
	if !m.completing {
		return
	}
	after := m.input[m.completeStart:]
	switch {
	case strings.HasPrefix(after, "@"):
		m.completeType = "tool"
		query := after[1:]
		results := core.SearchTools(m.tools, query)
		m.completeResults = make([]string, 0, len(results))
		for _, t := range results {
			m.completeResults = append(m.completeResults, t.Name)
		}
	case strings.HasPrefix(after, "/"):
		m.completeType = "command"
		query := after[1:]
		var cmds = []string{"help", "clear", "list", "search", "status", "install", "run", "sync", "shell", "query", "quit"}
		m.completeResults = nil
		for _, c := range cmds {
			if strings.HasPrefix(c, strings.ToLower(query)) {
				m.completeResults = append(m.completeResults, c)
			}
		}
	}
	if len(m.completeResults) > 20 {
		m.completeResults = m.completeResults[:20]
	}
	if m.completeCursor >= len(m.completeResults) {
		m.completeCursor = 0
	}
}

func (m *Model) acceptAutocomplete() {
	if len(m.completeResults) == 0 {
		m.completing = false
		return
	}
	sel := m.completeResults[m.completeCursor]
	// Replace from trigger to cursor with selected item
	prefix := m.input[:m.completeStart]
	suffix := m.input[m.inputCursor:]
	remainder := m.input[m.completeStart:m.inputCursor]
	if len(remainder) == 0 {
		// Nothing typed after trigger; close autocomplete silently
		m.completing = false
		return
	}
	// The query is the trigger char + whatever was typed
	trigger := string(remainder[0])
	if trigger == "@" {
		// Insert @ + toolname + space
		m.input = prefix + trigger + sel + " " + suffix
		m.inputCursor = len(prefix) + len(trigger) + len(sel) + 1
	} else if trigger == "/" {
		m.input = prefix + trigger + sel + " " + suffix
		m.inputCursor = len(prefix) + len(trigger) + len(sel) + 1
	}
	m.completing = false
}

func (m *Model) cancelAutocomplete() {
	m.completing = false
	m.completeResults = nil
	m.completeCursor = 0
}

func (m *Model) handleAutocompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.cancelAutocomplete()
		return m, nil
	case tea.KeyEnter:
		m.acceptAutocomplete()
		return m, nil
	case tea.KeyTab:
		m.acceptAutocomplete()
		return m, nil
	case tea.KeyUp:
		if m.completeCursor > 0 {
			m.completeCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.completeCursor < len(m.completeResults)-1 {
			m.completeCursor++
		}
		return m, nil
	case tea.KeyBackspace:
		if m.inputCursor > m.completeStart {
			m.pushUndo()
			_, size := utf8.DecodeLastRuneInString(m.input[:m.inputCursor])
			m.input = m.input[:m.inputCursor-size] + m.input[m.inputCursor:]
			m.inputCursor -= size
			m.updateAutocomplete()
		} else {
			m.cancelAutocomplete()
		}
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		m.pushUndo()
		char := msg.String()
		m.input = m.input[:m.inputCursor] + char + m.input[m.inputCursor:]
		m.inputCursor += len(char)
		m.updateAutocomplete()
		return m, nil
	}
	return m, nil
}

func (m *Model) transitionToCommand() {
	if m.currentScreen != screenHome {
		return
	}
	m.currentScreen = screenCommand

	installed := 0
	for _, s := range m.statuses {
		if s.OnPath || s.DockerImage || s.PackageManager != "" {
			installed++
		}
	}

	var initLines []scrollbackEntry
	// Logo
	logoRendered := m.renderLogo()
	for _, l := range strings.Split(logoRendered, "\n") {
		initLines = append(initLines, scrollbackEntry{Kind: "text", Content: l})
	}
	initLines = append(initLines, scrollbackEntry{Kind: "text", Content: ""})
	// System info
	sysRendered := m.renderSystemInfo()
	for _, l := range strings.Split(sysRendered, "\n") {
		initLines = append(initLines, scrollbackEntry{Kind: "text", Content: l})
	}
	initLines = append(initLines, scrollbackEntry{Kind: "text", Content: ""})
	m.output = initLines
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Sidebar overlay dismiss on Escape
	if m.sidebarOverlay() && msg.String() == "esc" {
		m.sidebarOpen = false
		return m, nil
	}

	// Overlay priority order: which-key > confirm > history search > browser > palette > autocomplete > command
	switch {
	case m.whichKeyVisible:
		return m.handleWhichKeyKey(msg)
	case m.confirmVisible:
		return m.handleConfirmKey(msg)
	case m.histSearchVisible:
		return m.handleHistSearchKey(msg)
	case m.browserVisible:
		return m.handleBrowserKey(msg)
	case m.paletteVisible:
		return m.handlePaletteKey(msg)
	}

	// Home screen: transition on any input key
	if m.currentScreen == screenHome {
		switch msg.String() {
		case "?", "ctrl+o", "ctrl+p", "ctrl+r", "ctrl+x", "ctrl+l":
			// These shortcuts work on home screen too — don't transition
		default:
			m.transitionToCommand()
		}
	}

	if m.running {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// Autocomplete mode
	if m.completing {
		return m.handleAutocompleteKey(msg)
	}

	if msg.Type == tea.KeyCtrlX {
		m.leaderActive = true
		return m, tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
			return leaderTimeoutMsg{}
		})
	}
	if m.leaderActive {
		m.leaderActive = false
		switch msg.String() {
		case "b":
			if m.sidebarVisible() {
				m.sidebarMode = "hide"
				m.sidebarOpen = false
			} else {
				m.sidebarMode = "auto"
				m.sidebarOpen = true
			}
			return m, nil
		case "p":
			m.openPalette()
			return m, nil
		}
		return m, nil
	}

	return m.handleCommandKey(msg)
}

// pushUndo saves the current input state for Ctrl+Z undo.
func (m *Model) pushUndo() {
	m.undoStack = append(m.undoStack, m.input)
	m.redoStack = nil
	if len(m.undoStack) > 100 {
		m.undoStack = m.undoStack[1:]
	}
}

// Word navigation helpers (rune-safe).
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func wordLeft(s string, pos int) int {
	if pos <= 0 {
		return 0
	}
	// skip non-word chars before the cursor
	for pos > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:pos])
		if isWordChar(r) {
			break
		}
		pos -= size
	}
	// skip word chars
	for pos > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:pos])
		if !isWordChar(r) {
			break
		}
		pos -= size
	}
	return pos
}

func wordRight(s string, pos int) int {
	n := len(s)
	if pos >= n {
		return n
	}
	// skip word chars
	for pos < n {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if !isWordChar(r) {
			break
		}
		pos += size
	}
	// skip non-word chars
	for pos < n {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if isWordChar(r) {
			break
		}
		pos += size
	}
	return pos
}

func (m *Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if msg.Alt {
			// Alt+Enter: insert newline
			m.pushUndo()
			m.input = m.input[:m.inputCursor] + "\n" + m.input[m.inputCursor:]
			m.inputCursor++
			return m, nil
		}
		return m.executeInput()

	case tea.KeyBackspace:
		if m.inputCursor > 0 && m.input != "" {
			m.pushUndo()
			_, size := utf8.DecodeLastRuneInString(m.input[:m.inputCursor])
			m.input = m.input[:m.inputCursor-size] + m.input[m.inputCursor:]
			m.inputCursor -= size
		}
		return m, nil

	case tea.KeyDelete:
		if m.inputCursor < len(m.input) {
			m.pushUndo()
			_, size := utf8.DecodeRuneInString(m.input[m.inputCursor:])
			m.input = m.input[:m.inputCursor] + m.input[m.inputCursor+size:]
		}
		return m, nil

	case tea.KeyUp:
		if len(m.history) > 0 && m.historyIdx < len(m.history)-1 {
			if m.historyIdx == -1 {
				m.historyDraft = m.input
			}
			m.historyIdx++
			m.input = m.history[m.historyIdx]
			m.inputCursor = len(m.input)
		}
		return m, nil

	case tea.KeyDown:
		if m.historyIdx >= 0 {
			m.historyIdx--
			if m.historyIdx < 0 {
				m.input = m.historyDraft
				m.historyDraft = ""
				m.inputCursor = len(m.input)
			} else {
				m.input = m.history[m.historyIdx]
				m.inputCursor = len(m.input)
			}
		}
		return m, nil

	case tea.KeyLeft:
		if m.inputCursor > 0 {
			_, size := utf8.DecodeLastRuneInString(m.input[:m.inputCursor])
			m.inputCursor -= size
		}
		return m, nil

	case tea.KeyRight:
		if m.inputCursor < len(m.input) {
			_, size := utf8.DecodeRuneInString(m.input[m.inputCursor:])
			m.inputCursor += size
		}
		return m, nil

	case tea.KeyTab:
		if m.isShellMode() {
			m.completeFilename()
			return m, nil
		}
		// Trigger @ autocomplete for tool names
		m.pushUndo()
		m.startAutocomplete(m.inputCursor)
		m.input = m.input[:m.inputCursor] + "@" + m.input[m.inputCursor:]
		m.inputCursor++
		return m, nil

	case tea.KeyHome:
		m.inputCursor = 0
		return m, nil

	case tea.KeyEnd:
		m.inputCursor = len(m.input)
		return m, nil

	case tea.KeyPgUp:
		visible := m.commandOutputHeight()
		m.outputOff += visible
		if m.outputOff > len(m.output) {
			m.outputOff = len(m.output)
		}
		return m, nil

	case tea.KeyPgDown:
		visible := m.commandOutputHeight()
		m.outputOff -= visible
		if m.outputOff < 0 {
			m.outputOff = 0
		}
		return m, nil

	case tea.KeyCtrlA:
		m.inputCursor = 0
		return m, nil

	case tea.KeyCtrlE:
		m.inputCursor = len(m.input)
		return m, nil

	case tea.KeyEscape:
		if m.completing {
			m.cancelAutocomplete()
		} else if m.input != "" {
			m.pushUndo()
			m.input = ""
			m.inputCursor = 0
		} else if m.isShellMode() {
			m.pushUndo()
			m.input = strings.TrimPrefix(m.input, "!")
			m.inputCursor = len(m.input)
		}
		return m, nil

	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyRunes, tea.KeySpace:
		char := msg.String()
		// Paste detection: more than 3 chars in a single runes event
		if len(char) > 3 {
			m.pushUndo()
			firstLine := strings.SplitN(char, "\n", 2)[0]
			lineCount := strings.Count(char, "\n") + 1
			if len(firstLine) > 25 {
				firstLine = firstLine[:25] + "…"
			}
			label := fmt.Sprintf("%s  [%d lines pasted]", firstLine, lineCount)
			m.pastedBuf = char
			m.pastedAt = m.inputCursor
			m.input = m.input[:m.inputCursor] + label + m.input[m.inputCursor:]
			m.inputCursor += len(label)
			return m, nil
		}
		m.pushUndo()
		m.input = m.input[:m.inputCursor] + char + m.input[m.inputCursor:]
		m.inputCursor += len(char)
		// Trigger autocomplete for @ mentions and / commands
		if char == "@" {
			m.startAutocomplete(m.inputCursor - 1)
		} else if char == "/" && (m.inputCursor == 1 || (m.inputCursor >= 2 && m.input[m.inputCursor-2] == ' ')) {
			m.startAutocomplete(m.inputCursor - 1)
		}
		return m, nil
	}

	switch msg.String() {
	// Word navigation
	case "ctrl+left":
		m.inputCursor = wordLeft(m.input, m.inputCursor)
		return m, nil
	case "ctrl+right":
		m.inputCursor = wordRight(m.input, m.inputCursor)
		return m, nil

	// Word deletion
	case "ctrl+w":
		if m.inputCursor > 0 {
			m.pushUndo()
			newPos := wordLeft(m.input, m.inputCursor)
			m.input = m.input[:newPos] + m.input[m.inputCursor:]
			m.inputCursor = newPos
		}
		return m, nil
	case "alt+d", "ctrl+delete":
		if m.inputCursor < len(m.input) {
			m.pushUndo()
			newPos := wordRight(m.input, m.inputCursor)
			m.input = m.input[:m.inputCursor] + m.input[newPos:]
		}
		return m, nil

	// Kill ring
	case "ctrl+k":
		if m.inputCursor < len(m.input) {
			m.pushUndo()
			m.killRing = m.input[m.inputCursor:]
			m.input = m.input[:m.inputCursor]
		}
		return m, nil
	case "ctrl+u":
		if m.inputCursor > 0 {
			m.pushUndo()
			m.killRing = m.input[:m.inputCursor]
			m.input = m.input[m.inputCursor:]
			m.inputCursor = 0
		}
		return m, nil
	case "ctrl+y":
		if m.killRing != "" {
			m.pushUndo()
			m.input = m.input[:m.inputCursor] + m.killRing + m.input[m.inputCursor:]
			m.inputCursor += len(m.killRing)
		}
		return m, nil

	// Undo / redo
	case "ctrl+z":
		if len(m.undoStack) > 0 {
			m.redoStack = append(m.redoStack, m.input)
			m.input = m.undoStack[len(m.undoStack)-1]
			m.undoStack = m.undoStack[:len(m.undoStack)-1]
			m.inputCursor = len(m.input)
		}
		return m, nil
	case "ctrl+shift+z":
		if len(m.redoStack) > 0 {
			m.undoStack = append(m.undoStack, m.input)
			m.input = m.redoStack[len(m.redoStack)-1]
			m.redoStack = m.redoStack[:len(m.redoStack)-1]
			m.inputCursor = len(m.input)
		}
		return m, nil

	case "?":
		m.whichKeyVisible = true
		return m, nil
	case "ctrl+s":
		// Stash current input
		if m.input != "" {
			e := historyEntry{Input: m.input, Timestamp: time.Now().Unix()}
			m.stash = append(m.stash, e)
			saveStash(m.stash)
			m.addToast("Input stashed", "info")
			m.input = ""
			m.inputCursor = 0
		}
		return m, nil
	case "alt+s":
		// Restore last stashed input
		if len(m.stash) > 0 {
			e := m.stash[len(m.stash)-1]
			m.stash = m.stash[:len(m.stash)-1]
			saveStash(m.stash)
			m.input = e.Input
			m.inputCursor = len(m.input)
			m.addToast("Stash restored", "success")
		}
		return m, nil
	case "ctrl+o":
		m.openBrowser()
		return m, nil
	case "ctrl+r":
		if len(m.history) > 0 {
			m.openHistSearch()
		}
		return m, nil
	case "ctrl+p":
		if !m.paletteVisible {
			m.openPalette()
		}
		return m, nil
	case "ctrl+l":
		m.output = nil
		m.outputLines = 0
		m.outputOff = 0
		return m, nil
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m *Model) handleTabComplete() {
	fields := strings.Fields(m.input)
	if len(fields) == 0 {
		return
	}
	prefix := fields[len(fields)-1]
	matches := core.SearchTools(m.tools, prefix)
	if len(matches) > 0 && strings.HasPrefix(strings.ToLower(matches[0].Name), strings.ToLower(prefix)) {
		prefix2 := ""
		for i, f := range fields[:len(fields)-1] {
			if i > 0 {
				prefix2 += " "
			}
			prefix2 += f
		}
		m.input = prefix2 + matches[0].Name + " "
		m.inputCursor = len(m.input)
	}
}

func (m *Model) isShellMode() bool {
	return strings.HasPrefix(strings.TrimSpace(m.input), "!")
}

func (m *Model) completeFilename() {
	fields := strings.Fields(m.input)
	if len(fields) == 0 {
		return
	}
	prefix := fields[len(fields)-1]
	// If prefix is empty or has no wildcards, glob with prefix*
	matches, err := filepath.Glob(prefix + "*")
	if err != nil || len(matches) == 0 {
		return
	}
	// Use first match
	match := matches[0]
	if stat, err := os.Stat(match); err == nil && stat.IsDir() {
		match += "/"
	}
	// Replace last field with match
	idx := strings.LastIndex(m.input, fields[len(fields)-1])
	if idx >= 0 {
		m.input = m.input[:idx] + match + m.input[idx+len(fields[len(fields)-1]):]
		m.inputCursor = len(m.input)
	}
}

func (m *Model) expandPasted() string {
	if m.pastedBuf != "" {
		return m.pastedBuf
	}
	return m.input
}

// --- Command Execution ---

func (m *Model) executeInput() (tea.Model, tea.Cmd) {
	// Expand pasted placeholder before processing
	if m.pastedBuf != "" && m.pastedAt >= 0 {
		marker := " lines pasted]"
		if idx := strings.Index(m.input[m.pastedAt:], marker); idx >= 0 {
			end := m.pastedAt + idx + len(marker)
			m.input = m.input[:m.pastedAt] + m.pastedBuf + m.input[end:]
			m.inputCursor = len(m.input)
		}
		m.pastedBuf = ""
		m.pastedAt = -1
	}
	input := strings.TrimSpace(m.input)
	m.input = ""
	m.inputCursor = 0

	if input == "" {
		return m, nil
	}

	// Add to history
	if len(m.history) == 0 || m.history[len(m.history)-1] != input {
		m.history = append(m.history, input)
		// Persist to disk
		appendJSONL(filepath.Join(cacheDir(), historyFile), historyEntry{
			Input:     input,
			Timestamp: time.Now().Unix(),
		}, maxHistoryEntries)
	}
	m.historyIdx = -1

	// Echo command
	m.writeOutput("> " + input)

	// Parse
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	// Shell mode: input starting with ! executes as shell command
	if strings.HasPrefix(cmd, "!") {
		shellCmd := input[1:] // remove the !
		if shellCmd == "" {
			m.writeOutput("  Usage: !<command>")
			return m, nil
		}
		return m, m.cmdShell(shellCmd)
	}

	switch cmd {
	case "help", "?":
		m.cmdHelp()
		return m, nil

	case "list", "ls":
		m.cmdList(args)
		return m, nil

	case "search":
		m.cmdSearch(args)
		return m, nil

	case "status":
		m.cmdStatus(args)
		return m, nil

	case "install":
		if len(args) == 0 {
			m.writeOutput("  Usage: install <tool>")
			return m, nil
		}
		tool := core.FindToolByName(m.tools, args[0])
		if tool == nil {
			m.writeOutput(fmt.Sprintf("  Tool not found: %s", args[0]))
			return m, nil
		}
		// Check if install might need sudo
		needsSudo := false
		for _, inst := range tool.Install {
			if strings.Contains(strings.Join(inst.Commands, " "), "sudo") {
				needsSudo = true
				break
			}
		}
		if needsSudo {
			m.confirmVisible = true
			m.confirmMessage = fmt.Sprintf("Install %s with sudo?", tool.Name)
			m.confirmYesLabel = "y: install"
			m.confirmNoLabel = "n: cancel"
			m.pendingCmd = func(confirmed bool) tea.Cmd {
				if confirmed {
					return m.cmdInstall(args[0])
				}
				m.writeOutput("  Install cancelled")
				return nil
			}
			return m, nil
		}
		return m, m.cmdInstall(args[0])

	case "run":
		if len(args) == 0 {
			m.writeOutput("  Usage: run <tool> [args...]")
			return m, nil
		}
		return m, m.cmdRun(args)

	case "sync":
		return m, m.cmdSync()

	case "shell", "!":
		if len(args) == 0 {
			m.writeOutput("  Usage: shell <command> or !<command>")
			return m, nil
		}
		return m, m.cmdShell(strings.Join(args, " "))

	case "clear", "cls":
		m.output = nil
		m.outputLines = 0
		m.outputOff = 0
		return m, nil

	case "quit", "exit":
		return m, tea.Quit

	case "query", "q":
		return m, m.cmdQuery(strings.Join(args, " "))

	default:
		// Auto-detect structured queries (contain ( and ))
		if strings.Contains(strings.Join(args, " "), "(") && strings.Contains(input, ")") {
			return m, m.cmdQuery(input)
		}
		// Unknown command — show help
		m.writeOutput(fmt.Sprintf("  Unknown command: %s", cmd))
		m.writeOutput("  Type 'help' for available commands")
		return m, nil
	}
}

func (m *Model) cmdHelp() {
	m.writeOutput("")
	m.writeOutput("  AutoMate v" + appVersion + " — Command Reference")
	m.writeOutput("  " + strings.Repeat("─", 50))
	m.writeOutput("")
	m.writeOutput("  list [domain]      List tools (optionally by domain)")
	m.writeOutput("  search <query>     Search tools by name/description")
	m.writeOutput("  status [tool]      Show install status")
	m.writeOutput("  install <tool>     Install a tool")
	m.writeOutput("  run <tool> [args]  Execute a tool with arguments")
	m.writeOutput("  sync               Refresh tools from the site")
	m.writeOutput("  shell <cmd>        Run a shell command")
	m.writeOutput("  query <q>          Run a structured/free-text query")
	m.writeOutput("  clear              Clear the output area")
	m.writeOutput("  help               Show this help")
	m.writeOutput("  quit               Exit AutoXMate")
	m.writeOutput("")
	m.writeOutput("  Keybindings:")
	m.writeOutput("    ↑/↓            Command history")
	m.writeOutput("    PgUp/PgDn      Scroll output")
	m.writeOutput("    Tab            Autocomplete tool name")
	m.writeOutput("    Ctrl+P         Command palette (fuzzy tool search)")
	m.writeOutput("    Ctrl+X B       Toggle domain sidebar")
	m.writeOutput("    Ctrl+L         Clear output")
	m.writeOutput("    q, Ctrl+C      Quit")
	m.writeOutput("")
}

func (m *Model) cmdList(args []string) {
	var tools []core.ToolDefinition
	if len(args) > 0 {
		domain := args[0]
		tools = core.FilterByDomain(m.tools, domain)
		if len(tools) == 0 {
			m.writeOutput(fmt.Sprintf("  No tools found in domain '%s'", domain))
			return
		}
		m.writeOutput(fmt.Sprintf("  Tools in %s (%d):", domain, len(tools)))
	} else {
		tools = m.tools
		installed := 0
		for _, s := range m.statuses {
			if s.OnPath || s.DockerImage || s.PackageManager != "" {
				installed++
			}
		}
		m.writeOutput(fmt.Sprintf("  All tools (%d total, %d installed):", len(tools), installed))
	}
	m.writeOutput("")

	for _, t := range tools {
		var statusStr string
		if s, ok := m.statuses[t.ID]; ok {
			if s.OnPath {
				statusStr = "✓"
			} else if s.PackageManager != "" || s.DockerImage {
				statusStr = "~"
			} else {
				statusStr = "✗"
			}
		} else {
			statusStr = " "
		}
		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		m.writeOutput(fmt.Sprintf("  %s %-25s %s", statusStr, t.Name, desc))
	}
}

func (m *Model) cmdSearch(args []string) {
	if len(args) == 0 {
		m.writeOutput("  Usage: search <query>")
		return
	}
	query := strings.Join(args, " ")
	results := core.SearchTools(m.tools, query)
	if len(results) == 0 {
		m.writeOutput(fmt.Sprintf("  No tools matching '%s'", query))
		return
	}
	m.writeOutput(fmt.Sprintf("  %d tools matching '%s':", len(results), query))
	for _, t := range results {
		var statusStr string
		if s, ok := m.statuses[t.ID]; ok {
			if s.OnPath {
				statusStr = "✓"
			} else if s.PackageManager != "" || s.DockerImage {
				statusStr = "~"
			}
		}
		if statusStr == "" {
			statusStr = " "
		}
		m.writeOutput(fmt.Sprintf("  %s %s  (%s)", statusStr, t.Name, t.Description))
	}
}

func (m *Model) cmdQuery(input string) tea.Cmd {
	if input == "" {
		m.writeOutput("  Usage: query <structured-query>")
		m.writeOutput("  Examples:")
		m.writeOutput(`    query "domain(network):action(port-scan):tool(nmap):flags(-sV -sC):target(10.0.0.0/24)"`)
		m.writeOutput(`    query "domain(web):action(sql-injection):target(http://example.com)"`)
		m.writeOutput(`    query "scan ports 10.10.10.1 with nmap"`)
		return nil
	}

	// Try to load queries index from cache
	var queriesIndex *core.QueriesIndex
	if cache, err := core.OpenCache(); err == nil {
		queriesIndex, _ = cache.LoadQueriesIndex()
		cache.Close()
	}

	// Parse query
	var filter core.QueryFilter
	if strings.Contains(input, "(") && strings.Contains(input, ")") {
		filter = core.ParseQuery(input)
	} else {
		filter = core.ParseFreeText(input, queriesIndex, m.tools)
	}

	results := core.ExecuteQuery(filter, m.tools, queriesIndex)

	if len(results) == 0 {
		m.writeOutput(fmt.Sprintf("  No matches for query"))
		return nil
	}

	result := results[0]
	confPct := int(result.Confidence * 100)

	m.writeOutput(fmt.Sprintf("  ┌─ %s (%s) ── [%d%%] ──┐", result.ToolName, result.Namespace, confPct))
	m.writeOutput(fmt.Sprintf("  │ %s", result.Command))
	m.writeOutput("  │")
	if result.Explanation != "" {
		m.writeOutput(fmt.Sprintf("  │ %s", result.Explanation))
		m.writeOutput("  │")
	}
	if result.Phase != "" {
		phaseInfo := fmt.Sprintf("  │ Phase: %s", result.Phase)
		if result.RiskLevel != "" {
			phaseInfo += fmt.Sprintf(" | Risk: %s", result.RiskLevel)
		}
		m.writeOutput(phaseInfo)
		m.writeOutput("  │")
	}
	m.writeOutput(fmt.Sprintf("  │ %s", result.Description))
	m.writeOutput("  └" + strings.Repeat("─", 50) + "┘")

	// Show a hint about how to run
	if filter.RawFlags != "" {
		m.writeOutput(fmt.Sprintf("  Type 'run %s %s' to execute", result.ToolName, filter.RawFlags))
	} else {
		m.writeOutput(fmt.Sprintf("  Type 'run %s' to execute", result.ToolName))
	}
	return nil
}

func (m *Model) cmdStatus(args []string) {
	if len(args) > 0 {
		name := args[0]
		tool := core.FindToolByName(m.tools, name)
		if tool == nil {
			m.writeOutput(fmt.Sprintf("  Tool not found: %s", name))
			return
		}
		status := core.DetectTool(*tool)
		m.statuses[tool.ID] = status
		m.writeOutput(fmt.Sprintf("  %s:", tool.Name))
		if status.OnPath {
			m.writeOutput(fmt.Sprintf("    ✓ Installed at %s", status.PathLocation))
			if status.Version != "" {
				m.writeOutput(fmt.Sprintf("    Version: %s", status.Version))
			}
		} else if status.PackageManager != "" {
			m.writeOutput(fmt.Sprintf("    ~ Available via %s (%s)", status.PackageManager, status.Version))
		} else if status.DockerImage {
			m.writeOutput("    ~ Docker image available")
		} else {
			m.writeOutput("    ✗ Not installed")
			if len(tool.Install) > 0 {
				methods := make([]string, len(tool.Install))
				for i, inst := range tool.Install {
					methods[i] = inst.Method
				}
				m.writeOutput(fmt.Sprintf("    Install via: %s", strings.Join(methods, ", ")))
			}
		}
	} else {
		installed := 0
		for _, s := range m.statuses {
			if s.OnPath || s.DockerImage || s.PackageManager != "" {
				installed++
			}
		}
		m.writeOutput(fmt.Sprintf("  %d of %d tools installed", installed, len(m.tools)))
		m.writeOutput("")
		for _, t := range m.tools {
			s := m.statuses[t.ID]
			if s.OnPath {
				m.writeOutput(fmt.Sprintf("  ✓ %-30s %s", t.Name, s.PathLocation))
			}
		}
		for _, t := range m.tools {
			s := m.statuses[t.ID]
			if !s.OnPath && !s.DockerImage && s.PackageManager == "" {
				m.writeOutput(fmt.Sprintf("  ✗ %-30s %s", t.Name, t.Description))
			}
		}
	}
}

func (m *Model) streamCommand(run func(io.Writer) error, successLabel string) tea.Cmd {
	if m.program == nil {
		var buf bytes.Buffer
		err := run(&buf)
		output := strings.TrimRight(buf.String(), "\n")
		var lines []string
		if output != "" {
			lines = append(lines, output)
		}
		if err != nil {
			lines = append(lines, fmt.Sprintf("  ✗ Command failed: %v", err))
		} else {
			lines = append(lines, successLabel)
		}
		return func() tea.Msg { return cmdDoneMsg{lines: lines} }
	}

	pr, pw := io.Pipe()
	var buf bytes.Buffer
	w := io.MultiWriter(pw, &buf)

	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			kind := detectOutputKind(line)
			m.program.Send(streamOutputMsg{line: line, kind: kind})
		}
	}()

	return func() tea.Msg {
		err := run(w)
		pw.Close()
		time.Sleep(50 * time.Millisecond)

		output := strings.TrimRight(buf.String(), "\n")
		var lines []string
		if output != "" {
			for _, line := range strings.Split(output, "\n") {
				lines = append(lines, "  "+line)
			}
		}
		if err != nil {
			lines = append(lines, fmt.Sprintf("  ✗ Command failed: %v", err))
		} else {
			lines = append(lines, successLabel)
		}
		return cmdDoneMsg{lines: lines}
	}
}

func (m *Model) cmdInstall(name string) tea.Cmd {
	tool := core.FindToolByName(m.tools, name)
	if tool == nil {
		m.writeOutput(fmt.Sprintf("  Tool not found: %s", name))
		return nil
	}
	if len(tool.Install) == 0 {
		m.writeOutput(fmt.Sprintf("  No install methods defined for %s", name))
		return nil
	}

	m.running = true
	m.runLabel = "installing " + name

	return m.streamCommand(func(w io.Writer) error {
		opts := core.InstallOptions{Output: w}
		return core.InstallTool(*tool, opts)
	}, fmt.Sprintf("  ✓ %s installed successfully", name))
}

func (m *Model) cmdRun(args []string) tea.Cmd {
	name := args[0]
	rawArgs := ""
	if len(args) > 1 {
		rawArgs = strings.Join(args[1:], " ")
	}

	tool := core.FindToolByName(m.tools, name)
	if tool == nil {
		m.writeOutput(fmt.Sprintf("  Tool not found: %s", name))
		return nil
	}

	m.running = true
	m.runLabel = "running " + name

	return func() tea.Msg {
		var params map[string]string
		if rawArgs != "" {
			params = make(map[string]string)
			for _, p := range tool.Parameters {
				key := p.TemplateKey
				if key == "" {
					key = p.Name
				}
				params[key] = rawArgs
				break
			}
		}

		opts := core.ExecOptions{Quiet: true}
		output, err := core.ExecuteTool(*tool, params, opts)

		var lines []string
		lines = append(lines, "  Running: "+name+" "+rawArgs)
		if output != "" {
			for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
				lines = append(lines, "  "+line)
			}
		}
		if err != nil {
			lines = append(lines, fmt.Sprintf("  Error: %v", err))
		}
		return cmdDoneMsg{lines: lines}
	}
}

func (m *Model) cmdSync() tea.Cmd {
	m.running = true
	m.runLabel = "syncing tools"

	return func() tea.Msg {
		lines := []string{"  Syncing tools from site API..."}

		cache, err := core.OpenCache()
		if err != nil {
			lines = append(lines, fmt.Sprintf("  Error opening cache: %v", err))
			return cmdDoneMsg{lines: lines}
		}
		defer cache.Close()

		client := site.NewClient("")
		count, err := site.Sync(client, cache)
		if err != nil {
			lines = append(lines, fmt.Sprintf("  Sync failed: %v", err))
			return cmdDoneMsg{lines: lines}
		}
		lines = append(lines, fmt.Sprintf("  ✓ Synced %d tools", count))

		// Reload tools from cache
		tools, err := cache.LoadTools()
		if err == nil && len(tools) > 0 {
			m.tools = tools
			m.filtered = tools
			m.statuses = core.DetectAllTools(tools)
		}
		lines = append(lines, fmt.Sprintf("  Using %d cached tools", len(m.tools)))

		return cmdDoneMsg{lines: lines}
	}
}

func (m *Model) cmdShell(command string) tea.Cmd {
	m.running = true
	m.runLabel = "running shell command"

	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", command)
		output, err := cmd.CombinedOutput()

		var lines []string
		lines = append(lines, "  $ "+command)
		if len(output) > 0 {
			for _, line := range strings.Split(strings.TrimRight(string(output), "\n"), "\n") {
				lines = append(lines, "  "+line)
			}
		}
		if err != nil {
			lines = append(lines, fmt.Sprintf("  Exit: %v", err))
		}
		return cmdDoneMsg{lines: lines}
	}
}

// --- Interactive Overlay Handlers ---

func (m *Model) openBrowser() {
	m.browserVisible = true
	m.browserQuery = ""
	m.browserResults = m.tools
	m.browserCursor = 0
}

func (m *Model) closeBrowser() {
	m.browserVisible = false
	m.browserQuery = ""
	m.browserResults = nil
	m.browserCursor = 0
}

func (m *Model) filterBrowser() {
	query := strings.ToLower(m.browserQuery)
	if query == "" {
		m.browserResults = m.tools
	} else {
		sources := make([]string, len(m.tools))
		for i, t := range m.tools {
			sources[i] = strings.ToLower(t.Name + " " + t.Description + " " + t.Namespace)
		}
		matches := fuzzy.Find(query, sources)
		m.browserResults = make([]core.ToolDefinition, 0, len(matches))
		for _, match := range matches {
			m.browserResults = append(m.browserResults, m.tools[match.Index])
		}
	}
	if m.browserCursor >= len(m.browserResults) {
		m.browserCursor = 0
	}
}

func (m *Model) handleBrowserKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.closeBrowser()
		return m, nil
	case "enter":
		if len(m.browserResults) > 0 && m.browserCursor < len(m.browserResults) {
			tool := m.browserResults[m.browserCursor]
			m.closeBrowser()
			m.showToolDetail(tool)
		}
		return m, nil
	case "i":
		if len(m.browserResults) > 0 && m.browserCursor < len(m.browserResults) {
			tool := m.browserResults[m.browserCursor]
			m.closeBrowser()
			return m, m.cmdInstall(tool.Name)
		}
		return m, nil
	case "r":
		if len(m.browserResults) > 0 && m.browserCursor < len(m.browserResults) {
			tool := m.browserResults[m.browserCursor]
			m.closeBrowser()
			m.input = "run " + tool.Name + " "
			m.inputCursor = len(m.input)
		}
		return m, nil
	case "up", "k":
		if m.browserCursor > 0 {
			m.browserCursor--
		}
		return m, nil
	case "down", "j":
		if m.browserCursor < len(m.browserResults)-1 {
			m.browserCursor++
		}
		return m, nil
	case "pgup":
		m.browserCursor -= 10
		if m.browserCursor < 0 {
			m.browserCursor = 0
		}
		return m, nil
	case "pgdown":
		m.browserCursor += 10
		if m.browserCursor >= len(m.browserResults) {
			m.browserCursor = len(m.browserResults) - 1
		}
		return m, nil
	case "home":
		m.browserCursor = 0
		return m, nil
	case "end":
		m.browserCursor = len(m.browserResults) - 1
		return m, nil
	case "backspace":
		if len(m.browserQuery) > 0 {
			m.browserQuery = m.browserQuery[:len(m.browserQuery)-1]
			m.filterBrowser()
		}
		return m, nil
	case "tab":
		// Switch to install mode on selected tool
		if len(m.browserResults) > 0 && m.browserCursor < len(m.browserResults) {
			tool := m.browserResults[m.browserCursor]
			m.closeBrowser()
			m.input = "install " + tool.Name
			m.inputCursor = len(m.input)
		}
		return m, nil
	default:
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			m.browserQuery += msg.String()
			m.filterBrowser()
		}
		return m, nil
	}
}

func (m *Model) showToolDetail(tool core.ToolDefinition) {
	m.writeOutput(fmt.Sprintf("  ── %s ──", tool.Name))
	m.writeOutput(fmt.Sprintf("  ID: %s", tool.ID))
	m.writeOutput(fmt.Sprintf("  Domain: %s", tool.Namespace))
	m.writeOutput(fmt.Sprintf("  Risk: %s  Trust: %s", tool.RiskLevel, tool.TrustLevel))
	m.writeOutput(fmt.Sprintf("  %s", tool.Description))
	if len(tool.Capabilities) > 0 {
		m.writeOutput(fmt.Sprintf("  Capabilities: %s", strings.Join(tool.Capabilities, ", ")))
	}
	if len(tool.Install) > 0 {
		methods := make([]string, len(tool.Install))
		for i, inst := range tool.Install {
			methods[i] = inst.Method
		}
		m.writeOutput(fmt.Sprintf("  Install: %s", strings.Join(methods, ", ")))
	}
	if tool.Execution.Template != "" {
		m.writeOutput(fmt.Sprintf("  Template: %s", tool.Execution.Template))
	}
	if s, ok := m.statuses[tool.ID]; ok {
		if s.OnPath {
			m.writeOutput(fmt.Sprintf("  Status: ✓ Installed at %s", s.PathLocation))
		} else if s.PackageManager != "" {
			m.writeOutput(fmt.Sprintf("  Status: ~ Available via %s", s.PackageManager))
		} else {
			m.writeOutput("  Status: ✗ Not installed")
		}
	}
	m.writeOutput(fmt.Sprintf("  Try: install %s | run %s {args}", tool.Name, tool.Name))
}

// --- Which-Key ---

func (m *Model) handleWhichKeyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.whichKeyVisible = false
	return m, nil
}

// --- Confirm Dialog ---

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.confirmVisible = false
		if m.pendingCmd != nil {
			return m, m.pendingCmd(true)
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c":
		m.confirmVisible = false
		if m.pendingCmd != nil {
			m.pendingCmd(false)
			m.pendingCmd = nil
		}
		return m, nil
	}
	return m, nil
}

// --- History Search ---

func (m *Model) openHistSearch() {
	m.histSearchVisible = true
	m.histSearchQuery = ""
	m.histSearchResults = nil
	m.histSearchCursor = 0
	m.filterHistSearch()
}

func (m *Model) closeHistSearch() {
	m.histSearchVisible = false
	m.histSearchQuery = ""
	m.histSearchResults = nil
	m.histSearchCursor = 0
}

func (m *Model) filterHistSearch() {
	query := strings.ToLower(m.histSearchQuery)
	m.histSearchResults = nil
	for i := len(m.history) - 1; i >= 0; i-- {
		if query == "" || strings.Contains(strings.ToLower(m.history[i]), query) {
			m.histSearchResults = append(m.histSearchResults, i)
		}
	}
	m.histSearchCursor = 0
}

func (m *Model) handleHistSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.closeHistSearch()
		return m, nil
	case "enter":
		if len(m.histSearchResults) > 0 && m.histSearchCursor < len(m.histSearchResults) {
			idx := m.histSearchResults[m.histSearchCursor]
			m.input = m.history[idx]
			m.inputCursor = len(m.input)
			m.closeHistSearch()
		}
		return m, nil
	case "up", "k":
		if m.histSearchCursor > 0 {
			m.histSearchCursor--
		}
		return m, nil
	case "down", "j":
		if m.histSearchCursor < len(m.histSearchResults)-1 {
			m.histSearchCursor++
		}
		return m, nil
	case "backspace":
		if len(m.histSearchQuery) > 0 {
			m.histSearchQuery = m.histSearchQuery[:len(m.histSearchQuery)-1]
			m.filterHistSearch()
		}
		return m, nil
	default:
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			m.histSearchQuery += msg.String()
			m.filterHistSearch()
		}
		return m, nil
	}
}

func (m *Model) openPalette() {
	m.paletteVisible = true
	m.paletteQuery = ""
	m.paletteResults = m.tools
	m.paletteCursor = 0
}

func (m *Model) closePalette() {
	m.paletteVisible = false
	m.paletteQuery = ""
	m.paletteResults = nil
	m.paletteCursor = 0
}

func (m *Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.closePalette()
		return m, nil
	case "enter":
		if len(m.paletteResults) > 0 && m.paletteCursor < len(m.paletteResults) {
			tool := m.paletteResults[m.paletteCursor]
			m.closePalette()
			m.writeOutput(fmt.Sprintf("  ── %s ──", tool.Name))
			m.writeOutput(fmt.Sprintf("  ID: %s", tool.ID))
			m.writeOutput(fmt.Sprintf("  Domain: %s", tool.Namespace))
			m.writeOutput(fmt.Sprintf("  Risk: %s  Trust: %s", tool.RiskLevel, tool.TrustLevel))
			m.writeOutput(fmt.Sprintf("  %s", tool.Description))
			if len(tool.Capabilities) > 0 {
				m.writeOutput(fmt.Sprintf("  Capabilities: %s", strings.Join(tool.Capabilities, ", ")))
			}
			if len(tool.Install) > 0 {
				methods := make([]string, len(tool.Install))
				for i, inst := range tool.Install {
					methods[i] = inst.Method
				}
				m.writeOutput(fmt.Sprintf("  Install: %s", strings.Join(methods, ", ")))
			}
			if tool.Execution.Template != "" {
				m.writeOutput(fmt.Sprintf("  Template: %s", tool.Execution.Template))
			}
			if s, ok := m.statuses[tool.ID]; ok {
				if s.OnPath {
					m.writeOutput(fmt.Sprintf("  Status: ✓ Installed at %s", s.PathLocation))
				} else if s.PackageManager != "" {
					m.writeOutput(fmt.Sprintf("  Status: ~ Available via %s", s.PackageManager))
				} else {
					m.writeOutput("  Status: ✗ Not installed")
				}
			}
			m.writeOutput(fmt.Sprintf("  Try: install %s | run %s {args}", tool.Name, tool.Name))
		}
		return m, nil
	case "up", "k":
		if m.paletteCursor > 0 {
			m.paletteCursor--
		}
		return m, nil
	case "down", "j":
		if m.paletteCursor < len(m.paletteResults)-1 {
			m.paletteCursor++
		}
		return m, nil
	case "backspace":
		if len(m.paletteQuery) > 0 {
			m.paletteQuery = m.paletteQuery[:len(m.paletteQuery)-1]
			m.filterPalette()
		}
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.paletteQuery += msg.String()
			m.filterPalette()
		}
		return m, nil
	}
}

func (m *Model) filterPalette() {
	m.paletteResults = core.SearchTools(m.tools, m.paletteQuery)
	m.paletteCursor = 0
}

// --- Helpers ---

func (m *Model) commandOutputHeight() int {
	reserved := m.inputDisplayLines() + 3 // 1 (\n before input) + 1 (\n before status) + 1 (status)

	if m.completing && len(m.completeResults) > 0 {
		acLines := len(m.completeResults)
		if acLines > 10 {
			acLines = 10
		}
		reserved += acLines + 4 + 1 // box borders (2) + prefix + results + footer + 1 (\n before autocomplete)
	}

	reserved += m.toastLineCount()

	h := m.height - reserved
	if h < 5 {
		h = 5
	}
	return h
}

func (m *Model) inputDisplayLines() int {
	if m.running || !strings.Contains(m.input, "\n") {
		return 1
	}
	lines := strings.Count(m.input, "\n") + 1
	maxH := 6
	if maxFromHeight := m.height / 3; maxFromHeight > maxH {
		maxH = maxFromHeight
	}
	if lines > maxH {
		return maxH
	}
	return lines
}

func (m *Model) toastLineCount() int {
	if len(m.toasts) == 0 {
		return 0
	}
	count := 0
	for _, t := range m.toasts {
		if time.Since(t.timestamp) <= 4*time.Second {
			count++
		}
	}
	return count
}

func centerBlock(block string, totalWidth int) string {
	lines := strings.Split(block, "\n")
	maxW := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}
	if maxW >= totalWidth {
		return block
	}
	leftPad := strings.Repeat(" ", (totalWidth-maxW)/2)
	var out []string
	for _, line := range lines {
		out = append(out, leftPad+line)
	}
	return strings.Join(out, "\n")
}

// --- Home Screen Methods ---

// logoSplits defines the column boundary between AUTO (left, muted) and
// MATE (right, bright) for each logo line (9 lines: index 0 is empty,
// indices 1-8 are the 8 logo rows). These are hardcoded because the
// logo is fixed artwork and no single algorithm can handle all lines.
var logoSplits = []int{0, 35, 35, 35, 35, 35, 35}

func (m *Model) renderLogo() string {
	lines := strings.Split(strings.TrimRight(logo, "\n"), "\n")
	var result []string

	for row, line := range lines {
		runes := []rune(line)
		if len(runes) == 0 {
			result = append(result, "")
			continue
		}
		var lineBuf strings.Builder

		for col, ch := range runes {
			if ch == ' ' {
				lineBuf.WriteRune(ch)
				continue
			}

			var styled string
			if col >= logoSplits[row] {
				styled = logoRightStyle.Render(string(ch))
			} else {
				styled = logoLeftStyle.Render(string(ch))
			}
			lineBuf.WriteString(styled)
		}
		result = append(result, lineBuf.String())
	}
	return strings.Join(result, "\n")
}

func (m *Model) renderSystemInfo() string {
	installed := 0
	for _, s := range m.statuses {
		if s.OnPath || s.DockerImage || s.PackageManager != "" {
			installed++
		}
	}

	rows := []struct{ label, val string }{
		{"Hostname", m.sysInfo.hostname},
		{"OS", m.sysInfo.os},
		{"Kernel", m.sysInfo.kernel},
		{"Arch", m.sysInfo.arch},
		{"CPU", fmt.Sprintf("%d cores", m.sysInfo.cpuCores)},
		{"Memory", m.sysInfo.memory},
		{"Uptime", m.sysInfo.uptime},
		{"Tools", fmt.Sprintf("%d loaded", m.sysInfo.toolCount)},
		{"Installed", fmt.Sprintf("%d/%d", installed, m.sysInfo.toolCount)},
		{"Version", "v" + m.sysInfo.buildVer},
	}

	var body strings.Builder
	body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colCyan)).Render(" System"))
	body.WriteString("\n\n")

	for _, r := range rows {
		body.WriteString(fmt.Sprintf("  %s%s\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color(colTextMuted)).Render(fmt.Sprintf("%-11s", r.label)),
			lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Render(r.val),
		))
	}

	return sysBoxStyle.Render(body.String())
}

var placeholders = []string{
	"Ask anything... \"install nmap\"",
	"Ask anything... \"search web\"",
	"Ask anything... \"run nikto\"",
	"Ask anything... \"list tools\"",
	"Ask anything... \"shell ls -la\"",
	"Ask anything... \"status --all\"",
}

func promptMaxWidth(totalWidth int) int {
	w := int(float64(totalWidth) * 0.7)
	if w < 60 {
		w = 60
	}
	return w
}

func (m *Model) homeScreenView() string {
	toast := m.toastView()

	logoRendered := m.renderLogo()
	sysInfoRendered := m.renderSystemInfo()
	hints := homeHintStyle.Render("  Type ? for help")

	// Build content block (logo + sysinfo + hints)
	var contentBuf strings.Builder
	contentBuf.WriteString(logoRendered)
	contentBuf.WriteString("\n\n")
	contentBuf.WriteString(sysInfoRendered)
	contentBuf.WriteString("\n\n")
	contentBuf.WriteString(hints)
	contentBuf.WriteString("\n")

	content := contentBuf.String()
	contentLines := strings.Count(content, "\n") + 1

	// Input centered at 70% width
	rawInput := m.inputView()
	pw := promptMaxWidth(m.width)
	inputH := m.inputDisplayLines()
	centeredInput := lipgloss.Place(m.width, inputH, lipgloss.Center, lipgloss.Top,
		lipgloss.NewStyle().MaxWidth(pw).Render(rawInput))

	status := m.statusBarView()

	toastLines := 0
	if toast != "" {
		toastLines = strings.Count(toast, "\n")
	}

	available := m.height - toastLines - inputH - 2 // 2 = 1 (\n before status) + 1 (status)
	if available < 0 {
		available = 0
	}
	topPad := (available - contentLines) / 2
	if topPad < 0 {
		topPad = 0
	}
	if available > 0 && topPad > available-1 {
		topPad = available - 1
	}

	// Store offset for mouse handler to map Y to logo row
	m.homeContentOffset = toastLines + topPad

	// Horizontally center the content block via plain space-padding
	centeredContent := centerBlock(content, m.width)

	var result strings.Builder
	if toast != "" {
		result.WriteString(toast)
	}
	result.WriteString(strings.Repeat("\n", topPad))
	result.WriteString(centeredContent)
	result.WriteString("\n")
	result.WriteString(centeredInput)
	result.WriteString("\n")
	result.WriteString(status)
	return result.String()
}

func (m *Model) loadingView() string {
	spinner := "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	frame := int(time.Since(m.loadingStartTime).Milliseconds() / 120 % 10)
	spinChar := string(spinner[frame])

	content := loadingStyle.Render(fmt.Sprintf(" %s Loading tools... ", spinChar))
	h := m.height
	if h < 3 {
		h = 3
	}
	topPad := (h - 1) / 2
	if topPad < 1 {
		topPad = 1
	}
	padding := strings.Repeat("\n", topPad)
	return padding + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(content)
}

func (m *Model) sidebarVisible() bool {
	if m.sidebarOpen {
		return true
	}
	if m.sidebarMode == "auto" && m.width > 120 {
		return true
	}
	return false
}

func (m *Model) sidebarOverlay() bool {
	return m.sidebarOpen && m.width <= 120
}

// --- View ---

func (m *Model) View() string {
	// Overlays take priority, shown centered over everything
	switch {
	case m.whichKeyVisible:
		return "\n\n" + m.whichKeyView()
	case m.confirmVisible:
		return "\n\n" + m.confirmView()
	case m.histSearchVisible:
		return "\n\n" + m.histSearchView()
	case m.browserVisible:
		return "\n\n" + m.browserView()
	case m.paletteVisible:
		return "\n\n" + m.paletteView()
	}

	switch m.currentScreen {
	case screenLoading:
		return m.loadingView()
	case screenHome:
		return m.homeScreenView()
	}

	const totalSidebarWidth = 42

	if m.sidebarVisible() {
		if m.width > 120 {
			mainWidth := m.width - totalSidebarWidth
			if mainWidth < 40 {
				mainWidth = 40
			}
			mainContent := lipgloss.NewStyle().Width(mainWidth).Render(m.commandView())
			sideContent := m.sidebarView()
			return lipgloss.JoinHorizontal(lipgloss.Top, mainContent, sideContent)
		}
		// Narrow terminal: overlay mode
		return m.overlaySidebarView()
	}

	return m.commandView()
}

func (m *Model) baseView() string { return "" }
func (m *Model) buildDetail()     {}

func (m *Model) setDomainFilter(domain string) {
	m.selectedDomain = domain
	m.filtered = core.FilterByDomain(m.tools, domain)
	m.cursor = 0
}

func (m *Model) domainAtSidebarY(y int) string {
	// Replicate sidebarView line layout:
	// Line 0: header " AutoMate "
	domains := core.Domains(m.tools)

	line := 1 // after header
	for _, d := range domains {
		domainTools := core.FilterByDomain(m.tools, d)
		sectionLines := 1 // domain name line
		showTools := len(domainTools)
		if showTools > 5 {
			showTools = 5
		}
		sectionLines += showTools // tool lines
		if len(domainTools) > 5 {
			sectionLines++ // "… N more" line
		}

		if y >= line && y < line+sectionLines {
			return d
		}
		line += sectionLines
	}
	return ""
}

func collectSysInfo(tools []core.ToolDefinition) sysInfo {
	info := sysInfo{
		arch:      runtime.GOARCH,
		cpuCores:  runtime.NumCPU(),
		toolCount: len(tools),
		buildVer:  appVersion,
		timeStr:   time.Now().Local().Format("Mon Jan 2 15:04:05 MST 2006"),
	}

	host, err := os.Hostname()
	if err == nil {
		info.hostname = host
	} else {
		info.hostname = "unknown"
	}

	switch runtime.GOOS {
	case "linux":
		info.os = "Linux"
		kernel, _ := os.ReadFile("/proc/sys/kernel/ostype")
		ver, _ := os.ReadFile("/proc/sys/kernel/osrelease")
		if len(kernel) > 0 && len(ver) > 0 {
			info.kernel = strings.TrimSpace(string(kernel)) + " " + strings.TrimSpace(string(ver))
		} else {
			if out, err := exec.Command("uname", "-sr").Output(); err == nil {
				info.kernel = strings.TrimSpace(string(out))
			}
		}
		mem, _ := os.ReadFile("/proc/meminfo")
		if len(mem) > 0 {
			for _, line := range strings.Split(string(mem), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						if kb, err := strconv.Atoi(parts[1]); err == nil {
							mb := kb / 1024
							if mb > 1024 {
								info.memory = fmt.Sprintf("%.1f GB", float64(mb)/1024)
							} else {
								info.memory = fmt.Sprintf("%d MB", mb)
							}
						}
					}
					break
				}
			}
		}
		uptimeBytes, _ := os.ReadFile("/proc/uptime")
		if len(uptimeBytes) > 0 {
			parts := strings.Fields(string(uptimeBytes))
			if len(parts) > 0 {
				if secs, err := strconv.ParseFloat(parts[0], 64); err == nil {
					d := time.Duration(secs) * time.Second
					days := int(d.Hours()) / 24
					hrs := int(d.Hours()) % 24
					mins := int(d.Minutes()) % 60
					if days > 0 {
						info.uptime = fmt.Sprintf("%dd %dh %dm", days, hrs, mins)
					} else if hrs > 0 {
						info.uptime = fmt.Sprintf("%dh %dm", hrs, mins)
					} else {
						info.uptime = fmt.Sprintf("%dm", mins)
					}
				}
			}
		}
	case "darwin":
		info.os = "macOS"
		if out, err := exec.Command("uname", "-sr").Output(); err == nil {
			info.kernel = strings.TrimSpace(string(out))
		}
		if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			if b, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
				gb := float64(b) / 1e9
				info.memory = fmt.Sprintf("%.1f GB", gb)
			}
		}
		if out, err := exec.Command("sysctl", "-n", "kern.boottime").Output(); err == nil {
			parts := strings.Fields(string(out))
			for i, p := range parts {
				if p == "sec" && i+2 < len(parts) {
					if secs, err := strconv.ParseInt(strings.TrimRight(parts[i+1], ","), 10, 64); err == nil {
						boot := time.Unix(secs, 0)
						d := time.Since(boot)
						days := int(d.Hours()) / 24
						hrs := int(d.Hours()) % 24
						mins := int(d.Minutes()) % 60
						if days > 0 {
							info.uptime = fmt.Sprintf("%dd %dh %dm", days, hrs, mins)
						} else if hrs > 0 {
							info.uptime = fmt.Sprintf("%dh %dm", hrs, mins)
						} else {
							info.uptime = fmt.Sprintf("%dm", mins)
						}
					}
					break
				}
			}
		}
	default:
		info.os = runtime.GOOS
		if out, err := exec.Command("uname", "-sr").Output(); err == nil {
			info.kernel = strings.TrimSpace(string(out))
		}
	}
	if info.kernel == "" {
		info.kernel = runtime.GOOS
	}
	if info.memory == "" {
		info.memory = "unknown"
	}
	if info.uptime == "" {
		info.uptime = "unknown"
	}
	return info
}


