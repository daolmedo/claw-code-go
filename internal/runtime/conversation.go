package runtime

import (
	"claw-code-go/internal/api"
	"claw-code-go/internal/tools"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

const systemPrompt = `You are Claude Code, an AI assistant for software engineering tasks. You have access to tools for running bash commands, reading and writing files, searching with glob patterns, and grepping for patterns in code. Use these tools to help users with coding tasks.`

// ConversationLoop manages the agentic conversation loop with tool use.
type ConversationLoop struct {
	Client      *api.Client
	Session     *Session
	Tools       []api.Tool
	Permissions *Permissions
	Config      *Config
}

// NewConversationLoop creates a new conversation loop with default tools.
func NewConversationLoop(cfg *Config, apiKey string) *ConversationLoop {
	client := api.NewClient(apiKey, cfg.Model)
	if cfg.BaseURL != "" {
		client.BaseURL = cfg.BaseURL
	}

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

// SendMessage sends a user message and runs the full agentic loop.
func (loop *ConversationLoop) SendMessage(ctx context.Context, userText string) error {
	// Append user message
	loop.Session.Messages = append(loop.Session.Messages, api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: "text", Text: userText},
		},
	})

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
		System:    systemPrompt,
		Messages:  loop.Session.Messages,
		Tools:     loop.Tools,
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
