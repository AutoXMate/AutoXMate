package tui

import "github.com/charmbracelet/lipgloss"

const logo = `
 █████╗ ██╗   ██╗████████╗ ██████╗ ███╗   ███╗ █████╗ ████████╗███████╗
██╔══██╗██║   ██║╚══██╔══╝██╔═══██╗████╗ ████║██╔══██╗╚══██╔══╝██╔════╝
███████║██║   ██║   ██║   ██║   ██║██╔████╔██║███████║   ██║   █████╗  
██╔══██║██║   ██║   ██║   ██║   ██║██║╚██╔╝██║██╔══██║   ██║   ██╔══╝  
██║  ██║╚██████╔╝   ██║   ╚██████╔╝██║ ╚═╝ ██║██║  ██║   ██║   ███████╗
╚═╝  ╚═╝ ╚═════╝    ╚═╝    ╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝   ╚═╝   ╚══════╝
`

// opencode-inspired color palette
const (
	colBg          = "#0a0a0a"
	colBgPanel     = "#141414"
	colBgElement   = "#1e1e1e"
	colBorder      = "#484848"
	colBorderAct   = "#606060"
	colText        = "#eeeeee"
	colTextMuted   = "#808080"
	colBlue        = "#5c9cf5"
	colCyan        = "#56b6c2"
	colGreen       = "#7fd88f"
	colOrange      = "#f5a742"
	colRed         = "#e06c75"
	colPurple      = "#9d7cd8"
	colYellow      = "#e5c07b"
	colWhite       = "#eeeeee"

	sideBg      = "#141414"
	sideText    = "#eeeeee"
	sideMuted   = "#808080"
	sideSuccess = "#7fd88f"
)

