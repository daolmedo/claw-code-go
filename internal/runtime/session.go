package runtime

import (
	"claw-code-go/internal/api"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
