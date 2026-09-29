package store

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/geo"
	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
)

type Store struct {
	mu sync.RWMutex
	data mimit.Dataset
}

type Query struct {
	Latitude float64
	Longitude float64
	RadiusKm float64
	Fuel string
	Service string
	Sort string
	Limit int
}

type Result struct {
	Station mimit.Station
	Price mimit.Price
	DistanceKm float64
}

func New() *Store { return &Store{} }

func (s *Store) Replace(data mimit.Dataset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
}

func (s *Store) Stats() (stations int, prices int, extracted time.Time, loaded time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stations = len(s.data.Stations)
	for _, p := range s.data.Prices {
		prices += len(p)
	}
	return stations, prices, s.data.Extracted, s.data.LoadedAt
}

func (s *Store) Nearby(q Query) ([]Result, error) {
	return s.NearbyWithOverlay(q, nil)
}

func (s *Store) NearbyWithOverlay(q Query, overlay map[int64][]mimit.Price) ([]Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data.Stations) == 0 {
		return nil, errors.New("dataset not loaded")
	}
	fuel := canonicalFuel(q.Fuel)
	if fuel == "" {
		return nil, errors.New("unsupported fuel")
	}
	if q.RadiusKm <= 0 { q.RadiusKm = 5 }
	if q.RadiusKm > 50 { q.RadiusKm = 50 }
	if q.Limit <= 0 { q.Limit = 30 }
	if q.Limit > 100 { q.Limit = 100 }

	results := make([]Result, 0, q.Limit)
	for id, station := range s.data.Stations {
		distance := geo.DistanceKm(q.Latitude, q.Longitude, station.Latitude, station.Longitude)
		if distance > q.RadiusKm { continue }
		prices := mergePrices(s.data.Prices[id], overlay[id])
		price, ok := bestPrice(prices, fuel, q.Service)
		if !ok { continue }
		results = append(results, Result{Station: station, Price: price, DistanceKm: distance})
	}

	sort.Slice(results, func(i, j int) bool {
		if q.Sort == "distance" {
			if results[i].DistanceKm == results[j].DistanceKm { return results[i].Price.Value < results[j].Price.Value }
			return results[i].DistanceKm < results[j].DistanceKm
		}
		if results[i].Price.Value == results[j].Price.Value { return results[i].DistanceKm < results[j].DistanceKm }
		return results[i].Price.Value < results[j].Price.Value
	})
	if len(results) > q.Limit { results = results[:q.Limit] }
	return results, nil
}

func mergePrices(base, live []mimit.Price) []mimit.Price {
	merged := make(map[string]mimit.Price, len(base)+len(live))
	order := make([]string, 0, len(base)+len(live))
	add := func(p mimit.Price) {
		fuel := canonicalFuel(p.Fuel)
		if fuel == "" { return }
		key := fuel + ":served"
		if p.Self { key = fuel + ":self" }
		current, exists := merged[key]
		if !exists {
			merged[key] = p
			order = append(order, key)
			return
		}
		if isNewer(p, current) { merged[key] = p }
	}
	for _, p := range base { add(p) }
	for _, p := range live { add(p) }
	out := make([]mimit.Price, 0, len(order))
	for _, key := range order { out = append(out, merged[key]) }
	return out
}

func isNewer(candidate, current mimit.Price) bool {
	if candidate.UpdatedAt.IsZero() { return false }
	if current.UpdatedAt.IsZero() { return true }
	return candidate.UpdatedAt.After(current.UpdatedAt)
}

func bestPrice(prices []mimit.Price, fuel, service string) (mimit.Price, bool) {
	var best mimit.Price
	found := false
	for _, p := range prices {
		if canonicalFuel(p.Fuel) != fuel { continue }
		if service == "self" && !p.Self { continue }
		if service == "served" && p.Self { continue }
		if !found || p.Value < best.Value { best, found = p, true }
	}
	return best, found
}

func canonicalFuel(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "benzina", "petrol", "gasoline": return "benzina"
	case "gasolio", "diesel": return "gasolio"
	case "gpl", "lpg": return "gpl"
	case "metano", "cng": return "metano"
	default: return ""
	}
}
