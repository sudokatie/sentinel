// Package anomaly provides latency anomaly detection for checks.
package anomaly

import (
	"fmt"
	"math"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

// Baseline represents the historical latency baseline for a check.
type Baseline struct {
	CheckID       int64     `json:"check_id"`
	Mean          float64   `json:"mean_ms"`
	StdDev        float64   `json:"std_dev_ms"`
	Min           float64   `json:"min_ms"`
	Max           float64   `json:"max_ms"`
	SampleCount   int       `json:"sample_count"`
	CalculatedAt  time.Time `json:"calculated_at"`
	PeriodHours   int       `json:"period_hours"`
}

// AnomalyType represents the type of detected anomaly.
type AnomalyType string

const (
	AnomalyTypeSpike    AnomalyType = "spike"    // Sudden increase
	AnomalyTypeDrop     AnomalyType = "drop"     // Sudden decrease (unusual but could indicate caching issue)
	AnomalyTypeSustained AnomalyType = "sustained" // Consistently elevated
)

// Anomaly represents a detected latency anomaly.
type Anomaly struct {
	CheckID      int64       `json:"check_id"`
	Type         AnomalyType `json:"type"`
	CurrentMs    float64     `json:"current_ms"`
	BaselineMean float64     `json:"baseline_mean_ms"`
	Deviation    float64     `json:"deviation"` // Number of standard deviations
	DetectedAt   time.Time   `json:"detected_at"`
	Message      string      `json:"message"`
}

// Config holds anomaly detection configuration.
type Config struct {
	// Threshold for spike detection (number of standard deviations)
	SpikeThreshold float64
	// Threshold for warning (lower than spike)
	WarnThreshold float64
	// Minimum samples required for baseline
	MinSamples int
	// How many hours of history to use for baseline
	BaselineHours int
	// Minimum std dev to avoid false positives on very stable services
	MinStdDev float64
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		SpikeThreshold: 3.0,  // 3 sigma
		WarnThreshold:  2.0,  // 2 sigma
		MinSamples:     30,   // Need at least 30 samples
		BaselineHours:  24,   // 24 hours of history
		MinStdDev:      10.0, // Minimum 10ms std dev
	}
}

// Detector detects latency anomalies.
type Detector struct {
	storage storage.Storage
	config  Config
}

// NewDetector creates a new anomaly detector.
func NewDetector(s storage.Storage, cfg Config) *Detector {
	return &Detector{
		storage: s,
		config:  cfg,
	}
}

// CalculateBaseline computes the baseline statistics for a check.
func (d *Detector) CalculateBaseline(checkID int64) (*Baseline, error) {
	end := time.Now()
	start := end.Add(-time.Duration(d.config.BaselineHours) * time.Hour)

	results, err := d.storage.GetResultsInRange(checkID, start, end)
	if err != nil {
		return nil, err
	}

	if len(results) < d.config.MinSamples {
		return nil, nil // Not enough data
	}

	// Filter to successful results only
	var latencies []float64
	for _, r := range results {
		if r.IsUp() && r.ResponseTimeMs > 0 {
			latencies = append(latencies, float64(r.ResponseTimeMs))
		}
	}

	if len(latencies) < d.config.MinSamples {
		return nil, nil // Not enough successful results
	}

	// Calculate statistics
	mean := calculateMean(latencies)
	stdDev := calculateStdDev(latencies, mean)
	min, max := calculateMinMax(latencies)

	// Apply minimum std dev
	if stdDev < d.config.MinStdDev {
		stdDev = d.config.MinStdDev
	}

	return &Baseline{
		CheckID:      checkID,
		Mean:         mean,
		StdDev:       stdDev,
		Min:          min,
		Max:          max,
		SampleCount:  len(latencies),
		CalculatedAt: time.Now(),
		PeriodHours:  d.config.BaselineHours,
	}, nil
}

// DetectAnomaly checks if the current latency is anomalous.
func (d *Detector) DetectAnomaly(checkID int64, currentMs int) (*Anomaly, error) {
	baseline, err := d.CalculateBaseline(checkID)
	if err != nil {
		return nil, err
	}
	if baseline == nil {
		return nil, nil // Not enough data for baseline
	}

	current := float64(currentMs)
	deviation := (current - baseline.Mean) / baseline.StdDev

	// Check for spike (high latency)
	if deviation >= d.config.SpikeThreshold {
		return &Anomaly{
			CheckID:      checkID,
			Type:         AnomalyTypeSpike,
			CurrentMs:    current,
			BaselineMean: baseline.Mean,
			Deviation:    deviation,
			DetectedAt:   time.Now(),
			Message:      formatSpikeMessage(current, baseline.Mean, deviation),
		}, nil
	}

	// Check for unusual drop (very fast response)
	if deviation <= -d.config.SpikeThreshold && current > 0 {
		return &Anomaly{
			CheckID:      checkID,
			Type:         AnomalyTypeDrop,
			CurrentMs:    current,
			BaselineMean: baseline.Mean,
			Deviation:    deviation,
			DetectedAt:   time.Now(),
			Message:      formatDropMessage(current, baseline.Mean, deviation),
		}, nil
	}

	return nil, nil // No anomaly
}

