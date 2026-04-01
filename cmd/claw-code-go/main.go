package main

import (
	"claw-code-go/internal/auth"
	"claw-code-go/internal/commands"
	"claw-code-go/internal/permissions"
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
	permModeFlag := flag.String("permission-mode", "default", "Permission mode: default, accept-edits, bypass, plan")
	_ = replFlag

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claw-code-go [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_API_KEY        Anthropic API key (takes precedence over stored credentials)\n")
		fmt.Fprintf(os.Stderr, "  OPENAI_API_KEY           OpenAI API key (takes precedence over stored credentials)\n")
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_MODEL          Model to use (default: %s)\n", runtime.DefaultModel)
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_BASE_URL       Base URL for the Anthropic API\n")
		fmt.Fprintf(os.Stderr, "  CLAUDE_CODE_USE_BEDROCK  Set to 1 to use AWS Bedrock (env-var fallback)\n")
		fmt.Fprintf(os.Stderr, "  CLAUDE_CODE_USE_VERTEX   Set to 1 to use Google Vertex AI (env-var fallback)\n")
		fmt.Fprintf(os.Stderr, "  CLAUDE_CODE_USE_FOUNDRY  Set to 1 to use Azure AI Foundry (env-var fallback)\n")
	}

	flag.Parse()

	cfg := runtime.LoadConfig()

	if *modelFlag != "" {
		cfg.Model = *modelFlag
	}
	if *sessionDirFlag != "" {
		cfg.SessionDir = *sessionDirFlag
	}

	// Resolve credentials using the multi-provider credential store.
	// Env vars take precedence (ANTHROPIC_API_KEY, OPENAI_API_KEY).
	// Falls back gracefully so the TUI can start and prompt the user to /login.
	provider, token, authMethod, credErr := auth.ResolveCredentials()
	if credErr == nil {
		cfg.ProviderName = provider
		cfg.AuthMethod = authMethod
		if authMethod == "oauth" {
			cfg.OAuthToken = token
		} else {
			cfg.APIKey = token
		}
	} else {
		// No credentials found — start with NoAuthClient so the TUI still opens.
		// The user can run /login inside the TUI.
		fmt.Fprintf(os.Stderr, "Note: no credentials found (%v).\n", credErr)
		fmt.Fprintln(os.Stderr, "      Use /login in the TUI to authenticate.")
	}

	// Create the provider client (or a no-auth placeholder).
	realClient, clientErr := runtime.NewProviderClient(cfg)
	if clientErr != nil {
		fmt.Fprintf(os.Stderr, "Note: could not create %s client: %v\n", cfg.ProviderName, clientErr)
		fmt.Fprintln(os.Stderr, "      Use /login in the TUI to authenticate.")
		realClient = runtime.NewNoAuthClient()
	}

	loop := runtime.NewConversationLoop(cfg, realClient)

	// Wire up the permission manager (Phase 5).
	permMode, err := permissions.ParsePermissionMode(*permModeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v; using default mode\n", err)
		permMode = permissions.ModeDefault
	}
	ruleset, rErr := permissions.LoadRuleset(".claude/settings.json")
	if rErr != nil {
		ruleset = &permissions.Ruleset{}
	}
	loop.PermManager = permissions.NewManager(permMode, ruleset)

	// Connect to MCP servers defined in config (non-fatal errors printed inside).
	loop.InitMCPFromConfig(context.Background())

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
		if credErr != nil {
			fmt.Fprintln(os.Stderr, "Error: cannot use --prompt without valid credentials.")
			fmt.Fprintln(os.Stderr, "Set ANTHROPIC_API_KEY or OPENAI_API_KEY, or run the TUI and use /login.")
			os.Exit(1)
		}
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
	commands.RegisterMCPCommand(registry)
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
