package indexer

import (
	"testing"
	"time"
)

func TestSimHashAndNearDuplicateDetection(t *testing.T) {
	textOrig := "오픈포털은 대한민국 사용자를 위한 고성능 독립 웹 검색엔진 시스템입니다. 분산 아키텍처를 기반으로 크롤러, 형태소 분석기, 역색인 및 랭킹 엔진이 긴밀하게 결합되어 동작합니다. 대규모 웹 문서에서 유용한 정보를 빠르고 정확하게 검색할 수 있도록 지원하며 사용자의 프라이버시를 철저히 보장합니다. 다양한 웹 문서 수집 정책과 RFC 9309 표준을 준수합니다."
	// Minor edit: changing only 1 word in a realistic paragraph
	textMinorDiff := "오픈포털은 대한민국 사용자를 위한 고성능 독립 웹 검색엔진 시스템입니다. 분산 아키텍처를 기반으로 크롤러, 형태소 분석기, 역색인 및 랭킹 엔진이 긴밀하게 결합되어 동작합니다. 대규모 웹 문서에서 유용한 정보를 빠르고 정확하게 찾아볼 수 있도록 지원하며 사용자의 프라이버시를 철저히 보장합니다. 다양한 웹 문서 수집 정책과 RFC 9309 표준을 준수합니다."
	// Completely different document
	textDifferent := "오늘 저녁 추천 요리 메뉴는 신선한 채소와 소고기를 넣은 된장찌개 레시피입니다. 뚝배기에 멸치 다시마 육수를 붓고 된장을 풀어 끓인 뒤 감자, 양파, 애호박, 두부를 썰어 넣어 보글보글 끓여냅니다. 영양이 풍부하고 누구나 쉽게 만들 수 있습니다."

	hOrig := SimHash(textOrig)
	hMinor := SimHash(textMinorDiff)
	hDiff := SimHash(textDifferent)

	distMinor := HammingDistance(hOrig, hMinor)
	distDiff := HammingDistance(hOrig, hDiff)

	t.Logf("SimHash orig:  %s", FormatSimHash(hOrig))
	t.Logf("SimHash minor: %s (distance: %d)", FormatSimHash(hMinor), distMinor)
	t.Logf("SimHash diff:  %s (distance: %d)", FormatSimHash(hDiff), distDiff)

	if !IsNearDuplicate(hOrig, hMinor, 4) {
		t.Errorf("expected textMinorDiff to be near-duplicate of textOrig, got distance %d", distMinor)
	}

	if IsNearDuplicate(hOrig, hDiff, 4) {
		t.Errorf("expected textDifferent NOT to be near-duplicate, got distance %d", distDiff)
	}

	if distDiff <= 10 {
		t.Errorf("expected high hamming distance for completely different text, got %d", distDiff)
	}
}

func TestSelectRepresentative(t *testing.T) {
	now := time.Now().UTC()
	cluster := []*DocCandidate{
		{
			ID:             "doc-http",
			URL:            "http://example.org/article?utm_source=fb",
			CanonicalURL:   "https://example.org/article",
			SimHash:        0x12345678,
			CrawledAt:      now.Add(-2 * time.Hour),
			DomainPriority: 50,
		},
		{
			ID:             "doc-canonical",
			URL:            "https://example.org/article",
			CanonicalURL:   "https://example.org/article", // Matches own canonical
			SimHash:        0x12345678,
			CrawledAt:      now.Add(-1 * time.Hour),
			DomainPriority: 50,
		},
		{
			ID:             "doc-mirror",
			URL:            "https://mirror.example.org/article",
			CanonicalURL:   "",
			SimHash:        0x12345679,
			CrawledAt:      now,
			DomainPriority: 30,
		},
	}

	result := SelectRepresentative(cluster)
	if result.Representative == nil {
		t.Fatal("expected representative, got nil")
	}

	if result.Representative.ID != "doc-canonical" {
		t.Errorf("expected doc-canonical to be representative, got %s", result.Representative.ID)
	}

	if len(result.Aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(result.Aliases))
	}
}

