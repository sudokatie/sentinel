package probe

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

// mockStorage implements storage.Storage for testing
type mockStorage struct {
	checks       []*storage.Check
	probes       []*storage.Probe
	probeResults []*storage.ProbeResult
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		checks:       []*storage.Check{},
		probes:       []*storage.Probe{},
		probeResults: []*storage.ProbeResult{},
	}
}

func (m *mockStorage) CreateCheck(check *storage.Check) error {
	check.ID = int64(len(m.checks) + 1)
	m.checks = append(m.checks, check)
	return nil
}

func (m *mockStorage) GetCheck(id int64) (*storage.Check, error) {
	for _, c := range m.checks {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockStorage) GetCheckByURL(url string) (*storage.Check, error) {
	for _, c := range m.checks {
		if c.URL == url {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockStorage) ListChecks() ([]*storage.Check, error) {
	return m.checks, nil
}

func (m *mockStorage) ListEnabledChecks() ([]*storage.Check, error) {
	var enabled []*storage.Check
	for _, c := range m.checks {
		if c.Enabled {
			enabled = append(enabled, c)
		}
	}
	return enabled, nil
}

func (m *mockStorage) ListChecksByTag(tag string) ([]*storage.Check, error) {
	var result []*storage.Check
	for _, c := range m.checks {
		for _, t := range c.Tags {
			if t == tag {
				result = append(result, c)
				break
			}
		}
	}
	return result, nil
}

func (m *mockStorage) UpdateCheck(check *storage.Check) error {
	for i, c := range m.checks {
		if c.ID == check.ID {
			m.checks[i] = check
			return nil
		}
	}
	return nil
}

func (m *mockStorage) DeleteCheck(id int64) error {
	for i, c := range m.checks {
		if c.ID == id {
			m.checks = append(m.checks[:i], m.checks[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockStorage) SaveResult(result *storage.CheckResult) error {
	return nil
}

func (m *mockStorage) GetResults(checkID int64, limit int, offset int) ([]*storage.CheckResult, error) {
	return nil, nil
}

func (m *mockStorage) GetLatestResult(checkID int64) (*storage.CheckResult, error) {
	return nil, nil
}

func (m *mockStorage) GetLatestResultsByRegion(checkID int64) (map[string]*storage.CheckResult, error) {
	return nil, nil
}

func (m *mockStorage) CountFailingRegions(checkID int64) (int, error) {
	return 0, nil
}

func (m *mockStorage) GetResultsInRange(checkID int64, start, end time.Time) ([]*storage.CheckResult, error) {
	return nil, nil
}

func (m *mockStorage) GetRecentResults(checkID int64, count int) ([]*storage.CheckResult, error) {
	return nil, nil
}

func (m *mockStorage) GetStats(checkID int64) (*storage.CheckStats, error) {
	return nil, nil
}

func (m *mockStorage) CreateIncident(incident *storage.Incident) error {
	return nil
}

func (m *mockStorage) GetIncident(id int64) (*storage.Incident, error) {
	return nil, nil
}

func (m *mockStorage) GetIncidentWithNotes(id int64) (*storage.Incident, error) {
	return nil, nil
}

func (m *mockStorage) GetActiveIncident(checkID int64) (*storage.Incident, error) {
	return nil, nil
}

func (m *mockStorage) CloseIncident(id int64, endedAt time.Time) error {
	return nil
}

func (m *mockStorage) UpdateIncidentStatus(id int64, status storage.IncidentStatus) error {
	return nil
}

func (m *mockStorage) UpdateIncidentTitle(id int64, title string) error {
	return nil
}

func (m *mockStorage) ListIncidents(limit int, offset int) ([]*storage.Incident, error) {
	return nil, nil
}

func (m *mockStorage) ListIncidentsForCheck(checkID int64, limit int) ([]*storage.Incident, error) {
	return nil, nil
}

func (m *mockStorage) ListActiveIncidents() ([]*storage.Incident, error) {
	return nil, nil
}

func (m *mockStorage) AddIncidentNote(note *storage.IncidentNote) error {
	return nil
}

func (m *mockStorage) GetIncidentNotes(incidentID int64) ([]*storage.IncidentNote, error) {
	return nil, nil
}

func (m *mockStorage) DeleteIncidentNote(id int64) error {
	return nil
}

func (m *mockStorage) LogAlert(log *storage.AlertLog) error {
	return nil
}

func (m *mockStorage) GetLastAlertForIncident(incidentID int64, channel string) (*storage.AlertLog, error) {
	return nil, nil
}

func (m *mockStorage) CreateHourlyAggregate(agg *storage.HourlyAggregate) error {
	return nil
}

func (m *mockStorage) GetHourlyAggregates(checkID int64, start, end time.Time) ([]*storage.HourlyAggregate, error) {
	return nil, nil
}

func (m *mockStorage) CleanupOldResults(olderThan time.Time) error {
	return nil
}

func (m *mockStorage) AggregateResults(olderThan time.Time) error {
	return nil
}

func (m *mockStorage) CleanupOldAggregates(olderThan time.Time) error {
	return nil
}

func (m *mockStorage) Close() error {
	return nil
}

func (m *mockStorage) CreateProbe(probe *storage.Probe) error {
	probe.ID = int64(len(m.probes) + 1)
	m.probes = append(m.probes, probe)
	return nil
}

func (m *mockStorage) GetProbe(id int64) (*storage.Probe, error) {
	for _, p := range m.probes {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockStorage) GetProbeByAPIKey(apiKey string) (*storage.Probe, error) {
	for _, p := range m.probes {
		if p.APIKey == apiKey {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockStorage) ListProbes() ([]*storage.Probe, error) {
	return m.probes, nil
}

func (m *mockStorage) ListActiveProbes() ([]*storage.Probe, error) {
	var active []*storage.Probe
	for _, p := range m.probes {
		if p.Status == "active" {
			active = append(active, p)
		}
	}
	return active, nil
}

func (m *mockStorage) ListProbesByRegion(region string) ([]*storage.Probe, error) {
	var result []*storage.Probe
	for _, p := range m.probes {
		if p.Region == region {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockStorage) UpdateProbeHeartbeat(id int64) error {
	return nil
}

func (m *mockStorage) UpdateProbeStatus(id int64, status string) error {
	for _, p := range m.probes {
		if p.ID == id {
			p.Status = status
			return nil
		}
	}
	return nil
}

func (m *mockStorage) DeleteProbe(id int64) error {
	for i, p := range m.probes {
		if p.ID == id {
			m.probes = append(m.probes[:i], m.probes[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockStorage) CleanupStaleProbes() (int, error) {
	return 0, nil
}

func (m *mockStorage) SaveProbeResult(result *storage.ProbeResult) error {
	result.ID = int64(len(m.probeResults) + 1)
	m.probeResults = append(m.probeResults, result)
	return nil
}

func (m *mockStorage) GetProbeResults(checkID int64, limit int, offset int) ([]*storage.ProbeResult, error) {
	var results []*storage.ProbeResult
	for _, r := range m.probeResults {
		if r.CheckID == checkID {
			results = append(results, r)
		}
	}
	// Apply offset and limit
	if offset >= len(results) {
		return []*storage.ProbeResult{}, nil
	}
	results = results[offset:]
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}
	return results, nil
}

func (m *mockStorage) GetProbeResultsByProbe(probeID int64, limit int) ([]*storage.ProbeResult, error) {
	var results []*storage.ProbeResult
	for _, r := range m.probeResults {
		if r.ProbeID == probeID {
			results = append(results, r)
		}
	}
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}
	return results, nil
}

func (m *mockStorage) GetLatestProbeResultsByRegion(checkID int64) (map[string]*storage.ProbeResult, error) {
	// Map probe ID to region
	probeRegions := make(map[int64]string)
	for _, p := range m.probes {
		probeRegions[p.ID] = p.Region
	}

	// Find latest result per region
	latestByRegion := make(map[string]*storage.ProbeResult)
	for _, r := range m.probeResults {
		if r.CheckID != checkID {
			continue
		}
		region := probeRegions[r.ProbeID]
		if region == "" {
			continue
		}
		existing, ok := latestByRegion[region]
		if !ok || r.CheckedAt.After(existing.CheckedAt) {
			latestByRegion[region] = r
		}
	}

	return latestByRegion, nil
}

func (m *mockStorage) CountFailingProbeRegions(checkID int64) (int, error) {
	latestByRegion, _ := m.GetLatestProbeResultsByRegion(checkID)
	count := 0
	for _, r := range latestByRegion {
		if r.Status == "down" {
			count++
		}
	}
	return count, nil
}

// Test helper to create a probe result with common fields
func newProbeResult(checkID, probeID int64, status string, responseTimeMs int64, checkedAt time.Time) *storage.ProbeResult {
	return &storage.ProbeResult{
		CheckID:        checkID,
		ProbeID:        probeID,
		Status:         status,
		ResponseTimeMs: sql.NullInt64{Int64: responseTimeMs, Valid: true},
		StatusCode:     sql.NullInt64{Int64: 200, Valid: true},
		CheckedAt:      checkedAt,
	}
}

func TestAssignChecks(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Register some probes in the registry
	registry.Register("probe-us", "us-east", "New York", "USA", 40.7128, -74.0060)
	registry.Register("probe-eu", "eu-west", "London", "UK", 51.5074, -0.1278)

	// Also add them to storage
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})

	// Create enabled checks
	store.CreateCheck(&storage.Check{
		Name:           "Check 1",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Regions:        []string{"us-east"},
	})

	store.CreateCheck(&storage.Check{
		Name:           "Check 2",
		URL:            "https://example.org",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	// Create a disabled check (should be ignored)
	store.CreateCheck(&storage.Check{
		Name:           "Disabled Check",
		URL:            "https://disabled.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        false,
	})

	ctx := context.Background()
	err := coord.AssignChecks(ctx)
	if err != nil {
		t.Fatalf("AssignChecks failed: %v", err)
	}

	// Verify enabled checks are returned by ListEnabledChecks
	enabledChecks, _ := store.ListEnabledChecks()
	if len(enabledChecks) != 2 {
		t.Errorf("expected 2 enabled checks, got %d", len(enabledChecks))
	}
}

func TestAggregateResultsUp(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// All probes report up
	store.SaveProbeResult(newProbeResult(1, 1, "up", 100, now))
	store.SaveProbeResult(newProbeResult(1, 2, "up", 150, now))

	ctx := context.Background()
	result, err := coord.AggregateResults(ctx, 1)
	if err != nil {
		t.Fatalf("AggregateResults failed: %v", err)
	}

	if result.OverallStatus != "up" {
		t.Errorf("expected overall status 'up', got '%s'", result.OverallStatus)
	}
	if result.UpProbes != 2 {
		t.Errorf("expected 2 up probes, got %d", result.UpProbes)
	}
	if result.DownProbes != 0 {
		t.Errorf("expected 0 down probes, got %d", result.DownProbes)
	}
}

func TestAggregateResultsDegraded(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// Some probes up, some down
	store.SaveProbeResult(newProbeResult(1, 1, "up", 100, now))
	store.SaveProbeResult(newProbeResult(1, 2, "down", 0, now))

	ctx := context.Background()
	result, err := coord.AggregateResults(ctx, 1)
	if err != nil {
		t.Fatalf("AggregateResults failed: %v", err)
	}

	if result.OverallStatus != "degraded" {
		t.Errorf("expected overall status 'degraded', got '%s'", result.OverallStatus)
	}
	if result.UpProbes != 1 {
		t.Errorf("expected 1 up probe, got %d", result.UpProbes)
	}
	if result.DownProbes != 1 {
		t.Errorf("expected 1 down probe, got %d", result.DownProbes)
	}
}

func TestAggregateResultsDown(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// All probes report down
	store.SaveProbeResult(newProbeResult(1, 1, "down", 0, now))
	store.SaveProbeResult(newProbeResult(1, 2, "down", 0, now))

	ctx := context.Background()
	result, err := coord.AggregateResults(ctx, 1)
	if err != nil {
		t.Fatalf("AggregateResults failed: %v", err)
	}

	if result.OverallStatus != "down" {
		t.Errorf("expected overall status 'down', got '%s'", result.OverallStatus)
	}
	if result.UpProbes != 0 {
		t.Errorf("expected 0 up probes, got %d", result.UpProbes)
	}
	if result.DownProbes != 2 {
		t.Errorf("expected 2 down probes, got %d", result.DownProbes)
	}
}

func TestDetectRegionalOutage(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes in different regions
	store.CreateProbe(&storage.Probe{Name: "probe-us-1", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-us-2", Region: "us-east", APIKey: "key2", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu-1", Region: "eu-west", APIKey: "key3", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu-2", Region: "eu-west", APIKey: "key4", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// US region failing (both probes down), EU region up
	store.SaveProbeResult(newProbeResult(1, 1, "down", 0, now))
	store.SaveProbeResult(newProbeResult(1, 2, "down", 0, now))
	store.SaveProbeResult(newProbeResult(1, 3, "up", 100, now))
	store.SaveProbeResult(newProbeResult(1, 4, "up", 120, now))

	ctx := context.Background()
	report, err := coord.DetectRegionalOutage(ctx, 1)
	if err != nil {
		t.Fatalf("DetectRegionalOutage failed: %v", err)
	}

	if report.OutageType != "regional" {
		t.Errorf("expected outage type 'regional', got '%s'", report.OutageType)
	}
	if len(report.AffectedRegions) != 1 {
		t.Errorf("expected 1 affected region, got %d", len(report.AffectedRegions))
	}
	if len(report.AffectedRegions) > 0 && report.AffectedRegions[0] != "us-east" {
		t.Errorf("expected affected region 'us-east', got '%s'", report.AffectedRegions[0])
	}
	if len(report.FailingProbes) != 2 {
		t.Errorf("expected 2 failing probes, got %d", len(report.FailingProbes))
	}
	if len(report.PassingProbes) != 2 {
		t.Errorf("expected 2 passing probes, got %d", len(report.PassingProbes))
	}
}

func TestDetectGlobalOutage(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes in different regions
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-apac", Region: "apac", APIKey: "key3", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// All regions failing
	store.SaveProbeResult(newProbeResult(1, 1, "down", 0, now))
	store.SaveProbeResult(newProbeResult(1, 2, "down", 0, now))
	store.SaveProbeResult(newProbeResult(1, 3, "down", 0, now))

	ctx := context.Background()
	report, err := coord.DetectRegionalOutage(ctx, 1)
	if err != nil {
		t.Fatalf("DetectRegionalOutage failed: %v", err)
	}

	if report.OutageType != "global" {
		t.Errorf("expected outage type 'global', got '%s'", report.OutageType)
	}
	if len(report.AffectedRegions) != 3 {
		t.Errorf("expected 3 affected regions, got %d", len(report.AffectedRegions))
	}
	if len(report.FailingProbes) != 3 {
		t.Errorf("expected 3 failing probes, got %d", len(report.FailingProbes))
	}
	if len(report.PassingProbes) != 0 {
		t.Errorf("expected 0 passing probes, got %d", len(report.PassingProbes))
	}
}

func TestDetectNoOutage(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes in different regions
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// All probes up
	store.SaveProbeResult(newProbeResult(1, 1, "up", 100, now))
	store.SaveProbeResult(newProbeResult(1, 2, "up", 120, now))

	ctx := context.Background()
	report, err := coord.DetectRegionalOutage(ctx, 1)
	if err != nil {
		t.Fatalf("DetectRegionalOutage failed: %v", err)
	}

	if report.OutageType != "none" {
		t.Errorf("expected outage type 'none', got '%s'", report.OutageType)
	}
	if len(report.AffectedRegions) != 0 {
		t.Errorf("expected 0 affected regions, got %d", len(report.AffectedRegions))
	}
	if len(report.FailingProbes) != 0 {
		t.Errorf("expected 0 failing probes, got %d", len(report.FailingProbes))
	}
	if len(report.PassingProbes) != 2 {
		t.Errorf("expected 2 passing probes, got %d", len(report.PassingProbes))
	}
}

func TestCompareLatency(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes in different regions
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// Add multiple results with varying latencies
	store.SaveProbeResult(newProbeResult(1, 1, "up", 100, now.Add(-3*time.Minute)))
	store.SaveProbeResult(newProbeResult(1, 1, "up", 150, now.Add(-2*time.Minute)))
	store.SaveProbeResult(newProbeResult(1, 1, "up", 200, now.Add(-1*time.Minute)))

	store.SaveProbeResult(newProbeResult(1, 2, "up", 50, now.Add(-3*time.Minute)))
	store.SaveProbeResult(newProbeResult(1, 2, "up", 75, now.Add(-2*time.Minute)))
	store.SaveProbeResult(newProbeResult(1, 2, "up", 100, now.Add(-1*time.Minute)))

	ctx := context.Background()
	stats, err := coord.CompareLatency(ctx, 1)
	if err != nil {
		t.Fatalf("CompareLatency failed: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 regions in stats, got %d", len(stats))
	}

	// Verify US-East stats
	usStats, ok := stats["us-east"]
	if !ok {
		t.Fatal("expected us-east in stats")
	}
	if usStats.MinMs != 100 {
		t.Errorf("expected us-east min latency 100, got %f", usStats.MinMs)
	}
	if usStats.MaxMs != 200 {
		t.Errorf("expected us-east max latency 200, got %f", usStats.MaxMs)
	}
	expectedAvg := (100 + 150 + 200) / 3.0
	if usStats.AvgMs != expectedAvg {
		t.Errorf("expected us-east avg latency %f, got %f", expectedAvg, usStats.AvgMs)
	}
	if usStats.SampleCount != 3 {
		t.Errorf("expected us-east sample count 3, got %d", usStats.SampleCount)
	}

	// Verify EU-West stats
	euStats, ok := stats["eu-west"]
	if !ok {
		t.Fatal("expected eu-west in stats")
	}
	if euStats.MinMs != 50 {
		t.Errorf("expected eu-west min latency 50, got %f", euStats.MinMs)
	}
	if euStats.MaxMs != 100 {
		t.Errorf("expected eu-west max latency 100, got %f", euStats.MaxMs)
	}
	expectedAvgEU := (50 + 75 + 100) / 3.0
	if euStats.AvgMs != expectedAvgEU {
		t.Errorf("expected eu-west avg latency %f, got %f", expectedAvgEU, euStats.AvgMs)
	}
	if euStats.SampleCount != 3 {
		t.Errorf("expected eu-west sample count 3, got %d", euStats.SampleCount)
	}
}

func TestAggregateResultsNoResults(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create check but no results
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	ctx := context.Background()
	result, err := coord.AggregateResults(ctx, 1)
	if err != nil {
		t.Fatalf("AggregateResults failed: %v", err)
	}

	if result.OverallStatus != "pending" {
		t.Errorf("expected overall status 'pending', got '%s'", result.OverallStatus)
	}
	if result.TotalProbes != 0 {
		t.Errorf("expected 0 total probes, got %d", result.TotalProbes)
	}
}

func TestDetectRegionalOutageNoResults(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	ctx := context.Background()
	report, err := coord.DetectRegionalOutage(ctx, 999)
	if err != nil {
		t.Fatalf("DetectRegionalOutage failed: %v", err)
	}

	if report.OutageType != "none" {
		t.Errorf("expected outage type 'none', got '%s'", report.OutageType)
	}
}

func TestCompareLatencyNoResults(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	ctx := context.Background()
	stats, err := coord.CompareLatency(ctx, 999)
	if err != nil {
		t.Fatalf("CompareLatency failed: %v", err)
	}

	if len(stats) != 0 {
		t.Errorf("expected 0 regions in stats, got %d", len(stats))
	}
}

func TestAggregateResultsWithRegionBreakdown(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Create probes in different regions
	store.CreateProbe(&storage.Probe{Name: "probe-us", Region: "us-east", APIKey: "key1", Status: "active"})
	store.CreateProbe(&storage.Probe{Name: "probe-eu", Region: "eu-west", APIKey: "key2", Status: "active"})

	// Create check
	store.CreateCheck(&storage.Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	})

	now := time.Now()

	// US up, EU down
	store.SaveProbeResult(newProbeResult(1, 1, "up", 100, now))
	store.SaveProbeResult(newProbeResult(1, 2, "down", 0, now))

	ctx := context.Background()
	result, err := coord.AggregateResults(ctx, 1)
	if err != nil {
		t.Fatalf("AggregateResults failed: %v", err)
	}

	if len(result.Regions) != 2 {
		t.Errorf("expected 2 regions, got %d", len(result.Regions))
	}

	usRegion, ok := result.Regions["us-east"]
	if !ok {
		t.Fatal("expected us-east in regions")
	}
	if usRegion.Status != "up" {
		t.Errorf("expected us-east status 'up', got '%s'", usRegion.Status)
	}
	if usRegion.AvgLatencyMs != 100 {
		t.Errorf("expected us-east avg latency 100, got %f", usRegion.AvgLatencyMs)
	}

	euRegion, ok := result.Regions["eu-west"]
	if !ok {
		t.Fatal("expected eu-west in regions")
	}
	if euRegion.Status != "down" {
		t.Errorf("expected eu-west status 'down', got '%s'", euRegion.Status)
	}
}

func TestAssignChecksWithContextCancellation(t *testing.T) {
	registry := NewProbeRegistry()
	store := newMockStorage()
	coord := NewCoordinator(registry, store)

	// Register probes in the registry so the loop runs
	registry.Register("probe-us", "us-east", "New York", "USA", 40.7128, -74.0060)

	// Create many checks to ensure we hit the context check
	for i := 0; i < 10; i++ {
		store.CreateCheck(&storage.Check{
			Name:           "Check",
			URL:            "https://example.com",
			IntervalSecs:   60,
			TimeoutSecs:    10,
			ExpectedStatus: 200,
			Enabled:        true,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := coord.AssignChecks(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}
