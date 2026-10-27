package vertical

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// LicenseType represents the licensing classification of a media asset.
type LicenseType string

const (
	LicenseCC0            LicenseType = "CC0-1.0"
	LicenseCCBY           LicenseType = "CC-BY-4.0"
	LicenseCCBYSA         LicenseType = "CC-BY-SA-4.0"
	LicensePublicDomain   LicenseType = "PublicDomain"
	LicenseODbL           LicenseType = "ODbL-1.0"
	LicenseLicensedPartner LicenseType = "LicensedEditorial"
	LicenseUnverified     LicenseType = "Unverified"
)

// ImageSafetyStatus represents the content safety classification.
type ImageSafetyStatus string

const (
	SafetySafe       ImageSafetyStatus = "safe"
	SafetyModerate   ImageSafetyStatus = "moderate"
	SafetyRestricted ImageSafetyStatus = "restricted"
	SafetyBlocked    ImageSafetyStatus = "blocked"
)

// ThumbnailSize defines target dimensions for image thumbnails.
type ThumbnailSize string

const (
	ThumbnailSmall  ThumbnailSize = "small"  // 150x150 bounding box
	ThumbnailMedium ThumbnailSize = "medium" // 300x300 bounding box
	ThumbnailLarge  ThumbnailSize = "large"  // 600x600 bounding box
)

// ThumbnailSpec contains computed dimensions for an image thumbnail.
type ThumbnailSpec struct {
	OriginalWidth  int           `json:"original_width"`
	OriginalHeight int           `json:"original_height"`
	TargetWidth    int           `json:"target_width"`
	TargetHeight   int           `json:"target_height"`
	SizeClass      ThumbnailSize `json:"size_class"`
}

// AttributionInfo encapsulates legal copyright and source attribution.
type AttributionInfo struct {
	Author       string      `json:"author"`
	License      LicenseType `json:"license"`
	SourceURL    string      `json:"source_url"`
	NoticeText   string      `json:"notice_text"`
	IsCommercial bool        `json:"is_commercial"`
}

// ImageItem represents a verified indexed image in OpenPortal search.
type ImageItem struct {
	ID          string            `json:"id"`
	URL         string            `json:"url"`
	ThumbnailURL string           `json:"thumbnail_url"`
	Title       string            `json:"title"`
	AltText     string            `json:"alt_text"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Attribution AttributionInfo   `json:"attribution"`
	Safety      ImageSafetyStatus `json:"safety"`
	IndexedAt   time.Time         `json:"indexed_at"`
}

var approvedLicenses = map[LicenseType]bool{
	LicenseCC0:            true,
	LicenseCCBY:           true,
	LicenseCCBYSA:         true,
	LicensePublicDomain:   true,
	LicenseODbL:           true,
	LicenseLicensedPartner: true,
}

var blockedDomains = map[string]bool{
	"pirated-images.test": true,
	"leak.test":           true,
	"scraping-target.test": true,
}

// ValidateImageLicense verifies whether an image license and source domain meet copyright boundaries.
func ValidateImageLicense(license LicenseType, sourceURL string) (bool, string) {
	if !approvedLicenses[license] {
		return false, fmt.Sprintf("unapproved or proprietary license: %s", license)
	}

	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Host == "" {
		return false, "invalid source URL"
	}

	host := strings.ToLower(parsed.Hostname())
	if blockedDomains[host] {
		return false, fmt.Sprintf("source domain %s is blocked for copyright violations", host)
	}

	return true, "valid"
}

// GenerateThumbnailSpec computes scaled bounding-box thumbnail dimensions while preserving aspect ratio.
// Rejects decompression bomb dimensions (>10,000px) or non-positive dimensions.
func GenerateThumbnailSpec(origWidth, origHeight int, size ThumbnailSize) (ThumbnailSpec, error) {
	if origWidth <= 0 || origHeight <= 0 {
		return ThumbnailSpec{}, errors.New("image dimensions must be positive integers")
	}
	if origWidth > 10000 || origHeight > 10000 {
		return ThumbnailSpec{}, errors.New("image dimensions exceed safe decompression threshold (10,000px)")
	}

	var maxBound int
	switch size {
	case ThumbnailSmall:
		maxBound = 150
	case ThumbnailMedium:
		maxBound = 300
	case ThumbnailLarge:
		maxBound = 600
	default:
		return ThumbnailSpec{}, fmt.Errorf("unknown thumbnail size class: %s", size)
	}

	var targetW, targetH int
	if origWidth >= origHeight {
		targetW = maxBound
		targetH = int(float64(origHeight) * float64(maxBound) / float64(origWidth))
		if targetH < 1 {
			targetH = 1
		}
	} else {
		targetH = maxBound
		targetW = int(float64(origWidth) * float64(maxBound) / float64(origHeight))
		if targetW < 1 {
			targetW = 1
		}
	}

	return ThumbnailSpec{
		OriginalWidth:  origWidth,
		OriginalHeight: origHeight,
		TargetWidth:    targetW,
		TargetHeight:   targetH,
		SizeClass:      size,
	}, nil
}

// ClassifyImageSafety evaluates explicit content scores and labels to classify safety.
func ClassifyImageSafety(labels []string, adultScore, racyScore float64) ImageSafetyStatus {
	// Check for explicit blocked keywords in labels
	for _, l := range labels {
		low := strings.ToLower(l)
		if low == "explicit_adult" || low == "csam" || low == "extreme_violence" {
			return SafetyBlocked
		}
	}

	if adultScore >= 0.85 || racyScore >= 0.90 {
		return SafetyBlocked
	}
	if adultScore >= 0.50 || racyScore >= 0.60 {
		return SafetyRestricted
	}
	if adultScore >= 0.20 || racyScore >= 0.30 {
		return SafetyModerate
	}
	return SafetySafe
}

// FormatAttribution creates formatted attribution text conforming to copyright policies.
func FormatAttribution(author string, license LicenseType, sourceURL string) AttributionInfo {
	if author == "" {
		author = "Anonymous / Public Source"
	}
	notice := fmt.Sprintf("Photo by %s under %s. Source: %s", author, license, sourceURL)
	isComm := license == LicenseCC0 || license == LicensePublicDomain || license == LicenseLicensedPartner

	return AttributionInfo{
		Author:       author,
		License:      license,
		SourceURL:    sourceURL,
		NoticeText:   notice,
		IsCommercial: isComm,
	}
}
