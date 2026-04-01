package tui

import (
	"claw-code-go/internal/runtime"
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const appVersion = "0.1.0"

// Known models available in the picker.
var knownModels = []struct {
	id   string
	desc string
}{
	{"claude-opus-4-6", "Most capable — complex reasoning and analysis"},
	{"claude-sonnet-4-6", "Balanced — great performance at speed"},
	{"claude-haiku-4-5-20251001", "Fast and lightweight — quick tasks"},
}

// appState tracks what the TUI is doing.
type appState int

const (
	stateInput    appState = iota // waiting for user input
	stateBusy                     // streaming response from API
	statePicker                   // model selection overlay
	stateHelp                     // help panel overlay
)

// Bubble Tea messages for async streaming events.
type (
	streamDeltaMsg    struct{ text string }
	streamToolMsg     struct{ name, input string }
	streamToolDoneMsg struct{ name, result string }
	streamUsageMsg    struct{ inputTokens, outputTokens int }
	streamDoneMsg     struct{}
	streamErrMsg      struct{ err error }
)

// Model is the Bubble Tea application model.
type Model struct {
	state  appState
	width  int
	height int
	ready  bool

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	// picker state
	pickerCursor int

	// content buffers
	viewBuf   string // finalized history (all complete turns)
	streamBuf string // in-progress streaming content

	// token counts for status bar
	inputTokens  int
	outputTokens int

	// whether any streaming content has arrived (for spinner visibility)
	hasStreamContent bool

	// channel from active streaming goroutine
	streamChan chan runtime.TurnEvent

	// app deps
	loop *runtime.ConversationLoop
	cfg  *runtime.Config
}

// NewModel creates a new TUI model.
func NewModel(cfg *runtime.Config, loop *runtime.ConversationLoop) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /help..."
	ti.CharLimit = 8192

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		state:   stateInput,
		input:   ti,
		spinner: s,
		loop:    loop,
		cfg:     cfg,
	}
}

// Init is the Bubble Tea Init function.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// --- Update -----------------------------------------------------------------

// Update is the Bubble Tea Update function.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.ready = true
			m = m.initViewport()
		} else {
			m = m.resizeViewport()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case streamDeltaMsg:
		if !m.hasStreamContent {
			m.hasStreamContent = true
		}
		m.streamBuf += msg.text
		m = m.refreshViewport()
		return m, waitForStream(m.streamChan)

	case streamToolMsg:
		line := toolStyle.Render(fmt.Sprintf("\n[Tool: %s %s]\n", msg.name, msg.input))
		m.streamBuf += line
		m = m.refreshViewport()
		return m, waitForStream(m.streamChan)

	case streamToolDoneMsg:
		preview := msg.result
		if len(preview) > 80 {
			preview = preview[:80] + "…"
		}
		line := toolStyle.Render(fmt.Sprintf("[Done: %s]\n", msg.name))
		m.streamBuf += line
		m = m.refreshViewport()
		return m, waitForStream(m.streamChan)

	case streamUsageMsg:
		m.inputTokens = msg.inputTokens
		m.outputTokens = msg.outputTokens
		return m, waitForStream(m.streamChan)

	case streamDoneMsg:
		// Commit streamBuf to viewBuf with token annotation
		if m.streamBuf != "" || m.hasStreamContent {
			tokLine := statusStyle.Render(fmt.Sprintf(
				"\n\nTokens: %s in / %s out\n\n",
				formatNum(m.inputTokens),
				formatNum(m.outputTokens),
			))
			m.viewBuf += m.streamBuf + tokLine
			m.streamBuf = ""
		}
		m.hasStreamContent = false
		m.state = stateInput
		m = m.refreshViewport()
		m.viewport.GotoBottom()
		return m, nil

	case streamErrMsg:
		m.viewBuf += errorStyle.Render(fmt.Sprintf("Error: %v\n\n", msg.err))
		m.streamBuf = ""
		m.hasStreamContent = false
		m.state = stateInput
		m = m.refreshViewport()
		return m, nil

	case spinner.TickMsg:
		if m.state == stateBusy && !m.hasStreamContent {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

// handleKey dispatches key events based on current state.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case statePicker:
		return m.handlePickerKey(msg)
	case stateHelp:
		return m.handleHelpKey(msg)
	case stateBusy:
		// Only allow quit during streaming
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m, nil
	}

	// stateInput
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEnter:
		return m.handleSubmit()
	case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleSubmit processes the current input field value.
func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}
	m.input.SetValue("")

	if strings.HasPrefix(text, "/") {
		return m.handleSlashCommand(text)
	}
	return m.startMessage(text)
}

