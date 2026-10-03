package store

import (
	"testing"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
)

const (
	testLat = 45.0
	testLng = 9.0
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
		Stations: map[int64]mimit.Station{123: {ID: 123, Latitude: testLat, Longitude: testLng}},
		Prices:   map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.699, Self: true, UpdatedAt: csvTime}}},
	})
	overlay := map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.659, Self: true, UpdatedAt: liveTime}}}
	results, err := s.NearbyWithOverlay(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 5, Fuel: "benzina", Service: "self"}, overlay)
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
		Stations: map[int64]mimit.Station{123: {ID: 123, Latitude: testLat, Longitude: testLng}},
		Prices:   map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.679, Self: true, UpdatedAt: csvTime}}},
	})
	overlay := map[int64][]mimit.Price{123: {{StationID: 123, Fuel: "Benzina", Value: 1.659, Self: true, UpdatedAt: liveTime}}}
	results, err := s.NearbyWithOverlay(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 5, Fuel: "benzina", Service: "self"}, overlay)
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Latitude: testLat, Longitude: testLng}},
		Prices:   map[int64][]mimit.Price{},
	})
	live := []LiveStation{{
		Station: mimit.Station{ID: 90001, Name: "Live Station", Brand: "Live Brand", Latitude: testLat + 0.001, Longitude: testLng + 0.001},
		Prices:  []mimit.Price{{StationID: 90001, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(0)}},
	}}

	results, err := s.NearbyWithLive(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 2, Fuel: "benzina", Service: "self"}, live)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Station.ID != 90001 {
		t.Fatalf("live-only station must remain visible, got %+v", results)
	}
	if results[0].Price.Value != 1.989 {
		t.Fatalf("unexpected live price %.3f", results[0].Price.Value)
	}
}

func TestNearbyWithLiveUsesLiveLocationAndCSVMetadataForSameID(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			90002: {ID: 90002, Name: "CSV Name", Address: "Via Test 1", City: "TESTVILLE", Province: "TT", Latitude: 46, Longitude: 10},
		},
		Prices: map[int64][]mimit.Price{},
	})
	live := []LiveStation{{
		Station: mimit.Station{ID: 90002, Name: "Live Name", Brand: "Live Brand", Latitude: testLat + 0.001, Longitude: testLng + 0.001},
		Prices:  []mimit.Price{{StationID: 90002, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(0)}},
	}}

	results, err := s.NearbyWithLive(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 2, Fuel: "benzina", Service: "self"}, live)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected live location to keep station in radius, got %+v", results)
	}
	got := results[0].Station
	if got.Address != "Via Test 1" || got.City != "TESTVILLE" || got.Province != "TT" {
		t.Fatalf("expected same-ID CSV metadata enrichment, got %+v", got)
	}
	if got.Latitude != testLat+0.001 || got.Longitude != testLng+0.001 {
		t.Fatalf("expected live coordinates, got %+v", got)
	}
}

func TestNearbyWithLiveDoesNotEnrichFromDifferentID(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations: map[int64]mimit.Station{
			90003: {ID: 90003, Address: "Wrong Address", City: "WRONG", Province: "XX", Latitude: testLat + 0.00101, Longitude: testLng + 0.00101},
		},
		Prices: map[int64][]mimit.Price{
			90003: {{StationID: 90003, Fuel: "Benzina", Value: 1.100, Self: true, UpdatedAt: freshTime(0)}},
		},
	})
	live := []LiveStation{{
		Station: mimit.Station{ID: 90004, Name: "Live Station", Brand: "Live Brand", Latitude: testLat + 0.001, Longitude: testLng + 0.001},
		Prices:  []mimit.Price{{StationID: 90004, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(0)}},
	}}

	results, err := s.NearbyWithLive(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 2, Fuel: "benzina", Service: "self"}, live)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one live result, got %+v", results)
	}
	got := results[0]
	if got.Station.Address != "" || got.Station.City != "" || got.Station.Province != "" {
		t.Fatalf("different-ID CSV station must not enrich live result, got %+v", got.Station)
	}
	if got.Price.Value != 1.989 {
		t.Fatalf("different-ID CSV price must not affect live result, got %.3f", got.Price.Value)
	}
}

