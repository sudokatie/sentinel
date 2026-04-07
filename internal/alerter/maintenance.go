package alerter

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Schedule represents how often a maintenance window recurs.
type Schedule string

const (
	ScheduleOnce    Schedule = "once"
	ScheduleDaily   Schedule = "daily"
	ScheduleWeekly  Schedule = "weekly"
	ScheduleMonthly Schedule = "monthly"
)

// MaintenanceWindow represents a scheduled maintenance period.
type MaintenanceWindow struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Schedule    Schedule  `json:"schedule"`
	StartTime   time.Time `json:"start_time"`  // First occurrence start
	Duration    time.Duration `json:"duration"`
	DayOfWeek   *time.Weekday `json:"day_of_week,omitempty"` // For weekly schedule
	DayOfMonth  *int          `json:"day_of_month,omitempty"` // For monthly schedule
	CheckIDs    []int64       `json:"check_ids,omitempty"` // Empty means all checks
	Matchers    []string      `json:"matchers,omitempty"`  // Glob patterns
	Enabled     bool          `json:"enabled"`
	CreatedBy   string        `json:"created_by"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// IsActiveAt returns true if the maintenance window is active at the given time.
func (mw *MaintenanceWindow) IsActiveAt(t time.Time) bool {
	if !mw.Enabled {
		return false
	}

	switch mw.Schedule {
	case ScheduleOnce:
		return mw.isInWindow(t, mw.StartTime)
	case ScheduleDaily:
		return mw.isDailyActive(t)
	case ScheduleWeekly:
		return mw.isWeeklyActive(t)
	case ScheduleMonthly:
		return mw.isMonthlyActive(t)
	default:
		return false
	}
}

// IsActive returns true if the maintenance window is currently active.
func (mw *MaintenanceWindow) IsActive() bool {
	return mw.IsActiveAt(time.Now())
}

// NextOccurrence returns the next start time of this maintenance window.
func (mw *MaintenanceWindow) NextOccurrence(after time.Time) time.Time {
	switch mw.Schedule {
	case ScheduleOnce:
		if mw.StartTime.After(after) {
			return mw.StartTime
		}
		return time.Time{} // No next occurrence
	case ScheduleDaily:
		return mw.nextDaily(after)
	case ScheduleWeekly:
		return mw.nextWeekly(after)
	case ScheduleMonthly:
		return mw.nextMonthly(after)
	default:
		return time.Time{}
	}
}

// Matches returns true if this maintenance window applies to the given check.
func (mw *MaintenanceWindow) Matches(checkID int64, checkName string) bool {
	// If specific check IDs, must be in list
	if len(mw.CheckIDs) > 0 {
		found := false
		for _, id := range mw.CheckIDs {
			if id == checkID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// If no matchers, match all
	if len(mw.Matchers) == 0 {
		return true
	}

	// Check matchers
	for _, matcher := range mw.Matchers {
		if matchGlob(matcher, checkName) {
			return true
		}
	}

	return false
}

func (mw *MaintenanceWindow) isInWindow(t time.Time, windowStart time.Time) bool {
	windowEnd := windowStart.Add(mw.Duration)
	return !t.Before(windowStart) && t.Before(windowEnd)
}

func (mw *MaintenanceWindow) isDailyActive(t time.Time) bool {
	// Get the time of day from StartTime
	startHour, startMin, startSec := mw.StartTime.Clock()
	
	// Create today's window start
	todayStart := time.Date(t.Year(), t.Month(), t.Day(), startHour, startMin, startSec, 0, t.Location())
	
	return mw.isInWindow(t, todayStart)
}

func (mw *MaintenanceWindow) isWeeklyActive(t time.Time) bool {
	if mw.DayOfWeek == nil {
		return false
	}

	if t.Weekday() != *mw.DayOfWeek {
		return false
	}

	return mw.isDailyActive(t)
}

func (mw *MaintenanceWindow) isMonthlyActive(t time.Time) bool {
	if mw.DayOfMonth == nil {
		return false
	}

	if t.Day() != *mw.DayOfMonth {
		return false
	}

	return mw.isDailyActive(t)
}

func (mw *MaintenanceWindow) nextDaily(after time.Time) time.Time {
	startHour, startMin, startSec := mw.StartTime.Clock()
	
	// Try today
	todayStart := time.Date(after.Year(), after.Month(), after.Day(), startHour, startMin, startSec, 0, after.Location())
	if todayStart.After(after) {
		return todayStart
	}
	
	// Tomorrow
	return todayStart.AddDate(0, 0, 1)
}

func (mw *MaintenanceWindow) nextWeekly(after time.Time) time.Time {
	if mw.DayOfWeek == nil {
		return time.Time{}
	}

	startHour, startMin, startSec := mw.StartTime.Clock()
	
	// Find next occurrence of the day of week
	current := after
	for i := 0; i < 8; i++ {
		if current.Weekday() == *mw.DayOfWeek {
			candidate := time.Date(current.Year(), current.Month(), current.Day(), startHour, startMin, startSec, 0, current.Location())
			if candidate.After(after) {
				return candidate
			}
		}
		current = current.AddDate(0, 0, 1)
	}
	
	return time.Time{}
}

func (mw *MaintenanceWindow) nextMonthly(after time.Time) time.Time {
	if mw.DayOfMonth == nil {
		return time.Time{}
	}

	startHour, startMin, startSec := mw.StartTime.Clock()
	day := *mw.DayOfMonth
	
	// Try this month
	thisMonth := time.Date(after.Year(), after.Month(), day, startHour, startMin, startSec, 0, after.Location())
	if thisMonth.After(after) && thisMonth.Day() == day { // Check day didn't overflow
		return thisMonth
	}
	
	// Next month
	nextMonth := time.Date(after.Year(), after.Month()+1, day, startHour, startMin, startSec, 0, after.Location())
	if nextMonth.Day() == day {
		return nextMonth
	}
	
	// Month after (handle short months)
	monthAfter := time.Date(after.Year(), after.Month()+2, day, startHour, startMin, startSec, 0, after.Location())
	return monthAfter
}

// MaintenanceManager manages maintenance windows.
type MaintenanceManager struct {
	mu      sync.RWMutex
	windows map[string]*MaintenanceWindow
	nextID  int
}

// NewMaintenanceManager creates a new maintenance manager.
func NewMaintenanceManager() *MaintenanceManager {
	return &MaintenanceManager{
		windows: make(map[string]*MaintenanceWindow),
		nextID:  1,
	}
}

// Add creates a new maintenance window.
func (m *MaintenanceManager) Add(mw *MaintenanceWindow) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mw.ID == "" {
		mw.ID = fmt.Sprintf("mw-%d", m.nextID)
		m.nextID++
	}
	now := time.Now()
	mw.CreatedAt = now
	mw.UpdatedAt = now
	m.windows[mw.ID] = mw

	return mw.ID
}

// Update modifies an existing maintenance window.
func (m *MaintenanceManager) Update(mw *MaintenanceWindow) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.windows[mw.ID]; !exists {
		return false
	}
	mw.UpdatedAt = time.Now()
	m.windows[mw.ID] = mw
	return true
}

// Remove deletes a maintenance window by ID.
func (m *MaintenanceManager) Remove(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.windows[id]; exists {
		delete(m.windows, id)
		return true
	}
	return false
}

// Get returns a maintenance window by ID.
func (m *MaintenanceManager) Get(id string) *MaintenanceWindow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.windows[id]
}

// Active returns all currently active maintenance windows.
func (m *MaintenanceManager) Active() []*MaintenanceWindow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*MaintenanceWindow
	now := time.Now()
	for _, mw := range m.windows {
		if mw.IsActiveAt(now) {
			active = append(active, mw)
		}
	}
	return active
}

// All returns all maintenance windows.
func (m *MaintenanceManager) All() []*MaintenanceWindow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*MaintenanceWindow, 0, len(m.windows))
	for _, mw := range m.windows {
		all = append(all, mw)
	}
	return all
}

// Enabled returns all enabled maintenance windows.
func (m *MaintenanceManager) Enabled() []*MaintenanceWindow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var enabled []*MaintenanceWindow
	for _, mw := range m.windows {
		if mw.Enabled {
			enabled = append(enabled, mw)
		}
	}
	return enabled
}

// IsInMaintenance checks if the given check is in a maintenance window.
func (m *MaintenanceManager) IsInMaintenance(checkID int64, checkName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	for _, mw := range m.windows {
		if mw.IsActiveAt(now) && mw.Matches(checkID, checkName) {
			return true
		}
	}
	return false
}

// Upcoming returns maintenance windows starting within the given duration.
func (m *MaintenanceManager) Upcoming(within time.Duration) []*MaintenanceWindow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var upcoming []*MaintenanceWindow
	now := time.Now()
	deadline := now.Add(within)

	for _, mw := range m.windows {
		if !mw.Enabled {
			continue
		}
		next := mw.NextOccurrence(now)
		if !next.IsZero() && next.Before(deadline) {
			upcoming = append(upcoming, mw)
		}
	}
	return upcoming
}

// ToJSON serializes all maintenance windows to JSON.
func (m *MaintenanceManager) ToJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.MarshalIndent(m.windows, "", "  ")
}

// FromJSON deserializes maintenance windows from JSON.
func (m *MaintenanceManager) FromJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var windows map[string]*MaintenanceWindow
	if err := json.Unmarshal(data, &windows); err != nil {
		return err
	}
	m.windows = windows
	return nil
}

// NewMaintenanceOnce creates a one-time maintenance window.
func NewMaintenanceOnce(name string, startTime time.Time, duration time.Duration, createdBy string) *MaintenanceWindow {
	return &MaintenanceWindow{
		Name:      name,
		Schedule:  ScheduleOnce,
		StartTime: startTime,
		Duration:  duration,
		Enabled:   true,
		CreatedBy: createdBy,
	}
}

// NewMaintenanceDaily creates a daily recurring maintenance window.
func NewMaintenanceDaily(name string, startHour, startMinute int, duration time.Duration, createdBy string) *MaintenanceWindow {
	// Use StartTime to store the time of day (date part is ignored for daily)
	startTime := time.Date(2000, 1, 1, startHour, startMinute, 0, 0, time.Local)
	return &MaintenanceWindow{
		Name:      name,
		Schedule:  ScheduleDaily,
		StartTime: startTime,
		Duration:  duration,
		Enabled:   true,
		CreatedBy: createdBy,
	}
}

// NewMaintenanceWeekly creates a weekly recurring maintenance window.
func NewMaintenanceWeekly(name string, dayOfWeek time.Weekday, startHour, startMinute int, duration time.Duration, createdBy string) *MaintenanceWindow {
	startTime := time.Date(2000, 1, 1, startHour, startMinute, 0, 0, time.Local)
	dow := dayOfWeek
	return &MaintenanceWindow{
		Name:      name,
		Schedule:  ScheduleWeekly,
		StartTime: startTime,
		Duration:  duration,
		DayOfWeek: &dow,
		Enabled:   true,
		CreatedBy: createdBy,
	}
}

// NewMaintenanceMonthly creates a monthly recurring maintenance window.
func NewMaintenanceMonthly(name string, dayOfMonth int, startHour, startMinute int, duration time.Duration, createdBy string) *MaintenanceWindow {
	startTime := time.Date(2000, 1, 1, startHour, startMinute, 0, 0, time.Local)
	dom := dayOfMonth
	return &MaintenanceWindow{
		Name:       name,
		Schedule:   ScheduleMonthly,
		StartTime:  startTime,
		Duration:   duration,
		DayOfMonth: &dom,
		Enabled:    true,
		CreatedBy:  createdBy,
	}
}
