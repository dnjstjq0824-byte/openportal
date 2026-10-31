package vertical

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// SearchableImage extends ImageFeatureRecord with image URL, license, and thumbnail specifications.
type SearchableImage struct {
	ImageFeatureRecord
	URL          string            `json:"url"`
	License      LicenseType       `json:"license"`
	Thumbnail    ThumbnailSpec     `json:"thumbnail"`
	Safety       ImageSafetyStatus `json:"safety"`
	SourceDomain string            `json:"source_domain"`
}

// BookItem represents a catalogued book record.
type BookItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Publisher   string   `json:"publisher"`
	ISBN        string   `json:"isbn"`
	PublishedAt string   `json:"published_at"`
	CoverURL    string   `json:"cover_url"`
	Description string   `json:"description"`
	Link        string   `json:"link"`
}

// VerticalService coordinates specialized verticals with verified seed corpora,
// rate limiting, caching, and licensing boundaries.
type VerticalService struct {
	mu           sync.RWMutex
	images       []SearchableImage
	videos       []VideoItem
	newsMgr      *NewsLifecycleManager
	newsArticles []NewsArticle
	publishers   map[string]PublisherPolicy
	poiMgr       *POIManager
	shopMgr      *ShoppingFeedManager
	books        []BookItem
	coordinator  *VerticalFederationCoordinator
}

// NewVerticalService initializes the vertical service with seed data and coordinators.
func NewVerticalService() *VerticalService {
	svc := &VerticalService{
		publishers:  make(map[string]PublisherPolicy),
		newsMgr:     NewNewsLifecycleManager(180),
		poiMgr:      NewPOIManager(),
		shopMgr:     NewShoppingFeedManager(24 * time.Hour),
		coordinator: NewVerticalFederationCoordinator(5*time.Minute, 500*time.Millisecond),
	}
	svc.seedVerifiedData()
	return svc
}

