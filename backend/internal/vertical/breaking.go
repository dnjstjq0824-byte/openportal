package vertical

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// BreakingAlert represents an active breaking news alert.
type BreakingAlert struct {
	StoryClusterID     string    `json:"story_cluster_id"`
	Topic              string    `json:"topic"`
	ArticleCount       int       `json:"article_count"`
	IndependentPubs    int       `json:"independent_pubs"`
	FirstReportedAt    time.Time `json:"first_reported_at"`
	VelocityPerMinute  float64   `json:"velocity_per_minute"`
	IsConfirmed        bool      `json:"is_confirmed"`
}

// RetractionRecord logs an editorial retraction or legal takedown.
type RetractionRecord struct {
	ArticleID    string    `json:"article_id"`
	Reason       string    `json:"reason"`
	RetractedAt  time.Time `json:"retracted_at"`
	IsTombstoned bool      `json:"is_tombstoned"`
}

// NewsLifecycleManager manages breaking alerts, stale article warnings, and retractions.
type NewsLifecycleManager struct {
	mu           sync.RWMutex
	retractions  map[string]RetractionRecord
	staleDays    int
}

// NewNewsLifecycleManager initializes a lifecycle manager with default 365 stale threshold days.
func NewNewsLifecycleManager(staleDays int) *NewsLifecycleManager {
	if staleDays <= 0 {
		staleDays = 365
	}
	return &NewsLifecycleManager{
		retractions: make(map[string]RetractionRecord),
		staleDays:   staleDays,
	}
}

// DetectBreakingBursts evaluates story clusters within a recent window (e.g. 1 hour).
// A breaking news alert is confirmed ONLY when at least minPubs independent publishers
// report the story and velocity reaches the minimum threshold, avoiding single-source unverified rumors.
func DetectBreakingBursts(clusters []StoryCluster, window time.Duration, minPubs int, minArticles int) []BreakingAlert {
	now := time.Now()
	var alerts []BreakingAlert

	for _, cl := range clusters {
		totalArticles := 1 + len(cl.RelatedArticles)
		if totalArticles < minArticles {
			continue
		}

		if cl.PublisherCount < minPubs {
			// Fail-safe: single source cannot trigger breaking news alert
			continue
		}

		age := now.Sub(cl.LatestPublishedAt)
		if age < 0 {
			age = 0
		}
		if age > window {
			continue
		}

		minutes := age.Minutes()
		if minutes < 1.0 {
			minutes = 1.0
		}
		velocity := float64(totalArticles) / minutes

		alerts = append(alerts, BreakingAlert{
			StoryClusterID:    cl.ClusterID,
			Topic:             cl.LeadArticle.Title,
			ArticleCount:      totalArticles,
			IndependentPubs:   cl.PublisherCount,
			FirstReportedAt:   cl.LeadArticle.PublishedAt,
			VelocityPerMinute: velocity,
			IsConfirmed:       true,
		})
	}

	return alerts
}

// EvaluateStaleness checks if an article was published more than staleDays ago and sets warning banner.
func (m *NewsLifecycleManager) EvaluateStaleness(art *NewsArticle) (bool, string) {
	age := time.Since(art.PublishedAt)
	maxAge := time.Duration(m.staleDays) * 24 * time.Hour
	if age > maxAge {
		art.IsStale = true
		years := int(age.Hours() / (24 * 365))
		if years < 1 {
			years = 1
		}
		msg := fmt.Sprintf("이 기사는 발행된 지 %d년 이상 지난 오래된 기사입니다. 최신 정보와 다를 수 있습니다.", years)
		return true, msg
	}
	art.IsStale = false
	return false, ""
}

// ProcessRetraction registers a retraction/takedown, immediately marking the article as tombstoned.
func (m *NewsLifecycleManager) ProcessRetraction(articleID, reason string) (*RetractionRecord, error) {
	if articleID == "" {
		return nil, errors.New("empty article ID")
	}
	if reason == "" {
		return nil, errors.New("retraction reason required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rec := RetractionRecord{
		ArticleID:    articleID,
		Reason:       reason,
		RetractedAt:  time.Now(),
		IsTombstoned: true,
	}
	m.retractions[articleID] = rec
	return &rec, nil
}

// IsRetracted returns whether an article has been retracted or tombstoned.
func (m *NewsLifecycleManager) IsRetracted(articleID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, found := m.retractions[articleID]
	return found
}

// FilterActiveArticles removes any retracted/tombstoned articles from active search results.
func (m *NewsLifecycleManager) FilterActiveArticles(articles []NewsArticle) []NewsArticle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []NewsArticle
	for _, a := range articles {
		if _, retracted := m.retractions[a.ID]; !retracted {
			// Also evaluate staleness in place
			if time.Since(a.PublishedAt) > time.Duration(m.staleDays)*24*time.Hour {
				a.IsStale = true
			}
			active = append(active, a)
		}
	}
	return active
}
