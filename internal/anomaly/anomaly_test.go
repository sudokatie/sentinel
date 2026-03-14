package anomaly

import (
	"testing"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

// MockStorage implements storage.Storage for testing
type MockStorage struct {
	results []*storage.CheckResult
}

func (m *MockStorage) GetResultsInRange(checkID int64, start, end time.Time) ([]*storage.CheckResult, error) {
	return m.results, nil
}

func (m *MockStorage) GetRecentResults(checkID int64, count int) ([]*storage.CheckResult, error) {
	if count > len(m.results) {
		return m.results, nil
	}
	return m.results[:count], nil
}

// Implement other storage.Storage methods as no-ops
func (m *MockStorage) CreateCheck(check *storage.Check) error                          { return nil }
func (m *MockStorage) GetCheck(id int64) (*storage.Check, error)                        { return nil, nil }
func (m *MockStorage) GetCheckByURL(url string) (*storage.Check, error)                 { return nil, nil }
func (m *MockStorage) ListChecks() ([]*storage.Check, error)                            { return nil, nil }
func (m *MockStorage) ListEnabledChecks() ([]*storage.Check, error)                     { return nil, nil }
func (m *MockStorage) ListChecksByTag(tag string) ([]*storage.Check, error)             { return nil, nil }
func (m *MockStorage) UpdateCheck(check *storage.Check) error                           { return nil }
func (m *MockStorage) DeleteCheck(id int64) error                                       { return nil }
func (m *MockStorage) SaveResult(result *storage.CheckResult) error                     { return nil }
func (m *MockStorage) GetResults(checkID int64, limit int, offset int) ([]*storage.CheckResult, error) {
	return nil, nil
}
func (m *MockStorage) GetLatestResult(checkID int64) (*storage.CheckResult, error)      { return nil, nil }
func (m *MockStorage) GetLatestResultsByRegion(checkID int64) (map[string]*storage.CheckResult, error) {
	return nil, nil
}
func (m *MockStorage) CountFailingRegions(checkID int64) (int, error)                   { return 0, nil }
func (m *MockStorage) GetStats(checkID int64) (*storage.CheckStats, error)              { return nil, nil }
func (m *MockStorage) CreateIncident(incident *storage.Incident) error                  { return nil }
func (m *MockStorage) GetIncident(id int64) (*storage.Incident, error)                  { return nil, nil }
func (m *MockStorage) GetIncidentWithNotes(id int64) (*storage.Incident, error)         { return nil, nil }
func (m *MockStorage) GetActiveIncident(checkID int64) (*storage.Incident, error)       { return nil, nil }
func (m *MockStorage) CloseIncident(id int64, endedAt time.Time) error                  { return nil }
func (m *MockStorage) UpdateIncidentStatus(id int64, status storage.IncidentStatus) error { return nil }
func (m *MockStorage) UpdateIncidentTitle(id int64, title string) error                 { return nil }
func (m *MockStorage) ListIncidents(limit int, offset int) ([]*storage.Incident, error) { return nil, nil }
func (m *MockStorage) ListIncidentsForCheck(checkID int64, limit int) ([]*storage.Incident, error) {
	return nil, nil
}
func (m *MockStorage) ListActiveIncidents() ([]*storage.Incident, error)                { return nil, nil }
func (m *MockStorage) AddIncidentNote(note *storage.IncidentNote) error                 { return nil }
func (m *MockStorage) GetIncidentNotes(incidentID int64) ([]*storage.IncidentNote, error) { return nil, nil }
func (m *MockStorage) DeleteIncidentNote(id int64) error                                { return nil }
func (m *MockStorage) LogAlert(log *storage.AlertLog) error                             { return nil }
func (m *MockStorage) GetLastAlertForIncident(incidentID int64, channel string) (*storage.AlertLog, error) {
	return nil, nil
}
func (m *MockStorage) CreateHourlyAggregate(agg *storage.HourlyAggregate) error         { return nil }
func (m *MockStorage) GetHourlyAggregates(checkID int64, start, end time.Time) ([]*storage.HourlyAggregate, error) {
	return nil, nil
}
func (m *MockStorage) CleanupOldResults(olderThan time.Time) error                      { return nil }
func (m *MockStorage) AggregateResults(olderThan time.Time) error                       { return nil }
func (m *MockStorage) CleanupOldAggregates(olderThan time.Time) error                   { return nil }
func (m *MockStorage) Close() error                                                     { return nil }

func makeResult(status string, responseMs int) *storage.CheckResult {
	return &storage.CheckResult{
		Status:         status,
		ResponseTimeMs: responseMs,
		CheckedAt:      time.Now(),
	}
}

func TestCalculateMean(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"simple", []float64{100, 200, 300}, 200},
		{"single", []float64{50}, 50},
		{"empty", []float64{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateMean(tt.values)
			if got != tt.want {
				t.Errorf("calculateMean() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateStdDev(t *testing.T) {
	// For [100, 200, 300], mean = 200, variance = 10000, stddev = 100
	values := []float64{100, 200, 300}
	mean := 200.0
	got := calculateStdDev(values, mean)
	want := 100.0

	if got != want {
		t.Errorf("calculateStdDev() = %v, want %v", got, want)
	}
}

func TestCalculateMinMax(t *testing.T) {
	values := []float64{50, 100, 25, 200, 75}
	minVal, maxVal := calculateMinMax(values)

	if minVal != 25 {
		t.Errorf("min = %v, want 25", minVal)
	}
	if maxVal != 200 {
		t.Errorf("max = %v, want 200", maxVal)
	}
}

func TestCalculateBaseline(t *testing.T) {
	// Create mock with stable results around 100ms
	results := make([]*storage.CheckResult, 50)
	for i := 0; i < 50; i++ {
		results[i] = makeResult("up", 90+i%20) // 90-109ms
	}

	mock := &MockStorage{results: results}
	detector := NewDetector(mock, DefaultConfig())

	baseline, err := detector.CalculateBaseline(1)
	if err != nil {
		t.Fatalf("CalculateBaseline error: %v", err)
	}
	if baseline == nil {
		t.Fatal("expected baseline, got nil")
	}

	// Mean should be around 99.5
	if baseline.Mean < 90 || baseline.Mean > 110 {
		t.Errorf("Mean = %v, expected around 99.5", baseline.Mean)
	}
	if baseline.SampleCount != 50 {
		t.Errorf("SampleCount = %v, want 50", baseline.SampleCount)
	}
}

func TestCalculateBaselineInsufficientData(t *testing.T) {
	// Only 10 results - below minimum
	results := make([]*storage.CheckResult, 10)
	for i := 0; i < 10; i++ {
		results[i] = makeResult("up", 100)
	}

	mock := &MockStorage{results: results}
	detector := NewDetector(mock, DefaultConfig())

	baseline, err := detector.CalculateBaseline(1)
	if err != nil {
		t.Fatalf("CalculateBaseline error: %v", err)
	}
	if baseline != nil {
		t.Error("expected nil baseline for insufficient data")
	}
}

func TestDetectAnomalySpike(t *testing.T) {
	// Create baseline around 100ms with stddev ~10ms
	results := make([]*storage.CheckResult, 50)
	for i := 0; i < 50; i++ {
		results[i] = makeResult("up", 95+i%10) // 95-104ms
	}

	mock := &MockStorage{results: results}
	detector := NewDetector(mock, DefaultConfig())

	// 200ms is way above baseline (>3 sigma)
	anomaly, err := detector.DetectAnomaly(1, 200)
	if err != nil {
		t.Fatalf("DetectAnomaly error: %v", err)
	}
	if anomaly == nil {
		t.Fatal("expected anomaly for spike, got nil")
	}
	if anomaly.Type != AnomalyTypeSpike {
		t.Errorf("Type = %v, want spike", anomaly.Type)
	}
}

func TestDetectAnomalyNormal(t *testing.T) {
	// Create baseline around 100ms
	results := make([]*storage.CheckResult, 50)
	for i := 0; i < 50; i++ {
		results[i] = makeResult("up", 95+i%10)
	}

	mock := &MockStorage{results: results}
	detector := NewDetector(mock, DefaultConfig())

	// 105ms is within normal range
	anomaly, err := detector.DetectAnomaly(1, 105)
	if err != nil {
		t.Fatalf("DetectAnomaly error: %v", err)
	}
	if anomaly != nil {
		t.Errorf("expected no anomaly for normal latency, got %v", anomaly.Type)
	}
}

func TestGetTrendIncreasing(t *testing.T) {
	// First half: 100ms, second half: 150ms
	results := make([]*storage.CheckResult, 20)
	for i := 0; i < 10; i++ {
		results[i] = makeResult("up", 100)
	}
	for i := 10; i < 20; i++ {
		results[i] = makeResult("up", 150)
	}

	mock := &MockStorage{results: results}
	detector := NewDetector(mock, DefaultConfig())

	trend, err := detector.GetTrend(1, 24)
	if err != nil {
		t.Fatalf("GetTrend error: %v", err)
	}
	if trend == nil {
		t.Fatal("expected trend, got nil")
	}
	if trend.Direction != TrendIncreasing {
		t.Errorf("Direction = %v, want increasing", trend.Direction)
	}
	if trend.ChangePercent <= 0 {
		t.Errorf("ChangePercent = %v, want positive", trend.ChangePercent)
	}
}

func TestGetTrendStable(t *testing.T) {
	// All results around 100ms
	results := make([]*storage.CheckResult, 20)
	for i := 0; i < 20; i++ {
		results[i] = makeResult("up", 98+i%5)
	}

	mock := &MockStorage{results: results}
	detector := NewDetector(mock, DefaultConfig())

	trend, err := detector.GetTrend(1, 24)
	if err != nil {
		t.Fatalf("GetTrend error: %v", err)
	}
	if trend == nil {
		t.Fatal("expected trend, got nil")
	}
	if trend.Direction != TrendStable {
		t.Errorf("Direction = %v, want stable", trend.Direction)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SpikeThreshold <= 0 {
		t.Error("SpikeThreshold should be positive")
	}
	if cfg.WarnThreshold <= 0 {
		t.Error("WarnThreshold should be positive")
	}
	if cfg.MinSamples <= 0 {
		t.Error("MinSamples should be positive")
	}
	if cfg.BaselineHours <= 0 {
		t.Error("BaselineHours should be positive")
	}
}
