package service

import (
	"math"
	"testing"
)

func TestDistanceMeters(t *testing.T) {
	tests := []struct {
		name                   string
		aLat, aLng, bLat, bLng float64
		want                   float64
		tol                    float64
	}{
		{"same point", 0, 0, 0, 0, 0, 0.001},
		{"one degree latitude", 0, 0, 1, 0, 111194.9, 50},
		{"short hop", 40.7128, -74.0060, 40.7138, -74.0060, 111.2, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DistanceMeters(tt.aLat, tt.aLng, tt.bLat, tt.bLng)
			if math.Abs(got-tt.want) > tt.tol {
				t.Fatalf("DistanceMeters = %.3f, want %.3f (±%.3f)", got, tt.want, tt.tol)
			}
		})
	}
}

func TestDistanceMetersSymmetric(t *testing.T) {
	d1 := DistanceMeters(51.5074, -0.1278, 48.8566, 2.3522) // London -> Paris
	d2 := DistanceMeters(48.8566, 2.3522, 51.5074, -0.1278)
	if math.Abs(d1-d2) > 0.001 {
		t.Fatalf("distance not symmetric: %.3f vs %.3f", d1, d2)
	}
	// London to Paris is ~343 km.
	if d1 < 330000 || d1 > 355000 {
		t.Fatalf("London-Paris distance out of expected range: %.0f m", d1)
	}
}
