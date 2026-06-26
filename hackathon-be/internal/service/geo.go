package service

import "math"

const earthRadiusM = 6371000.0

// DistanceMeters returns the great-circle distance in meters between two
// coordinates using the haversine formula.
func DistanceMeters(aLat, aLng, bLat, bLng float64) float64 {
	lat1 := radians(aLat)
	lat2 := radians(bLat)
	dLat := radians(bLat - aLat)
	dLng := radians(bLng - aLng)

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusM * c
}

func radians(deg float64) float64 {
	return deg * math.Pi / 180
}
