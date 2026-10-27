package vertical

import (
	"testing"
)

func TestValidateImageLicense(t *testing.T) {
	tests := []struct {
		name      string
		license   LicenseType
		sourceURL string
		wantValid bool
	}{
		{"valid CC0", LicenseCC0, "https://commons.wikimedia.org/wiki/File:Example.jpg", true},
		{"valid CC-BY", LicenseCCBY, "https://openportal.org/media/image1.png", true},
		{"unverified license", LicenseUnverified, "https://openportal.org/media/image1.png", false},
		{"blocked domain", LicenseCC0, "https://pirated-images.test/photo.jpg", false},
		{"invalid source URL", LicenseCC0, "invalid-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := ValidateImageLicense(tt.license, tt.sourceURL)
			if valid != tt.wantValid {
				t.Errorf("ValidateImageLicense(%s, %s) = %v, want %v", tt.license, tt.sourceURL, valid, tt.wantValid)
			}
		})
	}
}

func TestGenerateThumbnailSpec(t *testing.T) {
	// Landscape image 1920x1080 -> small (150x84)
	spec, err := GenerateThumbnailSpec(1920, 1080, ThumbnailSmall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.TargetWidth != 150 || spec.TargetHeight != 84 {
		t.Errorf("expected 150x84, got %dx%d", spec.TargetWidth, spec.TargetHeight)
	}

	// Portrait image 1080x1920 -> medium (168x300)
	specM, err := GenerateThumbnailSpec(1080, 1920, ThumbnailMedium)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if specM.TargetHeight != 300 || specM.TargetWidth != 168 {
		t.Errorf("expected 168x300, got %dx%d", specM.TargetWidth, specM.TargetHeight)
	}

	// Decompression bomb guard (>10000)
	_, err = GenerateThumbnailSpec(20000, 20000, ThumbnailSmall)
	if err == nil {
		t.Errorf("expected decompression bomb error, got nil")
	}

	// Zero or negative dimension guard
	_, err = GenerateThumbnailSpec(0, 500, ThumbnailSmall)
	if err == nil {
		t.Errorf("expected error for 0 width, got nil")
	}
}

func TestClassifyImageSafety(t *testing.T) {
	// Safe
	if got := ClassifyImageSafety([]string{"nature", "mountain"}, 0.05, 0.1); got != SafetySafe {
		t.Errorf("expected safe, got %s", got)
	}

	// Moderate
	if got := ClassifyImageSafety([]string{"beach", "swimwear"}, 0.25, 0.35); got != SafetyModerate {
		t.Errorf("expected moderate, got %s", got)
	}

	// Restricted
	if got := ClassifyImageSafety([]string{"art", "statue"}, 0.60, 0.65); got != SafetyRestricted {
		t.Errorf("expected restricted, got %s", got)
	}

	// Blocked by adult score
	if got := ClassifyImageSafety([]string{"unknown"}, 0.90, 0.95); got != SafetyBlocked {
		t.Errorf("expected blocked, got %s", got)
	}

	// Blocked by label
	if got := ClassifyImageSafety([]string{"explicit_adult"}, 0.1, 0.1); got != SafetyBlocked {
		t.Errorf("expected blocked by label, got %s", got)
	}
}

func TestFormatAttribution(t *testing.T) {
	attr := FormatAttribution("Alice Painter", LicenseCCBY, "https://openportal.org/art")
	if attr.Author != "Alice Painter" {
		t.Errorf("unexpected author: %s", attr.Author)
	}
	if attr.NoticeText == "" {
		t.Errorf("expected non-empty notice text")
	}
}
