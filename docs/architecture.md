# Architecture

The first milestone deliberately implements the smallest production-shaped vertical slice.

```text
MIMIT Open Data CSV
        |
        v
 Go data adapter ---- refresh every 6h
        |
        v
 immutable in-memory snapshot
        |
        v
 REST API /v1/stations/nearby
        |
        v
 Expo / React Native app
```

## Why no database in v0.1?

The daily MIMIT snapshot is small enough to hold in memory and the first product question is whether the nearby-price experience is useful. Starting without PostgreSQL removes infrastructure from the first test while keeping the MIMIT adapter and API contract independent from persistence.

PostgreSQL/PostGIS becomes useful in the next milestone for price history, spatial indexes, favourites and analytics. Moving there does not require a mobile API change.

## Data source

Official MIMIT Open Data:

- `https://www.mimit.gov.it/images/exportCSV/anagrafica_impianti_attivi.csv`
- `https://www.mimit.gov.it/images/exportCSV/prezzo_alle_8.csv`

Since 10 February 2026 the files use `|` as their field separator. The first line contains the extraction date and the second line is the CSV header.

## API

`GET /v1/stations/nearby`

Parameters:

- `lat`, `lng`: required position
- `radiusKm`: defaults to 5, max 50
- `fuel`: `benzina`, `gasolio`, `gpl`, `metano`
- `service`: `self`, `served`, `any`
- `sort`: `price` or `distance`
- `limit`: defaults to 30, max 100

Distance in v0.1 is straight-line distance, not driving distance. The UI intentionally labels it only as kilometres and does not invent a travel time.
