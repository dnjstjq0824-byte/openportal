package vertical

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// AvailabilityStatus represents inventory status.
type AvailabilityStatus string

const (
	InStock    AvailabilityStatus = "in_stock"
	OutOfStock AvailabilityStatus = "out_of_stock"
	PreOrder   AvailabilityStatus = "pre_order"
)

// ProductOffer models a verified merchant product listing.
type ProductOffer struct {
	ID            string             `json:"id"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	MerchantName  string             `json:"merchant_name"`
	Price         float64            `json:"price"`
	Currency      string             `json:"currency"`
	Availability  AvailabilityStatus `json:"availability"`
	AffiliateURL  string             `json:"affiliate_url"`
	IsSponsored   bool               `json:"is_sponsored"`
	SponsoredTag  string             `json:"sponsored_tag"`
	FeedUpdatedAt time.Time          `json:"feed_updated_at"`
	IsPriceStale  bool               `json:"is_price_stale"`
}

// PriceDiscrepancyReport logs a user report regarding fraudulent pricing or stock inaccuracies.
type PriceDiscrepancyReport struct {
	ReportID      string    `json:"report_id"`
	OfferID       string    `json:"offer_id"`
	AccountID     string    `json:"account_id"`
	ReportedPrice float64   `json:"reported_price"`
	ActualPrice   float64   `json:"actual_price"`
	Reason        string    `json:"reason"`
	Status        string    `json:"status"` // pending, reviewed, resolved
	CreatedAt     time.Time `json:"created_at"`
}

// ShoppingFeedManager manages shopping feed validation, freshness checks, and discrepancy reports.
type ShoppingFeedManager struct {
	mu           sync.RWMutex
	offers       map[string]ProductOffer
	reports      map[string]PriceDiscrepancyReport
	maxFeedAge   time.Duration
}

// NewShoppingFeedManager initializes the feed manager with max allowed feed age (e.g. 24 hours).
func NewShoppingFeedManager(maxFeedAge time.Duration) *ShoppingFeedManager {
	if maxFeedAge <= 0 {
		maxFeedAge = 24 * time.Hour
	}
	return &ShoppingFeedManager{
		offers:     make(map[string]ProductOffer),
		reports:    make(map[string]PriceDiscrepancyReport),
		maxFeedAge: maxFeedAge,
	}
}

var supportedCurrencies = map[string]bool{
	"KRW": true,
	"USD": true,
	"EUR": true,
	"JPY": true,
}

// IngestOffer validates product offer fields, currency, and enforces mandatory sponsored disclosures.
func (m *ShoppingFeedManager) IngestOffer(offer ProductOffer) (*ProductOffer, error) {
	if offer.ID == "" || strings.TrimSpace(offer.Title) == "" {
		return nil, errors.New("offer ID and title are required")
	}
	if offer.Price <= 0 {
		return nil, errors.New("price must be greater than zero")
	}
	if !supportedCurrencies[strings.ToUpper(offer.Currency)] {
		return nil, fmt.Errorf("unsupported currency: %s", offer.Currency)
	}

	// Mandatory FTC / Korean Fair Trade Commission sponsored affiliate disclosure
	if offer.IsSponsored {
		offer.SponsoredTag = "광고 / Sponsored"
	} else {
		offer.SponsoredTag = ""
	}

	// Check feed staleness
	if time.Since(offer.FeedUpdatedAt) > m.maxFeedAge {
		offer.IsPriceStale = true
	} else {
		offer.IsPriceStale = false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.offers[offer.ID] = offer
	return &offer, nil
}

// SearchOffers queries product offers with price sorting and optional sponsored exclusion.
func (m *ShoppingFeedManager) SearchOffers(query string, maxPrice float64, inStockOnly bool, sortBy string) []ProductOffer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	qTokens := strings.Fields(strings.ToLower(query))
	var matched []ProductOffer

	for _, off := range m.offers {
		if inStockOnly && off.Availability != InStock {
			continue
		}
		if maxPrice > 0 && off.Price > maxPrice {
			continue
		}

		titleLow := strings.ToLower(off.Title)
		descLow := strings.ToLower(off.Description)
		merchantLow := strings.ToLower(off.MerchantName)

		hit := false
		for _, tok := range qTokens {
			if strings.Contains(titleLow, tok) || strings.Contains(descLow, tok) || strings.Contains(merchantLow, tok) {
				hit = true
				break
			}
		}

		if len(qTokens) == 0 || hit {
			// Update freshness check on search
			if time.Since(off.FeedUpdatedAt) > m.maxFeedAge {
				off.IsPriceStale = true
			}
			matched = append(matched, off)
		}
	}

	switch sortBy {
	case "price_asc":
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].Price < matched[j].Price
		})
	case "price_desc":
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].Price > matched[j].Price
		})
	default:
		// Default: fresh and non-stale first, then non-sponsored first
		sort.Slice(matched, func(i, j int) bool {
			if matched[i].IsPriceStale != matched[j].IsPriceStale {
				return !matched[i].IsPriceStale
			}
			return matched[i].Price < matched[j].Price
		})
	}

	return matched
}

// SubmitDiscrepancyReport records a user report of inaccurate pricing or availability.
func (m *ShoppingFeedManager) SubmitDiscrepancyReport(offerID string, reportedPrice, actualPrice float64, reason, accountID string) (*PriceDiscrepancyReport, error) {
	if offerID == "" || reason == "" {
		return nil, errors.New("offer_id and reason are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.offers[offerID]; !exists {
		return nil, fmt.Errorf("offer %s not found", offerID)
	}

	reportID := fmt.Sprintf("rep-%s-%d", offerID, time.Now().UnixNano())
	rep := PriceDiscrepancyReport{
		ReportID:      reportID,
		OfferID:       offerID,
		AccountID:     accountID,
		ReportedPrice: reportedPrice,
		ActualPrice:   actualPrice,
		Reason:        reason,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}

	m.reports[reportID] = rep
	return &rep, nil
}
