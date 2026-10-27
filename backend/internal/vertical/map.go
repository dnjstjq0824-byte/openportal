package vertical

import (
	"errors"
	"fmt"
	"math"
)

// GeoPoint represents a WGS84 geographic coordinate.
type GeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// GeoBoundingBox defines an axis-aligned rectangular geographic boundary.
type GeoBoundingBox struct {
	MinLat float64 `json:"min_lat"`
	MaxLat float64 `json:"max_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLon float64 `json:"max_lon"`
}

// MapDatasetMeta contains verified licensing metadata for geographic data.
type MapDatasetMeta struct {
	Provider       string `json:"provider"`
	License        string `json:"license"`
	Attribution    string `json:"attribution"`
	TermsOfUseURL  string `json:"terms_of_use_url"`
}

// ValidateCoordinates ensures latitude is within [-90, 90] and longitude is within [-180, 180].
func ValidateCoordinates(lat, lon float64) error {
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90.0 || lat > 90.0 {
		return fmt.Errorf("latitude %f is out of bounds [-90, 90]", lat)
	}
	if math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180.0 || lon > 180.0 {
		return fmt.Errorf("longitude %f is out of bounds [-180, 180]", lon)
	}
	return nil
}

// ValidateMapLicense verifies if a map dataset follows OpenStreetMap ODbL or approved partner licenses.
func ValidateMapLicense(license string) (bool, string) {
	switch license {
	case "ODbL-1.0", "ODbL":
		return true, "© OpenStreetMap contributors (ODbL)"
	case "CC-BY-SA-2.0":
		return true, "© OpenStreetMap contributors (CC-BY-SA)"
	case "LicensedPartner-Geo":
		return true, "Licensed Geographic Data Provider"
	default:
		return false, "unauthorized geographic license: proprietary map scraping is strictly prohibited"
	}
}

// HaversineDistanceKm calculates the great-circle distance between two GeoPoints in kilometers.
func HaversineDistanceKm(p1, p2 GeoPoint) (float64, error) {
	if err := ValidateCoordinates(p1.Latitude, p1.Longitude); err != nil {
		return 0, err
	}
	if err := ValidateCoordinates(p2.Latitude, p2.Longitude); err != nil {
		return 0, err
	}

	const earthRadiusKm = 6371.0

	dLat := (p2.Latitude - p1.Latitude) * (math.Pi / 180.0)
	dLon := (p2.Longitude - p1.Longitude) * (math.Pi / 180.0)

	lat1Rad := p1.Latitude * (math.Pi / 180.0)
	lat2Rad := p2.Latitude * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c, nil
}

// InBoundingBox checks whether a point falls within a geographic bounding box.
func (b GeoBoundingBox) InBoundingBox(p GeoPoint) (bool, error) {
	if err := ValidateCoordinates(p.Latitude, p.Longitude); err != nil {
		return false, err
	}
	if b.MinLat > b.MaxLat || b.MinLon > b.MaxLon {
		return false, errors.New("malformed bounding box boundaries")
	}

	return p.Latitude >= b.MinLat && p.Latitude <= b.MaxLat &&
		p.Longitude >= b.MinLon && p.Longitude <= b.MaxLon, nil
}
