package alerter

import (
	"testing"
	"time"
)

func TestSilenceIsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		silence  *Silence
		expected bool
	}{
		{
			name: "active silence",
			silence: &Silence{
				StartsAt: now.Add(-1 * time.Hour),
				EndsAt:   now.Add(1 * time.Hour),
			},
			expected: true,
		},
		{
			name: "future silence",
			silence: &Silence{
				StartsAt: now.Add(1 * time.Hour),
				EndsAt:   now.Add(2 * time.Hour),
			},
			expected: false,
		},
		{
			name: "expired silence",
			silence: &Silence{
				StartsAt: now.Add(-2 * time.Hour),
				EndsAt:   now.Add(-1 * time.Hour),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.silence.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSilenceIsExpired(t *testing.T) {
	now := time.Now()

	expired := &Silence{EndsAt: now.Add(-1 * time.Hour)}
	if !expired.IsExpired() {
		t.Error("expected silence to be expired")
	}

	active := &Silence{EndsAt: now.Add(1 * time.Hour)}
	if active.IsExpired() {
		t.Error("expected silence to not be expired")
	}
}

func TestSilenceMatches(t *testing.T) {
	checkID := int64(42)

	tests := []struct {
		name      string
		silence   *Silence
		checkID   int64
		checkName string
		expected  bool
	}{
		{
			name:      "match all",
			silence:   &Silence{},
			checkID:   1,
			checkName: "anything",
			expected:  true,
		},
		{
			name:      "match specific check ID",
			silence:   &Silence{CheckID: &checkID},
			checkID:   42,
			checkName: "test",
			expected:  true,
		},
		{
			name:      "no match different check ID",
			silence:   &Silence{CheckID: &checkID},
			checkID:   99,
			checkName: "test",
			expected:  false,
		},
		{
			name:      "match prefix glob",
			silence:   &Silence{Matchers: []string{"api*"}},
			checkID:   1,
			checkName: "api-health",
			expected:  true,
		},
		{
			name:      "match suffix glob",
			silence:   &Silence{Matchers: []string{"*-prod"}},
			checkID:   1,
			checkName: "api-prod",
			expected:  true,
		},
		{
			name:      "no match glob",
			silence:   &Silence{Matchers: []string{"api*"}},
			checkID:   1,
			checkName: "web-health",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.silence.Matches(tt.checkID, tt.checkName); got != tt.expected {
				t.Errorf("Matches() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSilenceManagerAdd(t *testing.T) {
	m := NewSilenceManager()

	silence := &Silence{
		StartsAt: time.Now(),
		EndsAt:   time.Now().Add(1 * time.Hour),
		Comment:  "test silence",
	}

	id := m.Add(silence)
	if id == "" {
		t.Error("expected non-empty ID")
	}

	got := m.Get(id)
	if got == nil {
		t.Error("expected to find silence")
	}
	if got.Comment != "test silence" {
		t.Errorf("wrong comment: %s", got.Comment)
	}
}

func TestSilenceManagerRemove(t *testing.T) {
	m := NewSilenceManager()

	silence := &Silence{
		StartsAt: time.Now(),
		EndsAt:   time.Now().Add(1 * time.Hour),
	}

	id := m.Add(silence)
	if !m.Remove(id) {
		t.Error("expected Remove to return true")
	}

	if m.Get(id) != nil {
		t.Error("silence should be removed")
	}

	if m.Remove("nonexistent") {
		t.Error("expected Remove to return false for nonexistent")
	}
}

func TestSilenceManagerActive(t *testing.T) {
	m := NewSilenceManager()
	now := time.Now()

	// Add active silence
	m.Add(&Silence{
		StartsAt: now.Add(-1 * time.Hour),
		EndsAt:   now.Add(1 * time.Hour),
	})

	// Add expired silence
	m.Add(&Silence{
		StartsAt: now.Add(-2 * time.Hour),
		EndsAt:   now.Add(-1 * time.Hour),
	})

	active := m.Active()
	if len(active) != 1 {
		t.Errorf("expected 1 active silence, got %d", len(active))
	}
}

func TestSilenceManagerIsSilenced(t *testing.T) {
	m := NewSilenceManager()
	now := time.Now()

	checkID := int64(42)
	m.Add(&Silence{
		CheckID:  &checkID,
		StartsAt: now.Add(-1 * time.Hour),
		EndsAt:   now.Add(1 * time.Hour),
	})

	if !m.IsSilenced(42, "test") {
		t.Error("expected check 42 to be silenced")
	}

	if m.IsSilenced(99, "test") {
		t.Error("expected check 99 to not be silenced")
	}
}

func TestSilenceManagerCleanup(t *testing.T) {
	m := NewSilenceManager()
	now := time.Now()

	// Add expired silences
	m.Add(&Silence{
		StartsAt: now.Add(-2 * time.Hour),
		EndsAt:   now.Add(-1 * time.Hour),
	})
	m.Add(&Silence{
		StartsAt: now.Add(-3 * time.Hour),
		EndsAt:   now.Add(-2 * time.Hour),
	})

	// Add active silence
	m.Add(&Silence{
		StartsAt: now.Add(-1 * time.Hour),
		EndsAt:   now.Add(1 * time.Hour),
	})

	removed := m.Cleanup()
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	all := m.All()
	if len(all) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(all))
	}
}

func TestSilenceForDuration(t *testing.T) {
	checkID := int64(1)
	s := SilenceForDuration(&checkID, 2*time.Hour, "maintenance", "admin")

	if s.CheckID == nil || *s.CheckID != 1 {
		t.Error("wrong check ID")
	}

	if s.Comment != "maintenance" {
		t.Errorf("wrong comment: %s", s.Comment)
	}

	if s.CreatedBy != "admin" {
		t.Errorf("wrong created by: %s", s.CreatedBy)
	}

	duration := s.EndsAt.Sub(s.StartsAt)
	if duration < 1*time.Hour || duration > 3*time.Hour {
		t.Errorf("unexpected duration: %v", duration)
	}
}

func TestSilenceUntil(t *testing.T) {
	until := time.Now().Add(24 * time.Hour)
	s := SilenceUntil(nil, until, "scheduled maintenance", "system")

	if s.CheckID != nil {
		t.Error("expected nil check ID")
	}

	if !s.EndsAt.Equal(until) {
		t.Errorf("wrong end time: %v", s.EndsAt)
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern  string
		str      string
		expected bool
	}{
		{"*", "anything", true},
		{"exact", "exact", true},
		{"exact", "different", false},
		{"api*", "api-health", true},
		{"api*", "web-health", false},
		{"*-prod", "api-prod", true},
		{"*-prod", "api-staging", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.str, func(t *testing.T) {
			if got := matchGlob(tt.pattern, tt.str); got != tt.expected {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.str, got, tt.expected)
			}
		})
	}
}
