package indexer

import (
	"strings"
	"testing"
)

func TestBoilerplateRemovalAndExtraction(t *testing.T) {
	rawHTML := `<!DOCTYPE html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <title>오픈포털 - 검색엔진 기술 문서</title>
  <meta name="description" content="오픈포털 독립 검색엔진 아키텍처 및 색인 파이프라인 가이드">
  <link rel="canonical" href="https://example.org/docs/indexing">
  <style>body { background: #fff; } .ad-banner { display: block; }</style>
  <script>console.log("analytics tracker");</script>
</head>
<body>
  <header>
    <nav role="navigation">
      <a href="/home">홈</a>
      <a href="/about">소개</a>
    </nav>
  </header>

  <aside class="sidebar">
    <div class="ad-container">스폰서 광고 링크</div>
  </aside>

  <main role="main">
    <h1>오픈포털 분산 색인 시스템</h1>
    <p>오픈포털은 자체 크롤러가 수집한 웹 문서를 분석하여 역색인을 생성합니다.</p>
    <h2>형태소 분석 및 중복 제거</h2>
    <p>한국어 텍스트는 Lucene Nori 형태소 분석기를 통해 색인어를 추출하고, SimHash로 유사 중복을 필터링합니다.</p>
    <a href="https://example.org/docs/crawler">크롤러 상세 명세</a>
    <a href="/docs/ranking">랭킹 모델</a>
  </main>

  <footer role="contentinfo">
    <p class="site-footer">Copyright 2026 OpenPortal. All rights reserved.</p>
  </footer>
</body>
</html>`

	remover := NewBoilerplateRemover()
	doc, err := remover.Extract([]byte(rawHTML), "https://example.org/docs/indexing")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 1. Verify Title & Description
	if doc.Title != "오픈포털 - 검색엔진 기술 문서" {
		t.Errorf("unexpected Title: %q", doc.Title)
	}
	if doc.Description != "오픈포털 독립 검색엔진 아키텍처 및 색인 파이프라인 가이드" {
		t.Errorf("unexpected Description: %q", doc.Description)
	}

	// 2. Verify Canonical URL
	if doc.CanonicalURL != "https://example.org/docs/indexing" {
		t.Errorf("unexpected CanonicalURL: %q", doc.CanonicalURL)
	}

	// 3. Verify Headings
	if len(doc.Headings) != 2 {
		t.Fatalf("expected 2 headings, got %d: %v", len(doc.Headings), doc.Headings)
	}
	if doc.Headings[0] != "오픈포털 분산 색인 시스템" {
		t.Errorf("unexpected heading 0: %q", doc.Headings[0])
	}
	if doc.Headings[1] != "형태소 분석 및 중복 제거" {
		t.Errorf("unexpected heading 1: %q", doc.Headings[1])
	}

	// 4. Verify Boilerplate pruned from Body
	if strings.Contains(doc.Body, "analytics tracker") {
		t.Errorf("Body contains pruned script content")
	}
	if strings.Contains(doc.Body, "스폰서 광고 링크") {
		t.Errorf("Body contains pruned ad sidebar content")
	}
	if strings.Contains(doc.Body, "Copyright 2026") {
		t.Errorf("Body contains pruned footer content")
	}
	if !strings.Contains(doc.Body, "오픈포털은 자체 크롤러가 수집한") {
		t.Errorf("Body missing primary paragraph: %q", doc.Body)
	}

	// 5. Verify Links resolved
	if len(doc.Outlinks) < 2 {
		t.Fatalf("expected at least 2 outlinks, got %d: %v", len(doc.Outlinks), doc.Outlinks)
	}
	if doc.Outlinks[0] != "https://example.org/docs/crawler" {
		t.Errorf("unexpected link 0: %q", doc.Outlinks[0])
	}
	if doc.Outlinks[1] != "https://example.org/docs/ranking" {
		t.Errorf("unexpected relative link resolution: %q", doc.Outlinks[1])
	}

	// 6. Verify Language
	if doc.Language != "ko" {
		t.Errorf("expected language 'ko', got %q", doc.Language)
	}
}

func TestLanguageDetection(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"오픈포털 독립 검색엔진 프로젝트 개발 가이드", "ko"},
		{"OpenPortal full text search engine written in Go and Vue", "en"},
		{"안녕하세요 Hello World 한국어 우선 검색", "ko"},
		{"", "en"},
		{"1234567890 !@#$%", "en"},
	}

	for _, tt := range tests {
		got := DetectLanguage(tt.text)
		if got != tt.expected {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.text, got, tt.expected)
		}
	}
}

