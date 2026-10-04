package api

import (
	"testing"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/osservaprezzi"
)

func TestLiveSearchCacheRoundTrip(t *testing.T) {
	cache := newLiveSearchCache(time.Minute)
	query := osservaprezzi.Query{Latitude: 45, Longitude: 9, RadiusKm: 5, Fuel: "benzina", Service: "self"}
	cache.put(query, []osservaprezzi.Station{{ID: 42}})

	got, ok := cache.get(query)
	if !ok || len(got) != 1 || got[0].ID != 42 {
		t.Fatalf("expected cached station, got ok=%v stations=%+v", ok, got)
	}
}

func TestLiveSearchCacheSeparatesFilters(t *testing.T) {
	cache := newLiveSearchCache(time.Minute)
	base := osservaprezzi.Query{Latitude: 45, Longitude: 9, RadiusKm: 5, Fuel: "benzina", Service: "self"}
	cache.put(base, []osservaprezzi.Station{{ID: 42}})

	other := base
	other.Service = "served"
	if _, ok := cache.get(other); ok {
		t.Fatal("cache entry must not cross service filters")
	}
}

func TestLiveSearchCacheExpires(t *testing.T) {
	cache := newLiveSearchCache(time.Nanosecond)
	query := osservaprezzi.Query{Latitude: 45, Longitude: 9, RadiusKm: 5, Fuel: "benzina", Service: "self"}
	cache.put(query, []osservaprezzi.Station{{ID: 42}})
	time.Sleep(time.Millisecond)

	if _, ok := cache.get(query); ok {
		t.Fatal("expired cache entry must not be returned")
	}
}
