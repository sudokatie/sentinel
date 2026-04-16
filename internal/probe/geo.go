package probe

import "math"

const earthRadiusKm = 6371.0

// Haversine calculates the great circle distance between two points
// on Earth given their latitude and longitude in degrees.
// Returns distance in kilometers.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

// RegionFromCoords returns a rough geographic region based on latitude and longitude.
// Regions: US-East, US-West, EU-West, EU-Central, Asia-Pacific, South-America, Africa, Other
func RegionFromCoords(lat, lon float64) string {
	// North America
	if lat >= 25 && lat <= 50 && lon >= -130 && lon <= -60 {
		if lon >= -100 {
			return "US-East"
		}
		return "US-West"
	}

	// Europe
	if lat >= 35 && lat <= 72 && lon >= -10 && lon <= 40 {
		if lon <= 10 {
			return "EU-West"
		}
		return "EU-Central"
	}

	// Asia-Pacific (rough bounds)
	if lat >= -50 && lat <= 60 && lon >= 60 && lon <= 180 {
		return "Asia-Pacific"
	}
	if lat >= -50 && lat <= 60 && lon >= -180 && lon <= -100 {
		return "Asia-Pacific"
	}

	// South America
	if lat >= -60 && lat <= 15 && lon >= -85 && lon <= -30 {
		return "South-America"
	}

	// Africa
	if lat >= -40 && lat <= 40 && lon >= -20 && lon <= 55 {
		return "Africa"
	}

	return "Other"
}
