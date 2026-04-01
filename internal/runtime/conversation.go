package runtime

import (
	"claw-code-go/internal/api"
	"claw-code-go/internal/mcp"
	"claw-code-go/internal/permissions"
	"claw-code-go/internal/tools"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const systemPromptBase = `You are Claude Code, an AI assistant for software engineering tasks. You have access to tools for running bash commands, reading and writing files, searching with glob patterns, and grepping for patterns in code. Use these tools to help users with coding tasks.`

// ConversationLoop manages the agentic conversation loop with tool use.
type ConversationLoop struct {
	Client      api.APIClient // provider-agnostic client interface
	Session     *Session
	Tools       []api.Tool
	Permissions *Permissions
	PermManager *permissions.Manager // Phase 5 permission manager (may be nil)
	Config      *Config
	MCPRegistry *mcp.Registry // MCP server registry (may be nil)
	Compaction  CompactionState // Phase 6 token tracking and compaction state
}

// NewConversationLoop creates a new conversation loop with the given client.
// Use NewProviderClient to create an appropriate client for the configured provider.
func NewConversationLoop(cfg *Config, client api.APIClient) *ConversationLoop {
	return &ConversationLoop{
		Client:  client,
		Session: NewSession(),
		Tools: []api.Tool{
			tools.BashTool(),
			tools.ReadFileTool(),
			tools.WriteFileTool(),
			tools.GlobTool(),
			tools.GrepTool(),
		},
		Permissions: DefaultPermissions(),
		Config:      cfg,
	}
}

// systemPrompt returns the system prompt, optionally injecting a compaction
// summary and MCP tool context.
func (loop *ConversationLoop) systemPrompt() string {
	var parts []string
	parts = append(parts, systemPromptBase)

	// Inject compaction summary when the session has one (Phase 6).
	if loop.Session != nil && loop.Session.CompactionSummary != "" {
		parts = append(parts, FormatCompactSummary(loop.Session.CompactionSummary))
	}

	// Append MCP tool list if any servers are connected.
	if loop.MCPRegistry != nil {
		mcpTools := loop.MCPRegistry.AllTools()
		if len(mcpTools) > 0 {
			names := make([]string, len(mcpTools))
			for i, t := range mcpTools {
				names[i] = t.Name
			}
			parts = append(parts, "Additional tools available via MCP: "+strings.Join(names, ", ")+".")
		}
	}

	return strings.Join(parts, "\n\n")
}

// allTools returns built-in tools merged with any MCP tools.
func (loop *ConversationLoop) allTools() []api.Tool {
	if loop.MCPRegistry == nil {
		return loop.Tools
	}
	mcpAPITools := loop.MCPRegistry.AllAPITools()
	if len(mcpAPITools) == 0 {
		return loop.Tools
	}
	combined := make([]api.Tool, 0, len(loop.Tools)+len(mcpAPITools))
	combined = append(combined, loop.Tools...)
	combined = append(combined, mcpAPITools...)
	return combined
}

// SendMessage sends a user message and runs the full agentic loop.
func (loop *ConversationLoop) SendMessage(ctx context.Context, userText string) error {
	// Append user message
	loop.Session.Messages = append(loop.Session.Messages, api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: "text", Text: userText},
		},
	})

	// Compact history if approaching the token budget (Phase 6).
	if ShouldCompact(loop.Compaction.LastInputTokens, loop.Session.Messages, loop.Config) {
		summary, err := CompactSession(ctx, loop.Client, loop.Config, loop.Session)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[compact] warning: %v\n", err)
		} else {
			loop.Compaction.CompactionCount++
			// Prepend a continuation marker to the retained recent messages.
			contMsg := GetContinuationMessage(summary)
			loop.Session.Messages = append([]api.Message{contMsg}, loop.Session.Messages...)
		}
	}

	// Agentic loop: keep going until stop_reason is "end_turn"
	for {
		stopReason, err := loop.runOneTurn(ctx)
		if err != nil {
			return err
		}

		if stopReason != "tool_use" {
			break
		}
	}

	return nil
}

