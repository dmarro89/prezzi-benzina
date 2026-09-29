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

const DefaultFreshnessDays = 8

var italyLocation = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		return time.UTC
	}
	return loc
}()

type Store struct {
	mu   sync.RWMutex
	data mimit.Dataset
}

type Query struct {
	Latitude     float64
	Longitude    float64
	RadiusKm     float64
	Fuel         string
	Service      string
	Sort         string
	Limit        int
	IncludeStale bool
}

type Result struct {
	Station    mimit.Station
	Price      mimit.Price
	Prices     []mimit.Price
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
	if q.RadiusKm <= 0 {
		q.RadiusKm = 5
	}
	if q.RadiusKm > 50 {
		q.RadiusKm = 50
	}
	if q.Limit <= 0 {
		q.Limit = 30
	}
	if q.Limit > 100 {
		q.Limit = 100
	}

	reference := s.data.Extracted
	if reference.IsZero() {
		reference = s.data.LoadedAt
	}
	if reference.IsZero() {
		reference = time.Now().UTC()
	}

	results := make([]Result, 0, q.Limit)
	for id, station := range s.data.Stations {
		distance := geo.DistanceKm(q.Latitude, q.Longitude, station.Latitude, station.Longitude)
		if distance > q.RadiusKm {
			continue
		}
		merged := mergePrices(s.data.Prices[id], overlay[id])
		prices := matchingPrices(merged, fuel, q.Service)
		if !q.IncludeStale {
			prices = freshPrices(prices, reference)
		}
		if len(prices) == 0 {
			continue
		}
		primary := prices[0]
		for _, p := range prices[1:] {
			if p.Value < primary.Value {
				primary = p
			}
		}
		results = append(results, Result{Station: station, Price: primary, Prices: prices, DistanceKm: distance})
	}

	sort.Slice(results, func(i, j int) bool {
		if q.Sort == "distance" {
			if results[i].DistanceKm == results[j].DistanceKm {
				return results[i].Price.Value < results[j].Price.Value
			}
			return results[i].DistanceKm < results[j].DistanceKm
		}
		if results[i].Price.Value == results[j].Price.Value {
			return results[i].DistanceKm < results[j].DistanceKm
		}
		return results[i].Price.Value < results[j].Price.Value
	})
	if len(results) > q.Limit {
		results = results[:q.Limit]
	}
	return results, nil
}

func mergePrices(base, live []mimit.Price) []mimit.Price {
	merged := make(map[string]mimit.Price, len(base)+len(live))
	order := make([]string, 0, len(base)+len(live))
	add := func(p mimit.Price) {
		fuel := canonicalFuel(p.Fuel)
		if fuel == "" {
			return
		}
		key := fuel + ":served"
		if p.Self {
			key = fuel + ":self"
		}
		current, exists := merged[key]
		if !exists {
			merged[key] = p
			order = append(order, key)
			return
		}
		if isNewer(p, current) {
			merged[key] = p
		}
	}
	for _, p := range base {
		add(p)
	}
	for _, p := range live {
		add(p)
	}
	out := make([]mimit.Price, 0, len(order))
	for _, key := range order {
		out = append(out, merged[key])
	}
	return out
}

func isNewer(candidate, current mimit.Price) bool {
	if candidate.UpdatedAt.IsZero() {
		return false
	}
	if current.UpdatedAt.IsZero() {
		return true
	}
	return candidate.UpdatedAt.After(current.UpdatedAt)
}

func freshPrices(prices []mimit.Price, reference time.Time) []mimit.Price {
	fresh := make([]mimit.Price, 0, len(prices))
	for _, p := range prices {
		if isPriceFresh(p, reference) {
			fresh = append(fresh, p)
		}
	}
	return fresh
}

func isPriceFresh(p mimit.Price, reference time.Time) bool {
	if p.UpdatedAt.IsZero() || reference.IsZero() {
		return false
	}
	updated := p.UpdatedAt.In(italyLocation)
	ref := reference.In(italyLocation)
	updatedDay := time.Date(updated.Year(), updated.Month(), updated.Day(), 0, 0, 0, 0, time.UTC)
	refDay := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, time.UTC)
	days := int(refDay.Sub(updatedDay).Hours() / 24)
	return days >= 0 && days <= DefaultFreshnessDays
}

func matchingPrices(prices []mimit.Price, fuel, service string) []mimit.Price {
	matches := make([]mimit.Price, 0, 2)
	for _, p := range prices {
		if canonicalFuel(p.Fuel) != fuel {
			continue
		}
		if service == "self" && !p.Self {
			continue
		}
		if service == "served" && p.Self {
			continue
		}
		matches = append(matches, p)
	}
	// Keep the payload stable and easy to scan in the UI: Self first, Servito second.
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Self != matches[j].Self {
			return matches[i].Self
		}
		return matches[i].Value < matches[j].Value
	})
	return matches
}

func bestPrice(prices []mimit.Price, fuel, service string) (mimit.Price, bool) {
	matches := matchingPrices(prices, fuel, service)
	if len(matches) == 0 {
		return mimit.Price{}, false
	}
	best := matches[0]
	for _, p := range matches[1:] {
		if p.Value < best.Value {
			best = p
		}
	}
	return best, true
}

func canonicalFuel(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "benzina", "petrol", "gasoline":
		return "benzina"
	case "gasolio", "diesel":
		return "gasolio"
	case "gpl", "lpg":
		return "gpl"
	case "metano", "cng":
		return "metano"
	default:
		return ""
	}
}
