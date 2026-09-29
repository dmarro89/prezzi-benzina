package osservaprezzi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchZoneParsesLivePrices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/zone" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var request zoneRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.FuelType != "1-1" || request.Radius != 10 {
			t.Fatalf("unexpected request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"results":[{"id":123,"name":"Test","insertDate":"2026-09-29T12:15:00+02:00","fuels":[{"price":1.659,"name":"Benzina","fuelId":1,"isSelf":true}]}]}`))
	}))
	defer server.Close()

	client := NewClient()
	client.BaseURL = server.URL
	prices, err := client.SearchZone(context.Background(), Query{Latitude: 40.85, Longitude: 14.27, RadiusKm: 20, Fuel: "benzina", Service: "self"})
	if err != nil {
		t.Fatal(err)
	}
	p := prices[123][0]
	if p.Value != 1.659 || !p.Self || p.Fuel != "Benzina" {
		t.Fatalf("unexpected price: %+v", p)
	}
	if p.UpdatedAt.Format(time.RFC3339) != "2026-09-29T10:15:00Z" {
		t.Fatalf("unexpected update time: %s", p.UpdatedAt.Format(time.RFC3339))
	}
}

func TestSearchZoneAcceptsStationIDInName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"results":[{"id":"Station label","name":"8976","insertDate":"2026-09-29T12:15:00+02:00","fuels":[{"price":1.7,"name":"Gasolio","fuelId":2,"isSelf":false}]}]}`))
	}))
	defer server.Close()
	client := NewClient()
	client.BaseURL = server.URL
	prices, err := client.SearchZone(context.Background(), Query{Fuel: "gasolio", Service: "served"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prices[8976]) != 1 {
		t.Fatalf("expected fallback station ID, got %+v", prices)
	}
}
