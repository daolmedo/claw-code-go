package runtime

import (
	"claw-code-go/internal/api"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session holds a conversation session with its messages.
type Session struct {
	ID        string        `json:"id"`
	Messages  []api.Message `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`

	// Compaction state (Phase 6).
	// CompactionSummary holds the most recent compaction summary text. It is
	// injected into the system prompt so the model retains earlier context.
	CompactionSummary string `json:"compaction_summary,omitempty"`
	// CompactionCount is the number of times this session has been compacted.
	CompactionCount int `json:"compaction_count,omitempty"`

	// Usage tracking (Phase 13). Persisted so cost history survives resume.
	TotalInputTokens  int `json:"total_input_tokens,omitempty"`
	TotalOutputTokens int `json:"total_output_tokens,omitempty"`
	TotalTurns        int `json:"total_turns,omitempty"`
}

// NewSession creates a new session with a unique ID based on timestamp.
func NewSession() *Session {
	now := time.Now()
	id := fmt.Sprintf("session-%d", now.UnixNano())
	return &Session{
		ID:        id,
		Messages:  []api.Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// SaveSession persists a session to disk as JSON.
func SaveSession(dir string, s *Session) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	s.UpdatedAt = time.Now()

	path := filepath.Join(dir, s.ID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk by ID.
func LoadSession(dir, id string) (*Session, error) {
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &s, nil
}

// ListSessions returns all session IDs in the given directory.
func ListSessions(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") {
			ids = append(ids, strings.TrimSuffix(name, ".json"))
		}
	}

	return ids, nil
}

// SessionMeta holds lightweight metadata for a saved session without loading
// the full message slice.
type SessionMeta struct {
	ID                string
	UpdatedAt         time.Time
	MessageCount      int
	TotalInputTokens  int
	TotalOutputTokens int
	TotalTurns        int
}

// ListSessionsWithMeta returns metadata for all saved sessions, sorted newest
// first. Sessions that cannot be parsed are silently skipped.
func ListSessionsWithMeta(dir string) ([]SessionMeta, error) {
	ids, err := ListSessions(dir)
	if err != nil {
		return nil, err
	}
	var metas []SessionMeta
	for _, id := range ids {
		s, err := LoadSession(dir, id)
		if err != nil {
			continue
		}
		metas = append(metas, SessionMeta{
			ID:                id,
			UpdatedAt:         s.UpdatedAt,
			MessageCount:      len(s.Messages),
			TotalInputTokens:  s.TotalInputTokens,
			TotalOutputTokens: s.TotalOutputTokens,
			TotalTurns:        s.TotalTurns,
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})
	return metas, nil
}
