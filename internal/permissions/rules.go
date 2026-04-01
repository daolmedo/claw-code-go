package permissions

import (
	"encoding/json"
	"os"
	"strings"
)

// Decision is the outcome of a permission check.
type Decision int

const (
	DecisionAsk   Decision = iota // prompt the user
	DecisionAllow                 // proceed without asking
	DecisionDeny                  // block execution
)

// Scope controls how long a user-granted decision is remembered.
type Scope int

const (
	ScopeOnce   Scope = iota // apply only to this single invocation
	ScopeAlways              // cache for the rest of the session
)

// Rule is a single permission entry that matches a tool (and optionally an input substring).
type Rule struct {
	Tool     string   `json:"tool"`     // exact tool name, or "*" for any tool
	Pattern  string   `json:"pattern"`  // substring match on input summary (empty = match all)
	Decision Decision `json:"-"`        // resolved from the JSON "decision" field
	RawDecision string `json:"decision"` // "allow", "deny", "ask"
}

// Ruleset is an ordered list of rules; the first match wins.
type Ruleset struct {
	Rules []Rule
}

// Match returns the first Decision that matches the given tool and input summary.
// Returns (DecisionAsk, false) when no rule matches.
func (rs *Ruleset) Match(tool, input string) (Decision, bool) {
	if rs == nil {
		return DecisionAsk, false
	}
	for _, r := range rs.Rules {
		if r.Tool != "*" && r.Tool != tool {
			continue
		}
		if r.Pattern != "" && !strings.Contains(input, r.Pattern) {
			continue
		}
		return r.Decision, true
	}
	return DecisionAsk, false
}

// settingsFile is the on-disk format for .claude/settings.json.
type settingsFile struct {
	Rules []struct {
		Tool     string `json:"tool"`
		Pattern  string `json:"pattern"`
		Decision string `json:"decision"`
	} `json:"rules"`
}

// LoadRuleset reads .claude/settings.json relative to the current directory.
// If the file does not exist an empty (no-op) ruleset is returned without error.
func LoadRuleset(path string) (*Ruleset, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Ruleset{}, nil
	}
	if err != nil {
		return nil, err
	}

	var sf settingsFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}

	rs := &Ruleset{}
	for _, raw := range sf.Rules {
		r := Rule{
			Tool:    raw.Tool,
			Pattern: raw.Pattern,
		}
		switch strings.ToLower(raw.Decision) {
		case "allow":
			r.Decision = DecisionAllow
		case "deny":
			r.Decision = DecisionDeny
		default:
			r.Decision = DecisionAsk
		}
		rs.Rules = append(rs.Rules, r)
	}
	return rs, nil
}
