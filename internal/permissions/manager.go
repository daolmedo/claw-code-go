package permissions

import "sync"

// Manager enforces permissions for tool execution according to a mode and ruleset.
type Manager struct {
	Mode  PermissionMode
	Rules *Ruleset

	mu    sync.Mutex
	cache map[string]Decision // keyed by tool name for ScopeAlways grants
}

// NewManager creates a Manager with the given mode and ruleset.
// If rules is nil an empty ruleset is used.
func NewManager(mode PermissionMode, rules *Ruleset) *Manager {
	if rules == nil {
		rules = &Ruleset{}
	}
	return &Manager{
		Mode:  mode,
		Rules: rules,
		cache: make(map[string]Decision),
	}
}

// Check returns the Decision for the given tool and input summary.
//
//   - BypassPermissions → always Allow
//   - Plan             → always Deny
//   - Otherwise        → consult session cache, then ruleset, then Ask
func (m *Manager) Check(tool, input string) Decision {
	switch m.Mode {
	case ModeBypassPermissions:
		return DecisionAllow
	case ModePlan:
		return DecisionDeny
	}

	m.mu.Lock()
	if d, ok := m.cache[tool]; ok {
		m.mu.Unlock()
		return d
	}
	m.mu.Unlock()

	if d, ok := m.Rules.Match(tool, input); ok {
		return d
	}

	// AcceptEdits: auto-allow read-only and edit tools, ask for bash
	if m.Mode == ModeAcceptEdits {
		switch tool {
		case "read_file", "glob", "grep", "write_file":
			return DecisionAllow
		}
	}

	return DecisionAsk
}

// Remember stores a user decision in the session cache for ScopeAlways grants.
// ScopeOnce decisions are not cached.
func (m *Manager) Remember(tool, _ string, decision Decision, scope Scope) {
	if scope != ScopeAlways {
		return
	}
	m.mu.Lock()
	m.cache[tool] = decision
	m.mu.Unlock()
}
