package store

import (
	"testing"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
)

var testExtraction = time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)

func TestNearbyWithOverlayUsesNewerLivePrice(t *testing.T) {
	s := New()
	csvTime := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	liveTime := csvTime.Add(26 * time.Hour)
	s.Replace(mimit.Dataset{
		Stations:  map[int64]mimit.Station{123: {ID: 123, Latitude: 40.85, Longitude: 14.27}},
		Prices:    map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.699, Self: true, UpdatedAt: csvTime}}},
		Extracted: testExtraction,
	})
	overlay := map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.659, Self: true, UpdatedAt: liveTime}}}
	results, err := s.NearbyWithOverlay(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "benzina", Service: "self"}, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if got := results[0].Price.Value; got != 1.659 {
		t.Fatalf("expected live price, got %.3f", got)
	}
}

func TestNearbyWithOverlayKeepsNewerCSVPrice(t *testing.T) {
	s := New()
	csvTime := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	liveTime := csvTime.Add(-time.Hour)
	s.Replace(mimit.Dataset{
		Stations:  map[int64]mimit.Station{123: {ID: 123, Latitude: 40.85, Longitude: 14.27}},
		Prices:    map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.679, Self: true, UpdatedAt: csvTime}}},
		Extracted: testExtraction,
	})
	overlay := map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.659, Self: true, UpdatedAt: liveTime}}}
	results, err := s.NearbyWithOverlay(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "benzina", Service: "self"}, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if got := results[0].Price.Value; got != 1.679 {
		t.Fatalf("expected CSV price, got %.3f", got)
	}
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
	if self.Value != 1.65 {
		t.Fatalf("expected live self price, got %.2f", self.Value)
	}
	if served.Value != 1.82 {
		t.Fatalf("expected newer CSV served price, got %.2f", served.Value)
	}
}

func TestNearbyAnyReturnsSelfAndServed(t *testing.T) {
	s := New()
	updated := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			1: {ID: 1, Name: "Test", Latitude: 40.85, Longitude: 14.27},
		},
		Prices: map[int64][]mimit.Price{
			1: {
				{StationID: 1, Fuel: "Benzina", Value: 1.699, Self: true, UpdatedAt: updated},
				{StationID: 1, Fuel: "Benzina", Value: 1.899, Self: false, UpdatedAt: updated},
			},
		},
		Extracted: testExtraction,
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "benzina", Service: "any", Sort: "price"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one station, got %d", len(results))
	}
	if len(results[0].Prices) != 2 {
		t.Fatalf("expected self and served prices, got %+v", results[0].Prices)
	}
	if !results[0].Prices[0].Self || results[0].Prices[1].Self {
		t.Fatalf("expected Self first and Servito second, got %+v", results[0].Prices)
	}
	if results[0].Price.Value != 1.699 {
		t.Fatalf("expected cheapest primary price, got %.3f", results[0].Price.Value)
	}
}

func TestNearbyAnyKeepsServedOnlyStation(t *testing.T) {
	s := New()
	updated := time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			1: {ID: 1, Name: "Served only", Latitude: 40.85, Longitude: 14.27},
		},
		Prices: map[int64][]mimit.Price{
			1: {{StationID: 1, Fuel: "Gasolio", Value: 1.829, Self: false, UpdatedAt: updated}},
		},
		Extracted: testExtraction,
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "gasolio", Service: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("served-only station must not disappear, got %d results", len(results))
	}
	if len(results[0].Prices) != 1 || results[0].Prices[0].Self {
		t.Fatalf("unexpected prices: %+v", results[0].Prices)
	}
}

func TestNearbyExcludesPricesOlderThanEightDays(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			1: {ID: 1, Name: "Stale", Latitude: 40.85, Longitude: 14.27},
		},
		Prices: map[int64][]mimit.Price{
			1: {{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: time.Date(2026, 9, 11, 7, 27, 0, 0, time.UTC)}},
		},
		Extracted: testExtraction,
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 10, Fuel: "benzina", Service: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected stale price to be excluded, got %+v", results)
	}
}

func TestNearbyKeepsPriceOnEighthDay(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			1: {ID: 1, Name: "Fresh enough", Latitude: 40.85, Longitude: 14.27},
		},
		Prices: map[int64][]mimit.Price{
			1: {{StationID: 1, Fuel: "Benzina", Value: 2.019, Self: true, UpdatedAt: time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC)}},
		},
		Extracted: testExtraction,
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 10, Fuel: "benzina", Service: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected price on eighth day to remain visible, got %d results", len(results))
	}
}

func TestNearbyDropsOnlyStaleServicePrice(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			1: {ID: 1, Name: "Mixed freshness", Latitude: 40.85, Longitude: 14.27},
		},
		Prices: map[int64][]mimit.Price{
			1: {
				{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: time.Date(2026, 9, 11, 7, 27, 0, 0, time.UTC)},
				{StationID: 1, Fuel: "Benzina", Value: 2.149, Self: false, UpdatedAt: time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)},
			},
		},
		Extracted: testExtraction,
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 10, Fuel: "benzina", Service: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].Prices) != 1 {
		t.Fatalf("expected station with only fresh service price, got %+v", results)
	}
	if results[0].Prices[0].Self {
		t.Fatalf("expected stale self price to be removed, got %+v", results[0].Prices)
	}
}

func TestNearbyCanIncludeStalePricesForDiagnostics(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			1: {ID: 1, Name: "Stale diagnostic", Latitude: 40.85, Longitude: 14.27},
		},
		Prices: map[int64][]mimit.Price{
			1: {{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: time.Date(2026, 9, 11, 7, 27, 0, 0, time.UTC)}},
		},
		Extracted: testExtraction,
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 10, Fuel: "benzina", Service: "any", IncludeStale: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected diagnostic override to include stale price, got %d results", len(results))
	}
}
