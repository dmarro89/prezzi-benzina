package mimit

import "time"

type Station struct {
	ID int64
	Manager string
	Brand string
	Type string
	Name string
	Address string
	City string
	Province string
	Latitude float64
	Longitude float64
}

type Price struct {
	StationID int64
	Fuel string
	Value float64
	Self bool
	UpdatedAt time.Time
}

type Dataset struct {
	Stations map[int64]Station
	Prices map[int64][]Price
	Extracted time.Time
	LoadedAt time.Time
}
