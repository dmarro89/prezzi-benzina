package store

import (
	"testing"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
)

func TestNearbyWithOverlayUsesNewerLivePrice(t *testing.T) {
	s := New()
	csvTime := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	liveTime := csvTime.Add(26 * time.Hour)
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{123: {ID: 123, Latitude: 40.85, Longitude: 14.27}},
		Prices: map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.699, Self: true, UpdatedAt: csvTime}}},
	})
	overlay := map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.659, Self: true, UpdatedAt: liveTime}}}
	results, err := s.NearbyWithOverlay(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "benzina", Service: "self"}, overlay)
	if err != nil { t.Fatal(err) }
	if got := results[0].Price.Value; got != 1.659 { t.Fatalf("expected live price, got %.3f", got) }
}

func TestNearbyWithOverlayKeepsNewerCSVPrice(t *testing.T) {
	s := New()
	csvTime := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	liveTime := csvTime.Add(-time.Hour)
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{123: {ID: 123, Latitude: 40.85, Longitude: 14.27}},
		Prices: map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.679, Self: true, UpdatedAt: csvTime}}},
	})
	overlay := map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.659, Self: true, UpdatedAt: liveTime}}}
	results, err := s.NearbyWithOverlay(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "benzina", Service: "self"}, overlay)
	if err != nil { t.Fatal(err) }
	if got := results[0].Price.Value; got != 1.679 { t.Fatalf("expected CSV price, got %.3f", got) }
}

func TestMergePricesComparesFuelAndServiceIndependently(t *testing.T) {
	baseTime := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	liveTime := baseTime.Add(24 * time.Hour)
	base := []mimit.Price{
		{Fuel: "Benzina", Value: 1.70, Self: true, UpdatedAt: baseTime},
		{Fuel: "Benzina", Value: 1.82, Self: false, UpdatedAt: liveTime.Add(time.Hour)},
	}
	live := []mimit.Price{
		{Fuel: "Benzina", Value: 1.65, Self: true, UpdatedAt: liveTime},
		{Fuel: "Benzina", Value: 1.79, Self: false, UpdatedAt: liveTime},
	}
	merged := mergePrices(base, live)
	self, _ := bestPrice(merged, "benzina", "self")
	served, _ := bestPrice(merged, "benzina", "served")
	if self.Value != 1.65 { t.Fatalf("expected live self price, got %.2f", self.Value) }
	if served.Value != 1.82 { t.Fatalf("expected newer CSV served price, got %.2f", served.Value) }
}
