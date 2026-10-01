package mimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseStations2026Format(t *testing.T) {
	input := "Estrazione del 2026-09-28\n" +
		"idImpianto|Gestore|Bandiera|Tipo Impianto|Nome Impianto|Indirizzo|Comune|Provincia|Latitudine|Longitudine\n" +
		"90001|Gestore Test Srl|Brand Test|Stradale|Impianto Test|Via Test 1|Testville|TT|45.001|9.001\n"
	stations, extracted, err := ParseStations(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if extracted.Format("2006-01-02") != "2026-09-28" {
		t.Fatalf("unexpected extraction date: %v", extracted)
	}
	s := stations[90001]
	if s.Brand != "Brand Test" || s.City != "Testville" || s.Address != "Via Test 1" {
		t.Fatalf("unexpected station: %+v", s)
	}
}

func TestParsePrices2026Format(t *testing.T) {
	input := "Estrazione del 2026-09-28\n" +
		"idImpianto|descCarburante|prezzo|isSelf|dtComu\n" +
		"90001|Benzina|1.679|1|28/09/2026 07:42:00\n"
	prices, _, err := ParsePrices(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	p := prices[90001][0]
	if p.Value != 1.679 || !p.Self || p.Fuel != "Benzina" {
		t.Fatalf("unexpected price: %+v", p)
	}
	want := time.Date(2026, time.September, 28, 5, 42, 0, 0, time.UTC)
	if !p.UpdatedAt.Equal(want) {
		t.Fatalf("unexpected update timestamp: got %v want %v", p.UpdatedAt, want)
	}
}

func TestDownloadBypassesHTTPDataCaches(t *testing.T) {
	var gotCacheControl, gotPragma, gotCacheBust string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCacheControl = r.Header.Get("Cache-Control")
		gotPragma = r.Header.Get("Pragma")
		gotCacheBust = r.URL.Query().Get("_cb")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := &Client{HTTP: server.Client()}
	body, err := client.download(context.Background(), server.URL+"?existing=value")
	if err != nil {
		t.Fatal(err)
	}
	body.Close()

	if gotCacheBust == "" {
		t.Fatal("expected cache-busting query parameter")
	}
	if gotCacheControl != "no-cache, no-store, max-age=0" {
		t.Fatalf("unexpected Cache-Control: %q", gotCacheControl)
	}
	if gotPragma != "no-cache" {
		t.Fatalf("unexpected Pragma: %q", gotPragma)
	}
}