// runOneTurn sends the current session messages to the API and processes the response.
// Returns the stop_reason.
func (loop *ConversationLoop) runOneTurn(ctx context.Context) (string, error) {
	req := api.CreateMessageRequest{
		Model:     loop.Config.Model,
		MaxTokens: loop.Config.MaxTokens,
		System:    loop.systemPrompt(),
		Messages:  loop.Session.Messages,
		Tools:     loop.allTools(),
		Stream:    true,
	}

	ch, err := loop.Client.StreamResponse(ctx, req)
	if err != nil {
		return "", fmt.Errorf("stream response: %w", err)
	}

	// Accumulators for the current response
	type toolBlock struct {
		id          string
		name        string
		inputBuffer string
	}

	var (
		textBlocks    []api.ContentBlock
		toolBlocks    []toolBlock
		currentText   string
		currentTool   *toolBlock
		stopReason    string
		blockIndex    int
		blockTypeMap  = make(map[int]string) // index -> "text" or "tool_use"
	)

	_ = blockIndex // suppress unused warning

	for event := range ch {
		switch event.Type {
		case api.EventError:
			return "", fmt.Errorf("stream error: %s", event.ErrorMessage)

		case api.EventContentBlockStart:
			blockTypeMap[event.Index] = event.ContentBlock.Type
			if event.ContentBlock.Type == "tool_use" {
				tb := toolBlock{
					id:   event.ContentBlock.ID,
					name: event.ContentBlock.Name,
				}
				toolBlocks = append(toolBlocks, tb)
				currentTool = &toolBlocks[len(toolBlocks)-1]
			}

		case api.EventContentBlockDelta:
			switch event.Delta.Type {
			case "text_delta":
				currentText += event.Delta.Text
				fmt.Fprint(os.Stdout, event.Delta.Text)

			case "input_json_delta":
				if currentTool != nil {
					currentTool.inputBuffer += event.Delta.PartialJSON
				}
			}

		case api.EventContentBlockStop:
			bType, ok := blockTypeMap[event.Index]
			if ok && bType == "text" && currentText != "" {
				textBlocks = append(textBlocks, api.ContentBlock{
					Type: "text",
					Text: currentText,
				})
				currentText = ""
			}
			// Reset currentTool pointer (but keep toolBlocks slice)
			currentTool = nil

		case api.EventMessageDelta:
			stopReason = event.StopReason

		case api.EventMessageStop:
			// Stream complete
		}
	}

	// Ensure trailing newline after streaming text
	if len(textBlocks) > 0 || len(toolBlocks) > 0 {
		fmt.Fprintln(os.Stdout)
	}

	// Build the assistant message content
	var assistantContent []api.ContentBlock

	// Add text blocks first
	assistantContent = append(assistantContent, textBlocks...)

	// Add tool_use blocks
	for _, tb := range toolBlocks {
		var inputMap map[string]any
		if tb.inputBuffer != "" {
			if err := json.Unmarshal([]byte(tb.inputBuffer), &inputMap); err != nil {
				inputMap = map[string]any{"raw": tb.inputBuffer}
			}
		} else {
			inputMap = map[string]any{}
		}

		assistantContent = append(assistantContent, api.ContentBlock{
			Type:  "tool_use",
			ID:    tb.id,
			Name:  tb.name,
			Input: inputMap,
		})
	}

	// Append assistant message to session
	if len(assistantContent) > 0 {
		loop.Session.Messages = append(loop.Session.Messages, api.Message{
			Role:    "assistant",
			Content: assistantContent,
		})
	}

	// If stop_reason is tool_use, execute tools and append results
	if stopReason == "tool_use" {
		var toolResults []api.ContentBlock

		for _, tb := range toolBlocks {
			var inputMap map[string]any
			for _, cb := range assistantContent {
				if cb.Type == "tool_use" && cb.ID == tb.id {
					inputMap = cb.Input
					break
				}
			}
			if inputMap == nil {
				inputMap = map[string]any{}
			}

			fmt.Fprintf(os.Stdout, "\n[Tool: %s]\n", tb.name)
			result := loop.ExecuteTool(tb.name, inputMap)
			result.ToolUseID = tb.id
			toolResults = append(toolResults, result)
		}

		// Append tool results as a user message
		loop.Session.Messages = append(loop.Session.Messages, api.Message{
			Role:    "user",
			Content: toolResults,
		})
	}

	return stopReason, nil
}

