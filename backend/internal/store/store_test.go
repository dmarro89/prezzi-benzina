package store

import (
	"testing"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
)

func freshTime(daysAgo int) time.Time {
	now := time.Now().In(italyLocation)
	midday := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, italyLocation)
	return midday.AddDate(0, 0, -daysAgo).UTC()
}

func TestNearbyWithOverlayUsesNewerLivePrice(t *testing.T) {
	s := New()
	csvTime := freshTime(2)
	liveTime := freshTime(1)
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{123: {ID: 123, Latitude: 40.85, Longitude: 14.27}},
		Prices:   map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.699, Self: true, UpdatedAt: csvTime}}},
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
	csvTime := freshTime(0)
	liveTime := freshTime(1)
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{123: {ID: 123, Latitude: 40.85, Longitude: 14.27}},
		Prices:   map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.679, Self: true, UpdatedAt: csvTime}}},
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

func TestNearbyWithLiveKeepsStationAbsentFromCSV(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{1: {ID: 1, Latitude: 41, Longitude: 15}},
		Prices:   map[int64][]mimit.Price{},
	})
	live := []LiveStation{{
		Station: mimit.Station{ID: 38009, Name: "ESSO NOLA", Brand: "Esso", Latitude: 40.9285, Longitude: 14.5231},
		Prices:  []mimit.Price{{StationID: 38009, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(0)}},
	}}

	results, err := s.NearbyWithLive(Query{Latitude: 40.921459, Longitude: 14.5235855, RadiusKm: 2, Fuel: "benzina", Service: "self"}, live)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Station.ID != 38009 {
		t.Fatalf("live-only station must remain visible, got %+v", results)
	}
	if results[0].Price.Value != 1.989 {
		t.Fatalf("unexpected live price %.3f", results[0].Price.Value)
	}
}

func TestNearbyWithLiveUsesLiveLocationAndCSVMetadata(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
		38009: {ID: 38009, Name: "CSV name", Address: "Via Roma 1", City: "NOLA", Province: "NA", Latitude: 41.5, Longitude: 15.5},
		},
		Prices: map[int64][]mimit.Price{},
	})
	live := []LiveStation{{
		Station: mimit.Station{ID: 38009, Name: "ESSO NOLA", Brand: "Esso", Latitude: 40.9285, Longitude: 14.5231},
		Prices:  []mimit.Price{{StationID: 38009, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(0)}},
	}}

	results, err := s.NearbyWithLive(Query{Latitude: 40.921459, Longitude: 14.5235855, RadiusKm: 2, Fuel: "benzina", Service: "self"}, live)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected live location to keep station in radius, got %+v", results)
	}
	got := results[0].Station
	if got.Address != "Via Roma 1" || got.City != "NOLA" || got.Province != "NA" {
		t.Fatalf("expected CSV metadata enrichment, got %+v", got)
	}
	if got.Latitude != 40.9285 || got.Longitude != 14.5231 {
		t.Fatalf("expected live coordinates, got %+v", got)
	}
}

func TestNearbyWithLiveTreatsTodayPriceAsFresh(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations:  map[int64]mimit.Station{31268: {ID: 31268, Latitude: 40.912, Longitude: 14.5105}},
		Prices:    map[int64][]mimit.Price{},
		Extracted: freshTime(1),
	})
	live := []LiveStation{{
		Station: mimit.Station{ID: 31268, Name: "Saviano", Latitude: 40.912, Longitude: 14.5105},
		Prices:  []mimit.Price{{StationID: 31268, Fuel: "Benzina", Value: 2.020, Self: true, UpdatedAt: freshTime(0)}},
	}}

	results, err := s.NearbyWithLive(Query{Latitude: 40.921459, Longitude: 14.5235855, RadiusKm: 2, Fuel: "benzina", Service: "self"}, live)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("today's live price must not be stale just because CSV was extracted yesterday")
	}
}

func TestMergePricesComparesFuelAndServiceIndependently(t *testing.T) {
	baseTime := freshTime(2)
	liveTime := freshTime(1)
	base := []mimit.Price{
		{Fuel: "Benzina", Value: 1.70, Self: true, UpdatedAt: baseTime},
		{Fuel: "Benzina", Value: 1.82, Self: false, UpdatedAt: freshTime(0)},
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
	updated := freshTime(0)
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Test", Latitude: 40.85, Longitude: 14.27}},
		Prices: map[int64][]mimit.Price{1: {
			{StationID: 1, Fuel: "Benzina", Value: 1.699, Self: true, UpdatedAt: updated},
			{StationID: 1, Fuel: "Benzina", Value: 1.899, Self: false, UpdatedAt: updated},
		}},
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "benzina", Service: "any", Sort: "price"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].Prices) != 2 {
		t.Fatalf("expected self and served prices, got %+v", results)
	}
	if !results[0].Prices[0].Self || results[0].Prices[1].Self {
		t.Fatalf("expected Self first and Servito second, got %+v", results[0].Prices)
	}
}

func TestNearbyAnyKeepsServedOnlyStation(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Served only", Latitude: 40.85, Longitude: 14.27}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Gasolio", Value: 1.829, Self: false, UpdatedAt: freshTime(0)}}},
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 5, Fuel: "gasolio", Service: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].Prices) != 1 || results[0].Prices[0].Self {
		t.Fatalf("served-only station must remain visible, got %+v", results)
	}
}

func TestNearbyExcludesPricesOlderThanEightDays(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Stale", Latitude: 40.85, Longitude: 14.27}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(9)}}},
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Fresh enough", Latitude: 40.85, Longitude: 14.27}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Benzina", Value: 2.019, Self: true, UpdatedAt: freshTime(8)}}},
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Mixed freshness", Latitude: 40.85, Longitude: 14.27}},
		Prices: map[int64][]mimit.Price{1: {
			{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(9)},
			{StationID: 1, Fuel: "Benzina", Value: 2.149, Self: false, UpdatedAt: freshTime(1)},
		}},
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 10, Fuel: "benzina", Service: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].Prices) != 1 || results[0].Prices[0].Self {
		t.Fatalf("expected only fresh served price, got %+v", results)
	}
}

func TestNearbyCanIncludeStalePricesForDiagnostics(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Stale diagnostic", Latitude: 40.85, Longitude: 14.27}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(9)}}},
	})

	results, err := s.Nearby(Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 10, Fuel: "benzina", Service: "any", IncludeStale: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected diagnostic override to include stale price, got %d results", len(results))
	}
}
