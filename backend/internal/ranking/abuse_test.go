package ranking

import (
	"testing"
	"time"
)

func TestAbuseDetectorPIIAndPolicy(t *testing.T) {
	ad := NewAbuseDetector(5*time.Second, 10, 5)

	tests := []struct {
		name        string
		query       string
		wantAbusive bool
		wantType    AbuseType
	}{
		{
			name:        "valid benign query",
			query:       "오픈포털 검색엔진 아키텍처",
			wantAbusive: false,
			wantType:    AbuseNone,
		},
		{
			name:        "RRN leak",
			query:       "회원 950101-1234567 조회",
			wantAbusive: true,
			wantType:    AbusePIILeak,
		},
		{
			name:        "Credit Card leak",
			query:       "결제 카드 1234-5678-9012-3456 내역",
			wantAbusive: true,
			wantType:    AbusePIILeak,
		},
		{
			name:        "Phone number leak",
			query:       "연락처 010-1234-5678 문의",
			wantAbusive: true,
			wantType:    AbusePIILeak,
		},
		{
			name:        "Blacklist illegal gambling",
			query:       "인터넷 불법도박 사이트",
			wantAbusive: true,
			wantType:    AbusePolicyBlacklist,
		},
		{
			name:        "Repetition low entropy spam",
			query:       "aaaaaaaaaaaaaaaaaaaa",
			wantAbusive: true,
			wantType:    AbuseEntropyAnomaly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := ad.InspectQuery("127.0.0.1", tt.query)
			if rep.IsAbusive != tt.wantAbusive {
				t.Errorf("InspectQuery(%q) IsAbusive = %v, want %v", tt.query, rep.IsAbusive, tt.wantAbusive)
			}
			if rep.Type != tt.wantType {
				t.Errorf("InspectQuery(%q) Type = %v, want %v", tt.query, rep.Type, tt.wantType)
			}
		})
	}
}

func TestAbuseDetectorBurstAndManipulation(t *testing.T) {
	// window 2s, max 5 queries per IP, max 3 queries for same query
	ad := NewAbuseDetector(2*time.Second, 5, 3)

	clientIP := "192.168.1.100"
	query := "특정 키워드 랭킹 조작"

	// 1st, 2nd, 3rd requests should be benign
	for i := 1; i <= 3; i++ {
		rep := ad.InspectQuery(clientIP, query)
		if rep.IsAbusive {
			t.Fatalf("request %d should not be abusive, got %+v", i, rep)
		}
	}

	// 4th request with the exact same query in rapid succession should trigger manipulation burst
	rep := ad.InspectQuery(clientIP, query)
	if !rep.IsAbusive || rep.Type != AbuseManipulation {
		t.Fatalf("expected AbuseManipulation on 4th repeat, got %+v", rep)
	}

	// Other queries from same IP: 5th request
	rep2 := ad.InspectQuery(clientIP, "다른 일반 검색어 1")
	if rep2.IsAbusive {
		t.Fatalf("request 5 with new query should not be abusive, got %+v", rep2)
	}

	// 6th request from same IP exceeds maxBurstIP (5) -> triggers AbuseBurstRate
	rep3 := ad.InspectQuery(clientIP, "다른 일반 검색어 2")
	if !rep3.IsAbusive || rep3.Type != AbuseBurstRate {
		t.Fatalf("expected AbuseBurstRate on 6th IP request, got %+v", rep3)
	}
}

func TestAbuseDetectorPurgeAndFilterSuggestions(t *testing.T) {
	ad := NewAbuseDetector(5*time.Second, 10, 5)

	initial := []string{
		"오픈포털",
		"불법도박 사이트 바로가기",
		"강남역 맛집",
		"긴급 개인정보 900101-1234567",
		"임시 삭제 대상 검색어",
	}

	// Filter before purge
	filtered := ad.FilterSuggestions(initial)
	// Should remove "불법도박 사이트 바로가기" and "긴급 개인정보 900101-1234567"
	for _, s := range filtered {
		if s == "불법도박 사이트 바로가기" || s == "긴급 개인정보 900101-1234567" {
			t.Errorf("filtered suggestions contained policy/pii violation: %q", s)
		}
	}

	// Now purge "임시 삭제 대상 검색어"
	ad.Purge("임시 삭제 대상 검색어", "legal takedown request")

	filteredAfterPurge := ad.FilterSuggestions(initial)
	for _, s := range filteredAfterPurge {
		if s == "임시 삭제 대상 검색어" {
			t.Errorf("filtered suggestions contained purged term: %q", s)
		}
	}

	// Verify InspectQuery on purged term
	rep := ad.InspectQuery("127.0.0.1", "임시 삭제 대상 검색어")
	if !rep.IsAbusive || rep.Type != AbusePolicyBlacklist {
		t.Errorf("expected purged term to be abusive, got %+v", rep)
	}
}