// SendMessageStreaming sends a user message and runs the full agentic loop, emitting
// TurnEvents to the provided channel. The channel is NOT closed by this function;
// callers should close it after this returns.
func (loop *ConversationLoop) SendMessageStreaming(ctx context.Context, userText string, events chan<- TurnEvent) error {
	loop.Session.Messages = append(loop.Session.Messages, api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: "text", Text: userText},
		},
	})

	// Compact history if approaching the token budget (Phase 6).
	if ShouldCompact(loop.Compaction.LastInputTokens, loop.Session.Messages, loop.Config) {
		summary, err := CompactSession(ctx, loop.Client, loop.Config, loop.Session)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[compact] warning: %v\n", err)
		} else {
			loop.Compaction.CompactionCount++
			// Prepend a continuation marker to the retained recent messages.
			contMsg := GetContinuationMessage(summary)
			loop.Session.Messages = append([]api.Message{contMsg}, loop.Session.Messages...)
		}
	}

	var totalInput, totalOutput int

	for {
		stopReason, inTok, outTok, err := loop.runOneTurnStreaming(ctx, events)
		if err != nil {
			events <- TurnEvent{Type: TurnEventError, Err: err}
			return err
		}
		totalInput += inTok
		totalOutput += outTok

		if stopReason != "tool_use" {
			break
		}
	}

	// Update compaction state with the latest token counts (Phase 6).
	loop.Compaction.LastInputTokens = totalInput
	loop.Compaction.TotalInputTokens += totalInput
	loop.Compaction.TotalOutputTokens += totalOutput

	events <- TurnEvent{
		Type:         TurnEventUsage,
		InputTokens:  totalInput,
		OutputTokens: totalOutput,
	}
	events <- TurnEvent{Type: TurnEventDone}
	return nil
}

