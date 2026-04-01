package main

import (
	"claw-code-go/internal/auth"
	"claw-code-go/internal/commands"
	"claw-code-go/internal/runtime"
	"claw-code-go/internal/tui"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	promptFlag := flag.String("prompt", "", "Run a single prompt and exit")
	modelFlag := flag.String("model", "", "Override the model to use")
	replFlag := flag.Bool("repl", false, "Run in interactive REPL mode (default when no --prompt)")
	sessionFlag := flag.String("session", "", "Session ID to load")
	sessionDirFlag := flag.String("session-dir", "", "Directory to store sessions")
	_ = replFlag

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claw-code-go [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_API_KEY        Anthropic API key (or use /auth login for OAuth)\n")
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_MODEL          Model to use (default: %s)\n", runtime.DefaultModel)
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_BASE_URL       Base URL for the API\n")
		fmt.Fprintf(os.Stderr, "  CLAUDE_CODE_USE_BEDROCK  Set to 1 to use AWS Bedrock\n")
		fmt.Fprintf(os.Stderr, "  CLAUDE_CODE_USE_VERTEX   Set to 1 to use Google Vertex AI\n")
		fmt.Fprintf(os.Stderr, "  CLAUDE_CODE_USE_FOUNDRY  Set to 1 to use Azure AI Foundry\n")
	}

	flag.Parse()

	cfg := runtime.LoadConfig()

	if *modelFlag != "" {
		cfg.Model = *modelFlag
	}
	if *sessionDirFlag != "" {
		cfg.SessionDir = *sessionDirFlag
	}

	// Resolve credentials: ANTHROPIC_API_KEY takes precedence; fall back to OAuth tokens.
	if cfg.APIKey != "" {
		cfg.AuthMethod = "api_key"
	} else {
		// Attempt to load/refresh OAuth token from ~/.claw-code/auth.json.
		token, err := auth.GetAccessToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: no credentials found.")
			fmt.Fprintln(os.Stderr, "Set ANTHROPIC_API_KEY, or run the TUI and use /auth login.")
			os.Exit(1)
		}
		cfg.OAuthToken = token
		cfg.AuthMethod = "oauth"
	}

	// Create the provider client (stub providers return an error here).
	client, err := runtime.NewProviderClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initialising provider %q: %v\n", cfg.ProviderName, err)
		os.Exit(1)
	}

	loop := runtime.NewConversationLoop(cfg, client)

	if *sessionFlag != "" {
		sess, err := runtime.LoadSession(cfg.SessionDir, *sessionFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load session %s: %v\n", *sessionFlag, err)
		} else {
			loop.Session = sess
			fmt.Printf("Loaded session: %s\n", sess.ID)
		}
	}

	// Single prompt (non-interactive) mode — no TUI, plain stdout streaming.
	if *promptFlag != "" {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Fprintln(os.Stdout, "\nInterrupted. Saving session...")
			saveSessionSilent(cfg.SessionDir, loop)
			os.Exit(0)
		}()

		ctx := context.Background()
		if err := loop.SendMessage(ctx, *promptFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		saveSessionSilent(cfg.SessionDir, loop)
		return
	}

	// Interactive TUI mode.
	runTUI(cfg, loop)
}

// runTUI starts the Bubble Tea TUI for interactive use.
func runTUI(cfg *runtime.Config, loop *runtime.ConversationLoop) {
	// Register slash commands (available for future non-TUI REPL mode).
	registry := commands.NewRegistry()
	commands.RegisterAuthCommands(registry)
	_ = registry

	// Save session on SIGTERM (Ctrl+C is handled by Bubble Tea itself).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		<-sigCh
		saveSessionSilent(cfg.SessionDir, loop)
		os.Exit(0)
	}()

	model := tui.NewModel(cfg, loop)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}

	// Save session after the TUI exits (covers Ctrl+C via tea.Quit).
	saveSessionSilent(cfg.SessionDir, loop)
}

// saveSessionSilent saves the session, printing only to stderr on failure.
func saveSessionSilent(dir string, loop *runtime.ConversationLoop) {
	if err := runtime.SaveSession(dir, loop.Session); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save session: %v\n", err)
	}
}