// handleSlashCommand processes built-in slash commands.
func (m Model) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return m, nil
	}

	switch parts[0] {
	case "/model":
		m.state = statePicker
		m.pickerCursor = 0
		// Set cursor to current model
		for i, km := range knownModels {
			if km.id == m.cfg.Model {
				m.pickerCursor = i
				break
			}
		}
		return m, nil

	case "/help":
		m.state = stateHelp
		return m, nil

	case "/clear":
		m.loop.ClearSession()
		m.viewBuf = statusStyle.Render("Session cleared.\n\n")
		m.streamBuf = ""
		m.inputTokens = 0
		m.outputTokens = 0
		m = m.refreshViewport()
		return m, nil

	case "/session-list":
		sessions, err := m.loop.ListSessions()
		if err != nil {
			m.viewBuf += errorStyle.Render(fmt.Sprintf("Error listing sessions: %v\n\n", err))
		} else if len(sessions) == 0 {
			m.viewBuf += statusStyle.Render("No saved sessions.\n\n")
		} else {
			m.viewBuf += statusStyle.Render("Saved sessions:\n  " + strings.Join(sessions, "\n  ") + "\n\n")
		}
		m = m.refreshViewport()
		return m, nil

	case "/exit", "/quit":
		return m, tea.Quit

	default:
		m.viewBuf += errorStyle.Render(fmt.Sprintf("Unknown command: %s  (type /help for commands)\n\n", parts[0]))
		m = m.refreshViewport()
		return m, nil
	}
}

// startMessage begins a streaming conversation turn.
func (m Model) startMessage(text string) (tea.Model, tea.Cmd) {
	m.viewBuf += userLabelStyle.Render("You") + ": " + text + "\n\n"
	m.viewBuf += assistantLabelStyle.Render("Claude") + ": "
	m.state = stateBusy
	m.hasStreamContent = false

	ch := make(chan runtime.TurnEvent, 64)
	m.streamChan = ch

	loop := m.loop
	go func() {
		defer close(ch)
		loop.SendMessageStreaming(context.Background(), text, ch) //nolint:errcheck
	}()

	m = m.refreshViewport()
	return m, tea.Batch(
		m.spinner.Tick,
		waitForStream(ch),
	)
}

// handlePickerKey handles keys when the model picker overlay is shown.
func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.state = stateInput
		return m, nil
	case tea.KeyEnter:
		chosen := knownModels[m.pickerCursor]
		m.cfg.Model = chosen.id
		m.loop.Client.Model = chosen.id
		m.loop.Config.Model = chosen.id
		m.viewBuf += statusStyle.Render(fmt.Sprintf("Model changed to %s\n\n", chosen.id))
		m.state = stateInput
		m = m.refreshViewport()
		return m, nil
	case tea.KeyUp:
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.pickerCursor < len(knownModels)-1 {
			m.pickerCursor++
		}
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// handleHelpKey handles keys when the help overlay is shown.
func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc, msg.Type == tea.KeyEnter, msg.String() == "q":
		m.state = stateInput
		return m, nil
	case msg.Type == tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// --- View -------------------------------------------------------------------

// View is the Bubble Tea View function.
func (m Model) View() string {
	if !m.ready {
		return "Initializing…\n"
	}

	switch m.state {
	case statePicker:
		return m.viewPicker()
	case stateHelp:
		return m.viewHelp()
	}

	header := m.renderHeader()
	divider := dividerStyle.Render(strings.Repeat("─", m.width))
	inputLine := inputPromptStyle.Render("> ") + m.input.View()
	statusLine := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.viewport.View(),
		divider,
		inputLine,
		statusLine,
	)
}

func (m Model) renderHeader() string {
	title := headerStyle.Render("Claw Code v" + appVersion)
	tag := modelTagStyle.Render("  " + m.cfg.Model)
	return title + tag
}