var (
	statusBarLeft = lipgloss.NewStyle().
			Background(lipgloss.Color(colBgPanel)).
			Foreground(lipgloss.Color(colTextMuted)).
			Padding(0, 1).
			Width(60).
			Align(lipgloss.Left)

	statusBarRight = lipgloss.NewStyle().
			Background(lipgloss.Color(colBgPanel)).
			Foreground(lipgloss.Color(colTextMuted)).
			Padding(0, 1).
			Align(lipgloss.Right)

	sideTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colBlue)).
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colBorder)).
			BorderBottom(true).
			Width(24)

	sideItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colText)).
			Padding(0, 1).
			Width(22)

	sideActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colBlue)).
			Bold(true).
			Padding(0, 1).
			Width(22)

	// Sidebar panel (openCode-style — no border, minimal bg)
	sidePanelStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(sideBg)).
			Padding(0, 2)

	sidePanelHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(sideText)).
				Background(lipgloss.Color(sideBg)).
				Padding(0, 1)

	sideDomainStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(sideText)).
				Bold(true).
				Background(lipgloss.Color(sideBg)).
				Padding(0, 1)

	sideBadgeInstalled = lipgloss.NewStyle().
				Foreground(lipgloss.Color(sideSuccess)).
				Background(lipgloss.Color(sideBg))

	sideBadgePartial = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colOrange)).
				Background(lipgloss.Color(sideBg))

	sideBadgeMissing = lipgloss.NewStyle().
				Foreground(lipgloss.Color(sideMuted)).
				Background(lipgloss.Color(sideBg))

	sideToolName = lipgloss.NewStyle().
			Foreground(lipgloss.Color(sideText)).
			Background(lipgloss.Color(sideBg))

	sideFooterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(sideMuted)).
				Background(lipgloss.Color(sideBg)).
				Padding(0, 1)

	sideOverlayBg = lipgloss.NewStyle().
			Background(lipgloss.Color(colBg))

	itemSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colBlue)).
				Bold(true).
				Padding(0, 1)

	itemInstalledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colGreen)).
				Bold(true)

	itemNotInstalledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colRed))

	itemDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colTextMuted))

	paletteStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colBlue)).
			Background(lipgloss.Color(colBgPanel)).
			Padding(1, 2).
			Width(60)

	paletteInputStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colBgElement)).
				Foreground(lipgloss.Color(colText)).
				Padding(0, 1).
				Width(56)

	paletteItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colText)).
				Padding(0, 1).
				Width(54)

	paletteActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colBlue)).
				Bold(true).
				Padding(0, 1).
				Width(54)

	paletteHintStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colTextMuted)).
				Padding(0, 1)

	sysBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colBlue)).
			Padding(0, 2).
			Width(70)

	sysLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colCyan)).
			Bold(true)

	sysValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colText))

	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colCyan)).
			Bold(true)

	// Command entry mode styles
	inputLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(colBgElement)).
			Foreground(lipgloss.Color(colText)).
			Padding(0, 1, 0, 1)

	promptCharStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colBlue)).
			Bold(true)

	shellPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colYellow)).
				Bold(true)

	inputCursorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colBlue)).
				Foreground(lipgloss.Color(colBg))

	inputModeBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colYellow)).
				Bold(true).
				Padding(0, 0)

	inputCounterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colTextMuted)).
				Padding(0, 0)

	outputLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colText)).
			Padding(0, 1)

	promptLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colCyan)).
			Bold(true).
			Padding(0, 1)

	errorLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colRed)).
			Padding(0, 1)

	successLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colGreen)).
			Padding(0, 1)

	scrollHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colTextMuted)).
			Italic(true).
			Padding(0, 1)

	// Autocomplete dropdown
	acBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colBlue)).
			Background(lipgloss.Color(colBgPanel)).
			Padding(0, 1).
			Width(50)

	acItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colText)).
			Padding(0, 1)

	acActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colBlue)).
			Bold(true).
			Background(lipgloss.Color(colBgElement)).
			Padding(0, 1)

	acHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colTextMuted)).
			Padding(0, 1)

	// Toast notifications
	toastStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(48)

	toastSuccessStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colGreen)).
				Foreground(lipgloss.Color(colGreen)).
				Bold(true).
				Padding(0, 1).
				Width(48)

	toastErrorStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colRed)).
			Foreground(lipgloss.Color(colRed)).
			Bold(true).
			Padding(0, 1).
			Width(48)

	toastInfoStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colBlue)).
			Foreground(lipgloss.Color(colBlue)).
			Bold(true).
			Padding(0, 1).
			Width(48)

	// Markdown rendering
	mdCodeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(colBgElement)).
			Foreground(lipgloss.Color(colOrange)).
			Padding(0, 1)

	mdBoldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colWhite))

	mdHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colCyan)).
			Underline(true)

	// Dialog overlays (which-key, confirm, tool browser, history search)
	dialogOverlay = lipgloss.NewStyle().
			Background(lipgloss.Color(colBgPanel)).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colBlue)).
			Padding(1, 2)

	dialogTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colCyan)).
			Padding(0, 1)

	dialogSection = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colTextMuted)).
			Padding(0, 1)

	dialogKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colBlue)).
			Bold(true)

	dialogDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colText)).
			Padding(0, 1)

	dialogHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colTextMuted)).
			Italic(true).
			Padding(0, 1)

	// Tool browser
	browserInputStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colBgElement)).
				Foreground(lipgloss.Color(colText)).
				Padding(0, 1).
				Width(56)

	browserItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colText)).
				Padding(0, 1).
				Width(54)

	browserActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colBlue)).
				Bold(true).
				Background(lipgloss.Color(colBgElement)).
				Padding(0, 1).
				Width(54)

	browserInstalled = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colGreen))

	browserNotInstalled = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colRed))

	// Confirm dialog
	confirmPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colYellow)).
				Bold(true).
				Padding(0, 1)

	confirmYesStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colGreen)).
				Bold(true).
				Padding(0, 1)

	confirmNoStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colRed)).
				Bold(true).
				Padding(0, 1)

	// History search
	histActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colBlue)).
				Bold(true).
				Background(lipgloss.Color(colBgElement)).
				Padding(0, 1)

	histItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colText)).
			Padding(0, 1)

	// Diff rendering
	diffHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colCyan)).
			Bold(true).
			Padding(0, 1)

	diffMetaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colCyan)).
			Padding(0, 1)

	diffAddedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colGreen)).
			Padding(0, 1)

	diffRemovedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colRed)).
			Padding(0, 1)

	// Scrollbar
	scrollbarTrackStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colBgElement)).
				Padding(0, 0)

	scrollbarThumbStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colBlue)).
				Padding(0, 0)

	// Collapsible output
	collapsibleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colOrange)).
				Bold(true).
				Padding(0, 1)

	collapsibleExpanded = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colOrange)).
				Padding(0, 1)
)

// Home screen styles
var (
	homeBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colBlue)).
			Padding(0, 2)

	homeHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colTextMuted)).
			Padding(0, 1)

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colTextMuted))

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colCyan)).
			Bold(true).
			Padding(0, 1)

	logoLeftStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colTextMuted))

	logoRightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colText)).
			Bold(true)


)

// max returns the larger of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
