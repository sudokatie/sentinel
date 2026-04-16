package probe

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrProbeNotFound = errors.New("probe not found")
	ErrInvalidProbe  = errors.New("invalid probe")
)

// ProbeInfo represents a monitoring probe at a specific location.
type ProbeInfo struct {
	ID            int64
	Name          string
	Region        string
	City          string
	Country       string
	Latitude      float64
	Longitude     float64
	APIKey        string
	Status        string // "active" or "inactive"
	LastHeartbeat time.Time
	CreatedAt     time.Time
}

// ProbeRegistry manages monitoring probes.
type ProbeRegistry struct {
	probes           map[int64]*ProbeInfo
	nextID           int64
	mu               sync.RWMutex
	heartbeatTimeout time.Duration
}

// NewProbeRegistry creates a new probe registry with default settings.
func NewProbeRegistry() *ProbeRegistry {
	return &ProbeRegistry{
		probes:           make(map[int64]*ProbeInfo),
		nextID:           1,
		heartbeatTimeout: 90 * time.Second,
	}
}

// generateAPIKey creates a UUID-based API key.
func generateAPIKey() (string, error) {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		return "", err
	}
	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// Register adds a new probe to the registry.
func (r *ProbeRegistry) Register(name, region, city, country string, lat, lon float64) (*ProbeInfo, error) {
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	probe := &ProbeInfo{
		ID:            r.nextID,
		Name:          name,
		Region:        region,
		City:          city,
		Country:       country,
		Latitude:      lat,
		Longitude:     lon,
		APIKey:        apiKey,
		Status:        "active",
		LastHeartbeat: now,
		CreatedAt:     now,
	}

	r.probes[probe.ID] = probe
	r.nextID++

	return probe, nil
}

// Deregister removes a probe from the registry.
func (r *ProbeRegistry) Deregister(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.probes[id]; !exists {
		return ErrProbeNotFound
	}

	delete(r.probes, id)
	return nil
}

// Heartbeat updates the last heartbeat time and sets status to active.
func (r *ProbeRegistry) Heartbeat(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	probe, exists := r.probes[id]
	if !exists {
		return ErrProbeNotFound
	}

	probe.LastHeartbeat = time.Now()
	probe.Status = "active"
	return nil
}

// Get returns a probe by ID.
func (r *ProbeRegistry) Get(id int64) (*ProbeInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	probe, exists := r.probes[id]
	if !exists {
		return nil, ErrProbeNotFound
	}

	return probe, nil
}

// ListByRegion returns all probes in a specific region.
func (r *ProbeRegistry) ListByRegion(region string) []*ProbeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ProbeInfo
	for _, probe := range r.probes {
		if probe.Region == region {
			result = append(result, probe)
		}
	}
	return result
}

// ListActive returns all probes that are active and have sent a heartbeat
// within the timeout period.
func (r *ProbeRegistry) ListActive() []*ProbeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cutoff := time.Now().Add(-r.heartbeatTimeout)
	var result []*ProbeInfo
	for _, probe := range r.probes {
		if probe.Status == "active" && probe.LastHeartbeat.After(cutoff) {
			result = append(result, probe)
		}
	}
	return result
}

// CleanupStale removes probes with heartbeats older than the timeout.
// Returns the number of probes removed.
func (r *ProbeRegistry) CleanupStale() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-r.heartbeatTimeout)
	removed := 0
	for id, probe := range r.probes {
		if probe.LastHeartbeat.Before(cutoff) {
			delete(r.probes, id)
			removed++
		}
	}
	return removed
}

// Distance calculates the haversine distance between two probes in kilometers.
func (r *ProbeRegistry) Distance(id1, id2 int64) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	probe1, exists := r.probes[id1]
	if !exists {
		return 0, fmt.Errorf("probe %d: %w", id1, ErrProbeNotFound)
	}

	probe2, exists := r.probes[id2]
	if !exists {
		return 0, fmt.Errorf("probe %d: %w", id2, ErrProbeNotFound)
	}

	return Haversine(probe1.Latitude, probe1.Longitude, probe2.Latitude, probe2.Longitude), nil
}
