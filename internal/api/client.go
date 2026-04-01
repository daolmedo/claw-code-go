package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultBaseURL        = "https://api.anthropic.com"
	anthropicVersion      = "2023-06-01"
	anthropicBetaHeader   = "anthropic-beta"
	anthropicVersionHeader = "anthropic-version"
)

// Client is the Anthropic API client.
type Client struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// NewClient creates a new API client with the given API key and model.
func NewClient(apiKey, model string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
		Model:      model,
		HTTPClient: &http.Client{},
	}
}

// StreamResponse sends a streaming message request and returns a channel of StreamEvents.
// The channel is closed when the stream ends or an error occurs.
func (c *Client) StreamResponse(ctx context.Context, req CreateMessageRequest) (<-chan StreamEvent, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set(anthropicVersionHeader, anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		// Increase buffer for large tool inputs
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and event: lines (type is also in the JSON)
			if line == "" || strings.HasPrefix(line, "event:") {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			event, err := parseSSEData(data)
			if err != nil {
				// Send an error event
				ch <- StreamEvent{
					Type:         EventError,
					ErrorMessage: fmt.Sprintf("parse SSE: %v", err),
				}
				continue
			}

			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			select {
			case ch <- StreamEvent{
				Type:         EventError,
				ErrorMessage: fmt.Sprintf("scanner: %v", err),
			}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// parseSSEData parses a single SSE data line into a StreamEvent.
func parseSSEData(data string) (StreamEvent, error) {
	// Parse into a raw map first to handle the varying structure
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return StreamEvent{}, fmt.Errorf("unmarshal raw: %w", err)
	}

	var event StreamEvent

	// Parse "type"
	if typeRaw, ok := raw["type"]; ok {
		var t string
		if err := json.Unmarshal(typeRaw, &t); err == nil {
			event.Type = StreamEventType(t)
		}
	}

	// Parse "index"
	if indexRaw, ok := raw["index"]; ok {
		json.Unmarshal(indexRaw, &event.Index) //nolint:errcheck
	}

	// Parse "delta" — used in content_block_delta and message_delta
	if deltaRaw, ok := raw["delta"]; ok {
		var deltaMap map[string]json.RawMessage
		if err := json.Unmarshal(deltaRaw, &deltaMap); err == nil {
			if typeRaw, ok := deltaMap["type"]; ok {
				json.Unmarshal(typeRaw, &event.Delta.Type) //nolint:errcheck
			}
			if textRaw, ok := deltaMap["text"]; ok {
				json.Unmarshal(textRaw, &event.Delta.Text) //nolint:errcheck
			}
			if partialRaw, ok := deltaMap["partial_json"]; ok {
				json.Unmarshal(partialRaw, &event.Delta.PartialJSON) //nolint:errcheck
			}
			// For message_delta, delta contains stop_reason
			if stopRaw, ok := deltaMap["stop_reason"]; ok {
				var stopReason string
				if err := json.Unmarshal(stopRaw, &stopReason); err == nil {
					event.StopReason = stopReason
				}
			}
		}
	}

	// Parse "content_block" for content_block_start events
	if cbRaw, ok := raw["content_block"]; ok {
		var cbMap map[string]json.RawMessage
		if err := json.Unmarshal(cbRaw, &cbMap); err == nil {
			if typeRaw, ok := cbMap["type"]; ok {
				json.Unmarshal(typeRaw, &event.ContentBlock.Type) //nolint:errcheck
			}
			if idRaw, ok := cbMap["id"]; ok {
				json.Unmarshal(idRaw, &event.ContentBlock.ID) //nolint:errcheck
			}
			if nameRaw, ok := cbMap["name"]; ok {
				json.Unmarshal(nameRaw, &event.ContentBlock.Name) //nolint:errcheck
			}
		}
		event.ContentBlock.Index = event.Index
	}

	// Parse "usage" for message_delta events
	if usageRaw, ok := raw["usage"]; ok {
		json.Unmarshal(usageRaw, &event.Usage) //nolint:errcheck
	}

	// Parse "message.usage.input_tokens" for message_start events
	if messageRaw, ok := raw["message"]; ok {
		var msgMap map[string]json.RawMessage
		if err := json.Unmarshal(messageRaw, &msgMap); err == nil {
			if usageRaw, ok := msgMap["usage"]; ok {
				var usage struct {
					InputTokens int `json:"input_tokens"`
				}
				if err := json.Unmarshal(usageRaw, &usage); err == nil {
					event.InputTokens = usage.InputTokens
				}
			}
		}
	}

	// Parse error details
	if event.Type == EventError {
		if errRaw, ok := raw["error"]; ok {
			var errMap map[string]json.RawMessage
			if err := json.Unmarshal(errRaw, &errMap); err == nil {
				if msgRaw, ok := errMap["message"]; ok {
					json.Unmarshal(msgRaw, &event.ErrorMessage) //nolint:errcheck
				}
			}
		}
	}

	return event, nil
}
