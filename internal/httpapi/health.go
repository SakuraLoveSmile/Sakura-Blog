package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StorePinger is the minimal store capability required by the readiness
// endpoint. Keeping this interface here avoids exposing *sql.DB to HTTP code.
type StorePinger interface {
	PingContext(context.Context) error
}

type healthResponse struct {
	OK bool `json:"ok"`
}

// RegisterHealthRoutes installs process liveness and local-store readiness
// probes. Optional integrations are deliberately not part of readiness.
func RegisterHealthRoutes(r *gin.Engine, store StorePinger) {
	r.GET("/livez", func(c *gin.Context) {
		c.JSON(http.StatusOK, healthResponse{OK: true})
	})
	r.GET("/readyz", func(c *gin.Context) {
		if store == nil {
			c.JSON(http.StatusServiceUnavailable, healthResponse{OK: false})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if err := store.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, healthResponse{OK: false})
			return
		}
		c.JSON(http.StatusOK, healthResponse{OK: true})
	})
}
