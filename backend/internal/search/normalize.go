package search

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeQuery trims leading/trailing whitespace, collapses internal whitespace runs to a single space,
// strips control characters, and validates UTF-8 and length bounds (1..200 runes).
func NormalizeQuery(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errors.New("query must be valid UTF-8 and at most 200 characters")
	}
	var b strings.Builder
	b.Grow(len(raw))
	inSpace := false
	for _, r := range raw {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			continue
		}
		if unicode.IsSpace(r) {
			if b.Len() > 0 {
				inSpace = true
			}
			continue
		}
		if inSpace {
			b.WriteByte(' ')
			inSpace = false
		}
		b.WriteRune(r)
	}
	out := b.String()
	if out == "" {
		return "", errors.New("query is required")
	}
	if utf8.RuneCountInString(out) > 200 {
		return "", errors.New("query must be valid UTF-8 and at most 200 characters")
	}
	return out, nil
}

var defaultRewrites = map[string]string{
	"rjatordpswls":  "검색엔진",
	"dhvmsvhxjf":    "오픈포털",
	"gksrmf":        "한글",
	"검섹엔진":          "검색엔진",
	"검색엔지":          "검색엔진",
	"오픈포탈":          "오픈포털",
	"openprotal":    "OpenPortal",
	"openporatl":    "OpenPortal",
	"serach engine": "search engine",
}

var knownCorpusTerms = []string{
	"검색엔진", "오픈포털", "인덱싱", "크롤러", "랭킹", "역색인",
	"형태소", "데이터베이스", "아키텍처", "OpenPortal",
	"search engine", "crawler", "indexer", "ranking", "database",
}

func levenshteinRunes(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	dp := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		dp[j] = j
	}
	for i := 1; i <= la; i++ {
		prev := dp[0]
		dp[0] = i
		for j := 1; j <= lb; j++ {
			temp := dp[j]
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			minVal := dp[j] + 1
			if dp[j-1]+1 < minVal {
				minVal = dp[j-1] + 1
			}
			if prev+cost < minVal {
				minVal = prev + cost
			}
			dp[j] = minVal
			prev = temp
		}
	}
	return dp[lb]
}

// SuggestCorrection returns a suggested query correction if the normalized query matches known Korean/English typo or QWERTY-Hangul patterns, or has Levenshtein distance <= 2 with corpus terms.
func SuggestCorrection(normalizedQuery string) string {
	key := strings.ToLower(strings.TrimSpace(normalizedQuery))
	if corr, ok := defaultRewrites[key]; ok && !strings.EqualFold(corr, normalizedQuery) {
		return corr
	}
	if corr, ok := defaultRewrites[normalizedQuery]; ok && corr != normalizedQuery {
		return corr
	}

	// Levenshtein edit distance check for terms >= 3 runes
	qRunes := []rune(key)
	if len(qRunes) >= 3 {
		bestDist := 3 // looking for <= 2
		bestCandidate := ""
		for _, term := range knownCorpusTerms {
			tRunes := []rune(strings.ToLower(term))
			diff := len(qRunes) - len(tRunes)
			if diff < -2 || diff > 2 {
				continue
			}
			dist := levenshteinRunes(qRunes, tRunes)
			if dist > 0 && dist <= 2 && dist < bestDist {
				bestDist = dist
				bestCandidate = term
			}
		}
		if bestCandidate != "" && !strings.EqualFold(bestCandidate, normalizedQuery) {
			return bestCandidate
		}
	}

	return ""
}

// VerticalTab describes a search vertical tab controlled by feature flags (Task 013).
type VerticalTab struct {
	ID       string `json:"id"`
	LabelKo  string `json:"label_ko"`
	LabelEn  string `json:"label_en"`
	Enabled  bool   `json:"enabled"`
	Phase    int    `json:"phase"`
	GateNote string `json:"gate_note,omitempty"`
}

// DefaultVerticalTabs returns the Phase 1 feature-flagged vertical tabs.
// Only 'web' is enabled; non-web verticals remain gated behind Phase 5 licensing and ingestion pipelines.
func DefaultVerticalTabs() []VerticalTab {
	return []VerticalTab{
		{ID: "web", LabelKo: "웹", LabelEn: "Web", Enabled: true, Phase: 1},
		{ID: "image", LabelKo: "이미지", LabelEn: "Images", Enabled: false, Phase: 5, GateNote: "Phase 5 이미지 라이선스·썸네일·안전분류 파이프라인(051-052) 검증 전 비활성"},
		{ID: "news", LabelKo: "뉴스", LabelEn: "News", Enabled: false, Phase: 5, GateNote: "Phase 5 언론사 RSS/API 제휴 계약 및 원문 권리경계(054-055) 검증 전 비활성"},
		{ID: "video", LabelKo: "동영상", LabelEn: "Videos", Enabled: false, Phase: 5, GateNote: "Phase 5 동영상 메타·허가된 임베드 경로(053) 검증 전 비활성"},
		{ID: "map", LabelKo: "지도", LabelEn: "Maps", Enabled: false, Phase: 5, GateNote: "Phase 5 라이선스 지리 데이터 및 POI 정정 절차(056-057) 검증 전 비활성"},
		{ID: "shopping", LabelKo: "쇼핑", LabelEn: "Shopping", Enabled: false, Phase: 5, GateNote: "Phase 5 판매자 피드·가격 갱신·제휴 표기(058) 검증 전 비활성"},
	}
}

