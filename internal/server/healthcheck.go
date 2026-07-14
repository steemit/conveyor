package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// healthcheckResponse mirrors the original TS healthcheck shape.
type healthcheckResponse struct {
	OK      bool      `json:"ok"`
	Version string    `json:"version"`
	Date    time.Time `json:"date"`
}

// healthcheck handles GET / and GET /.well-known/healthcheck.json, returning
// {ok:true, version, date}.
func healthcheck(c *gin.Context) {
	c.JSON(http.StatusOK, healthcheckResponse{
		OK:      true,
		Version: Version,
		Date:    time.Now().UTC(),
	})
}