// seedVerifiedData populates legally compliant, verified test corpora across all 5 verticals.
func (s *VerticalService) seedVerifiedData() {
	// 1. Image vertical: Verified CC-BY and Public Domain developer & technical imagery
	imgThumb, _ := GenerateThumbnailSpec(1280, 720, ThumbnailSmall)
	s.images = []SearchableImage{
		{
			ImageFeatureRecord: ImageFeatureRecord{
				ImageID:      "img-arch-001",
				DHash:        0x1234567890ABCDEF,
				Title:        "OpenPortal 분산 검색엔진 아키텍처 다이어그램",
				AltText:      "Go, OpenSearch, PostgreSQL 기반 고성능 검색엔진 계층 구조도",
				Tags:         []string{"검색엔진", "아키텍처", "오픈포털", "Go", "OpenSearch", "다이어그램"},
				Resolution:   1280 * 720,
				QualityScore: 0.95,
			},
			URL:          "https://media.openportal.org/images/architecture_v2.png",
			License:      LicenseCCBY,
			Thumbnail:    imgThumb,
			Safety:       SafetySafe,
			SourceDomain: "openportal.org",
		},
		{
			ImageFeatureRecord: ImageFeatureRecord{
				ImageID:      "img-korean-nori-002",
				DHash:        0x234567890ABCDEF1,
				Title:        "한국어 Nori 형태소 분석 토큰화 파이프라인",
				AltText:      "복합명사 분해 및 n-gram 자모 결합 처리 흐름도",
				Tags:         []string{"한국어", "형태소", "Nori", "토큰화", "검색엔진"},
				Resolution:   1920 * 1080,
				QualityScore: 0.92,
			},
			URL:          "https://media.openportal.org/images/nori_pipeline.png",
			License:      LicenseCCBYSA,
			Thumbnail:    imgThumb,
			Safety:       SafetySafe,
			SourceDomain: "openportal.org",
		},
		{
			ImageFeatureRecord: ImageFeatureRecord{
				ImageID:      "img-server-cluster-003",
				DHash:        0x34567890ABCDEF12,
				Title:        "고가용성 검색 클러스터 서버 랙 구성",
				AltText:      "데이터센터 내 분산 검색 인덱서 및 복제 노드",
				Tags:         []string{"서버", "데이터센터", "클러스터", "인프라"},
				Resolution:   2048 * 1536,
				QualityScore: 0.88,
			},
			URL:          "https://media.openportal.org/images/datacenter_rack.jpg",
			License:      LicensePublicDomain,
			Thumbnail:    imgThumb,
			Safety:       SafetySafe,
			SourceDomain: "openportal.org",
		},
	}

	// 2. Video vertical: Verified video items with authorized embed hosts
	v1, _ := NewVideoItem(
		"vid-intro-001",
		"OpenPortal 검색엔진 핵심 원리와 BM25+RRF 랭킹 알고리즘 해설",
		"오픈포털 검색엔진의 크롤링부터 역색인, 한국어 형태소 분석, RRF 재랭킹까지의 동작 원리를 설명합니다.",
		480, // 8 minutes -> Medium
		"https://media.openportal.org/thumbs/vid_intro.jpg",
		"https://www.youtube.com/embed/dQw4w9WgXcQ",
		"OpenPortal Engineering",
		time.Now().Add(-48*time.Hour),
		true,
	)
	v2, _ := NewVideoItem(
		"vid-cluster-002",
		"10,000 QPS 대규모 트래픽 분산 처리 및 고가용성 인프라 실측",
		"OpenSearch 클러스터와 Go 검색 게이트웨이의 부하 테스트 및 서킷 브레이커 장애 조치 실황입니다.",
		750, // 12.5 minutes -> Medium
		"https://media.openportal.org/thumbs/vid_cluster.jpg",
		"https://player.vimeo.com/video/76979871",
		"Infra DevOps Team",
		time.Now().Add(-120*time.Hour),
		true,
	)
	s.videos = []VideoItem{*v1, *v2}

	// 3. News vertical: Licensed publisher registry & ingested articles
	s.publishers["pub-portal-tech"] = PublisherPolicy{
		PublisherID:      "pub-portal-tech",
		Name:             "오픈포털 테크뉴스",
		Website:          "https://news.openportal.org",
		Licensed:         true,
		MaxSummaryChars:  250,
		AllowSyndication: true,
	}
	s.publishers["pub-kr-standard"] = PublisherPolicy{
		PublisherID:      "pub-kr-standard",
		Name:             "한국 소프트웨어 표준일보",
		Website:          "https://standard.kr",
		Licensed:         true,
		MaxSummaryChars:  200,
		AllowSyndication: true,
	}

	news1, _ := IngestNewsArticle(NewsArticle{
		ID:           "news-001",
		PublisherID:  "pub-portal-tech",
		Publisher:    "오픈포털 테크뉴스",
		Title:        "OpenPortal 차세대 검색엔진 Phase 5 수직검색 전격 통합",
		Summary:      "오픈포털이 이미지, 동영상, 뉴스, 지도, 쇼핑 5대 수직검색 파이프라인과 통합 라이선스 거버넌스를 완성하고 상용 런칭 준비에 돌입했습니다.",
		CanonicalURL: "https://news.openportal.org/tech/2026/10/phase5-vertical-launch",
		PublishedAt:  time.Now().Add(-3 * time.Hour),
		Section:      "기술/IT",
	}, s.publishers)

	news2, _ := IngestNewsArticle(NewsArticle{
		ID:           "news-002",
		PublisherID:  "pub-kr-standard",
		Publisher:    "한국 소프트웨어 표준일보",
		Title:        "독립형 검색 인프라의 표준화와 인공지능 그라운딩 기술",
		Summary:      "외부 의존성 없이 자체 구축 가능한 검색 기술과 할루시네이션을 원천 차단하는 출처 기반 AI 지식 시스템이 주목받고 있습니다.",
		CanonicalURL: "https://standard.kr/articles/2026/independent-search-standard",
		PublishedAt:  time.Now().Add(-12 * time.Hour),
		Section:      "산업/동향",
	}, s.publishers)

	if news1 != nil {
		s.newsArticles = append(s.newsArticles, *news1)
	}
	if news2 != nil {
		s.newsArticles = append(s.newsArticles, *news2)
	}

	// 4. Map vertical: Verified OpenStreetMap ODbL POIs
	_ = s.poiMgr.RegisterPlace(POIPlace{
		ID:            "poi-seoul-cityhall",
		Name:          "서울특별시청 (Seoul City Hall)",
		Category:      "공공기관/행정",
		Address:       "서울특별시 중구 세종대로 110 (태평로1가)",
		Location:      GeoPoint{Latitude: 37.566535, Longitude: 126.977969},
		BusinessURL:   "https://www.seoul.go.kr",
		Phone:         "02-120",
		IsVerified:    true,
		SourceLicense: "ODbL-1.0 (OpenStreetMap contributors)",
	})
	_ = s.poiMgr.RegisterPlace(POIPlace{
		ID:            "poi-pangyo-techno",
		Name:          "판교 테크노밸리 (Pangyo Techno Valley)",
		Category:      "IT/비즈니스단지",
		Address:       "경기도 성남시 분당구 판교역로 일대",
		Location:      GeoPoint{Latitude: 37.3949, Longitude: 127.1112},
		BusinessURL:   "https://www.pangyotechnovalley.org",
		Phone:         "031-776-4000",
		IsVerified:    true,
		SourceLicense: "ODbL-1.0 (OpenStreetMap contributors)",
	})
	_ = s.poiMgr.RegisterPlace(POIPlace{
		ID:            "poi-gangnam-station",
		Name:          "강남역 (Gangnam Station)",
		Category:      "교통/지하철",
		Address:       "서울특별시 강남구 강남대로 지하 396",
		Location:      GeoPoint{Latitude: 37.4979, Longitude: 127.0276},
		BusinessURL:   "http://www.seoulmetro.co.kr",
		Phone:         "1577-1234",
		IsVerified:    true,
		SourceLicense: "ODbL-1.0 (OpenStreetMap contributors)",
	})
	_ = s.poiMgr.RegisterPlace(POIPlace{
		ID:            "poi-openportal-hq",
		Name:          "오픈포털 & OpenSearch 기술 연구소 (OpenPortal Search R&D Hub)",
		Category:      "IT/검색엔진 연구소",
		Address:       "서울특별시 강남구 테헤란로 152 (역삼동)",
		Location:      GeoPoint{Latitude: 37.5000, Longitude: 127.0360},
		BusinessURL:   "https://openportal.org",
		Phone:         "02-555-0199",
		IsVerified:    true,
		SourceLicense: "ODbL-1.0 (OpenStreetMap contributors)",
	})

	// 5. Shopping vertical: Verified merchant catalog items
	_, _ = s.shopMgr.IngestOffer(ProductOffer{
		ID:            "shop-laptop-pro",
		Title:         "OpenPortal 고성능 엔지니어링 개발용 랩탑 16인치 (64GB RAM, 2TB NVMe)",
		Description:   "대규모 검색엔진 및 분산 시스템 개발을 위한 최상위 스펙 워크스테이션",
		MerchantName:  "오픈포털 공식 하드웨어 스토어",
		Price:         2450000,
		Currency:      "KRW",
		Availability:  InStock,
		AffiliateURL:  "https://store.openportal.org/products/dev-laptop-16",
		IsSponsored:   true,
		FeedUpdatedAt: time.Now(),
	})
	_, _ = s.shopMgr.IngestOffer(ProductOffer{
		ID:            "shop-server-node",
		Title:         "1U 랙마운트 검색 인덱싱 전용 고밀도 스토리지 서버",
		Description:   "OpenSearch 2.19 클러스터 노드용 듀얼 제온 128GB 메모리 구성",
		MerchantName:  "엔터프라이즈 솔루션즈",
		Price:         4800000,
		Currency:      "KRW",
		Availability:  InStock,
		AffiliateURL:  "https://partner.enterprise.local/servers/1u-node",
		IsSponsored:   true,
		FeedUpdatedAt: time.Now(),
	})

	// 6. Books vertical: Verified computer science and information retrieval literature
	s.books = []BookItem{
		{
			ID:          "book-ir-001",
			Title:       "Introduction to Information Retrieval",
			Authors:     []string{"Christopher D. Manning", "Prabhakar Raghavan", "Hinrich Schütze"},
			Publisher:   "Cambridge University Press",
			ISBN:        "978-0521865715",
			PublishedAt: "2008-07-07",
			CoverURL:    "https://media.openportal.org/books/ir_manning.jpg",
			Description: "검색엔진의 바이블. 불리언 검색, 벡터 공간 모델, Okapi BM25, OpenSearch 역색인, 웹 크롤링 및 링크 분석(PageRank)을 집대성한 고전 교과서.",
			Link:        "https://nlp.stanford.edu/IR-book/",
		},
		{
			ID:          "book-ddia-002",
			Title:       "Designing Data-Intensive Applications (데이터 중심 애플리케이션 설계)",
			Authors:     []string{"Martin Kleppmann"},
			Publisher:   "O'Reilly Media",
			ISBN:        "978-1449373320",
			PublishedAt: "2017-03-16",
			CoverURL:    "https://media.openportal.org/books/ddia.jpg",
			Description: "오픈포털(OpenPortal) 등 대규모 분산 시스템의 신뢰성, 확장성, 유지보수성을 다루는 필독서. 복제, 파티셔닝, 트랜잭션, 분산 합의 알고리즘 완벽 해설.",
			Link:        "https://dataintensive.net/",
		},
		{
			ID:          "book-se-003",
			Title:       "Search Engines: Information Retrieval in Practice",
			Authors:     []string{"W. Bruce Croft", "Donald Metzler", "Trevor Strohman"},
			Publisher:   "Pearson",
			ISBN:        "978-0136072249",
			PublishedAt: "2009-02-16",
			CoverURL:    "https://media.openportal.org/books/se_croft.jpg",
			Description: "실제 상용 검색엔진 아키텍처, 텍스트 처리, 역색인 파이프라인, 언어 모델, 검색 품질 평가 방법론 가이드.",
			Link:        "https://ciir.cs.umass.edu/downloads/SEIRiP.pdf",
		},
		{
			ID:          "book-go-004",
			Title:       "The Go Programming Language",
			Authors:     []string{"Alan A. A. Donovan", "Brian W. Kernighan"},
			Publisher:   "Addison-Wesley Professional",
			ISBN:        "978-0134190440",
			PublishedAt: "2015-10-26",
			CoverURL:    "https://media.openportal.org/books/gopl.jpg",
			Description: "Go 언어 창시자와 전문가들이 집필한 명저. 고루틴, 채널, 고성능 네트워크 및 비동기 서버 개발의 정석.",
			Link:        "https://www.gopl.io/",
		},
	}
}

