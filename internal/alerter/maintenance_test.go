package alerter

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMaintenanceOnce(t *testing.T) {
	// Create a maintenance window starting now for 2 hours
	now := time.Now()
	mw := NewMaintenanceOnce("Test Maintenance", now.Add(-time.Minute), 2*time.Hour, "tester")

	if !mw.IsActive() {
		t.Error("expected maintenance window to be active")
	}

	if !mw.IsActiveAt(now.Add(time.Hour)) {
		t.Error("expected maintenance window to be active 1 hour in")
	}

	if mw.IsActiveAt(now.Add(3 * time.Hour)) {
		t.Error("expected maintenance window to be inactive after duration")
	}
}

func TestMaintenanceOnceNotYetStarted(t *testing.T) {
	future := time.Now().Add(time.Hour)
	mw := NewMaintenanceOnce("Future Maintenance", future, time.Hour, "tester")

	if mw.IsActive() {
		t.Error("expected maintenance window to not be active yet")
	}

	if !mw.IsActiveAt(future.Add(30 * time.Minute)) {
		t.Error("expected maintenance window to be active during window")
	}
}

func TestMaintenanceDaily(t *testing.T) {
	now := time.Now()
	hour := now.Hour()
	minute := now.Minute()

	// Create daily maintenance that includes now
	mw := NewMaintenanceDaily("Daily Backup", hour, minute, time.Hour, "tester")

	if !mw.IsActive() {
		t.Error("expected daily maintenance to be active now")
	}

	// Test at same time tomorrow
	tomorrow := now.Add(24 * time.Hour)
	if !mw.IsActiveAt(tomorrow) {
		t.Error("expected daily maintenance to be active tomorrow at same time")
	}
}

func TestMaintenanceWeekly(t *testing.T) {
	now := time.Now()
	dayOfWeek := now.Weekday()
	hour := now.Hour()
	minute := now.Minute()

	mw := NewMaintenanceWeekly("Weekly Update", dayOfWeek, hour, minute, time.Hour, "tester")

	if !mw.IsActive() {
		t.Error("expected weekly maintenance to be active now")
	}

	// Test on wrong day
	wrongDayTime := now.AddDate(0, 0, 1)
	// Adjust to make sure it's a different weekday
	for wrongDayTime.Weekday() == dayOfWeek {
		wrongDayTime = wrongDayTime.AddDate(0, 0, 1)
	}
	if mw.IsActiveAt(wrongDayTime) {
		t.Errorf("expected weekly maintenance to be inactive on wrong day (%v)", wrongDayTime.Weekday())
	}
}

func TestMaintenanceMonthly(t *testing.T) {
	now := time.Now()
	dayOfMonth := now.Day()
	hour := now.Hour()
	minute := now.Minute()

	mw := NewMaintenanceMonthly("Monthly Review", dayOfMonth, hour, minute, time.Hour, "tester")

	if !mw.IsActive() {
		t.Error("expected monthly maintenance to be active now")
	}

	// Test on wrong day of month
	wrongDay := dayOfMonth%28 + 1 // Different day, valid for all months
	if wrongDay == dayOfMonth {
		wrongDay = (wrongDay % 28) + 1
	}
	wrongDayTime := time.Date(now.Year(), now.Month(), wrongDay, hour, minute, 0, 0, now.Location())
	if mw.IsActiveAt(wrongDayTime) {
		t.Error("expected monthly maintenance to be inactive on wrong day of month")
	}
}

func TestMaintenanceDisabled(t *testing.T) {
	mw := NewMaintenanceOnce("Disabled", time.Now().Add(-time.Minute), 2*time.Hour, "tester")
	mw.Enabled = false

	if mw.IsActive() {
		t.Error("expected disabled maintenance window to be inactive")
	}
}

func TestMaintenanceManager(t *testing.T) {
	mgr := NewMaintenanceManager()

	mw := NewMaintenanceOnce("Test", time.Now().Add(-time.Minute), 2*time.Hour, "tester")
	id := mgr.Add(mw)

	if id == "" {
		t.Error("expected non-empty ID")
	}

	if len(mgr.All()) != 1 {
		t.Error("expected 1 maintenance window")
	}

	if len(mgr.Active()) != 1 {
		t.Error("expected 1 active maintenance window")
	}

	// Test get
	got := mgr.Get(id)
	if got == nil || got.Name != "Test" {
		t.Error("expected to get the maintenance window")
	}

	// Test remove
	if !mgr.Remove(id) {
		t.Error("expected remove to succeed")
	}

	if len(mgr.All()) != 0 {
		t.Error("expected 0 maintenance windows after remove")
	}
}

