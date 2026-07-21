// Package summarizer implements the conveyor.summarize_url RPC method,
// mirroring the original TS src/summarizer.ts. It fetches a URL, extracts
// metadata (title, description, favicon, image, videos), and caches the
// result in an LRU cache with a 1-hour TTL.
package summarizer

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/steemit/conveyor/internal/jsonrpc"
)

const (
	cacheSize   = 10000
	cacheTTL    = 1 * time.Hour
	fetchTimeout = 2 * time.Second
)

// summarizedURL is the RPC result, matching the TS SummarizedUrl interface
// and the schema "summarized_url" definition.
type summarizedURL struct {
	Blacklisted bool              `json:"blacklisted"`
	Description string            `json:"description,omitempty"`
	Favicon     string            `json:"favicon,omitempty"`
	Image       string            `json:"image,omitempty"`
	Videos      []summarizedVideo `json:"videos,omitempty"`
	Title       string            `json:"title,omitempty"`
}

type summarizedVideo struct {
	Src    string `json:"src"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

// cacheEntry holds a cached result with its insertion time for TTL checking.
type cacheEntry struct {
	result summarizedURL
	storedAt time.Time
}

// Summarizer holds the LRU cache and HTTP client.
type Summarizer struct {
	cache  *lru.Cache[string, *cacheEntry]
	client *http.Client
}

// New creates a Summarizer with a fresh LRU cache and 2s-timeout HTTP client.
func New() *Summarizer {
	c, _ := lru.New[string, *cacheEntry](cacheSize)
	return &Summarizer{
		cache: c,
		client: &http.Client{
			Timeout: fetchTimeout,
			// Many sites block the default Go User-Agent; set a descriptive one.
			Transport: &userAgentTransport{base: http.DefaultTransport},
		},
	}
}

// userAgentTransport wraps an http.RoundTripper to inject a User-Agent header
// on every request.
type userAgentTransport struct{ base http.RoundTripper }

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone to avoid mutating the caller's request (which may be reused).
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", "conveyor/1.0 (+https://github.com/steemit/conveyor)")
	return t.base.RoundTrip(clone)
}

// Register registers the conveyor.summarize_url method (public, no auth).
func (s *Summarizer) Register(rpc *jsonrpc.Server) {
	rpc.Register("conveyor.summarize_url", s.SummarizeUrl)
}

// SummarizeUrl fetches a URL, extracts metadata, and returns a summary.
// Mirrors TS summarizeUrl.
func (s *Summarizer) SummarizeUrl(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		URL string `json:"url"`
	}
	// The TS handler takes a positional string arg; also accept named "url".
	if err := req.UnmarshalParams(&p); err != nil {
		// Try positional: params might be ["http://..."]
		var raw []any
		if err2 := req.UnmarshalParams(&raw); err2 == nil && len(raw) > 0 {
			if str, ok := raw[0].(string); ok {
				p.URL = str
			}
		}
	}
	urlStr := p.URL

	// Parse URL.
	parsed, err := url.Parse(urlStr)
	if err != nil || parsed.Host == "" {
		return nil, jsonrpc.NewError(400, nil, "Cannot parse URL")
	}

	// Blacklist check.
	blacklisted := isBlacklisted(parsed.Host, parsed.Path)

	// Cache check (with TTL).
	if cached, ok := s.cache.Get(urlStr); ok {
		if time.Since(cached.storedAt) < cacheTTL {
			return cached.result, nil
		}
		s.cache.Remove(urlStr)
	}

	// Fetch URL.
	resp, err := s.client.Get(urlStr)
	if err != nil {
		return nil, jsonrpc.NewError(400, nil, "Cannot fetch URL")
	}
	defer resp.Body.Close()

	// Parse HTML and extract metadata via goquery.
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, jsonrpc.NewError(400, nil, "Cannot parse HTML")
	}

	result := summarizedURL{
		Blacklisted: blacklisted,
		Title:       doc.Find("title").First().Text(),
		Description: getMetaContent(doc, "name", "description"),
		Favicon:     getFavicon(doc),
		Image:       getMetaContent(doc, "property", "og:image"),
	}

	// Extract videos.
	doc.Find("video").Each(func(_ int, sel *goquery.Selection) {
		v := summarizedVideo{
			Src:    sel.AttrOr("src", ""),
			Height: parseIntAttr(sel, "height"),
			Width:  parseIntAttr(sel, "width"),
		}
		if v.Src != "" {
			result.Videos = append(result.Videos, v)
		}
	})
	// Also check <video><source src="..."> elements.
	doc.Find("video source").Each(func(_ int, sel *goquery.Selection) {
		v := summarizedVideo{
			Src: sel.AttrOr("src", ""),
		}
		if v.Src != "" {
			result.Videos = append(result.Videos, v)
		}
	})

	// Cache and return.
	s.cache.Add(urlStr, &cacheEntry{result: result, storedAt: time.Now()})
	return result, nil
}

// getMetaContent finds a <meta> tag by attribute key/value and returns its
// content attribute. e.g. getMetaContent(doc, "name", "description").
func getMetaContent(doc *goquery.Document, key, value string) string {
	selector := "meta[" + key + "=\"" + value + "\"]"
	val, _ := doc.Find(selector).Attr("content")
	return val
}

// getFavicon extracts the favicon URL from <link rel="icon"> or
// <link rel="shortcut icon">.
func getFavicon(doc *goquery.Document) string {
	if href, ok := doc.Find(`link[rel="icon"]`).Attr("href"); ok {
		return href
	}
	if href, ok := doc.Find(`link[rel="shortcut icon"]`).Attr("href"); ok {
		return href
	}
	return ""
}

// parseIntAttr parses an HTML attribute as int, returning 0 on failure.
func parseIntAttr(sel *goquery.Selection, attr string) int {
	val, ok := sel.Attr(attr)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return n
}
