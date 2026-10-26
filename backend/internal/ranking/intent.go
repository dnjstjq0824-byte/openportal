package ranking

import (
	"strings"
	"unicode"
)

type IntentType string

const (
	IntentInformational IntentType = "informational"
	IntentNavigational  IntentType = "navigational"
	IntentTransactional IntentType = "transactional"
	IntentLocal         IntentType = "local"
)

type QueryIntent struct {
	Intent          IntentType `json:"intent"`
	Confidence      float64    `json:"confidence"`
	Language        string     `json:"language"`
	Region          string     `json:"region"`
	Entities        []string   `json:"entities,omitempty"`
	NeedsCorrection bool       `json:"needs_correction"`
	CorrectedQuery  string     `json:"corrected_query,omitempty"`
}

type IntentClassifier struct{}

func NewIntentClassifier() *IntentClassifier {
	return &IntentClassifier{}
}

var (
	navigationalSuffixes = []string{".com", ".net", ".org", ".kr", ".co.kr", ".go.kr", ".io", ".dev", ".ai"}
	navigationalKeywords = []string{"홈페이지", "공식", "사이트", "웹사이트", "포털", "로그인", "바로가기", "포탈", "login", "official", "portal"}

	transactionalKeywords = []string{
		"구매", "가격", "예약", "할인", "쿠폰", "쇼핑", "주문", "최저가", "다운로드", "판매", "결제",
		"buy", "price", "order", "cheap", "ticket", "download", "sale", "discount", "reserve",
	}

	localKeywords = []string{
		"서울", "강남", "홍대", "종로", "부산", "대구", "인천", "광주", "대전", "제주", "수원", "성남", "분당",
		"맛집", "카페", "병원", "약국", "위치", "지도", "근처", "역", "주변", "호텔",
		"restaurant", "cafe", "hotel", "near me", "location", "map", "clinic",
	}

	informationalKeywords = []string{
		"어떻게", "방법", "원인", "이유", "뜻", "의미", "위키", "설명", "역사", "증상", "차이", "원리",
		"how to", "what is", "why", "who", "definition", "guide", "tutorial", "meaning",
	}

	englishTypoToKorean = map[string]string{
		"gksrmf":   "한글",
		"dhvms":    "오픈",
		"dhvmsvhxjf": "오픈포털",
		"qjadnl":   "범위",
		"rjatordpswls": "검색엔진",
	}

	commonTypos = map[string]string{
		"오푼포털":  "오픈포털",
		"검색엔진ㄴ": "검색엔진",
		"구글검색":  "구글 검색",
		"인덱싱":   "인덱싱",
		"opensearc": "opensearch",
	}
)

// Classify determines the primary query intent, language, and regional preferences.
func (c *IntentClassifier) Classify(query, userLang, userRegion string) QueryIntent {
	q := strings.TrimSpace(strings.ToLower(query))
	lang := userLang
	if lang == "" {
		lang = detectQueryLanguage(q)
	}
	region := strings.ToUpper(userRegion)
	if region == "" {
		if lang == "ko" {
			region = "KR"
		} else {
			region = "US"
		}
	}

	res := QueryIntent{
		Intent:     IntentInformational,
		Confidence: 0.60,
		Language:   lang,
		Region:     region,
	}

	// 1. Check Navigational
	for _, sfx := range navigationalSuffixes {
		if strings.Contains(q, sfx) {
			res.Intent = IntentNavigational
			res.Confidence = 0.95
			res.Entities = append(res.Entities, sfx)
			return res
		}
	}
	for _, kw := range navigationalKeywords {
		if strings.Contains(q, kw) {
			res.Intent = IntentNavigational
			res.Confidence = 0.88
			res.Entities = append(res.Entities, kw)
			return res
		}
	}

	// 2. Check Local Intent
	var matchedLocal []string
	for _, kw := range localKeywords {
		if strings.Contains(q, kw) {
			matchedLocal = append(matchedLocal, kw)
		}
	}
	if len(matchedLocal) > 0 {
		res.Intent = IntentLocal
		res.Confidence = 0.75 + float64(len(matchedLocal))*0.1
		if res.Confidence > 0.95 {
			res.Confidence = 0.95
		}
		res.Entities = matchedLocal
		return res
	}

	// 3. Check Transactional Intent
	var matchedTrans []string
	for _, kw := range transactionalKeywords {
		if strings.Contains(q, kw) {
			matchedTrans = append(matchedTrans, kw)
		}
	}
	if len(matchedTrans) > 0 {
		res.Intent = IntentTransactional
		res.Confidence = 0.80 + float64(len(matchedTrans))*0.08
		if res.Confidence > 0.95 {
			res.Confidence = 0.95
		}
		res.Entities = matchedTrans
		return res
	}

	// 4. Check Informational
	for _, kw := range informationalKeywords {
		if strings.Contains(q, kw) {
			res.Intent = IntentInformational
			res.Confidence = 0.85
			res.Entities = append(res.Entities, kw)
			return res
		}
	}

	return res
}

// ApplyCorrection provides opt-in spell/keyboard mistake correction.
// When optIn is false, returns original query and false.
func (c *IntentClassifier) ApplyCorrection(query string, optIn bool) (string, bool) {
	if !optIn || strings.TrimSpace(query) == "" {
		return query, false
	}
	raw := strings.TrimSpace(query)
	lower := strings.ToLower(raw)

	if fixed, ok := englishTypoToKorean[lower]; ok {
		return fixed, true
	}
	if fixed, ok := commonTypos[lower]; ok {
		return fixed, true
	}

	// Remove trailing single jamo if present (e.g. "검색ㄱ" -> "검색")
	if len([]rune(raw)) > 1 {
		runes := []rune(raw)
		last := runes[len(runes)-1]
		if (last >= 0x3131 && last <= 0x3163) || (last >= 0x3165 && last <= 0x318E) {
			return string(runes[:len(runes)-1]), true
		}
	}

	return query, false
}

func detectQueryLanguage(text string) string {
	hangulCount := 0
	latinCount := 0
	for _, r := range text {
		if (r >= 0xAC00 && r <= 0xD7AF) || (r >= 0x1100 && r <= 0x11FF) || (r >= 0x3130 && r <= 0x318F) {
			hangulCount++
		} else if unicode.Is(unicode.Latin, r) {
			latinCount++
		}
	}
	if hangulCount > 0 {
		return "ko"
	}
	if latinCount > 0 {
		return "en"
	}
	return "ko"
}
