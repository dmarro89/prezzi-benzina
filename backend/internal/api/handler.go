package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
	"github.com/dmarro89/prezzi-benzina/backend/internal/osservaprezzi"
	"github.com/dmarro89/prezzi-benzina/backend/internal/store"
)

type Handler struct {
	Store *store.Store
	Live  *osservaprezzi.Client
}

type nearbyResponse struct {
	DataSource string            `json:"dataSource"`
	LiveData   bool              `json:"liveData"`
	Count      int               `json:"count"`
	Stations   []stationResponse `json:"stations"`
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /v1/stations/nearby", h.nearby)
	return cors(mux)
}

func (h Handler) health(w http.ResponseWriter, _ *http.Request) {
	stations, prices, extracted, loaded := h.Store.Stats()
	status := "ok"
	code := http.StatusOK
	if stations == 0 {
		status, code = "loading", http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"status": status, "stations": stations, "prices": prices, "datasetExtractedAt": timeOrNil(extracted), "loadedAt": timeOrNil(loaded)})
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

	query := store.Query{Latitude: lat, Longitude: lng, RadiusKm: radius, Fuel: r.URL.Query().Get("fuel"), Service: service, Sort: sortBy, Limit: queryIntDefault(r, "limit", 30)}
	var overlay map[int64][]mimit.Price
	liveData := false
	if h.Live != nil {
		liveCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		overlay, err = h.Live.SearchZone(liveCtx, osservaprezzi.Query{Latitude: lat, Longitude: lng, RadiusKm: radius, Fuel: query.Fuel, Service: service})
		cancel()
		if err == nil {
			liveData = true
		} else {
			overlay = nil
		}
	}

	results, err := h.Store.NearbyWithOverlay(query, overlay)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	out := make([]stationResponse, 0, len(results))
	for _, item := range results {
		unit := "EUR/L"
		if strings.EqualFold(item.Price.Fuel, "Metano") {
			unit = "EUR/kg"
		}
		serviceName := "served"
		if item.Price.Self {
			serviceName = "self"
		}
		var updated *time.Time
		if !item.Price.UpdatedAt.IsZero() {
			t := item.Price.UpdatedAt
			updated = &t
		}
		name := strings.TrimSpace(item.Station.Name)
		if name == "" {
			name = strings.TrimSpace(item.Station.Brand)
		}
		out = append(out, stationResponse{ID: item.Station.ID, Brand: item.Station.Brand, Name: name, Address: item.Station.Address, City: item.Station.City, Province: item.Station.Province, Location: location{Lat: item.Station.Latitude, Lng: item.Station.Longitude}, Distance: round(item.DistanceKm, 2), Price: price{Fuel: item.Price.Fuel, Value: item.Price.Value, Unit: unit, Service: serviceName, UpdatedAt: updated}})
	}
	dataSource := "MIMIT Open Data"
	if liveData {
		dataSource = "MIMIT Open Data + Osservaprezzi live"
	}
	writeJSON(w, http.StatusOK, nearbyResponse{DataSource: dataSource, LiveData: liveData, Count: len(out), Stations: out})
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
func queryIntDefault(r *http.Request, name string, fallback int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
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
