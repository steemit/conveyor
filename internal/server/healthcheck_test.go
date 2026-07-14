package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthcheck(t *testing.T) {
	Version = "test-1.0"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", healthcheck)
	r.GET("/.well-known/healthcheck.json", healthcheck)

	for _, path := range []string{"/", "/.well-known/healthcheck.json"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, w.Code)
		}
		var body struct {
			OK      bool   `json:"ok"`
			Version string `json:"version"`
			Date    string `json:"date"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: invalid json: %v", path, err)
		}
		if !body.OK {
			t.Errorf("%s: expected ok=true", path)
		}
		if body.Version != "test-1.0" {
			t.Errorf("%s: expected version test-1.0, got %s", path, body.Version)
		}
		if body.Date == "" {
			t.Errorf("%s: expected non-empty date", path)
		}
	}
}
