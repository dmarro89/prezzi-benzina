package api

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/osservaprezzi"
)

type liveCacheEntry struct {
	stations []osservaprezzi.Station
	storedAt time.Time
}

type liveSearchCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]liveCacheEntry
}

func newLiveSearchCache(ttl time.Duration) *liveSearchCache {
	return &liveSearchCache{ttl: ttl, entries: make(map[string]liveCacheEntry)}
}

func (c *liveSearchCache) put(q osservaprezzi.Query, stations []osservaprezzi.Station) {
	if c == nil || c.ttl <= 0 {
		return
	}
	copyStations := append([]osservaprezzi.Station(nil), stations...)
	c.mu.Lock()
	c.entries[liveCacheKey(q)] = liveCacheEntry{stations: copyStations, storedAt: time.Now()}
	c.mu.Unlock()
}

func (c *liveSearchCache) get(q osservaprezzi.Query) ([]osservaprezzi.Station, bool) {
	if c == nil || c.ttl <= 0 {
		return nil, false
	}
	key := liveCacheKey(q)
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Since(entry.storedAt) > c.ttl {
		return nil, false
	}
	return append([]osservaprezzi.Station(nil), entry.stations...), true
}

func liveCacheKey(q osservaprezzi.Query) string {
	return fmt.Sprintf("%.5f|%.5f|%.1f|%s|%s",
		q.Latitude,
		q.Longitude,
		q.RadiusKm,
		strings.ToLower(strings.TrimSpace(q.Fuel)),
		strings.ToLower(strings.TrimSpace(q.Service)),
	)
}
