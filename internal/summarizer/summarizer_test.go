package summarizer

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/steemit/conveyor/internal/jsonrpc"
)

// newTestSummarizer returns a Summarizer whose HTTP client uses the default
// transport. The production client blocks loopback (SSRF defense), but
// httptest servers listen on 127.0.0.1, so tests that need to hit a local
// fixture server use this seam instead.
func newTestSummarizer() *Summarizer {
	return newWithClient(&http.Client{
		Timeout:   fetchTimeout,
		Transport: http.DefaultTransport,
	})
}

func TestSummarizeUrl_BasicFetch(t *testing.T) {
	// Serve a test HTML page.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Test</title></head><body>Hi</body></html>`))
	}))
	defer srv.Close()

	s := newTestSummarizer()
	req := &jsonrpc.Request{Params: []byte(`{"url":"` + srv.URL + `"}`)}
	result, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err != nil {
		t.Fatal(err)
	}
	res := result.(summarizedURL)
	if res.Title != "Test" {
		t.Errorf("expected title 'Test', got '%s'", res.Title)
	}
	// blacklisted is always false now (placeholder blacklist removed).
	if res.Blacklisted {
		t.Error("expected blacklisted=false")
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

	s := newTestSummarizer()
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

	s := newTestSummarizer()
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

// --- SSRF defense tests ---

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.1.2.3",           // loopback
		"10.0.0.1", "192.168.1.1",          // RFC1918
		"172.16.0.1", "172.31.255.255",     // RFC1918 172.16/12
		"169.254.169.254",                  // link-local / AWS IMDS
		"0.0.0.0",                          // unspecified
		"::1",                              // IPv6 loopback
		"::ffff:127.0.0.1",                 // IPv4-in-IPv6 mapped loopback
		"fe80::1",                          // IPv6 link-local
		"fc00::1",                          // IPv6 ULA/private
	}
	for _, s := range blocked {
		if !isBlockedIP(parseIP(t, s)) {
			t.Errorf("expected %s to be blocked", s)
		}
	}
	allowed := []string{
		"8.8.8.8", "1.1.1.1",               // public
		"93.184.216.34",                    // example.com
		"2606:4700:4700::1111",             // public IPv6
	}
	for _, s := range allowed {
		if isBlockedIP(parseIP(t, s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
}

func parseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("bad test IP literal: %s", s)
	}
	return ip
}

// TestSafeDialContext_RefusesPrivate verifies that dialing a literal private IP
// is refused outright (the entrypoint IP-validation path).
func TestSafeDialContext_RefusesPrivate(t *testing.T) {
	cases := []string{
		"127.0.0.1:80",        // loopback
		"169.254.169.254:80",  // AWS IMDS
		"10.0.0.1:80",         // RFC1918
		"192.168.1.1:80",
	}
	for _, addr := range cases {
		conn, err := safeDialContext(context.Background(), "tcp", addr)
		if err == nil {
			if conn != nil {
				conn.Close()
			}
			t.Errorf("expected dial to %s to be refused, but it succeeded", addr)
		}
	}
}

// TestSummarizeUrl_NonHTTPScheme verifies that non-http(s) schemes are rejected
// before any network activity (defends against file:/// reading local files).
func TestSummarizeUrl_NonHTTPScheme(t *testing.T) {
	s := New()
	for _, u := range []string{
		"file:///etc/passwd",
		"ftp://example.com/",
		"gopher://example.com/",
	} {
		req := &jsonrpc.Request{Params: []byte(`{"url":"` + u + `"}`)}
		_, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
		if err == nil {
			t.Fatalf("expected error for scheme of %s", u)
		}
		e, ok := err.(*jsonrpc.Error)
		if !ok || e.Code != 400 {
			t.Fatalf("expected 400 for %s, got %v", u, err)
		}
	}
}

// TestSummarizeUrl_LoopbackBlockedByProductionClient confirms the production
// Summarizer (New, with the SSRF dialer) cannot reach a loopback httptest
// server, even though the test-seam one can. This is the negative-control
// counterpart to the newTestSummarizer() tests above.
func TestSummarizeUrl_LoopbackBlockedByProductionClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html><title>should not be reached</title></html>"))
	}))
	defer srv.Close()

	s := New() // production client: blocks loopback
	req := &jsonrpc.Request{Params: []byte(`{"url":"` + srv.URL + `"}`)}
	_, err := s.SummarizeUrl(&jsonrpc.Context{}, req)
	if err == nil {
		t.Fatal("expected production client to refuse loopback URL, but it succeeded")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
}