// runOneTurnStreaming streams one API turn and sends TurnEvents.
// Returns stop_reason, inputTokens, outputTokens, error.
func (loop *ConversationLoop) runOneTurnStreaming(ctx context.Context, events chan<- TurnEvent) (string, int, int, error) {
	req := api.CreateMessageRequest{
		Model:     loop.Config.Model,
		MaxTokens: loop.Config.MaxTokens,
		System:    loop.systemPrompt(),
		Messages:  loop.Session.Messages,
		Tools:     loop.allTools(),
		Stream:    true,
	}

	ch, err := loop.Client.StreamResponse(ctx, req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("stream response: %w", err)
	}

	type toolBlock struct {
		id          string
		name        string
		inputBuffer string
	}

	var (
		textBlocks   []api.ContentBlock
		toolBlocks   []toolBlock
		currentText  string
		currentTool  *toolBlock
		stopReason   string
		blockTypeMap = make(map[int]string)
		inputTokens  int
		outputTokens int
	)

	for event := range ch {
		switch event.Type {
		case api.EventError:
			return "", 0, 0, fmt.Errorf("stream error: %s", event.ErrorMessage)

		case api.EventMessageStart:
			inputTokens = event.InputTokens

		case api.EventContentBlockStart:
			blockTypeMap[event.Index] = event.ContentBlock.Type
			if event.ContentBlock.Type == "tool_use" {
				tb := toolBlock{
					id:   event.ContentBlock.ID,
					name: event.ContentBlock.Name,
				}
				toolBlocks = append(toolBlocks, tb)
				currentTool = &toolBlocks[len(toolBlocks)-1]
			}

		case api.EventContentBlockDelta:
			switch event.Delta.Type {
			case "text_delta":
				currentText += event.Delta.Text
				select {
				case events <- TurnEvent{Type: TurnEventTextDelta, Text: event.Delta.Text}:
				case <-ctx.Done():
					return "", 0, 0, ctx.Err()
				}
			case "input_json_delta":
				if currentTool != nil {
					currentTool.inputBuffer += event.Delta.PartialJSON
				}
			}

		case api.EventContentBlockStop:
			if bType, ok := blockTypeMap[event.Index]; ok && bType == "text" && currentText != "" {
				textBlocks = append(textBlocks, api.ContentBlock{Type: "text", Text: currentText})
				currentText = ""
			}
			currentTool = nil

		case api.EventMessageDelta:
			stopReason = event.StopReason
			outputTokens = event.Usage.OutputTokens

		case api.EventMessageStop:
			// stream complete
		}
	}

	// Build assistant message content
	var assistantContent []api.ContentBlock
	assistantContent = append(assistantContent, textBlocks...)

	for _, tb := range toolBlocks {
		var inputMap map[string]any
		if tb.inputBuffer != "" {
			if err := json.Unmarshal([]byte(tb.inputBuffer), &inputMap); err != nil {
				inputMap = map[string]any{"raw": tb.inputBuffer}
			}
		} else {
			inputMap = map[string]any{}
		}
		assistantContent = append(assistantContent, api.ContentBlock{
			Type:  "tool_use",
			ID:    tb.id,
			Name:  tb.name,
			Input: inputMap,
		})
	}

	if len(assistantContent) > 0 {
		loop.Session.Messages = append(loop.Session.Messages, api.Message{
			Role:    "assistant",
			Content: assistantContent,
		})
	}

	// Execute tools if needed
	if stopReason == "tool_use" {
		var toolResults []api.ContentBlock

		for _, tb := range toolBlocks {
			var inputMap map[string]any
			for _, cb := range assistantContent {
				if cb.Type == "tool_use" && cb.ID == tb.id {
					inputMap = cb.Input
					break
				}
			}
			if inputMap == nil {
				inputMap = map[string]any{}
			}

			summary := summarizeToolInput(inputMap)

			// --- Permission check (Phase 5) ---
			if loop.PermManager != nil {
				decision := loop.PermManager.Check(tb.name, summary)

				// Plan mode: describe without executing.
				if loop.PermManager.Mode == permissions.ModePlan {
					planResult := api.ContentBlock{
						Type:    "tool_result",
						ToolUseID: tb.id,
						Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("[Plan: %s %s]", tb.name, summary)}},
					}
					toolResults = append(toolResults, planResult)
					continue
				}

				switch decision {
				case permissions.DecisionDeny:
					denied := api.ContentBlock{
						Type:    "tool_result",
						ToolUseID: tb.id,
						Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("Permission denied for tool: %s", tb.name)}},
						IsError: true,
					}
					toolResults = append(toolResults, denied)
					continue

				case permissions.DecisionAsk:
					replyCh := make(chan PermDecision, 1)
					select {
					case events <- TurnEvent{
						Type:      TurnEventPermissionAsk,
						ToolName:  tb.name,
						ToolInput: summary,
						PermReply: replyCh,
					}:
					case <-ctx.Done():
						return "", 0, 0, ctx.Err()
					}

					var userDecision PermDecision
					select {
					case userDecision = <-replyCh:
					case <-ctx.Done():
						return "", 0, 0, ctx.Err()
					}

					switch userDecision {
					case PermDecisionDeny:
						denied := api.ContentBlock{
							Type:    "tool_result",
							ToolUseID: tb.id,
							Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("Permission denied for tool: %s", tb.name)}},
							IsError: true,
						}
						toolResults = append(toolResults, denied)
						continue
					case PermDecisionAllowAlways:
						loop.PermManager.Remember(tb.name, summary, permissions.DecisionAllow, permissions.ScopeAlways)
					}
					// PermDecisionAllowOnce falls through to execution
				}
				// DecisionAllow falls through to execution
			}

			select {
			case events <- TurnEvent{Type: TurnEventToolStart, ToolName: tb.name, ToolInput: summary}:
			case <-ctx.Done():
				return "", 0, 0, ctx.Err()
			}

			result := loop.ExecuteToolQuiet(tb.name, inputMap)
			result.ToolUseID = tb.id
			toolResults = append(toolResults, result)

			resultText := ""
			if len(result.Content) > 0 {
				resultText = result.Content[0].Text
			}
			select {
			case events <- TurnEvent{Type: TurnEventToolDone, ToolName: tb.name, ToolResult: resultText}:
			case <-ctx.Done():
				return "", 0, 0, ctx.Err()
			}
		}

		loop.Session.Messages = append(loop.Session.Messages, api.Message{
			Role:    "user",
			Content: toolResults,
		})
	}

	return stopReason, inputTokens, outputTokens, nil
}

