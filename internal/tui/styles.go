package tui

import "github.com/charmbracelet/lipgloss"

// All TUI styles are derived from currentTheme.
// Call rebuildStyles (via SetTheme) to refresh after a theme switch.
var (
	headerStyle          lipgloss.Style
	modelTagStyle        lipgloss.Style
	userLabelStyle       lipgloss.Style
	assistantLabelStyle  lipgloss.Style
	toolRunningStyle     lipgloss.Style
	toolDoneStyle        lipgloss.Style
	toolFailedStyle      lipgloss.Style
	statusStyle          lipgloss.Style
	warnStyle            lipgloss.Style
	errorStyle           lipgloss.Style
	helpBoxStyle         lipgloss.Style
	dividerStyle         lipgloss.Style
	inputPromptStyle     lipgloss.Style
	pickerHeaderStyle    lipgloss.Style
	selectedModelStyle   lipgloss.Style
	unselectedModelStyle lipgloss.Style
)

// init seeds styles from the default theme before the first render.
func init() { rebuildStyles(currentTheme) }

// rebuildStyles recreates all styles from the given theme tokens.
func rebuildStyles(t Theme) {
	headerStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	modelTagStyle = lipgloss.NewStyle().
		Foreground(t.Muted)

	userLabelStyle = lipgloss.NewStyle().
		Foreground(t.UserLabel).
		Bold(true)

	assistantLabelStyle = lipgloss.NewStyle().
		Foreground(t.AssistantLabel).
		Bold(true)

	toolRunningStyle = lipgloss.NewStyle().
		Foreground(t.ToolRunning).
		Italic(true)

	toolDoneStyle = lipgloss.NewStyle().
		Foreground(t.ToolDone)

	toolFailedStyle = lipgloss.NewStyle().
		Foreground(t.ToolFailed).
		Bold(true)

	statusStyle = lipgloss.NewStyle().
		Foreground(t.Muted)

	warnStyle = lipgloss.NewStyle().
		Foreground(t.Warning)

	errorStyle = lipgloss.NewStyle().
		Foreground(t.Error)

	helpBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Secondary).
		Padding(1, 2)

	dividerStyle = lipgloss.NewStyle().
		Foreground(t.Subtle)

	inputPromptStyle = lipgloss.NewStyle().
		Foreground(t.InputPrompt).
		Bold(true)

	pickerHeaderStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Padding(0, 1)

	selectedModelStyle = lipgloss.NewStyle().
		Foreground(t.SelectedItem).
		Bold(true)

	unselectedModelStyle = lipgloss.NewStyle().
		Foreground(t.UnselectedItem)
}
