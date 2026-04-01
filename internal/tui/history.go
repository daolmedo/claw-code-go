package tui

const maxHistoryEntries = 500

// inputHistory maintains a ring buffer of submitted messages for ↑/↓ navigation.
type inputHistory struct {
	entries []string
	pos     int    // navigation index; -1 when not navigating
	draft   string // text saved when navigation begins
}

func newInputHistory() *inputHistory {
	return &inputHistory{pos: -1}
}

// Push adds a new entry. Ignores empty strings and consecutive duplicates.
func (h *inputHistory) Push(s string) {
	if s == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == s {
		return
	}
	h.entries = append(h.entries, s)
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[1:]
	}
	h.pos = -1
}

// Prev navigates to the previous (older) entry.
// currentText is saved as a draft when navigation begins.
func (h *inputHistory) Prev(currentText string) string {
	if len(h.entries) == 0 {
		return currentText
	}
	if h.pos == -1 {
		h.draft = currentText
		h.pos = len(h.entries) - 1
	} else if h.pos > 0 {
		h.pos--
	}
	return h.entries[h.pos]
}

// Next navigates to the next (newer) entry; returns the draft when past the newest.
func (h *inputHistory) Next() string {
	if h.pos == -1 {
		return ""
	}
	h.pos++
	if h.pos >= len(h.entries) {
		h.pos = -1
		return h.draft
	}
	return h.entries[h.pos]
}

// Reset exits navigation mode. Call after any non-navigation edit.
func (h *inputHistory) Reset() {
	h.pos = -1
	h.draft = ""
}
