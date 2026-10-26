package ranking

import (
	"testing"
)

func TestIntentClassification(t *testing.T) {
	classifier := NewIntentClassifier()

	tests := []struct {
		name       string
		query      string
		lang       string
		region     string
		wantIntent IntentType
		wantLang   string
		wantRegion string
	}{
		{
			name:       "navigational domain suffix",
			query:      "naver.com",
			wantIntent: IntentNavigational,
			wantLang:   "en",
			wantRegion: "US",
		},
		{
			name:       "navigational official portal keyword",
			query:      "대한민국 정부 홈페이지",
			wantIntent: IntentNavigational,
			wantLang:   "ko",
			wantRegion: "KR",
		},
		{
			name:       "local intent gangnam restaurant",
			query:      "강남역 맛집 추천",
			wantIntent: IntentLocal,
			wantLang:   "ko",
			wantRegion: "KR",
		},
		{
			name:       "transactional intent buy discount",
			query:      "아이폰 최저가 할인 구매",
			wantIntent: IntentTransactional,
			wantLang:   "ko",
			wantRegion: "KR",
		},
		{
			name:       "informational intent how to",
			query:      "검색엔진 만드는 방법",
			wantIntent: IntentInformational,
			wantLang:   "ko",
			wantRegion: "KR",
		},
		{
			name:       "english local query",
			query:      "best cafe near me",
			wantIntent: IntentLocal,
			wantLang:   "en",
			wantRegion: "US",
		},
		{
			name:       "english transactional",
			query:      "buy mechanical keyboard cheap",
			wantIntent: IntentTransactional,
			wantLang:   "en",
			wantRegion: "US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := classifier.Classify(tt.query, tt.lang, tt.region)
			if res.Intent != tt.wantIntent {
				t.Errorf("Classify(%q) Intent = %v, want %v", tt.query, res.Intent, tt.wantIntent)
			}
			if res.Language != tt.wantLang {
				t.Errorf("Classify(%q) Language = %v, want %v", tt.query, res.Language, tt.wantLang)
			}
			if res.Region != tt.wantRegion {
				t.Errorf("Classify(%q) Region = %v, want %v", tt.query, res.Region, tt.wantRegion)
			}
			if res.Confidence <= 0.0 || res.Confidence > 1.0 {
				t.Errorf("Classify(%q) invalid confidence = %v", tt.query, res.Confidence)
			}
		})
	}
}

func TestApplyCorrection(t *testing.T) {
	classifier := NewIntentClassifier()

	// Opt-out test: must never change query
	fixed, corrected := classifier.ApplyCorrection("gksrmf", false)
	if corrected || fixed != "gksrmf" {
		t.Fatalf("expected unchanged query when opt-in is false, got %q, %v", fixed, corrected)
	}

	// Opt-in tests
	optTests := []struct {
		input     string
		wantFixed string
		wantBool  bool
	}{
		{"gksrmf", "한글", true},
		{"dhvmsvhxjf", "오픈포털", true},
		{"오푼포털", "오픈포털", true},
		{"검색엔진ㄴ", "검색엔진", true},
		{"정상적인 검색어", "정상적인 검색어", false},
	}

	for _, tc := range optTests {
		gotFixed, gotBool := classifier.ApplyCorrection(tc.input, true)
		if gotBool != tc.wantBool || gotFixed != tc.wantFixed {
			t.Errorf("ApplyCorrection(%q, true) = (%q, %v), want (%q, %v)", tc.input, gotFixed, gotBool, tc.wantFixed, tc.wantBool)
		}
	}
}

