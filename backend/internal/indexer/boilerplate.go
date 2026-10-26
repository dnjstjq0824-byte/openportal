package indexer

import (
	"bytes"
	"io"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ExtractedDoc contains cleaned document text and metadata after boilerplate removal.
type ExtractedDoc struct {
	Title        string
	Description  string
	CanonicalURL string
	Language     string
	Headings     []string
	Body         string
	AnchorTexts  []string
	Outlinks     []string
	SimHash      uint64
	ContentHash  string
	WordCount    int
}

// BoilerplateRemover strips scripts, styles, navigations, footers, headers, ads,
// and extracts main readable textual content with high text-to-tag density.
type BoilerplateRemover struct {
	minTextLength int
}

// NewBoilerplateRemover constructs a new boilerplate remover.
func NewBoilerplateRemover() *BoilerplateRemover {
	return &BoilerplateRemover{minTextLength: 3}
}

// Extract parses raw HTML, removes non-content elements, extracts title, description,
// headings, clean body, and links.
func (b *BoilerplateRemover) Extract(rawHTML []byte, baseURL string) (*ExtractedDoc, error) {
	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		return nil, err
	}

	result := &ExtractedDoc{}
	var bodyBuf strings.Builder
	var headings []string
	var outlinks []string
	var anchorTexts []string

	var baseParsed *url.URL
	if baseURL != "" {
		baseParsed, _ = url.Parse(baseURL)
	}

	// 1. First pass: extract metadata from <head> and filter unwanted tags
	var walkMeta func(*html.Node)
	walkMeta = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Title:
				if result.Title == "" {
					result.Title = cleanText(extractNodeText(n))
				}
			case atom.Meta:
				name := getAttr(n, "name")
				prop := getAttr(n, "property")
				content := getAttr(n, "content")
				if strings.EqualFold(name, "description") || strings.EqualFold(prop, "og:description") {
					if result.Description == "" {
						result.Description = cleanText(content)
					}
				}
			case atom.Link:
				rel := getAttr(n, "rel")
				href := getAttr(n, "href")
				if strings.EqualFold(rel, "canonical") && href != "" {
					if baseParsed != nil {
						if resolved, err := baseParsed.Parse(href); err == nil {
							result.CanonicalURL = resolved.String()
						}
					}
					if result.CanonicalURL == "" {
						result.CanonicalURL = href
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkMeta(c)
		}
	}
	walkMeta(doc)

	// 2. Second pass: remove boilerplate nodes in place
	b.pruneBoilerplate(doc)

	// 3. Third pass: collect headings, paragraphs, main content and anchor texts
	var walkContent func(*html.Node)
	walkContent = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.H1, atom.H2, atom.H3:
				text := cleanText(extractNodeText(n))
				if text != "" {
					headings = append(headings, text)
					if bodyBuf.Len() > 0 {
						bodyBuf.WriteString("\n")
					}
					bodyBuf.WriteString(text)
					bodyBuf.WriteString("\n")
				}
				return // Don't traverse children twice
			case atom.A:
				href := getAttr(n, "href")
				text := cleanText(extractNodeText(n))
				if href != "" && !strings.HasPrefix(href, "javascript:") && !strings.HasPrefix(href, "#") {
					if baseParsed != nil {
						if resolved, err := baseParsed.Parse(href); err == nil {
							outlinks = append(outlinks, resolved.String())
						}
					} else {
						outlinks = append(outlinks, href)
					}
					if text != "" {
						anchorTexts = append(anchorTexts, text)
					}
				}
			case atom.P, atom.Blockquote, atom.Li, atom.Article, atom.Section:
				text := cleanText(extractNodeText(n))
				if len(text) >= b.minTextLength {
					if bodyBuf.Len() > 0 {
						bodyBuf.WriteString("\n")
					}
					bodyBuf.WriteString(text)
				}
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkContent(c)
		}
	}
	walkContent(doc)

	cleanBody := strings.TrimSpace(bodyBuf.String())
	if cleanBody == "" {
		// Fallback to all remaining text
		cleanBody = cleanText(extractNodeText(doc))
	}

	result.Headings = headings
	result.Body = cleanBody
	result.AnchorTexts = anchorTexts
	result.Outlinks = outlinks
	result.WordCount = len(strings.Fields(cleanBody))
	result.Language = DetectLanguage(result.Title + " " + cleanBody)

	return result, nil
}

// pruneBoilerplate removes scripts, styles, navs, headers, footers, asides, and noisy elements.
func (b *BoilerplateRemover) pruneBoilerplate(n *html.Node) {
	var toRemove []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b.isBoilerplateNode(c) {
			toRemove = append(toRemove, c)
		} else {
			b.pruneBoilerplate(c)
		}
	}
	for _, target := range toRemove {
		n.RemoveChild(target)
	}
}

func (b *BoilerplateRemover) isBoilerplateNode(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	// Unwanted tags
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Template, atom.Iframe,
		atom.Object, atom.Embed, atom.Svg, atom.Canvas, atom.Audio, atom.Video,
		atom.Nav, atom.Header, atom.Footer, atom.Aside:
		return true
	}

	// Unwanted roles
	role := strings.ToLower(getAttr(n, "role"))
	switch role {
	case "navigation", "banner", "contentinfo", "complementary":
		return true
	}

	// Hidden attributes
	if getAttr(n, "aria-hidden") == "true" {
		return true
	}

	// Noisy classes / IDs
	classOrID := strings.ToLower(getAttr(n, "class") + " " + getAttr(n, "id"))
	noisyTerms := []string{
		"ad-", "ads-", "advertisement", "banner-ad", "cookie-banner", "cookie-consent",
		"disclaimer", "footer-links", "site-footer", "navbar", "nav-menu", "sidebar",
		"social-share", "share-buttons", "widget-area",
	}
	for _, term := range noisyTerms {
		if strings.Contains(classOrID, term) {
			return true
		}
	}

	return false
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func extractNodeText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
			buf.WriteString(" ")
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}

func cleanText(s string) string {
	var b strings.Builder
	lastSpace := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		} else if !unicode.IsControl(r) {
			b.WriteRune(r)
			lastSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// Ensure reader implements io.Reader
var _ io.Reader = (*bytes.Reader)(nil)

