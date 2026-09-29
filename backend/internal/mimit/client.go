package mimit

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultStationsURL = "https://www.mimit.gov.it/images/exportCSV/anagrafica_impianti_attivi.csv"
	DefaultPricesURL   = "https://www.mimit.gov.it/images/exportCSV/prezzo_alle_8.csv"
)

type Client struct {
	HTTP        *http.Client
	StationsURL string
	PricesURL   string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 45 * time.Second}, StationsURL: DefaultStationsURL, PricesURL: DefaultPricesURL}
}

func (c *Client) Load(ctx context.Context) (Dataset, error) {
	stationsBody, err := c.download(ctx, c.StationsURL)
	if err != nil { return Dataset{}, fmt.Errorf("download station registry: %w", err) }
	defer stationsBody.Close()
	stations, stationExtraction, err := ParseStations(stationsBody)
	if err != nil { return Dataset{}, fmt.Errorf("parse station registry: %w", err) }

	pricesBody, err := c.download(ctx, c.PricesURL)
	if err != nil { return Dataset{}, fmt.Errorf("download prices: %w", err) }
	defer pricesBody.Close()
	prices, priceExtraction, err := ParsePrices(pricesBody)
	if err != nil { return Dataset{}, fmt.Errorf("parse prices: %w", err) }

	extracted := priceExtraction
	if extracted.IsZero() { extracted = stationExtraction }
	return Dataset{Stations: stations, Prices: prices, Extracted: extracted, LoadedAt: time.Now().UTC()}, nil
}

func (c *Client) download(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil { return nil, err }
	req.Header.Set("User-Agent", "prezzi-benzina/0.1 (+https://github.com/dmarro89/prezzi-benzina)")
	req.Header.Set("Accept", "text/csv,*/*")
	resp, err := c.HTTP.Do(req)
	if err != nil { return nil, err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return resp.Body, nil
}

func ParseStations(r io.Reader) (map[int64]Station, time.Time, error) {
	records, extracted, err := readDataset(r)
	if err != nil { return nil, time.Time{}, err }
	if len(records) == 0 { return nil, extracted, fmt.Errorf("dataset has no header") }
	header := indexHeader(records[0])
	if err := requireColumns(header, "idimpianto", "bandiera", "nomeimpianto", "indirizzo", "comune", "provincia", "latitudine", "longitudine"); err != nil { return nil, extracted, err }

	stations := make(map[int64]Station, len(records)-1)
	for _, row := range records[1:] {
		id, err := parseInt(field(row, header, "idimpianto")); if err != nil { continue }
		lat, err1 := parseFloat(field(row, header, "latitudine")); lng, err2 := parseFloat(field(row, header, "longitudine"))
		if err1 != nil || err2 != nil || lat < 35 || lat > 48 || lng < 5 || lng > 20 { continue }
		stations[id] = Station{ID: id, Manager: field(row, header, "gestore"), Brand: field(row, header, "bandiera"), Type: field(row, header, "tipoimpianto"), Name: field(row, header, "nomeimpianto"), Address: field(row, header, "indirizzo"), City: field(row, header, "comune"), Province: field(row, header, "provincia"), Latitude: lat, Longitude: lng}
	}
	if len(stations) == 0 { return nil, extracted, fmt.Errorf("dataset contains no valid stations") }
	return stations, extracted, nil
}

func ParsePrices(r io.Reader) (map[int64][]Price, time.Time, error) {
	records, extracted, err := readDataset(r)
	if err != nil { return nil, time.Time{}, err }
	if len(records) == 0 { return nil, extracted, fmt.Errorf("dataset has no header") }
	header := indexHeader(records[0])
	if err := requireColumns(header, "idimpianto", "desccarburante", "prezzo", "isself", "dtcomu"); err != nil { return nil, extracted, err }

	prices := make(map[int64][]Price)
	for _, row := range records[1:] {
		id, err := parseInt(field(row, header, "idimpianto")); if err != nil { continue }
		value, err := parseFloat(field(row, header, "prezzo")); if err != nil || value <= 0 { continue }
		self := strings.TrimSpace(field(row, header, "isself")) == "1"
		updatedAt, _ := parseMIMITDateTime(field(row, header, "dtcomu"))
		prices[id] = append(prices[id], Price{StationID: id, Fuel: strings.TrimSpace(field(row, header, "desccarburante")), Value: value, Self: self, UpdatedAt: updatedAt})
	}
	if len(prices) == 0 { return nil, extracted, fmt.Errorf("dataset contains no valid prices") }
	return prices, extracted, nil
}

func readDataset(r io.Reader) ([][]string, time.Time, error) {
	reader := csv.NewReader(r)
	reader.Comma = '|'
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	first, err := reader.Read(); if err != nil { return nil, time.Time{}, err }
	var extracted time.Time
	var rows [][]string
	if len(first) == 1 && strings.HasPrefix(strings.ToLower(strings.TrimSpace(first[0])), "estrazione") { extracted = parseExtractionDate(first[0]) } else { rows = append(rows, first) }
	for {
		row, err := reader.Read()
		if err == io.EOF { break }
		if err != nil { return nil, extracted, err }
		if len(row) > 0 { rows = append(rows, row) }
	}
	return rows, extracted, nil
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "\ufeff")))
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(s)
}
func indexHeader(row []string) map[string]int { m := make(map[string]int, len(row)); for i, name := range row { m[normalizeHeader(name)] = i }; return m }
func requireColumns(header map[string]int, names ...string) error { for _, name := range names { if _, ok := header[normalizeHeader(name)]; !ok { return fmt.Errorf("missing required column %q", name) } }; return nil }
func field(row []string, header map[string]int, name string) string { i, ok := header[normalizeHeader(name)]; if !ok || i >= len(row) { return "" }; return strings.TrimSpace(row[i]) }
func parseInt(s string) (int64, error) { return strconv.ParseInt(strings.TrimSpace(s), 10, 64) }
func parseFloat(s string) (float64, error) { return strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64) }
func parseExtractionDate(s string) time.Time { for _, part := range strings.Fields(s) { if t, err := time.Parse("2006-01-02", strings.TrimSpace(part)); err == nil { return t.UTC() } }; return time.Time{} }
func parseMIMITDateTime(s string) (time.Time, error) { for _, layout := range []string{"02/01/2006 15:04:05", "02/01/2006 15:04", "2006-01-02 15:04:05"} { if t, err := time.ParseInLocation(layout, strings.TrimSpace(s), time.Local); err == nil { return t.UTC(), nil } }; return time.Time{}, fmt.Errorf("unsupported date %q", s) }
