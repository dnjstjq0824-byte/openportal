package vertical

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// DurationCategory groups videos by length.
type DurationCategory string

const (
	DurationAny    DurationCategory = "any"
	DurationShort  DurationCategory = "short"  // < 4 minutes (< 240s)
	DurationMedium DurationCategory = "medium" // 4 - 20 minutes (240s - 1200s)
	DurationLong   DurationCategory = "long"   // > 20 minutes (> 1200s)
)

// VideoItem represents an indexed video with authorized embed permissions.
type VideoItem struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	DurationSec  int              `json:"duration_sec"`
	DurationText string           `json:"duration_text"`
	ThumbnailURL string           `json:"thumbnail_url"`
	EmbedURL     string           `json:"embed_url"`
	Publisher    string           `json:"publisher"`
	PublishedAt  time.Time        `json:"published_at"`
	IsHD         bool             `json:"is_hd"`
	Category     DurationCategory `json:"category"`
}

var authorizedEmbedDomains = map[string]bool{
	"youtube.com":               true,
	"www.youtube.com":           true,
	"player.vimeo.com":          true,
	"www.dailymotion.com":       true,
	"upload.wikimedia.org":      true,
	"commons.wikimedia.org":     true,
	"media.openportal.org":      true,
}

// ValidateEmbedURL verifies that a video embed URL belongs to an authorized embed domain
// and uses secure HTTPS scheme without dangerous protocol payloads.
func ValidateEmbedURL(rawURL string) (string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return "", errors.New("empty embed URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	if strings.ToLower(parsed.Scheme) != "https" {
		return "", errors.New("embed URL must use secure https protocol")
	}

	host := strings.ToLower(parsed.Hostname())
	if !authorizedEmbedDomains[host] {
		return "", fmt.Errorf("domain %s is not an authorized embed provider", host)
	}

	// Guard against script injection in path or query
	lowRaw := strings.ToLower(rawURL)
	if strings.Contains(lowRaw, "<script") || strings.Contains(lowRaw, "javascript:") || strings.Contains(lowRaw, "data:") {
		return "", errors.New("malicious content detected in embed URL")
	}

	return parsed.String(), nil
}

// FormatDuration formats an integer number of seconds into standard video time strings:
// e.g. 252 -> "04:12", 3665 -> "01:01:05"
func FormatDuration(sec int) string {
	if sec < 0 {
		sec = 0
	}
	hours := sec / 3600
	minutes := (sec % 3600) / 60
	seconds := sec % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// CategorizeDuration classifies a duration in seconds into short, medium, or long.
func CategorizeDuration(sec int) DurationCategory {
	if sec < 240 {
		return DurationShort
	}
	if sec <= 1200 {
		return DurationMedium
	}
	return DurationLong
}

// NewVideoItem creates and validates a VideoItem with formatted duration and verified embed URL.
func NewVideoItem(id, title, desc string, sec int, thumbURL, embedURL, publisher string, pubDate time.Time, isHD bool) (*VideoItem, error) {
	cleanEmbed, err := ValidateEmbedURL(embedURL)
	if err != nil {
		return nil, err
	}

	return &VideoItem{
		ID:           id,
		Title:        title,
		Description:  desc,
		DurationSec:  sec,
		DurationText: FormatDuration(sec),
		ThumbnailURL: thumbURL,
		EmbedURL:     cleanEmbed,
		Publisher:    publisher,
		PublishedAt:  pubDate,
		IsHD:         isHD,
		Category:     CategorizeDuration(sec),
	}, nil
}

// SearchVideos filters and ranks videos matching a text query and optional duration filter.
func SearchVideos(query string, videos []VideoItem, filter DurationCategory) []VideoItem {
	qTokens := strings.Fields(strings.ToLower(query))

	type scoredVideo struct {
		item  VideoItem
		score float64
	}

	var candidates []scoredVideo
	for _, v := range videos {
		if filter != "" && filter != DurationAny && v.Category != filter {
			continue
		}

		titleLower := strings.ToLower(v.Title)
		descLower := strings.ToLower(v.Description)
		pubLower := strings.ToLower(v.Publisher)

		matchScore := 0.0
		for _, tok := range qTokens {
			if strings.Contains(titleLower, tok) {
				matchScore += 4.0
			}
			if strings.Contains(descLower, tok) {
				matchScore += 1.5
			}
			if strings.Contains(pubLower, tok) {
				matchScore += 2.0
			}
		}

		if matchScore > 0 {
			if v.IsHD {
				matchScore += 0.5
			}
			candidates = append(candidates, scoredVideo{item: v, score: matchScore})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	res := make([]VideoItem, len(candidates))
	for i, c := range candidates {
		res[i] = c.item
	}
	return res
}