// DetectSustainedAnomaly checks for consistently elevated latency over recent results.
func (d *Detector) DetectSustainedAnomaly(checkID int64, recentCount int) (*Anomaly, error) {
	baseline, err := d.CalculateBaseline(checkID)
	if err != nil {
		return nil, err
	}
	if baseline == nil {
		return nil, nil
	}

	recent, err := d.storage.GetRecentResults(checkID, recentCount)
	if err != nil {
		return nil, err
	}

	if len(recent) < recentCount {
		return nil, nil // Not enough recent data
	}

	// Check if all recent results are above threshold
	aboveCount := 0
	var totalLatency float64
	threshold := baseline.Mean + (d.config.WarnThreshold * baseline.StdDev)

	for _, r := range recent {
		if r.IsUp() && r.ResponseTimeMs > 0 {
			lat := float64(r.ResponseTimeMs)
			if lat > threshold {
				aboveCount++
			}
			totalLatency += lat
		}
	}

	// If 80%+ of recent results are above warn threshold
	if float64(aboveCount)/float64(len(recent)) >= 0.8 {
		avgRecent := totalLatency / float64(len(recent))
		deviation := (avgRecent - baseline.Mean) / baseline.StdDev

		return &Anomaly{
			CheckID:      checkID,
			Type:         AnomalyTypeSustained,
			CurrentMs:    avgRecent,
			BaselineMean: baseline.Mean,
			Deviation:    deviation,
			DetectedAt:   time.Now(),
			Message:      formatSustainedMessage(avgRecent, baseline.Mean, len(recent)),
		}, nil
	}

	return nil, nil
}

// GetTrend analyzes the latency trend over a period.
func (d *Detector) GetTrend(checkID int64, hours int) (*Trend, error) {
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)

	results, err := d.storage.GetResultsInRange(checkID, start, end)
	if err != nil {
		return nil, err
	}

	if len(results) < 2 {
		return nil, nil
	}

	// Split into first and second half
	mid := len(results) / 2
	firstHalf := results[:mid]
	secondHalf := results[mid:]

	var firstLatencies, secondLatencies []float64
	for _, r := range firstHalf {
		if r.IsUp() && r.ResponseTimeMs > 0 {
			firstLatencies = append(firstLatencies, float64(r.ResponseTimeMs))
		}
	}
	for _, r := range secondHalf {
		if r.IsUp() && r.ResponseTimeMs > 0 {
			secondLatencies = append(secondLatencies, float64(r.ResponseTimeMs))
		}
	}

	if len(firstLatencies) == 0 || len(secondLatencies) == 0 {
		return nil, nil
	}

	firstMean := calculateMean(firstLatencies)
	secondMean := calculateMean(secondLatencies)
	change := ((secondMean - firstMean) / firstMean) * 100

	direction := TrendStable
	if change > 10 {
		direction = TrendIncreasing
	} else if change < -10 {
		direction = TrendDecreasing
	}

	return &Trend{
		CheckID:       checkID,
		Direction:     direction,
		ChangePercent: change,
		PeriodHours:   hours,
		FirstMean:     firstMean,
		SecondMean:    secondMean,
		CalculatedAt:  time.Now(),
	}, nil
}

// Trend represents the latency trend direction.
type TrendDirection string

const (
	TrendIncreasing TrendDirection = "increasing"
	TrendDecreasing TrendDirection = "decreasing"
	TrendStable     TrendDirection = "stable"
)

// Trend represents a latency trend analysis.
type Trend struct {
	CheckID       int64          `json:"check_id"`
	Direction     TrendDirection `json:"direction"`
	ChangePercent float64        `json:"change_percent"`
	PeriodHours   int            `json:"period_hours"`
	FirstMean     float64        `json:"first_mean_ms"`
	SecondMean    float64        `json:"second_mean_ms"`
	CalculatedAt  time.Time      `json:"calculated_at"`
}

// Helper functions

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(values)-1)
	return math.Sqrt(variance)
}

func calculateMinMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	min, max := values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

func formatSpikeMessage(current, baseline, deviation float64) string {
	return formatMessage("Latency spike detected", current, baseline, deviation)
}

func formatDropMessage(current, baseline, deviation float64) string {
	return formatMessage("Unusual latency drop", current, baseline, deviation)
}

func formatSustainedMessage(avg, baseline float64, count int) string {
	pctIncrease := ((avg - baseline) / baseline) * 100
	return formatMessagef("Sustained elevated latency: %.0fms avg over last %d checks (%.0f%% above baseline)",
		avg, count, pctIncrease)
}

func formatMessage(prefix string, current, baseline, deviation float64) string {
	return formatMessagef("%s: %.0fms (baseline: %.0fms, %.1f sigma)",
		prefix, current, baseline, deviation)
}

func formatMessagef(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
