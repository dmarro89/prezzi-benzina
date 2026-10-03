package geo

import "testing"

func TestDistanceKm(t *testing.T) {
	d := DistanceKm(40.8518, 14.2681, 40.8271, 14.1931)
	if d < 4 || d > 8 { t.Fatalf("unexpected distance %.2f km", d) }
}
