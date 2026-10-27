package vertical

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// POIPlace represents a verified Point of Interest.
type POIPlace struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Address       string   `json:"address"`
	Location      GeoPoint `json:"location"`
	BusinessURL   string   `json:"business_url"`
	Phone         string   `json:"phone"`
	IsVerified    bool     `json:"is_verified"`
	SourceLicense string   `json:"source_license"`
}

// POISearchResult holds a POI along with its distance from query center.
type POISearchResult struct {
	Place      POIPlace `json:"place"`
	DistanceKm float64  `json:"distance_km"`
	Score      float64  `json:"score"`
}

// POICorrectionStatus represents the state of a user-submitted business correction.
type POICorrectionStatus string

const (
	CorrectionPending  POICorrectionStatus = "pending"
	CorrectionApproved POICorrectionStatus = "approved"
	CorrectionRejected POICorrectionStatus = "rejected"
)

// POICorrectionRequest models a business information correction submitted by a user or business owner.
type POICorrectionRequest struct {
	RequestID   string              `json:"request_id"`
	PlaceID     string              `json:"place_id"`
	AccountID   string              `json:"account_id"`
	FieldName   string              `json:"field_name"`
	OldValue    string              `json:"old_value"`
	NewValue    string              `json:"new_value"`
	Status      POICorrectionStatus `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	ReviewedAt  *time.Time          `json:"reviewed_at,omitempty"`
	ReviewNotes string              `json:"review_notes,omitempty"`
}

// POIManager manages POI search, autocomplete, and user business correction workflows.
type POIManager struct {
	mu          sync.RWMutex
	places      map[string]POIPlace
	corrections map[string]POICorrectionRequest
}

// NewPOIManager initializes a POIManager.
func NewPOIManager() *POIManager {
	return &POIManager{
		places:      make(map[string]POIPlace),
		corrections: make(map[string]POICorrectionRequest),
	}
}

// RegisterPlace registers a verified POI into the manager.
func (m *POIManager) RegisterPlace(place POIPlace) error {
	if place.ID == "" || place.Name == "" {
		return errors.New("place ID and name are required")
	}
	if err := ValidateCoordinates(place.Location.Latitude, place.Location.Longitude); err != nil {
		return fmt.Errorf("invalid place location: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.places[place.ID] = place
	return nil
}

// SearchPOI finds places matching query text within radiusKm of center, ordered by combined relevance + proximity.
func (m *POIManager) SearchPOI(query string, center GeoPoint, radiusKm float64) []POISearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	qTokens := strings.Fields(strings.ToLower(query))
	var results []POISearchResult

	for _, place := range m.places {
		dist, err := HaversineDistanceKm(center, place.Location)
		if err != nil || (radiusKm > 0 && dist > radiusKm) {
			continue
		}

		nameLow := strings.ToLower(place.Name)
		addrLow := strings.ToLower(place.Address)
		catLow := strings.ToLower(place.Category)

		matchScore := 0.0
		for _, tok := range qTokens {
			if strings.Contains(nameLow, tok) {
				matchScore += 5.0
			}
			if strings.Contains(catLow, tok) {
				matchScore += 3.0
			}
			if strings.Contains(addrLow, tok) {
				matchScore += 2.0
			}
		}

		// If query is empty or matches
		if len(qTokens) == 0 || matchScore > 0 {
			// Proximity boost: closer gets higher score
			proximityScore := 10.0 / (1.0 + dist)
			totalScore := matchScore + proximityScore
			if place.IsVerified {
				totalScore += 1.0
			}

			results = append(results, POISearchResult{
				Place:      place,
				DistanceKm: dist,
				Score:      totalScore,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// AutocompletePOI provides prefix and substring suggestions for place search.
func (m *POIManager) AutocompletePOI(prefix string, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	prefixLow := strings.TrimSpace(strings.ToLower(prefix))
	if prefixLow == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var suggestions []string
	seen := make(map[string]bool)

	for _, p := range m.places {
		nameLow := strings.ToLower(p.Name)
		if strings.HasPrefix(nameLow, prefixLow) || strings.Contains(nameLow, prefixLow) {
			if !seen[p.Name] {
				suggestions = append(suggestions, p.Name)
				seen[p.Name] = true
				if len(suggestions) >= limit {
					break
				}
			}
		}
	}

	return suggestions
}

// SubmitCorrection records a business information correction request.
func (m *POIManager) SubmitCorrection(placeID, field, oldValue, newValue, accountID string) (*POICorrectionRequest, error) {
	if placeID == "" || field == "" || newValue == "" {
		return nil, errors.New("place_id, field_name, and new_value are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	place, exists := m.places[placeID]
	if !exists {
		return nil, fmt.Errorf("place %s not found", placeID)
	}

	reqID := fmt.Sprintf("corr-%s-%d", placeID, time.Now().UnixNano())
	req := POICorrectionRequest{
		RequestID:   reqID,
		PlaceID:     placeID,
		AccountID:   accountID,
		FieldName:   field,
		OldValue:    oldValue,
		NewValue:    newValue,
		Status:      CorrectionPending,
		CreatedAt:   time.Now(),
		ReviewNotes: fmt.Sprintf("Correction request submitted for %s", place.Name),
	}

	m.corrections[reqID] = req
	return &req, nil
}
