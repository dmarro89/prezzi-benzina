package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
	"github.com/dmarro89/prezzi-benzina/backend/internal/osservaprezzi"
	"github.com/dmarro89/prezzi-benzina/backend/internal/store"
)

type Handler struct {
	Store     *store.Store
	Live      *osservaprezzi.Client
	LiveCache *liveSearchCache
}

type nearbyResponse struct {
	DataSource    string            `json:"dataSource"`
	LiveData      bool              `json:"liveData"`
	LiveCache     bool              `json:"liveCache,omitempty"`
	FreshnessDays int               `json:"freshnessDays"`
	IncludeStale  bool              `json:"includeStale"`
	Count         int               `json:"count"`
	Stations      []stationResponse `json:"stations"`
}
type stationResponse struct {
	ID       int64    `json:"id"`
	Brand    string   `json:"brand"`
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	City     string   `json:"city"`
	Province string   `json:"province"`
	Location location `json:"location"`
	Distance float64  `json:"distanceKm"`
	Price    price    `json:"price"`
	Prices   []price  `json:"prices"`
}
type location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}
type price struct {
	Fuel      string     `json:"fuel"`
	Value     float64    `json:"value"`
	Unit      string     `json:"unit"`
	Service   string     `json:"service"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

func (h Handler) Routes() http.Handler {
	if h.LiveCache == nil {
		h.LiveCache = newLiveSearchCache(10 * time.Minute)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("GET /v1/stations/nearby", h.nearby)
	return requestLog(cors(mux))
}

func (h Handler) health(w http.ResponseWriter, _ *http.Request) {
	stations, prices, extracted, loaded := h.Store.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"dataReady":          stations > 0,
		"stations":           stations,
		"prices":             prices,
		"datasetExtractedAt": timeOrNil(extracted),
		"loadedAt":           timeOrNil(loaded),
		"freshnessDays":      store.DefaultFreshnessDays,
	})
}

func (h Handler) ready(w http.ResponseWriter, _ *http.Request) {
	stations, prices, extracted, loaded := h.Store.Stats()
	status := http.StatusOK
	state := "ready"
	if stations == 0 {
		status = http.StatusServiceUnavailable
		state = "loading"
	}
	writeJSON(w, status, map[string]any{
		"status":             state,
		"stations":           stations,
		"prices":             prices,
		"datasetExtractedAt": timeOrNil(extracted),
		"loadedAt":           timeOrNil(loaded),
	})
}

func (h Handler) nearby(w http.ResponseWriter, r *http.Request) {
	lat, err := queryFloat(r, "lat")
	if err != nil || lat < -90 || lat > 90 {
		writeError(w, http.StatusBadRequest, "invalid lat")
		return
	}
	lng, err := queryFloat(r, "lng")
	if err != nil || lng < -180 || lng > 180 {
		writeError(w, http.StatusBadRequest, "invalid lng")
		return
	}
	radius := queryFloatDefault(r, "radiusKm", 5)
	if radius <= 0 || radius > 50 {
		writeError(w, http.StatusBadRequest, "radiusKm must be between 0 and 50")
		return
	}
	service := strings.ToLower(r.URL.Query().Get("service"))
	if service == "" {
		service = "self"
	}
	if service != "self" && service != "served" && service != "any" {
		writeError(w, http.StatusBadRequest, "service must be self, served or any")
		return
	}
	sortBy := strings.ToLower(r.URL.Query().Get("sort"))
	if sortBy == "" {
		sortBy = "price"
	}
	if sortBy != "price" && sortBy != "distance" {
		writeError(w, http.StatusBadRequest, "sort must be price or distance")
		return
	}
	includeStale, err := queryBoolDefault(r, "includeStale", false)
	if err != nil {
		writeError(w, http.StatusBadRequest, "includeStale must be true or false")
		return
	}

	query := store.Query{Latitude: lat, Longitude: lng, RadiusKm: radius, Fuel: r.URL.Query().Get("fuel"), Service: service, Sort: sortBy, IncludeStale: includeStale}
	liveQuery := osservaprezzi.Query{Latitude: lat, Longitude: lng, RadiusKm: radius, Fuel: query.Fuel, Service: service}
	var liveStations []store.LiveStation
	useLiveCandidates := false
	liveData := false
	liveCache := false
	if h.Live != nil {
		liveCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		liveResults, liveErr := h.Live.SearchZone(liveCtx, liveQuery)
		cancel()
		if liveErr == nil {
			liveData = true
			useLiveCandidates = true
			h.LiveCache.put(liveQuery, liveResults)
			liveStations = toStoreLiveStations(liveResults)
		} else {
			log.Printf("Osservaprezzi request failed path=%s err=%q", r.URL.Path, liveErr)
			if cached, ok := h.LiveCache.get(liveQuery); ok {
				liveCache = true
				useLiveCandidates = true
				liveStations = toStoreLiveStations(cached)
			}
		}
	}

	var results []store.Result
	if useLiveCandidates {
		results, err = h.Store.NearbyWithLive(query, liveStations)
	} else {
		results, err = h.Store.Nearby(query)
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	out := make([]stationResponse, 0, len(results))
	for _, item := range results {
		name := strings.TrimSpace(item.Station.Name)
		if name == "" {
			name = strings.TrimSpace(item.Station.Brand)
		}
		prices := make([]price, 0, len(item.Prices))
		for _, itemPrice := range item.Prices {
			prices = append(prices, priceResponse(itemPrice))
		}
		out = append(out, stationResponse{
			ID: item.Station.ID, Brand: item.Station.Brand, Name: name, Address: item.Station.Address, City: item.Station.City, Province: item.Station.Province,
			Location: location{Lat: item.Station.Latitude, Lng: item.Station.Longitude}, Distance: round(item.DistanceKm, 2), Price: priceResponse(item.Price), Prices: prices,
		})
	}
	dataSource := "MIMIT Open Data"
	if liveData {
		dataSource = "Osservaprezzi live + MIMIT Open Data enrichment"
	} else if liveCache {
		dataSource = "Osservaprezzi recent cache + MIMIT Open Data enrichment"
	}
	writeJSON(w, http.StatusOK, nearbyResponse{DataSource: dataSource, LiveData: liveData, LiveCache: liveCache, FreshnessDays: store.DefaultFreshnessDays, IncludeStale: includeStale, Count: len(out), Stations: out})
}

func toStoreLiveStations(items []osservaprezzi.Station) []store.LiveStation {
	out := make([]store.LiveStation, 0, len(items))
	for _, item := range items {
		out = append(out, store.LiveStation{
			Station: mimit.Station{
				ID:        item.ID,
				Name:      item.Name,
				Brand:     item.Brand,
				Latitude:  item.Latitude,
				Longitude: item.Longitude,
			},
			Prices: item.Prices,
		})
	}
	return out
}

func priceResponse(item mimit.Price) price {
	unit := "EUR/L"
	if strings.EqualFold(item.Fuel, "Metano") {
		unit = "EUR/kg"
	}
	serviceName := "served"
	if item.Self {
		serviceName = "self"
	}
	var updated *time.Time
	if !item.UpdatedAt.IsZero() {
		t := item.UpdatedAt
		updated = &t
	}
	return price{Fuel: item.Fuel, Value: item.Value, Unit: unit, Service: serviceName, UpdatedAt: updated}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("http method=%s path=%s status=%d duration=%s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func queryFloat(r *http.Request, name string) (float64, error) {
	return strconv.ParseFloat(r.URL.Query().Get(name), 64)
}
func queryFloatDefault(r *http.Request, name string, fallback float64) float64 {
	v := r.URL.Query().Get(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}
func queryBoolDefault(r *http.Request, name string, fallback bool) (bool, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return fallback, nil
	}
	return strconv.ParseBool(v)
}
func round(v float64, decimals int) float64 {
	p := 1.0
	for i := 0; i < decimals; i++ {
		p *= 10
	}
	return float64(int(v*p+0.5)) / p
}
func timeOrNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Printf("encode response: %v\n", err)
	}
}
