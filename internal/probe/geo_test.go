package probe

import (
	"math"
	"testing"
)

func TestHaversineNYCLondon(t *testing.T) {
	// NYC: 40.7128, -74.0060
	// London: 51.5074, -0.1278
	// Expected distance: ~5570 km
	nycLat, nycLon := 40.7128, -74.0060
	londonLat, londonLon := 51.5074, -0.1278

	distance := Haversine(nycLat, nycLon, londonLat, londonLon)

	expected := 5570.0
	tolerance := 50.0

	if math.Abs(distance-expected) > tolerance {
		t.Errorf("NYC to London distance = %v km, expected ~%v km (tolerance %v km)", distance, expected, tolerance)
	}
}

func TestHaversineZeroDistance(t *testing.T) {
	lat, lon := 40.7128, -74.0060

	distance := Haversine(lat, lon, lat, lon)

	if distance != 0 {
		t.Errorf("same point distance = %v, expected 0", distance)
	}
}

func TestRegionFromCoords(t *testing.T) {
	tests := []struct {
		name     string
		lat      float64
		lon      float64
		expected string
	}{
		{
			name:     "New York City",
			lat:      40.7128,
			lon:      -74.0060,
			expected: "US-East",
		},
		{
			name:     "Los Angeles",
			lat:      34.0522,
			lon:      -118.2437,
			expected: "US-West",
		},
		{
			name:     "London",
			lat:      51.5074,
			lon:      -0.1278,
			expected: "EU-West",
		},
		{
			name:     "Berlin",
			lat:      52.5200,
			lon:      13.4050,
			expected: "EU-Central",
		},
		{
			name:     "Tokyo",
			lat:      35.6762,
			lon:      139.6503,
			expected: "Asia-Pacific",
		},
		{
			name:     "Sydney",
			lat:      -33.8688,
			lon:      151.2093,
			expected: "Asia-Pacific",
		},
		{
			name:     "Sao Paulo",
			lat:      -23.5505,
			lon:      -46.6333,
			expected: "South-America",
		},
		{
			name:     "Cairo",
			lat:      30.0444,
			lon:      31.2357,
			expected: "Africa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region := RegionFromCoords(tt.lat, tt.lon)
			if region != tt.expected {
				t.Errorf("RegionFromCoords(%v, %v) = %v, expected %v", tt.lat, tt.lon, region, tt.expected)
			}
		})
	}
}