// ExecuteToolQuiet dispatches to the appropriate tool without printing to stdout/stderr.
func (loop *ConversationLoop) ExecuteToolQuiet(name string, input map[string]any) api.ContentBlock {
	if !CheckPermission(loop.Permissions, name) {
		return api.ContentBlock{
			Type:    "tool_result",
			Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("Permission denied for tool: %s", name)}},
			IsError: true,
		}
	}

	var result string
	var err error

	switch name {
	case "bash":
		result, err = tools.ExecuteBash(input)
	case "read_file":
		result, err = tools.ExecuteReadFile(input)
	case "write_file":
		result, err = tools.ExecuteWriteFile(input)
	case "glob":
		result, err = tools.ExecuteGlob(input)
	case "grep":
		result, err = tools.ExecuteGrep(input)
	default:
		// Fall back to MCP registry.
		if loop.MCPRegistry != nil {
			if client, _, ok := loop.MCPRegistry.FindTool(name); ok {
				mcpResult, mcpErr := client.CallTool(context.Background(), name, input)
				if mcpErr != nil {
					return api.ContentBlock{
						Type:    "tool_result",
						Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", mcpErr)}},
						IsError: true,
					}
				}
				text := mcpResultText(mcpResult)
				return api.ContentBlock{
					Type:    "tool_result",
					Content: []api.ContentBlock{{Type: "text", Text: text}},
					IsError: mcpResult.IsError,
				}
			}
		}
		err = fmt.Errorf("unknown tool: %s", name)
	}

	isError := err != nil
	text := result
	if err != nil {
		text = fmt.Sprintf("Error: %v", err)
	}

	return api.ContentBlock{
		Type:    "tool_result",
		Content: []api.ContentBlock{{Type: "text", Text: text}},
		IsError: isError,
	}
}

// summarizeToolInput returns a short human-readable summary of tool inputs.
func summarizeToolInput(input map[string]any) string {
	for _, key := range []string{"command", "path", "file_path", "pattern"} {
		if v, ok := input[key].(string); ok {
			if len(v) > 60 {
				return v[:60] + "..."
			}
			return v
		}
	}
	return ""
}

// ClearSession resets the conversation history in the current session.
func (loop *ConversationLoop) ClearSession() {
	loop.Session.Messages = []api.Message{}
}

// ListSessions returns all session IDs saved in the configured session directory.
func (loop *ConversationLoop) ListSessions() ([]string, error) {
	return ListSessions(loop.Config.SessionDir)
}

// ExecuteTool dispatches to the appropriate tool implementation.
func (loop *ConversationLoop) ExecuteTool(name string, input map[string]any) api.ContentBlock {
	if !CheckPermission(loop.Permissions, name) {
		return api.ContentBlock{
			Type:    "tool_result",
			Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("Permission denied for tool: %s", name)}},
			IsError: true,
		}
	}

	var result string
	var err error

	switch name {
	case "bash":
		result, err = tools.ExecuteBash(input)
	case "read_file":
		result, err = tools.ExecuteReadFile(input)
	case "write_file":
		result, err = tools.ExecuteWriteFile(input)
	case "glob":
		result, err = tools.ExecuteGlob(input)
	case "grep":
		result, err = tools.ExecuteGrep(input)
	default:
		// Fall back to MCP registry.
		if loop.MCPRegistry != nil {
			if client, _, ok := loop.MCPRegistry.FindTool(name); ok {
				mcpResult, mcpErr := client.CallTool(context.Background(), name, input)
				if mcpErr != nil {
					fmt.Fprintf(os.Stderr, "[MCP tool %s error]: %v\n", name, mcpErr)
					return api.ContentBlock{
						Type:    "tool_result",
						Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", mcpErr)}},
						IsError: true,
					}
				}
				text := mcpResultText(mcpResult)
				fmt.Fprintf(os.Stdout, "%s\n", text)
				return api.ContentBlock{
					Type:    "tool_result",
					Content: []api.ContentBlock{{Type: "text", Text: text}},
					IsError: mcpResult.IsError,
				}
			}
		}
		err = fmt.Errorf("unknown tool: %s", name)
	}

	isError := err != nil
	text := result
	if err != nil {
		text = fmt.Sprintf("Error: %v", err)
		fmt.Fprintf(os.Stderr, "[Tool %s error]: %v\n", name, err)
	} else {
		fmt.Fprintf(os.Stdout, "%s\n", result)
	}

	return api.ContentBlock{
		Type: "tool_result",
		Content: []api.ContentBlock{
			{Type: "text", Text: text},
		},
		IsError: isError,
	}
}

