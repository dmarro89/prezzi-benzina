package osservaprezzi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
)

const DefaultBaseURL = "https://carburanti.mise.gov.it/ospzApi"

type Client struct {
	HTTP    *http.Client
	BaseURL string
}

type Query struct {
	Latitude  float64
	Longitude float64
	RadiusKm  float64
	Fuel      string
	Service   string
}

type zoneRequest struct {
	Points     []point `json:"points"`
	FuelType   string  `json:"fuelType,omitempty"`
	PriceOrder string  `json:"priceOrder"`
	Radius     float64 `json:"radius"`
}

type point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type zoneResponse struct {
	Success bool          `json:"success"`
	Results []liveStation `json:"results"`
}

type liveStation struct {
	ID         json.RawMessage `json:"id"`
	Name       string          `json:"name"`
	InsertDate string          `json:"insertDate"`
	Fuels      []liveFuel      `json:"fuels"`
}

type liveFuel struct {
	Price  float64 `json:"price"`
	Name   string  `json:"name"`
	FuelID int     `json:"fuelId"`
	IsSelf bool    `json:"isSelf"`
}

func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 5 * time.Second},
		BaseURL: DefaultBaseURL,
	}
}

func (c *Client) SearchZone(ctx context.Context, q Query) (map[int64][]mimit.Price, error) {
	fuelType, err := fuelType(q.Fuel, q.Service)
	if err != nil {
		return nil, err
	}
	radius := q.RadiusKm
	if radius <= 0 {
		radius = 5
	}
	if radius > 10 {
		radius = 10
	}
	body, err := json.Marshal(zoneRequest{
		Points:     []point{{Lat: q.Latitude, Lng: q.Longitude}},
		FuelType:   fuelType,
		PriceOrder: "asc",
		Radius:     radius,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/search/zone", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "prezzi-benzina/0.1 (+https://github.com/dmarro89/prezzi-benzina)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	var decoded zoneResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !decoded.Success {
		return nil, fmt.Errorf("Osservaprezzi search failed")
	}

	prices := make(map[int64][]mimit.Price)
	for _, station := range decoded.Results {
		stationID := parseStationID(station)
		if stationID == 0 {
			continue
		}
		updatedAt, _ := parseLiveDateTime(station.InsertDate)
		for _, fuel := range station.Fuels {
			if fuel.Price <= 0 || canonicalFuel(fuel.Name) == "" {
				continue
			}
			prices[stationID] = append(prices[stationID], mimit.Price{
				StationID: stationID,
				Fuel:      fuel.Name,
				Value:     fuel.Price,
				Self:      fuel.IsSelf,
				UpdatedAt: updatedAt,
			})
		}
	}
	return prices, nil
}

func fuelType(fuel, service string) (string, error) {
	id := ""
	switch canonicalFuel(fuel) {
	case "benzina":
		id = "1"
	case "gasolio":
		id = "2"
	case "metano":
		id = "3"
	case "gpl":
		id = "4"
	default:
		return "", fmt.Errorf("unsupported fuel %q", fuel)
	}
	mode := "x"
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "", "any":
	case "self":
		mode = "1"
	case "served":
		mode = "0"
	default:
		return "", fmt.Errorf("unsupported service %q", service)
	}
	return id + "-" + mode, nil
}

func parseStationID(station liveStation) int64 {
	if id := parseRawInt(station.ID); id > 0 {
		return id
	}
	id, _ := strconv.ParseInt(strings.TrimSpace(station.Name), 10, 64)
	return id
}

func parseRawInt(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n, _ = strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		return n
	}
	return 0
}

func parseLiveDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	rome, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		return time.Time{}, err
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "02/01/2006 15:04:05"} {
		if t, err := time.ParseInLocation(layout, value, rome); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date %q", value)
}

func canonicalFuel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
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
