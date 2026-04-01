package compat

import (
	"claw-code-go/internal/commands"
	"claw-code-go/internal/runtime"
	"claw-code-go/internal/tools"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ToolManifest describes a registered tool.
type ToolManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CommandManifest describes a registered slash command.
type CommandManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ManifestSummary is the output of dump-manifests.
type ManifestSummary struct {
	Tools    []ToolManifest    `json:"tools"`
	Commands []CommandManifest `json:"commands"`
	Source   *SourceManifest   `json:"source,omitempty"`
}

// BootstrapPhase describes one phase of startup.
type BootstrapPhase struct {
	Phase       int    `json:"phase"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RunDumpManifests implements the dump-manifests subcommand.
// It lists all registered tools, slash commands, and optionally a source manifest.
func RunDumpManifests(args []string) {
	fs := flag.NewFlagSet("dump-manifests", flag.ExitOnError)
	srcDir := fs.String("src", "", "Path to upstream TypeScript source directory (optional)")
	jsonOut := fs.Bool("json", false, "Output as JSON (default: human-readable text)")
	_ = fs.Parse(args)

	// Collect built-in tools.
	builtinTools := []struct {
		name string
		desc string
	}{
		{tools.BashTool().Name, tools.BashTool().Description},
		{tools.ReadFileTool().Name, tools.ReadFileTool().Description},
		{tools.WriteFileTool().Name, tools.WriteFileTool().Description},
		{tools.GlobTool().Name, tools.GlobTool().Description},
		{tools.GrepTool().Name, tools.GrepTool().Description},
	}
	toolManifests := make([]ToolManifest, len(builtinTools))
	for i, t := range builtinTools {
		toolManifests[i] = ToolManifest{Name: t.name, Description: t.desc}
	}

	// Collect slash commands via registry.
	reg := commands.NewRegistry()
	commands.RegisterAuthCommands(reg)
	commands.RegisterMCPCommand(reg)
	cmds := reg.List()
	cmdManifests := make([]CommandManifest, len(cmds))
	for i, cmd := range cmds {
		cmdManifests[i] = CommandManifest{Name: cmd.Name, Description: cmd.Description}
	}

	summary := ManifestSummary{
		Tools:    toolManifests,
		Commands: cmdManifests,
	}

	// Optionally load source file manifest.
	if *srcDir != "" {
		m, err := LoadManifest(*srcDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load source manifest from %s: %v\n", *srcDir, err)
		} else {
			summary.Source = m
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(summary)
		return
	}

	// Human-readable output.
	fmt.Println("=== Tools ===")
	for _, t := range summary.Tools {
		fmt.Printf("  %-20s %s\n", t.Name, t.Description)
	}
	fmt.Println()
	fmt.Println("=== Slash Commands ===")
	for _, c := range summary.Commands {
		fmt.Printf("  %-20s %s\n", c.Name, c.Description)
	}
	if summary.Source != nil {
		fmt.Println()
		fmt.Printf("=== Source Manifest (%s) ===\n", summary.Source.Version)
		fmt.Printf("  %d files indexed\n", len(summary.Source.Files))
	}
}

// RunBootstrapPlan implements the bootstrap-plan subcommand.
// It prints the ordered startup phase plan.
func RunBootstrapPlan(args []string) {
	fs := flag.NewFlagSet("bootstrap-plan", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	_ = fs.Parse(args)

	phases := []BootstrapPhase{
		{1, "Config", "Load configuration from environment variables and defaults (model, session dir, provider settings)"},
		{2, "Credentials", "Resolve provider credentials (API key or OAuth token) via multi-provider auth store; fall back gracefully if none found"},
		{3, "Provider Client", "Create the appropriate provider API client (Anthropic, OpenAI, Bedrock, Vertex, Foundry) or a no-auth placeholder"},
		{4, "Conversation Loop", "Initialize ConversationLoop with built-in tools: bash, read_file, write_file, glob, grep"},
		{5, "Permission Manager", "Set up permission manager with configured mode (default, accept-edits, bypass, plan) and ruleset from .claude/settings.json"},
		{6, "MCP Servers", "Connect to Model Context Protocol servers defined in config (stdio and SSE transports)"},
		{7, "Session", "Load existing session from disk if --session flag provided; merge into conversation loop"},
		{8, "UI Launch", "Start Bubble Tea TUI (interactive mode) or execute single --prompt (non-interactive streaming mode)"},
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(phases)
		return
	}

	fmt.Println("Bootstrap Plan — startup phases in order:")
	fmt.Println()
	for _, p := range phases {
		fmt.Printf("  Phase %d: %s\n", p.Phase, p.Name)
		fmt.Printf("           %s\n", p.Description)
		fmt.Println()
	}
}

// RunPrintSystemPrompt implements the print-system-prompt subcommand.
// It renders the full system prompt that would be sent to the model.
func RunPrintSystemPrompt(args []string) {
	fs := flag.NewFlagSet("print-system-prompt", flag.ExitOnError)
	cwdFlag := fs.String("cwd", "", "Working directory to inject as context")
	dateFlag := fs.String("date", "", "Date to inject as context (e.g. 2024-01-15)")
	_ = fs.Parse(args)

	cfg := runtime.LoadConfig()
	loop := runtime.NewConversationLoop(cfg, runtime.NewNoAuthClient())
	prompt := loop.SystemPrompt()

	// Inject cwd and date as additional context when provided.
	var extras []string
	if *cwdFlag != "" {
		extras = append(extras, fmt.Sprintf("Working directory: %s", *cwdFlag))
	}
	if *dateFlag != "" {
		extras = append(extras, fmt.Sprintf("Current date: %s", *dateFlag))
	}
	if len(extras) > 0 {
		prompt = prompt + "\n\n" + strings.Join(extras, "\n")
	}

	fmt.Println(prompt)
}

// RunResumeSession implements the resume-session subcommand.
// It loads a saved session JSON file, prints its history, and optionally
// executes additional commands against it.
func RunResumeSession(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: claw-code-go resume-session <session-file> [commands...]")
		os.Exit(1)
	}
	sessionPath := args[0]
	extraCmds := args[1:]

	// Resolve absolute path so we can split dir and base correctly.
	absPath, err := filepath.Abs(sessionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}
	dir := filepath.Dir(absPath)
	id := strings.TrimSuffix(filepath.Base(absPath), ".json")

	sess, err := runtime.LoadSession(dir, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
		os.Exit(1)
	}

	// Print conversation history header.
	fmt.Printf("=== Session: %s ===\n", sess.ID)
	fmt.Printf("Created:  %s\n", sess.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:  %s\n", sess.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("Messages: %d\n", len(sess.Messages))
	if sess.CompactionCount > 0 {
		fmt.Printf("Compactions: %d\n", sess.CompactionCount)
	}
	fmt.Println()

	for i, msg := range sess.Messages {
		fmt.Printf("[%d] %s:\n", i+1, strings.ToUpper(msg.Role))
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				text := block.Text
				if len(text) > 300 {
					text = text[:300] + "… [truncated]"
				}
				fmt.Printf("  %s\n", text)
			case "tool_use":
				fmt.Printf("  [tool_use: %s]\n", block.Name)
			case "tool_result":
				if len(block.Content) > 0 {
					snippet := block.Content[0].Text
					if len(snippet) > 120 {
						snippet = snippet[:120] + "…"
					}
					fmt.Printf("  [tool_result: %s]\n", snippet)
				}
			}
		}
		fmt.Println()
	}

	if len(extraCmds) == 0 {
		return
	}

	// Execute additional commands against the loaded session.
	cfg := runtime.LoadConfig()
	realClient, err := runtime.NewProviderClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating provider client: %v\n", err)
		os.Exit(1)
	}
	loop := runtime.NewConversationLoop(cfg, realClient)
	loop.Session = sess

	ctx := context.Background()
	for _, cmd := range extraCmds {
		fmt.Printf(">>> %s\n", cmd)
		if err := loop.SendMessage(ctx, cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing command %q: %v\n", cmd, err)
		}
	}
}
