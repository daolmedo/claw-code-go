package runtime

// TurnEventType identifies the kind of streaming event from a conversation turn.
type TurnEventType int

const (
	TurnEventTextDelta TurnEventType = iota // streaming text chunk
	TurnEventToolStart                      // tool execution starting
	TurnEventToolDone                       // tool execution complete
	TurnEventUsage                          // token usage update
	TurnEventDone                           // turn fully complete
	TurnEventError                          // error occurred
)

// TurnEvent carries information from a streaming API turn to the caller.
type TurnEvent struct {
	Type         TurnEventType
	Text         string // TurnEventTextDelta: the chunk
	ToolName     string // TurnEventToolStart/Done: tool name
	ToolInput    string // TurnEventToolStart: brief input summary
	ToolResult   string // TurnEventToolDone: output excerpt
	InputTokens  int    // TurnEventUsage/Done: input token count
	OutputTokens int    // TurnEventUsage/Done: output token count
	Err          error  // TurnEventError: the error
}