func (m Model) renderStatusBar() string {
	if m.inputTokens > 0 || m.outputTokens > 0 {
		return statusStyle.Render(fmt.Sprintf(
			"Tokens: %s in / %s out  │  Session: %s",
			formatNum(m.inputTokens), formatNum(m.outputTokens),
			m.loop.Session.ID,
		))
	}
	return statusStyle.Render("Session: " + m.loop.Session.ID)
}

func (m Model) viewPicker() string {
	var b strings.Builder
	b.WriteString(pickerHeaderStyle.Render("Select Model") + "\n")
	b.WriteString(statusStyle.Render("  ↑/↓ navigate  Enter select  Esc cancel") + "\n\n")

	for i, km := range knownModels {
		cursor := "  "
		style := unselectedModelStyle
		if i == m.pickerCursor {
			cursor = "▶ "
			style = selectedModelStyle
		}
		b.WriteString(cursor + style.Render(km.id) + "\n")
		b.WriteString("    " + statusStyle.Render(km.desc) + "\n")
	}

	return b.String()
}

func (m Model) viewHelp() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Render("Claw Code — Commands"),
		"",
		"  "+userLabelStyle.Render("/help")+"           Show this help",
		"  "+userLabelStyle.Render("/model")+"          Change the active model",
		"  "+userLabelStyle.Render("/clear")+"          Clear session history",
		"  "+userLabelStyle.Render("/session-list")+"   List saved sessions",
		"  "+userLabelStyle.Render("/exit")+" / "+userLabelStyle.Render("/quit")+"   Exit (session auto-saved)",
		"",
		"  "+userLabelStyle.Render("Ctrl+C")+"          Exit",
		"",
		statusStyle.Render("Navigation:  ↑/↓/PgUp/PgDn to scroll  │  Esc/Enter/q to close this panel"),
	)
	box := helpBoxStyle.Width(min(72, m.width-4)).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// --- Helpers ----------------------------------------------------------------

// waitForStream returns a tea.Cmd that reads the next event from the stream channel.
func waitForStream(ch <-chan runtime.TurnEvent) tea.Cmd {
	return func() tea.Msg {
		for {
			ev, ok := <-ch
			if !ok {
				return streamDoneMsg{}
			}
			switch ev.Type {
			case runtime.TurnEventTextDelta:
				return streamDeltaMsg{text: ev.Text}
			case runtime.TurnEventToolStart:
				return streamToolMsg{name: ev.ToolName, input: ev.ToolInput}
			case runtime.TurnEventToolDone:
				return streamToolDoneMsg{name: ev.ToolName, result: ev.ToolResult}
			case runtime.TurnEventUsage:
				return streamUsageMsg{inputTokens: ev.InputTokens, outputTokens: ev.OutputTokens}
			case runtime.TurnEventDone:
				return streamDoneMsg{}
			case runtime.TurnEventError:
				return streamErrMsg{err: ev.Err}
			}
		}
	}
}

// initViewport creates the viewport with appropriate dimensions.
func (m Model) initViewport() Model {
	vpHeight := m.viewportHeight()
	m.viewport = viewport.New(m.width, vpHeight)
	m.viewport.SetContent(m.viewBuf)
	m.viewport.GotoBottom()
	m.input.Width = max(m.width-3, 10)
	return m
}

// resizeViewport updates viewport dimensions after a window resize.
func (m Model) resizeViewport() Model {
	m.viewport.Width = m.width
	m.viewport.Height = m.viewportHeight()
	m.input.Width = max(m.width-3, 10)
	return m
}

// viewportHeight calculates the viewport height from terminal height.
// Layout: 1 header + viewport + 1 divider + 1 input + 1 status = 4 overhead lines.
func (m Model) viewportHeight() int {
	h := m.height - 4
	if h < 1 {
		h = 1
	}
	return h
}

// refreshViewport rebuilds viewport content from current buffers.
func (m Model) refreshViewport() Model {
	content := m.viewBuf + m.streamBuf
	if m.state == stateBusy && !m.hasStreamContent {
		content += m.spinner.View() + statusStyle.Render(" Thinking…\n")
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
	return m
}

// formatNum formats an integer with comma separators.
func formatNum(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, c)
	}
	return string(result)
}
