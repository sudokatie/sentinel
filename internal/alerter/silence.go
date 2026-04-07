package alerter

import (
	"sync"
	"time"
)

// Silence represents a time period during which alerts are suppressed.
type Silence struct {
	ID        string
	CheckID   *int64   // Specific check, or nil for all checks
	Matchers  []string // Additional matchers (e.g., "name=api*")
	StartsAt  time.Time
	EndsAt    time.Time
	CreatedBy string
	Comment   string
	CreatedAt time.Time
}

// IsActive returns true if the silence is currently active.
func (s *Silence) IsActive() bool {
	now := time.Now()
	return now.After(s.StartsAt) && now.Before(s.EndsAt)
}

// IsExpired returns true if the silence has ended.
func (s *Silence) IsExpired() bool {
	return time.Now().After(s.EndsAt)
}

// Matches returns true if this silence applies to the given check.
func (s *Silence) Matches(checkID int64, checkName string) bool {
	// If CheckID is set, only match that specific check
	if s.CheckID != nil && *s.CheckID != checkID {
		return false
	}

	// If no matchers, match all (when CheckID is nil)
	if len(s.Matchers) == 0 {
		return true
	}

	// Check matchers (simple glob matching)
	for _, matcher := range s.Matchers {
		if matchGlob(matcher, checkName) {
			return true
		}
	}

	return false
}

// SilenceManager manages alert silences.
type SilenceManager struct {
	mu       sync.RWMutex
	silences map[string]*Silence
	nextID   int
}

// NewSilenceManager creates a new silence manager.
func NewSilenceManager() *SilenceManager {
	return &SilenceManager{
		silences: make(map[string]*Silence),
		nextID:   1,
	}
}

// Add creates a new silence.
func (m *SilenceManager) Add(silence *Silence) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if silence.ID == "" {
		silence.ID = m.generateID()
	}
	silence.CreatedAt = time.Now()
	m.silences[silence.ID] = silence

	return silence.ID
}

// Remove deletes a silence by ID.
func (m *SilenceManager) Remove(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.silences[id]; exists {
		delete(m.silences, id)
		return true
	}
	return false
}

// Get returns a silence by ID.
func (m *SilenceManager) Get(id string) *Silence {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.silences[id]
}

// Active returns all active silences.
func (m *SilenceManager) Active() []*Silence {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*Silence
	for _, s := range m.silences {
		if s.IsActive() {
			active = append(active, s)
		}
	}
	return active
}

// All returns all silences (including expired).
func (m *SilenceManager) All() []*Silence {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*Silence, 0, len(m.silences))
	for _, s := range m.silences {
		all = append(all, s)
	}
	return all
}

// IsSilenced checks if alerts for a check should be suppressed.
func (m *SilenceManager) IsSilenced(checkID int64, checkName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.silences {
		if s.IsActive() && s.Matches(checkID, checkName) {
			return true
		}
	}
	return false
}

// Cleanup removes expired silences.
func (m *SilenceManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	for id, s := range m.silences {
		if s.IsExpired() {
			delete(m.silences, id)
			removed++
		}
	}
	return removed
}

// SilenceForDuration creates a convenience silence for a specific duration.
func SilenceForDuration(checkID *int64, duration time.Duration, comment string, createdBy string) *Silence {
	now := time.Now()
	return &Silence{
		CheckID:   checkID,
		StartsAt:  now,
		EndsAt:    now.Add(duration),
		Comment:   comment,
		CreatedBy: createdBy,
	}
}

// SilenceUntil creates a convenience silence until a specific time.
func SilenceUntil(checkID *int64, until time.Time, comment string, createdBy string) *Silence {
	return &Silence{
		CheckID:   checkID,
		StartsAt:  time.Now(),
		EndsAt:    until,
		Comment:   comment,
		CreatedBy: createdBy,
	}
}

func (m *SilenceManager) generateID() string {
	id := m.nextID
	m.nextID++
	return time.Now().Format("20060102") + "-" + string(rune('A'+id%26)) + string(rune('0'+id/26%10))
}

// matchGlob performs simple glob matching with * wildcard.
func matchGlob(pattern, str string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == str {
		return true
	}

	// Simple prefix matching with trailing *
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	}

	// Simple suffix matching with leading *
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(str) >= len(suffix) && str[len(str)-len(suffix):] == suffix
	}

	return false
}