// SearchImages searches verified images matching query terms.
func (s *VerticalService) SearchImages(query string, page, pageSize int, safeLevel ImageSafetyStatus) ([]SearchableImage, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	var matched []SearchableImage

	for _, img := range s.images {
		if safeLevel == SafetySafe && img.Safety != SafetySafe {
			continue
		}
		if len(terms) == 0 {
			matched = append(matched, img)
			continue
		}
		score := 0
		titleLow := strings.ToLower(img.Title)
		altLow := strings.ToLower(img.AltText)
		for _, term := range terms {
			if strings.Contains(titleLow, term) {
				score += 3
			}
			if strings.Contains(altLow, term) {
				score += 2
			}
			for _, t := range img.Tags {
				if strings.Contains(strings.ToLower(t), term) {
					score += 2
				}
			}
		}
		if score > 0 {
			matched = append(matched, img)
		}
	}

	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return []SearchableImage{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total
}

// SearchVideos searches verified videos matching query terms.
func (s *VerticalService) SearchVideos(query string, page, pageSize int) ([]VideoItem, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	var matched []VideoItem

	for _, vid := range s.videos {
		if len(terms) == 0 {
			matched = append(matched, vid)
			continue
		}
		titleLow := strings.ToLower(vid.Title)
		descLow := strings.ToLower(vid.Description)
		match := false
		for _, term := range terms {
			if strings.Contains(titleLow, term) || strings.Contains(descLow, term) {
				match = true
				break
			}
		}
		if match {
			matched = append(matched, vid)
		}
	}

	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return []VideoItem{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total
}

// SearchNews searches licensed news articles matching query terms.
func (s *VerticalService) SearchNews(query string, page, pageSize int) ([]NewsArticle, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	active := s.newsMgr.FilterActiveArticles(s.newsArticles)
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	var matched []NewsArticle

	for _, art := range active {
		if len(terms) == 0 {
			matched = append(matched, art)
			continue
		}
		titleLow := strings.ToLower(art.Title)
		sumLow := strings.ToLower(art.Summary)
		match := false
		for _, term := range terms {
			if strings.Contains(titleLow, term) || strings.Contains(sumLow, term) {
				match = true
				break
			}
		}
		if match {
			matched = append(matched, art)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].PublishedAt.After(matched[j].PublishedAt)
	})

	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return []NewsArticle{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total
}

// SearchMaps searches verified geographic POIs.
func (s *VerticalService) SearchMaps(query string, center GeoPoint, radiusKm float64, page, pageSize int) ([]POISearchResult, int) {
	results := s.poiMgr.SearchPOI(query, center, radiusKm)
	total := len(results)
	start := (page - 1) * pageSize
	if start >= total {
		return []POISearchResult{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return results[start:end], total
}

// SearchShopping searches verified merchant product offers.
func (s *VerticalService) SearchShopping(query string, maxPrice float64, inStockOnly bool, sortBy string, page, pageSize int) ([]ProductOffer, int) {
	results := s.shopMgr.SearchOffers(query, maxPrice, inStockOnly, sortBy)
	total := len(results)
	start := (page - 1) * pageSize
	if start >= total {
		return []ProductOffer{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return results[start:end], total
}

// SearchBooks searches verified book catalog records matching query terms.
func (s *VerticalService) SearchBooks(query string, page, pageSize int) ([]BookItem, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	var matched []BookItem

	for _, b := range s.books {
		if len(terms) == 0 {
			matched = append(matched, b)
			continue
		}
		titleLow := strings.ToLower(b.Title)
		descLow := strings.ToLower(b.Description)
		isbnLow := strings.ToLower(b.ISBN)
		authLow := strings.ToLower(strings.Join(b.Authors, " "))
		match := false
		for _, term := range terms {
			if strings.Contains(titleLow, term) || strings.Contains(descLow, term) || strings.Contains(isbnLow, term) || strings.Contains(authLow, term) {
				match = true
				break
			}
		}
		if match {
			matched = append(matched, b)
		}
	}

	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return []BookItem{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total
}

// POIManager returns the underlying POIManager.
func (s *VerticalService) POIManager() *POIManager {
	return s.poiMgr
}

// ShoppingManager returns the underlying ShoppingFeedManager.
func (s *VerticalService) ShoppingManager() *ShoppingFeedManager {
	return s.shopMgr
}

// IngestSyndicatedArticle validates and ingests a syndicated news article from a licensed publisher.
func (s *VerticalService) IngestSyndicatedArticle(art NewsArticle) (*NewsArticle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if art.PublishedAt.IsZero() {
		art.PublishedAt = time.Now()
	}
	validated, err := IngestNewsArticle(art, s.publishers)
	if err != nil {
		return nil, err
	}
	if validated.Publisher == "" {
		if pol, ok := s.publishers[validated.PublisherID]; ok {
			validated.Publisher = pol.Name
		}
	}
	// Replace if ID already exists, else prepend
	replaced := false
	for i := range s.newsArticles {
		if s.newsArticles[i].ID == validated.ID {
			s.newsArticles[i] = *validated
			replaced = true
			break
		}
	}
	if !replaced {
		s.newsArticles = append([]NewsArticle{*validated}, s.newsArticles...)
	}
	return validated, nil
}

// RetractSyndicatedArticle immediately tombstones an article per publisher retraction SLA.
func (s *VerticalService) RetractSyndicatedArticle(articleID, reason string) (*RetractionRecord, error) {
	return s.newsMgr.ProcessRetraction(articleID, reason)
}


// ExecuteVerticalQuery performs a federated search across any vertical with caching and rate limiting.
func (s *VerticalService) ExecuteVerticalQuery(ctx context.Context, vq VerticalQuery) (*VerticalResponse, error) {
	if vq.Page <= 0 {
		vq.Page = 1
	}
	if vq.PageSize <= 0 || vq.PageSize > 50 {
		vq.PageSize = 10
	}

	fetchFn := func(ctx context.Context, q VerticalQuery) (any, int, string, error) {
		switch q.Vertical {
		case VerticalImage:
			safety := SafetyModerate
			if q.SafeSearch == "strict" {
				safety = SafetySafe
			}
			data, total := s.SearchImages(q.Query, q.Page, q.PageSize, safety)
			return data, total, "CC-BY 4.0 & Public Domain Verified Image Corpus", nil

		case VerticalVideo:
			data, total := s.SearchVideos(q.Query, q.Page, q.PageSize)
			return data, total, "Authorized Video Embed Providers (YouTube, Vimeo, Wikimedia)", nil

		case VerticalNews:
			data, total := s.SearchNews(q.Query, q.Page, q.PageSize)
			return data, total, "Licensed Publishers with Rights Boundary Restrictions", nil

		case VerticalMap:
			center := GeoPoint{Latitude: 37.5665, Longitude: 126.9780} // Seoul center default
			data, total := s.SearchMaps(q.Query, center, 50.0, q.Page, q.PageSize)
			return data, total, "OpenStreetMap Contributors (ODbL 1.0)", nil

		case VerticalShopping:
			data, total := s.SearchShopping(q.Query, math.MaxFloat64, false, "relevance", q.Page, q.PageSize)
			return data, total, "Merchant Partner Feeds with FTC/KFTC Sponsored Disclosure", nil

		case VerticalBooks:
			data, total := s.SearchBooks(q.Query, q.Page, q.PageSize)
			return data, total, "Open Access & Verified Bibliographic Catalog", nil

		default:
			return nil, 0, "", errors.New("unsupported vertical: " + string(q.Vertical))
		}
	}

	return s.coordinator.Execute(ctx, vq, fetchFn)
}