func TestNearbyWithLiveTreatsTodayPriceAsFresh(t *testing.T) {
	s := New()
	s.Replace(mimit.Dataset{
		Stations:  map[int64]mimit.Station{90005: {ID: 90005, Latitude: testLat + 0.001, Longitude: testLng + 0.001}},
		Prices:    map[int64][]mimit.Price{},
		Extracted: freshTime(1),
	})
	live := []LiveStation{{
		Station: mimit.Station{ID: 90005, Name: "Fresh Live Station", Latitude: testLat + 0.001, Longitude: testLng + 0.001},
		Prices:  []mimit.Price{{StationID: 90005, Fuel: "Benzina", Value: 2.020, Self: true, UpdatedAt: freshTime(0)}},
	}}

	results, err := s.NearbyWithLive(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 2, Fuel: "benzina", Service: "self"}, live)
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Test", Latitude: testLat, Longitude: testLng}},
		Prices: map[int64][]mimit.Price{1: {
			{StationID: 1, Fuel: "Benzina", Value: 1.699, Self: true, UpdatedAt: updated},
			{StationID: 1, Fuel: "Benzina", Value: 1.899, Self: false, UpdatedAt: updated},
		}},
	})

	results, err := s.Nearby(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 5, Fuel: "benzina", Service: "any", Sort: "price"})
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Served only", Latitude: testLat, Longitude: testLng}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Gasolio", Value: 1.829, Self: false, UpdatedAt: freshTime(0)}}},
	})

	results, err := s.Nearby(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 5, Fuel: "gasolio", Service: "any"})
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Stale", Latitude: testLat, Longitude: testLng}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(9)}}},
	})

	results, err := s.Nearby(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 10, Fuel: "benzina", Service: "any"})
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Fresh enough", Latitude: testLat, Longitude: testLng}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Benzina", Value: 2.019, Self: true, UpdatedAt: freshTime(8)}}},
	})

	results, err := s.Nearby(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 10, Fuel: "benzina", Service: "any"})
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Mixed freshness", Latitude: testLat, Longitude: testLng}},
		Prices: map[int64][]mimit.Price{1: {
			{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(9)},
			{StationID: 1, Fuel: "Benzina", Value: 2.149, Self: false, UpdatedAt: freshTime(1)},
		}},
	})

	results, err := s.Nearby(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 10, Fuel: "benzina", Service: "any"})
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
		Stations: map[int64]mimit.Station{1: {ID: 1, Name: "Stale diagnostic", Latitude: testLat, Longitude: testLng}},
		Prices:   map[int64][]mimit.Price{1: {{StationID: 1, Fuel: "Benzina", Value: 1.989, Self: true, UpdatedAt: freshTime(9)}}},
	})

	results, err := s.Nearby(Query{Latitude: testLat, Longitude: testLng, RadiusKm: 10, Fuel: "benzina", Service: "any", IncludeStale: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected diagnostic override to include stale price, got %d results", len(results))
	}
}


func TestNearbyDoesNotTruncateResults(t *testing.T) {
	s := New()
	stations := make(map[int64]mimit.Station)
	prices := make(map[int64][]mimit.Price)
	updated := freshTime(0)

	for i := int64(1); i <= 125; i++ {
		stations[i] = mimit.Station{ID: i, Name: "Test station", Latitude: 45.0, Longitude: 9.0}
		prices[i] = []mimit.Price{{StationID: i, Fuel: "Benzina", Value: 1.8 + float64(i)/10000, Self: true, UpdatedAt: updated}}
	}
	s.Replace(mimit.Dataset{Stations: stations, Prices: prices})

	results, err := s.Nearby(Query{Latitude: 45.0, Longitude: 9.0, RadiusKm: 10, Fuel: "benzina", Service: "self"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 125 {
		t.Fatalf("expected all 125 nearby stations, got %d", len(results))
	}
}
