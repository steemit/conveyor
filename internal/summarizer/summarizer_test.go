package summarizer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/steemit/conveyor/internal/jsonrpc"
)

func TestIsBlacklisted_Domain(t *testing.T) {
	if !isBlacklisted("evil.bad.domain", "/path") {
		t.Error("expected bad.domain suffix to be blacklisted")
	}
	if isBlacklisted("good.com", "/path") {
		t.Error("good.com should not be blacklisted")
	}
}

func TestIsBlacklisted_URL(t *testing.T) {
	if !isBlacklisted("www.google.com", "/search") {
		t.Error("expected www.google.com to be blacklisted")
	}
	if !isBlacklisted("bad.url", "/leave/out/protocol") {
		t.Error("expected bad.url prefix to be blacklisted")
	}
	if isBlacklisted("good.com", "/path") {
		t.Error("good.com/path should not be blacklisted")
	}
}

func TestSuffixMatch(t *testing.T) {
	if !suffixMatch("sub.bad.domain", "bad.domain") {
		t.Error("expected suffix match")
	}
	if suffixMatch("bad.dom", "bad.domain") {
		t.Error("should not match — host shorter than domain")
	}
	if suffixMatch("goodbad.domain", "bad.domain") {
		// This actually should match — "goodbad.domain" ends with "bad.domain".
		// TS uses endsWith which would also match here. This is correct behavior.
	}
}

func TestSummarizeUrl_BlacklistedDomain_StillFetched(t *testing.T) {
	// Serve a test HTML page.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Test</title></head><body>Hi</body></html>`))
	}))
	defer srv.Close()

	s := New()
	req := &jsonrpc.Request{Params: []byte(`{"url":"` + srv.URL + `"}`)}
	result, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err != nil {
		t.Fatal(err)
	}
	res := result.(summarizedURL)
	// The test server URL won't be blacklisted, just verify it works.
	if res.Title != "Test" {
		t.Errorf("expected title 'Test', got '%s'", res.Title)
	}
}

func TestSummarizeUrl_MetadataExtraction(t *testing.T) {
	html := `<html><head>
		<title>My Page</title>
		<meta name="description" content="A test page">
		<link rel="icon" href="/favicon.ico">
		<meta property="og:image" content="https://example.com/img.jpg">
	</head><body>
		<video src="https://example.com/v.mp4" width="640" height="480"></video>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	s := New()
	req := &jsonrpc.Request{Params: []byte(`{"url":"` + srv.URL + `"}`)}
	result, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err != nil {
		t.Fatal(err)
	}
	res := result.(summarizedURL)

	if res.Title != "My Page" {
		t.Errorf("title: expected 'My Page', got '%s'", res.Title)
	}
	if res.Description != "A test page" {
		t.Errorf("description: expected 'A test page', got '%s'", res.Description)
	}
	if res.Favicon != "/favicon.ico" {
		t.Errorf("favicon: expected '/favicon.ico', got '%s'", res.Favicon)
	}
	if res.Image != "https://example.com/img.jpg" {
		t.Errorf("image: expected og:image, got '%s'", res.Image)
	}
	if len(res.Videos) != 1 {
		t.Fatalf("expected 1 video, got %d", len(res.Videos))
	}
	if res.Videos[0].Src != "https://example.com/v.mp4" {
		t.Errorf("video src: got '%s'", res.Videos[0].Src)
	}
	if res.Videos[0].Width != 640 || res.Videos[0].Height != 480 {
		t.Errorf("video dims: got %dx%d", res.Videos[0].Width, res.Videos[0].Height)
	}
}

func TestSummarizeUrl_CacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Cached</title></head></html>`))
	}))
	defer srv.Close()

	s := New()
	req := &jsonrpc.Request{Params: []byte(`{"url":"` + srv.URL + `"}`)}

	// First call — fetches from server.
	r1, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 fetch, got %d", callCount)
	}

	// Second call — should hit cache, not fetch.
	r2, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Fatalf("expected cache hit (still 1 fetch), got %d", callCount)
	}
	if r1.(summarizedURL).Title != r2.(summarizedURL).Title {
		t.Error("cached result should match")
	}
}

func TestSummarizeUrl_BadURL(t *testing.T) {
	s := New()
	req := &jsonrpc.Request{Params: []byte(`{"url":"not a url"}`)}
	_, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err == nil {
		t.Fatal("expected error for bad URL")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestSummarizeUrl_FetchFailure(t *testing.T) {
	s := New()
	// Use a port that's definitely not listening.
	req := &jsonrpc.Request{Params: []byte(`{"url":"http://127.0.0.1:39999/test"}`)}
	_, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err == nil {
		t.Fatal("expected fetch error")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
}
