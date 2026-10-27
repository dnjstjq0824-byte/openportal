package vertical

import (
	"testing"
)

func TestValidateEmbedURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantValid bool
	}{
		{"valid youtube embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", true},
		{"valid vimeo player", "https://player.vimeo.com/video/76979871", true},
		{"valid wikimedia media", "https://upload.wikimedia.org/wikipedia/commons/video.webm", true},
		{"insecure http", "http://www.youtube.com/embed/dQw4w9WgXcQ", false},
		{"unauthorized domain", "https://attacker.site/video/123", false},
		{"malicious script injection", "https://www.youtube.com/embed/<script>alert(1)</script>", false},
		{"javascript pseudo-protocol", "javascript:alert(1)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateEmbedURL(tt.url)
			if (err == nil) != tt.wantValid {
				t.Errorf("ValidateEmbedURL(%q) valid = %v, want %v (err: %v)", tt.url, err == nil, tt.wantValid, err)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		sec  int
		want string
	}{
		{0, "00:00"},
		{45, "00:45"},
		{252, "04:12"},
		{3600, "01:00:00"},
		{3665, "01:01:05"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.sec)
		if got != tt.want {
			t.Errorf("FormatDuration(%d) = %s, want %s", tt.sec, got, tt.want)
		}
	}
}

func TestSearchVideos(t *testing.T) {
	videos := []VideoItem{
		{ID: "v1", Title: "Go Concurrency Patterns", Description: "Learn channels and goroutines", DurationSec: 180, Category: DurationShort, Publisher: "GopherCon", IsHD: true},
		{ID: "v2", Title: "Full Go Masterclass", Description: "Complete Go course in 45 minutes", DurationSec: 2700, Category: DurationLong, Publisher: "Tech Academy", IsHD: true},
		{ID: "v3", Title: "Python for Beginners", Description: "Intro to Python", DurationSec: 600, Category: DurationMedium, Publisher: "Pythonista", IsHD: false},
	}

	// Search "go" without duration filter
	allGo := SearchVideos("go", videos, DurationAny)
	if len(allGo) != 2 {
		t.Fatalf("expected 2 Go videos, got %d", len(allGo))
	}

	// Search "go" with short duration filter
	shortGo := SearchVideos("go", videos, DurationShort)
	if len(shortGo) != 1 || shortGo[0].ID != "v1" {
		t.Fatalf("expected only v1, got %+v", shortGo)
	}
}
