package mimit

import (
	"strings"
	"testing"
)

func TestParseStations2026Format(t *testing.T) {
	input := "Estrazione del 2026-09-28\n" +
		"idImpianto|Gestore|Bandiera|Tipo Impianto|Nome Impianto|Indirizzo|Comune|Provincia|Latitudine|Longitudine\n" +
		"123|Gestore Srl|Q8|Stradale|Q8 Fuorigrotta|Via Test 1|Napoli|NA|40.8271|14.1931\n"
	stations, extracted, err := ParseStations(strings.NewReader(input))
	if err != nil { t.Fatal(err) }
	if extracted.Format("2006-01-02") != "2026-09-28" { t.Fatalf("unexpected extraction date: %v", extracted) }
	s := stations[123]
	if s.Brand != "Q8" || s.City != "Napoli" { t.Fatalf("unexpected station: %+v", s) }
}

func TestParsePrices2026Format(t *testing.T) {
	input := "Estrazione del 2026-09-28\n" +
		"idImpianto|descCarburante|prezzo|isSelf|dtComu\n" +
		"123|Benzina|1.679|1|28/09/2026 07:42:00\n"
	prices, _, err := ParsePrices(strings.NewReader(input))
	if err != nil { t.Fatal(err) }
	p := prices[123][0]
	if p.Value != 1.679 || !p.Self || p.Fuel != "Benzina" { t.Fatalf("unexpected price: %+v", p) }
}