func TestMaintenanceMatchers(t *testing.T) {
	mw := NewMaintenanceOnce("Infra Maintenance", time.Now().Add(-time.Minute), 2*time.Hour, "tester")
	mw.Matchers = []string{"api-*", "web-*"}

	if !mw.Matches(1, "api-server") {
		t.Error("expected api-server to match")
	}

	if !mw.Matches(2, "web-frontend") {
		t.Error("expected web-frontend to match")
	}

	if mw.Matches(3, "db-primary") {
		t.Error("expected db-primary to not match")
	}
}

func TestMaintenanceCheckIDs(t *testing.T) {
	mw := NewMaintenanceOnce("Specific Checks", time.Now().Add(-time.Minute), 2*time.Hour, "tester")
	mw.CheckIDs = []int64{1, 2, 3}

	if !mw.Matches(1, "anything") {
		t.Error("expected check ID 1 to match")
	}

	if !mw.Matches(3, "anything") {
		t.Error("expected check ID 3 to match")
	}

	if mw.Matches(5, "anything") {
		t.Error("expected check ID 5 to not match")
	}
}

func TestIsInMaintenance(t *testing.T) {
	mgr := NewMaintenanceManager()

	mw := NewMaintenanceOnce("API Maintenance", time.Now().Add(-time.Minute), 2*time.Hour, "tester")
	mw.Matchers = []string{"api-*"}
	mgr.Add(mw)

	if !mgr.IsInMaintenance(1, "api-server") {
		t.Error("expected api-server to be in maintenance")
	}

	if mgr.IsInMaintenance(2, "db-server") {
		t.Error("expected db-server to not be in maintenance")
	}
}

func TestMaintenanceJSON(t *testing.T) {
	mgr := NewMaintenanceManager()

	mw := NewMaintenanceDaily("Nightly Backup", 2, 0, time.Hour, "tester")
	mw.Description = "Backup job"
	mgr.Add(mw)

	// Serialize
	data, err := mgr.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize into new manager
	mgr2 := NewMaintenanceManager()
	if err := mgr2.FromJSON(data); err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if len(mgr2.All()) != 1 {
		t.Error("expected 1 maintenance window after deserialization")
	}
}

func TestNextOccurrence(t *testing.T) {
	now := time.Now()

	// Once - in the future
	mwOnce := NewMaintenanceOnce("Future", now.Add(time.Hour), time.Hour, "tester")
	next := mwOnce.NextOccurrence(now)
	if next.IsZero() || next.Before(now) {
		t.Error("expected next occurrence to be in the future")
	}

	// Once - in the past
	mwPast := NewMaintenanceOnce("Past", now.Add(-2*time.Hour), time.Hour, "tester")
	next = mwPast.NextOccurrence(now)
	if !next.IsZero() {
		t.Error("expected no next occurrence for past one-time window")
	}

	// Daily
	mwDaily := NewMaintenanceDaily("Daily", (now.Hour()+1)%24, 0, time.Hour, "tester")
	next = mwDaily.NextOccurrence(now)
	if next.IsZero() {
		t.Error("expected next daily occurrence")
	}
}

func TestUpcoming(t *testing.T) {
	mgr := NewMaintenanceManager()
	now := time.Now()

	// Add window starting in 30 minutes
	mw := NewMaintenanceOnce("Soon", now.Add(30*time.Minute), time.Hour, "tester")
	mgr.Add(mw)

	// Add window starting in 2 hours
	mw2 := NewMaintenanceOnce("Later", now.Add(2*time.Hour), time.Hour, "tester")
	mgr.Add(mw2)

	// Check upcoming within 1 hour
	upcoming := mgr.Upcoming(time.Hour)
	if len(upcoming) != 1 {
		t.Errorf("expected 1 upcoming window within 1 hour, got %d", len(upcoming))
	}

	// Check upcoming within 3 hours
	upcoming = mgr.Upcoming(3 * time.Hour)
	if len(upcoming) != 2 {
		t.Errorf("expected 2 upcoming windows within 3 hours, got %d", len(upcoming))
	}
}

func TestMaintenanceWindowJSONMarshal(t *testing.T) {
	mw := NewMaintenanceWeekly("Weekend Deploy", time.Sunday, 3, 0, 2*time.Hour, "ops")
	mw.Description = "Weekly deployment window"

	data, err := json.Marshal(mw)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded MaintenanceWindow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Name != mw.Name {
		t.Errorf("expected name %q, got %q", mw.Name, decoded.Name)
	}

	if decoded.Schedule != ScheduleWeekly {
		t.Errorf("expected schedule %v, got %v", ScheduleWeekly, decoded.Schedule)
	}
}
