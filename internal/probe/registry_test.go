package probe

import (
	"math"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	r := NewProbeRegistry()

	probe, err := r.Register("probe-1", "US-East", "New York", "USA", 40.7128, -74.0060)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if probe.ID != 1 {
		t.Errorf("probe.ID = %v, expected 1", probe.ID)
	}
	if probe.Name != "probe-1" {
		t.Errorf("probe.Name = %v, expected probe-1", probe.Name)
	}
	if probe.Region != "US-East" {
		t.Errorf("probe.Region = %v, expected US-East", probe.Region)
	}
	if probe.City != "New York" {
		t.Errorf("probe.City = %v, expected New York", probe.City)
	}
	if probe.Country != "USA" {
		t.Errorf("probe.Country = %v, expected USA", probe.Country)
	}
	if probe.Latitude != 40.7128 {
		t.Errorf("probe.Latitude = %v, expected 40.7128", probe.Latitude)
	}
	if probe.Longitude != -74.0060 {
		t.Errorf("probe.Longitude = %v, expected -74.0060", probe.Longitude)
	}
	if probe.APIKey == "" {
		t.Error("probe.APIKey is empty")
	}
	if probe.Status != "active" {
		t.Errorf("probe.Status = %v, expected active", probe.Status)
	}
	if probe.CreatedAt.IsZero() {
		t.Error("probe.CreatedAt is zero")
	}
	if probe.LastHeartbeat.IsZero() {
		t.Error("probe.LastHeartbeat is zero")
	}

	// Register second probe and verify ID increments
	probe2, err := r.Register("probe-2", "EU-West", "London", "UK", 51.5074, -0.1278)
	if err != nil {
		t.Fatalf("Register second probe failed: %v", err)
	}
	if probe2.ID != 2 {
		t.Errorf("probe2.ID = %v, expected 2", probe2.ID)
	}
}

func TestDeregister(t *testing.T) {
	r := NewProbeRegistry()

	probe, _ := r.Register("probe-1", "US-East", "New York", "USA", 40.7128, -74.0060)

	err := r.Deregister(probe.ID)
	if err != nil {
		t.Fatalf("Deregister failed: %v", err)
	}

	_, err = r.Get(probe.ID)
	if err != ErrProbeNotFound {
		t.Errorf("Get after deregister: err = %v, expected ErrProbeNotFound", err)
	}

	// Deregister non-existent probe
	err = r.Deregister(999)
	if err != ErrProbeNotFound {
		t.Errorf("Deregister non-existent: err = %v, expected ErrProbeNotFound", err)
	}
}

func TestHeartbeat(t *testing.T) {
	r := NewProbeRegistry()

	probe, _ := r.Register("probe-1", "US-East", "New York", "USA", 40.7128, -74.0060)
	originalHeartbeat := probe.LastHeartbeat

	time.Sleep(10 * time.Millisecond)

	err := r.Heartbeat(probe.ID)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	updatedProbe, _ := r.Get(probe.ID)
	if !updatedProbe.LastHeartbeat.After(originalHeartbeat) {
		t.Error("LastHeartbeat was not updated")
	}
	if updatedProbe.Status != "active" {
		t.Errorf("Status = %v, expected active", updatedProbe.Status)
	}

	// Heartbeat non-existent probe
	err = r.Heartbeat(999)
	if err != ErrProbeNotFound {
		t.Errorf("Heartbeat non-existent: err = %v, expected ErrProbeNotFound", err)
	}
}

func TestListActive(t *testing.T) {
	r := NewProbeRegistry()
	r.heartbeatTimeout = 50 * time.Millisecond

	// Register two probes
	probe1, _ := r.Register("probe-1", "US-East", "New York", "USA", 40.7128, -74.0060)
	probe2, _ := r.Register("probe-2", "EU-West", "London", "UK", 51.5074, -0.1278)

	// Both should be active initially
	active := r.ListActive()
	if len(active) != 2 {
		t.Errorf("ListActive() returned %v probes, expected 2", len(active))
	}

	// Wait for timeout, then update only probe1's heartbeat
	time.Sleep(60 * time.Millisecond)
	r.Heartbeat(probe1.ID)

	active = r.ListActive()
	if len(active) != 1 {
		t.Errorf("ListActive() after timeout returned %v probes, expected 1", len(active))
	}
	if len(active) > 0 && active[0].ID != probe1.ID {
		t.Errorf("ListActive() returned probe %v, expected %v", active[0].ID, probe1.ID)
	}

	// Manually set probe2 status to inactive
	probe2.Status = "inactive"
	r.Heartbeat(probe2.ID) // This should reactivate it
	probe2Updated, _ := r.Get(probe2.ID)
	if probe2Updated.Status != "active" {
		t.Errorf("probe2 status after heartbeat = %v, expected active", probe2Updated.Status)
	}
}

