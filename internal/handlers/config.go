package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ssl-expire-checker/internal/config"
)

// PublicConfig exposes non-secret values for the browser client.
func PublicConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"supabaseUrl":     cfg.SupabaseProjectURL,
			"publishableKey":  cfg.SupabasePublishableKey,
			"expiryThreshold": cfg.ExpiryThresholdDays,
		})
	}
}
