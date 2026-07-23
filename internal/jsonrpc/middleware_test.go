package jsonrpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := NewServer()
	s.Register("hello", func(ctx *Context, req *Request) (any, error) {
		var p struct{ Name string `json:"name"` }
		_ = req.UnmarshalParams(&p)
		if p.Name == "" {
			p.Name = "Anonymous"
		}
		return "I'm sorry, " + p.Name + ", I can't do that.", nil
	})
	r.POST("/", s.Handler(testLog()))
	return r
}

func TestMiddleware_SingleRequest(t *testing.T) {
	r := setupTestRouter(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"hello","params":{"name":"Dave"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["result"] != "I'm sorry, Dave, I can't do that." {
		t.Fatalf("unexpected result: %v", resp["result"])
	}
}

func TestMiddleware_Batch(t *testing.T) {
	r := setupTestRouter(t)
	body := `[
		{"jsonrpc":"2.0","id":1,"method":"hello","params":{"name":"A"}},
		{"jsonrpc":"2.0","id":2,"method":"hello","params":{"name":"B"}}
	]`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var responses []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &responses); err != nil {
		t.Fatalf("expected array, got: %s, err: %v", w.Body.String(), err)
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
}

func TestMiddleware_EmptyBatch_400(t *testing.T) {
	r := setupTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`[]`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got: %v", resp)
	}
	if int(errObj["code"].(float64)) != InvalidRequest {
		t.Fatalf("expected InvalidRequest, got %v", errObj["code"])
	}
}

func TestMiddleware_ParseError_400(t *testing.T) {
	r := setupTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{not json`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]any)
	if int(errObj["code"].(float64)) != ParseError {
		t.Fatalf("expected ParseError, got %v", errObj["code"])
	}
}

// TestMiddleware_OversizedBody_400 verifies that a request body exceeding the
// 1 MiB cap is rejected with a 400 ParseError (memory-exhaustion DoS defense).
func TestMiddleware_OversizedBody_400(t *testing.T) {
	r := setupTestRouter(t)
	// 2 MiB body — well over the 1 MiB cap.
	big := strings.Repeat("x", 2<<20)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got: %v", resp)
	}
	if int(errObj["code"].(float64)) != ParseError {
		t.Fatalf("expected ParseError for oversized body, got %v", errObj["code"])
	}
}

func TestMiddleware_NonPost_NotRoutedByRPC(t *testing.T) {
	// In this architecture, GET / is healthcheck (registered in server.app),
	// POST / is RPC. The test router only registers POST /, so a GET / is a
	// 404 (no matching route). The server package's healthcheck test covers
	// the GET / = healthcheck case. The original koa-jsonrpc 405 is vestigial;
	// we don't reproduce it since method routing is Gin's job.
	r := setupTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	// POST-only router: GET / is not found (404), confirming RPC doesn't
	// intercept non-POST requests.
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for GET on POST-only router, got %d", w.Code)
	}
}

func TestMiddleware_Notification_EmptyBody(t *testing.T) {
	r := setupTestRouter(t)
	body := `{"jsonrpc":"2.0","method":"hello","params":{"name":"X"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.Len() > 0 {
		t.Fatalf("notification should have empty body, got: %s", w.Body.String())
	}
}

func TestMiddleware_MethodNotFound(t *testing.T) {
	r := setupTestRouter(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"ghost"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.ServeHTTP(w, req)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]any)
	if int(errObj["code"].(float64)) != MethodNotFound {
		t.Fatalf("expected MethodNotFound, got %v", errObj["code"])
	}
}
