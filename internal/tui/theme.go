package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the named color tokens used throughout the TUI.
// All lipgloss styles are derived from the active theme.
type Theme struct {
	Primary        lipgloss.Color // app accent (header, spinner)
	Secondary      lipgloss.Color // secondary accent (overlay borders)
	Success        lipgloss.Color // positive outcomes
	Warning        lipgloss.Color // advisory messages
	Error          lipgloss.Color // error messages
	Muted          lipgloss.Color // de-emphasized text (status bar)
	Subtle         lipgloss.Color // very subtle (dividers)
	UserLabel      lipgloss.Color // "You:" label
	AssistantLabel lipgloss.Color // "Claude:" label
	ToolRunning    lipgloss.Color // in-progress tool indicator
	ToolDone       lipgloss.Color // completed tool indicator
	ToolFailed     lipgloss.Color // failed tool indicator
	InputPrompt    lipgloss.Color // ">" prompt glyph
	SelectedItem   lipgloss.Color // highlighted item in pickers
	UnselectedItem lipgloss.Color // unselected item in pickers
}

// DarkTheme is the default color scheme for dark-background terminals.
var DarkTheme = Theme{
	Primary:        "205",
	Secondary:      "62",
	Success:        "82",
	Warning:        "214",
	Error:          "196",
	Muted:          "240",
	Subtle:         "238",
	UserLabel:      "33",
	AssistantLabel: "82",
	ToolRunning:    "214",
	ToolDone:       "240",
	ToolFailed:     "196",
	InputPrompt:    "33",
	SelectedItem:   "170",
	UnselectedItem: "252",
}

// LightTheme is a color scheme for light-background terminals.
var LightTheme = Theme{
	Primary:        "125",
	Secondary:      "61",
	Success:        "28",
	Warning:        "130",
	Error:          "160",
	Muted:          "246",
	Subtle:         "250",
	UserLabel:      "25",
	AssistantLabel: "28",
	ToolRunning:    "130",
	ToolDone:       "246",
	ToolFailed:     "160",
	InputPrompt:    "25",
	SelectedItem:   "125",
	UnselectedItem: "236",
}

// currentTheme is the active theme; defaults to DarkTheme.
var currentTheme = DarkTheme

// SetTheme sets the active theme and rebuilds all derived styles.
func SetTheme(t Theme) {
	currentTheme = t
	rebuildStyles(t)
}
