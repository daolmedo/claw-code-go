package main

import (
	"bufio"
	"claw-code-go/internal/commands"
	"claw-code-go/internal/runtime"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	version = "0.1.0"
	banner  = `Claw Code Go v%s — Claude Code CLI (Go port)
Model: %s
Type /help for commands, /exit to quit.
`
)

func main() {
	// Define flags
	promptFlag := flag.String("prompt", "", "Run a single prompt and exit")
	modelFlag := flag.String("model", "", "Override the model to use")
	replFlag := flag.Bool("repl", false, "Run in interactive REPL mode")
	sessionFlag := flag.String("session", "", "Session ID to load")
	sessionDirFlag := flag.String("session-dir", "", "Directory to store sessions")
	_ = replFlag // used implicitly via mode detection

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claw-code-go [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_API_KEY   Anthropic API key (required)\n")
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_MODEL     Model to use (default: %s)\n", runtime.DefaultModel)
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_BASE_URL  Base URL for the API\n")
	}

	flag.Parse()

	// Load configuration
	cfg := runtime.LoadConfig()

	// Override with flags
	if *modelFlag != "" {
		cfg.Model = *modelFlag
	}
	if *sessionDirFlag != "" {
		cfg.SessionDir = *sessionDirFlag
	}

	// Check for API key
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable is not set.")
		fmt.Fprintln(os.Stderr, "Please set it to your Anthropic API key.")
		os.Exit(1)
	}

	// Create the conversation loop
	loop := runtime.NewConversationLoop(cfg, cfg.APIKey)

	// Load existing session if requested
	if *sessionFlag != "" {
		sess, err := runtime.LoadSession(cfg.SessionDir, *sessionFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load session %s: %v\n", *sessionFlag, err)
		} else {
			loop.Session = sess
			fmt.Printf("Loaded session: %s\n", sess.ID)
		}
	}

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stdout, "\nInterrupted. Saving session...")
		saveSessionSilent(cfg.SessionDir, loop)
		os.Exit(0)
	}()

	// Single prompt mode
	if *promptFlag != "" {
		ctx := context.Background()
		if err := loop.SendMessage(ctx, *promptFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		saveSessionSilent(cfg.SessionDir, loop)
		return
	}

	// Interactive REPL mode
	runREPL(cfg, loop)
}

// runREPL runs the interactive REPL loop.
func runREPL(cfg *runtime.Config, loop *runtime.ConversationLoop) {
	fmt.Printf(banner, version, cfg.Model)

	registry := commands.NewRegistry()
	scanner := bufio.NewScanner(os.Stdin)

	ctx := context.Background()

	for {
		fmt.Print("> ")

		if !scanner.Scan() {
			// EOF or error
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for slash commands
		if strings.HasPrefix(line, "/") {
			handled, err := registry.Execute(line, loop)
			if handled {
				if err == commands.ErrExit {
					fmt.Println("Goodbye!")
					saveSessionSilent(cfg.SessionDir, loop)
					return
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "Command error: %v\n", err)
				}
				continue
			}
		}

		// Send as a message to the model
		if err := loop.SendMessage(ctx, line); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		// Auto-save session after each turn
		saveSessionSilent(cfg.SessionDir, loop)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
	}

	fmt.Println("\nGoodbye!")
	saveSessionSilent(cfg.SessionDir, loop)
}

// saveSessionSilent saves the session without printing errors to stdout.
func saveSessionSilent(dir string, loop *runtime.ConversationLoop) {
	if err := runtime.SaveSession(dir, loop.Session); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save session: %v\n", err)
	}
}
