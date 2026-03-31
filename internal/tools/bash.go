package tools

import (
	"bytes"
	"claw-code-go/internal/api"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const (
	bashTimeout   = 30 * time.Second
	maxOutputSize = 10000
)

// BashTool returns the tool definition for the bash tool.
func BashTool() api.Tool {
	return api.Tool{
		Name:        "bash",
		Description: "Execute a bash command and return the output. Use this for running shell commands, scripts, and system operations.",
		InputSchema: api.InputSchema{
			Type: "object",
			Properties: map[string]api.Property{
				"command": {
					Type:        "string",
					Description: "The bash command to execute",
				},
			},
			Required: []string{"command"},
		},
	}
}

// ExecuteBash runs a bash command and returns combined stdout+stderr.
func ExecuteBash(input map[string]any) (string, error) {
	command, ok := input["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("bash: 'command' input is required and must be a string")
	}

	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	output := buf.String()

	// Truncate output if too long
	if len(output) > maxOutputSize {
		output = output[:maxOutputSize] + "\n... [output truncated]"
	}

	if err != nil {
		// Return output + error description; the caller decides if it's a hard error
		if ctx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("command timed out after %s", bashTimeout)
		}
		// For non-zero exit codes, return output with error appended
		return output, fmt.Errorf("command exited with error: %v", err)
	}

	return output, nil
}
