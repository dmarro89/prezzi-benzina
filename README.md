# Prezzi Benzina

A minimal mobile app for finding the cheapest fuel stations nearby in Italy using official MIMIT open data.

## v0.1 vertical slice

- official MIMIT station registry + daily prices
- Go backend with resilient `|`-separated CSV parsing
- nearby search for Benzina, Diesel, GPL and Metano
- self/served filtering
- sort by price or distance
- Expo / React Native mobile UI
- foreground geolocation
- tap a station to open navigation

## Run the backend

Requirements: Go 1.23+

```bash
cd backend
go test ./...
go run ./cmd/api
```

The first startup downloads the current MIMIT datasets. Check readiness with:

```bash
curl http://localhost:8080/healthz
```

Try a query (sample coordinates for central Naples):

```bash
curl 'http://localhost:8080/v1/stations/nearby?lat=40.8518&lng=14.2681&radiusKm=10&fuel=benzina&service=self&sort=price'
```

## Run the mobile app

Requirements: Node.js 22.13+ and an Expo account / Expo Go compatible with SDK 57.

```bash
cd mobile
npm install
npx expo install --fix
npm start
```

The iOS simulator can use the default backend URL `http://localhost:8080`.

For Expo Go on a physical phone, point the app at the Mac's LAN address before starting Expo:

```bash
EXPO_PUBLIC_API_URL=http://192.168.1.50:8080 npm start
```

Replace `192.168.1.50` with the Mac's local IP and make sure the phone and Mac are on the same Wi-Fi.

## Data accuracy

The app shows the timestamp communicated by the station manager when available. MIMIT publishes the downloadable open-data snapshot daily; it is not presented as second-by-second telemetry.

Source: MIMIT, dataset “Carburanti - Prezzi praticati e anagrafica degli impianti”, IODL 2.0.

See [docs/architecture.md](docs/architecture.md) for design decisions and the path to PostgreSQL/PostGIS.
