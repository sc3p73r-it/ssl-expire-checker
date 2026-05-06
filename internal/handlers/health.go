package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health returns liveness.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