// mcpResultText extracts the concatenated text from an MCP tool result.
func mcpResultText(r mcp.MCPToolResult) string {
	var parts []string
	for _, c := range r.Content {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// InitMCPFromConfig connects to all MCP servers defined in the config.
// Errors are printed but do not abort startup.
func (loop *ConversationLoop) InitMCPFromConfig(ctx context.Context) {
	if len(loop.Config.MCPServers) == 0 {
		return
	}
	if loop.MCPRegistry == nil {
		loop.MCPRegistry = mcp.NewRegistry()
	}
	for _, srv := range loop.Config.MCPServers {
		var transport mcp.Transport
		var err error

		switch strings.ToLower(srv.Transport) {
		case "stdio":
			var envPairs []string
			for k, v := range srv.Env {
				envPairs = append(envPairs, k+"="+v)
			}
			transport, err = mcp.NewStdioTransport(srv.Command, srv.Args, envPairs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[MCP] failed to start stdio server %q: %v\n", srv.Name, err)
				continue
			}
		case "sse", "http":
			auth := ""
			if tok, ok := srv.Env["AUTHORIZATION"]; ok {
				auth = tok
			}
			transport = mcp.NewSSETransport(srv.URL, auth)
		default:
			fmt.Fprintf(os.Stderr, "[MCP] unknown transport %q for server %q\n", srv.Transport, srv.Name)
			continue
		}

		if err := loop.MCPRegistry.AddServer(ctx, srv.Name, transport); err != nil {
			fmt.Fprintf(os.Stderr, "[MCP] failed to connect to server %q: %v\n", srv.Name, err)
		} else {
			toolCount := len(loop.MCPRegistry.ServerTools(srv.Name))
			fmt.Fprintf(os.Stdout, "[MCP] connected to %q (%d tools)\n", srv.Name, toolCount)
		}
	}
}

// MCPConnect connects to a named MCP server defined in config.
func (loop *ConversationLoop) MCPConnect(ctx context.Context, name string) error {
	if loop.MCPRegistry == nil {
		loop.MCPRegistry = mcp.NewRegistry()
	}
	for _, srv := range loop.Config.MCPServers {
		if srv.Name != name {
			continue
		}
		var transport mcp.Transport
		var err error
		switch strings.ToLower(srv.Transport) {
		case "stdio":
			var envPairs []string
			for k, v := range srv.Env {
				envPairs = append(envPairs, k+"="+v)
			}
			transport, err = mcp.NewStdioTransport(srv.Command, srv.Args, envPairs)
			if err != nil {
				return err
			}
		case "sse", "http":
			auth := ""
			if tok, ok := srv.Env["AUTHORIZATION"]; ok {
				auth = tok
			}
			transport = mcp.NewSSETransport(srv.URL, auth)
		default:
			return fmt.Errorf("unknown transport %q", srv.Transport)
		}
		return loop.MCPRegistry.AddServer(ctx, name, transport)
	}
	return fmt.Errorf("MCP server %q not found in config", name)
}

// MCPDisconnect disconnects from a named MCP server.
func (loop *ConversationLoop) MCPDisconnect(name string) error {
	if loop.MCPRegistry == nil {
		return fmt.Errorf("no MCP servers connected")
	}
	return loop.MCPRegistry.Disconnect(name)
}

// MCPList returns a human-readable summary of connected MCP servers and their tools.
func (loop *ConversationLoop) MCPList() string {
	if loop.MCPRegistry == nil {
		return "No MCP servers connected.\n"
	}
	names := loop.MCPRegistry.ServerNames()
	if len(names) == 0 {
		return "No MCP servers connected.\n"
	}
	var sb strings.Builder
	for _, name := range names {
		tools := loop.MCPRegistry.ServerTools(name)
		fmt.Fprintf(&sb, "Server: %s (%d tools)\n", name, len(tools))
		for _, t := range tools {
			desc := t.Description
			if len(desc) > 60 {
				desc = desc[:60] + "..."
			}
			fmt.Fprintf(&sb, "  - %s: %s\n", t.Name, desc)
		}
	}
	return sb.String()
}
