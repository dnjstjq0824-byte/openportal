package vertical

import (
	"math"
	"testing"
)

func TestValidateCoordinates(t *testing.T) {
	// Valid Seoul City Hall
	if err := ValidateCoordinates(37.566535, 126.977969); err != nil {
		t.Errorf("expected valid coordinates, got %v", err)
	}

	// Invalid latitude
	if err := ValidateCoordinates(95.0, 120.0); err == nil {
		t.Errorf("expected error for latitude > 90, got nil")
	}

	// Invalid longitude
	if err := ValidateCoordinates(37.0, -185.0); err == nil {
		t.Errorf("expected error for longitude < -180, got nil")
	}

	// NaN check
	if err := ValidateCoordinates(math.NaN(), 120.0); err == nil {
		t.Errorf("expected error for NaN latitude, got nil")
	}
}

func TestValidateMapLicense(t *testing.T) {
	valid, attr := ValidateMapLicense("ODbL-1.0")
	if !valid || attr == "" {
		t.Errorf("expected valid ODbL license, got valid=%v", valid)
	}

	invalid, _ := ValidateMapLicense("GoogleMapsScraped")
	if invalid {
		t.Errorf("expected invalid for unapproved scraped license")
	}
}

func TestHaversineDistanceKm(t *testing.T) {
	// Seoul City Hall (37.5665, 126.9780) to Gangnam Station (37.4979, 127.0276)
	// Approximate distance is ~8.7 km
	p1 := GeoPoint{Latitude: 37.5665, Longitude: 126.9780}
	p2 := GeoPoint{Latitude: 37.4979, Longitude: 127.0276}

	dist, err := HaversineDistanceKm(p1, p2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist < 8.0 || dist > 9.5 {
		t.Errorf("expected distance between 8.0 and 9.5 km, got %f km", dist)
	}

	// Same point distance must be 0
	distSame, _ := HaversineDistanceKm(p1, p1)
	if distSame != 0.0 {
		t.Errorf("expected distance 0.0 for identical point, got %f", distSame)
	}
}

func TestInBoundingBox(t *testing.T) {
	seoulBox := GeoBoundingBox{
		MinLat: 37.4,
		MaxLat: 37.7,
		MinLon: 126.8,
		MaxLon: 127.2,
	}

	seoulCityHall := GeoPoint{Latitude: 37.5665, Longitude: 126.9780}
	inBox, err := seoulBox.InBoundingBox(seoulCityHall)
	if err != nil || !inBox {
		t.Errorf("expected Seoul City Hall inside bounding box, got %v (err: %v)", inBox, err)
	}

	busanPoint := GeoPoint{Latitude: 35.1796, Longitude: 129.0756}
	inBoxBusan, _ := seoulBox.InBoundingBox(busanPoint)
	if inBoxBusan {
		t.Errorf("expected Busan to be outside Seoul bounding box")
	}
}
