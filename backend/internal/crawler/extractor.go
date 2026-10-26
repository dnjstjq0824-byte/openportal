package crawler

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// DocumentDirectives encapsulates indexing and following directives extracted from HTTP headers and HTML markup.
type DocumentDirectives struct {
	NoIndex     bool   `json:"no_index"`
	NoFollow    bool   `json:"no_follow"`
	NoArchive   bool   `json:"no_archive"`
	NoSnippet   bool   `json:"no_snippet"`
	Canonical   string `json:"canonical,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// DiscoveredLink represents an extracted hyperlink from a crawled document.
type DiscoveredLink struct {
	RawURL       string `json:"raw_url"`
	CanonicalURL string `json:"canonical_url"`
	AnchorText   string `json:"anchor_text"`
	NoFollow     bool   `json:"no_follow"`
}

// ParseXRobotsTag extracts directives from the X-Robots-Tag HTTP header (RFC 9309 / search standards).
func ParseXRobotsTag(header http.Header) DocumentDirectives {
	var dir DocumentDirectives
	tags := header.Values("X-Robots-Tag")
	for _, tag := range tags {
		parts := strings.Split(tag, ",")
		for _, part := range parts {
			val := strings.ToLower(strings.TrimSpace(part))
			switch val {
			case "none":
				dir.NoIndex = true
				dir.NoFollow = true
			case "noindex":
				dir.NoIndex = true
			case "nofollow":
				dir.NoFollow = true
			case "noarchive":
				dir.NoArchive = true
			case "nosnippet":
				dir.NoSnippet = true
			}
		}
	}
	return dir
}

// ParseHTMLDirectives parses an HTML document tree to extract meta robots, canonical link, and title/description.
func ParseHTMLDirectives(docBytes []byte, baseURL string, initial DocumentDirectives) (DocumentDirectives, []DiscoveredLink, error) {
	dir := initial
	base, err := url.Parse(baseURL)
	if err != nil {
		return dir, nil, err
	}

	doc, err := html.Parse(bytes.NewReader(docBytes))
	if err != nil {
		return dir, nil, err
	}

	var links []DiscoveredLink

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "title":
				if dir.Title == "" && n.FirstChild != nil {
					dir.Title = strings.TrimSpace(n.FirstChild.Data)
				}

			case "meta":
				var name, property, content string
				for _, attr := range n.Attr {
					k := strings.ToLower(attr.Key)
					switch k {
					case "name":
						name = strings.ToLower(attr.Val)
					case "property":
						property = strings.ToLower(attr.Val)
					case "content":
						content = attr.Val
					}
				}

				if name == "robots" || name == "openportalbot" {
					parts := strings.Split(content, ",")
					for _, p := range parts {
						val := strings.ToLower(strings.TrimSpace(p))
						switch val {
						case "none":
							dir.NoIndex = true
							dir.NoFollow = true
						case "noindex":
							dir.NoIndex = true
						case "nofollow":
							dir.NoFollow = true
						case "noarchive":
							dir.NoArchive = true
						case "nosnippet":
							dir.NoSnippet = true
						}
					}
				} else if (name == "description" || property == "og:description") && dir.Description == "" {
					dir.Description = strings.TrimSpace(content)
				}

			case "link":
				var rel, href string
				for _, attr := range n.Attr {
					switch strings.ToLower(attr.Key) {
					case "rel":
						rel = strings.ToLower(attr.Val)
					case "href":
						href = strings.TrimSpace(attr.Val)
					}
				}
				if rel == "canonical" && href != "" {
					// Resolve relative canonical against base URL
					if canonicalU, err := base.Parse(href); err == nil {
						if cu, err := Canonicalize(canonicalU.String()); err == nil {
							dir.Canonical = cu.Canonical
						}
					}
				}

			case "a":
				var href, rel string
				for _, attr := range n.Attr {
					switch strings.ToLower(attr.Key) {
					case "href":
						href = strings.TrimSpace(attr.Val)
					case "rel":
						rel = strings.ToLower(attr.Val)
					}
				}
				if href != "" && !strings.HasPrefix(href, "javascript:") && !strings.HasPrefix(href, "mailto:") && !strings.HasPrefix(href, "#") {
					anchorText := extractText(n)
					isNoFollow := dir.NoFollow || strings.Contains(rel, "nofollow")
					if resolved, err := base.Parse(href); err == nil {
						if cu, err := Canonicalize(resolved.String()); err == nil {
							links = append(links, DiscoveredLink{
								RawURL:       href,
								CanonicalURL: cu.Canonical,
								AnchorText:   strings.TrimSpace(anchorText),
								NoFollow:     isNoFollow,
							})
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return dir, links, nil
}

// extractText extracts the visible text inside an HTML element node.
func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(curr *html.Node) {
		if curr.Type == html.TextNode {
			sb.WriteString(curr.Data)
			sb.WriteString(" ")
		}
		for c := curr.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(sb.String()), " ")
}

// XML Sitemap models (RFC / sitemaps.org schema)
type SitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []SitemapURL `xml:"url"`
}

type SitemapURL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod"`
	ChangeFreq string  `xml:"changefreq"`
	Priority   float64 `xml:"priority"`
}

type SitemapIndex struct {
	XMLName  xml.Name       `xml:"sitemapindex"`
	Sitemaps []SitemapEntry `xml:"sitemap"`
}

type SitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// ParseSitemapXML parses XML sitemap content into a list of URLs or child sitemap index URLs.
func ParseSitemapXML(xmlData []byte) ([]SitemapURL, []string, error) {
	// Try parsing as urlset first
	var urlSet SitemapURLSet
	if err := xml.Unmarshal(xmlData, &urlSet); err == nil && len(urlSet.URLs) > 0 {
		return urlSet.URLs, nil, nil
	}

	// Try parsing as sitemapindex
	var sitemapIndex SitemapIndex
	if err := xml.Unmarshal(xmlData, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		var childSitemaps []string
		for _, s := range sitemapIndex.Sitemaps {
			if strings.TrimSpace(s.Loc) != "" {
				childSitemaps = append(childSitemaps, strings.TrimSpace(s.Loc))
			}
		}
		return nil, childSitemaps, nil
	}

	return nil, nil, nil
}

