package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns the health status of the API server.
// Used by Docker health checks and monitoring systems.
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "hanfledge-api",
		"version": "0.1.0",
	})
}