func TestCleanupStale(t *testing.T) {
	r := NewProbeRegistry()
	r.heartbeatTimeout = 50 * time.Millisecond

	// Register probes
	r.Register("probe-1", "US-East", "New York", "USA", 40.7128, -74.0060)
	probe2, _ := r.Register("probe-2", "EU-West", "London", "UK", 51.5074, -0.1278)

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Keep probe2 alive
	r.Heartbeat(probe2.ID)

	// Cleanup should remove probe1
	removed := r.CleanupStale()
	if removed != 1 {
		t.Errorf("CleanupStale() removed %v probes, expected 1", removed)
	}

	// Verify only probe2 remains
	_, err := r.Get(1)
	if err != ErrProbeNotFound {
		t.Error("probe1 should have been removed")
	}

	_, err = r.Get(probe2.ID)
	if err != nil {
		t.Error("probe2 should still exist")
	}
}

func TestListByRegion(t *testing.T) {
	r := NewProbeRegistry()

	r.Register("probe-1", "US-East", "New York", "USA", 40.7128, -74.0060)
	r.Register("probe-2", "US-East", "Boston", "USA", 42.3601, -71.0589)
	r.Register("probe-3", "EU-West", "London", "UK", 51.5074, -0.1278)

	usEastProbes := r.ListByRegion("US-East")
	if len(usEastProbes) != 2 {
		t.Errorf("ListByRegion(US-East) returned %v probes, expected 2", len(usEastProbes))
	}

	euWestProbes := r.ListByRegion("EU-West")
	if len(euWestProbes) != 1 {
		t.Errorf("ListByRegion(EU-West) returned %v probes, expected 1", len(euWestProbes))
	}

	asiaProbes := r.ListByRegion("Asia-Pacific")
	if len(asiaProbes) != 0 {
		t.Errorf("ListByRegion(Asia-Pacific) returned %v probes, expected 0", len(asiaProbes))
	}
}

func TestDistance(t *testing.T) {
	r := NewProbeRegistry()

	// NYC and London
	probeNYC, _ := r.Register("probe-nyc", "US-East", "New York", "USA", 40.7128, -74.0060)
	probeLondon, _ := r.Register("probe-london", "EU-West", "London", "UK", 51.5074, -0.1278)

	distance, err := r.Distance(probeNYC.ID, probeLondon.ID)
	if err != nil {
		t.Fatalf("Distance failed: %v", err)
	}

	expected := 5570.0
	tolerance := 50.0
	if math.Abs(distance-expected) > tolerance {
		t.Errorf("Distance = %v km, expected ~%v km (tolerance %v km)", distance, expected, tolerance)
	}

	// Test with non-existent probe
	_, err = r.Distance(probeNYC.ID, 999)
	if err == nil {
		t.Error("Distance with non-existent probe should fail")
	}

	_, err = r.Distance(999, probeLondon.ID)
	if err == nil {
		t.Error("Distance with non-existent probe should fail")
	}
}

func TestHaversine(t *testing.T) {
	// Test known distances
	tests := []struct {
		name      string
		lat1      float64
		lon1      float64
		lat2      float64
		lon2      float64
		expected  float64
		tolerance float64
	}{
		{
			name:      "NYC to London",
			lat1:      40.7128,
			lon1:      -74.0060,
			lat2:      51.5074,
			lon2:      -0.1278,
			expected:  5570,
			tolerance: 50,
		},
		{
			name:      "Same point",
			lat1:      40.7128,
			lon1:      -74.0060,
			lat2:      40.7128,
			lon2:      -74.0060,
			expected:  0,
			tolerance: 0.001,
		},
		{
			name:      "Antipodal points",
			lat1:      0,
			lon1:      0,
			lat2:      0,
			lon2:      180,
			expected:  20015, // Half earth circumference
			tolerance: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := Haversine(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(distance-tt.expected) > tt.tolerance {
				t.Errorf("Haversine(%v, %v, %v, %v) = %v, expected ~%v (tolerance %v)",
					tt.lat1, tt.lon1, tt.lat2, tt.lon2, distance, tt.expected, tt.tolerance)
			}
		})
	}
}
