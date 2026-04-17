package probe

import (
	"context"
	"sync"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

// AggregatedResult contains the combined status of a check across all probes.
type AggregatedResult struct {
	CheckID       int64
	OverallStatus string // "up", "degraded", "down"
	TotalProbes   int
	UpProbes      int
	DownProbes    int
	Regions       map[string]RegionStatus
	CheckedAt     time.Time
}

// RegionStatus contains the status of probes in a specific region.
type RegionStatus struct {
	Status       string // "up", "down"
	Probes       int
	UpProbes     int
	AvgLatencyMs float64
}

// OutageReport contains information about detected outages.
type OutageReport struct {
	CheckID         int64
	OutageType      string // "regional", "global", "none"
	AffectedRegions []string
	FailingProbes   []int64
	PassingProbes   []int64
	DetectedAt      time.Time
}

// LatencyStats contains latency statistics for a region.
type LatencyStats struct {
	Region      string
	MinMs       float64
	MaxMs       float64
	AvgMs       float64
	SampleCount int
}

// Coordinator manages probe assignments and result aggregation.
type Coordinator struct {
	registry *ProbeRegistry
	storage  storage.Storage
	mu       sync.RWMutex
}

// NewCoordinator creates a new coordinator with the given registry and storage.
func NewCoordinator(registry *ProbeRegistry, store storage.Storage) *Coordinator {
	return &Coordinator{
		registry: registry,
		storage:  store,
	}
}

// AssignChecks assigns enabled checks to available probes.
// For each enabled check, it finds active probes that can run it.
func (c *Coordinator) AssignChecks(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get all enabled checks
	checks, err := c.storage.ListEnabledChecks()
	if err != nil {
		return err
	}

	// Get all active probes
	activeProbes := c.registry.ListActive()
	if len(activeProbes) == 0 {
		// No active probes available
		return nil
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Find probes that can run this check
		var assignedProbes []*ProbeInfo

		if len(check.Regions) > 0 {
			// Check has specific regions, assign probes from those regions
			for _, region := range check.Regions {
				regionProbes := c.registry.ListByRegion(region)
				for _, p := range regionProbes {
					if p.Status == "active" {
						assignedProbes = append(assignedProbes, p)
					}
				}
			}
		} else {
			// No specific regions, use all active probes
			assignedProbes = activeProbes
		}

		// Store assignments - in this implementation, we simply ensure
		// probes are available. The actual assignment is implicit via
		// region matching in the poll API.
		_ = assignedProbes
	}

	return nil
}

// AggregateResults collects results from all probes for a check and determines overall status.
func (c *Coordinator) AggregateResults(ctx context.Context, checkID int64) (*AggregatedResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get latest results by region
	resultsByRegion, err := c.storage.GetLatestProbeResultsByRegion(checkID)
	if err != nil {
		return nil, err
	}

	if len(resultsByRegion) == 0 {
		return &AggregatedResult{
			CheckID:       checkID,
			OverallStatus: "pending",
			TotalProbes:   0,
			UpProbes:      0,
			DownProbes:    0,
			Regions:       make(map[string]RegionStatus),
			CheckedAt:     time.Now(),
		}, nil
	}

	// Group results by region and calculate stats
	regions := make(map[string]RegionStatus)
	totalProbes := 0
	upProbes := 0
	downProbes := 0
	var latestCheck time.Time

	// First pass: collect all results and group by region
	regionResults := make(map[string][]*storage.ProbeResult)
	for region, result := range resultsByRegion {
		regionResults[region] = append(regionResults[region], result)
	}

	// Also get recent results to get more data per region
	recentResults, err := c.storage.GetProbeResults(checkID, 100, 0)
	if err != nil {
		return nil, err
	}

	// Group recent results by region (using probe lookup)
	probeRegions := make(map[int64]string)
	probes, err := c.storage.ListProbes()
	if err != nil {
		return nil, err
	}
	for _, p := range probes {
		probeRegions[p.ID] = p.Region
	}

	for _, result := range recentResults {
		region := probeRegions[result.ProbeID]
		if region != "" {
			regionResults[region] = append(regionResults[region], result)
		}
		if result.CheckedAt.After(latestCheck) {
			latestCheck = result.CheckedAt
		}
	}

	// Calculate per-region stats
	for region, results := range regionResults {
		regionUp := 0
		regionDown := 0
		var totalLatency float64
		latencyCount := 0

		// Dedupe by probe ID, keep only latest per probe
		latestByProbe := make(map[int64]*storage.ProbeResult)
		for _, r := range results {
			existing, ok := latestByProbe[r.ProbeID]
			if !ok || r.CheckedAt.After(existing.CheckedAt) {
				latestByProbe[r.ProbeID] = r
			}
		}

		for _, r := range latestByProbe {
			if r.Status == "up" {
				regionUp++
				upProbes++
			} else {
				regionDown++
				downProbes++
			}
			totalProbes++

			if r.ResponseTimeMs.Valid {
				totalLatency += float64(r.ResponseTimeMs.Int64)
				latencyCount++
			}
		}

		avgLatency := 0.0
		if latencyCount > 0 {
			avgLatency = totalLatency / float64(latencyCount)
		}

		status := "up"
		if regionUp == 0 && regionDown > 0 {
			status = "down"
		}

		regions[region] = RegionStatus{
			Status:       status,
			Probes:       regionUp + regionDown,
			UpProbes:     regionUp,
			AvgLatencyMs: avgLatency,
		}
	}

	// Determine overall status
	overallStatus := "up"
	if downProbes > 0 && upProbes > 0 {
		overallStatus = "degraded"
	} else if downProbes > 0 && upProbes == 0 {
		overallStatus = "down"
	}

	return &AggregatedResult{
		CheckID:       checkID,
		OverallStatus: overallStatus,
		TotalProbes:   totalProbes,
		UpProbes:      upProbes,
		DownProbes:    downProbes,
		Regions:       regions,
		CheckedAt:     latestCheck,
	}, nil
}

// DetectRegionalOutage analyzes probe results to detect regional or global outages.
// If >=50% of probes in a region fail but others succeed, it's a regional outage.
// If all fail, it's a global outage.
func (c *Coordinator) DetectRegionalOutage(ctx context.Context, checkID int64) (*OutageReport, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get latest results by region
	resultsByRegion, err := c.storage.GetLatestProbeResultsByRegion(checkID)
	if err != nil {
		return nil, err
	}

	if len(resultsByRegion) == 0 {
		return &OutageReport{
			CheckID:         checkID,
			OutageType:      "none",
			AffectedRegions: []string{},
			FailingProbes:   []int64{},
			PassingProbes:   []int64{},
			DetectedAt:      time.Now(),
		}, nil
	}

	// Get probe info for region mapping
	probeRegions := make(map[int64]string)
	probes, err := c.storage.ListProbes()
	if err != nil {
		return nil, err
	}
	for _, p := range probes {
		probeRegions[p.ID] = p.Region
	}

	// Get recent results to analyze by region
	recentResults, err := c.storage.GetProbeResults(checkID, 100, 0)
	if err != nil {
		return nil, err
	}

	// Group by region and keep latest per probe
	regionProbeResults := make(map[string]map[int64]*storage.ProbeResult)
	for _, r := range recentResults {
		region := probeRegions[r.ProbeID]
		if region == "" {
			continue
		}
		if regionProbeResults[region] == nil {
			regionProbeResults[region] = make(map[int64]*storage.ProbeResult)
		}
		existing, ok := regionProbeResults[region][r.ProbeID]
		if !ok || r.CheckedAt.After(existing.CheckedAt) {
			regionProbeResults[region][r.ProbeID] = r
		}
	}

	// Also include results from GetLatestProbeResultsByRegion
	for region, r := range resultsByRegion {
		if regionProbeResults[region] == nil {
			regionProbeResults[region] = make(map[int64]*storage.ProbeResult)
		}
		existing, ok := regionProbeResults[region][r.ProbeID]
		if !ok || r.CheckedAt.After(existing.CheckedAt) {
			regionProbeResults[region][r.ProbeID] = r
		}
	}

	// Analyze each region
	var affectedRegions []string
	var failingProbes []int64
	var passingProbes []int64
	totalRegions := 0
	failingRegions := 0

	for region, probeResults := range regionProbeResults {
		totalRegions++
		upCount := 0
		downCount := 0

		for probeID, r := range probeResults {
			if r.Status == "up" {
				upCount++
				passingProbes = append(passingProbes, probeID)
			} else {
				downCount++
				failingProbes = append(failingProbes, probeID)
			}
		}

		totalProbesInRegion := upCount + downCount
		if totalProbesInRegion > 0 {
			failRatio := float64(downCount) / float64(totalProbesInRegion)
			if failRatio >= 0.5 {
				affectedRegions = append(affectedRegions, region)
				failingRegions++
			}
		}
	}

	// Determine outage type
	outageType := "none"
	if failingRegions > 0 {
		if failingRegions == totalRegions {
			outageType = "global"
		} else {
			outageType = "regional"
		}
	}

	return &OutageReport{
		CheckID:         checkID,
		OutageType:      outageType,
		AffectedRegions: affectedRegions,
		FailingProbes:   failingProbes,
		PassingProbes:   passingProbes,
		DetectedAt:      time.Now(),
	}, nil
}

// CompareLatency computes latency statistics per region from recent probe results.
func (c *Coordinator) CompareLatency(ctx context.Context, checkID int64) (map[string]LatencyStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get probe info for region mapping
	probeRegions := make(map[int64]string)
	probes, err := c.storage.ListProbes()
	if err != nil {
		return nil, err
	}
	for _, p := range probes {
		probeRegions[p.ID] = p.Region
	}

	// Get recent results
	recentResults, err := c.storage.GetProbeResults(checkID, 100, 0)
	if err != nil {
		return nil, err
	}

	// Group latencies by region
	regionLatencies := make(map[string][]float64)
	for _, r := range recentResults {
		region := probeRegions[r.ProbeID]
		if region == "" {
			continue
		}
		if r.ResponseTimeMs.Valid {
			regionLatencies[region] = append(regionLatencies[region], float64(r.ResponseTimeMs.Int64))
		}
	}

	// Calculate stats per region
	result := make(map[string]LatencyStats)
	for region, latencies := range regionLatencies {
		if len(latencies) == 0 {
			continue
		}

		minMs := latencies[0]
		maxMs := latencies[0]
		totalMs := 0.0

		for _, lat := range latencies {
			if lat < minMs {
				minMs = lat
			}
			if lat > maxMs {
				maxMs = lat
			}
			totalMs += lat
		}

		avgMs := totalMs / float64(len(latencies))

		result[region] = LatencyStats{
			Region:      region,
			MinMs:       minMs,
			MaxMs:       maxMs,
			AvgMs:       avgMs,
			SampleCount: len(latencies),
		}
	}

	return result, nil
}
