package permissions

import "fmt"

// PermissionMode controls the overall permission enforcement strategy.
type PermissionMode int

const (
	ModeDefault          PermissionMode = iota // normal: consult ruleset, ask when needed
	ModeAcceptEdits                            // auto-allow edit tools, ask for bash
	ModeBypassPermissions                      // allow everything without asking
	ModePlan                                   // deny all execution; describe only
)

// ParsePermissionMode converts a CLI string to a PermissionMode.
func ParsePermissionMode(s string) (PermissionMode, error) {
	switch s {
	case "default", "":
		return ModeDefault, nil
	case "accept-edits":
		return ModeAcceptEdits, nil
	case "bypass":
		return ModeBypassPermissions, nil
	case "plan":
		return ModePlan, nil
	default:
		return ModeDefault, fmt.Errorf("unknown permission mode %q (want: default, accept-edits, bypass, plan)", s)
	}
}

// String returns the CLI string representation of the mode.
func (m PermissionMode) String() string {
	switch m {
	case ModeDefault:
		return "default"
	case ModeAcceptEdits:
		return "accept-edits"
	case ModeBypassPermissions:
		return "bypass"
	case ModePlan:
		return "plan"
	default:
		return "unknown"
	}
}
