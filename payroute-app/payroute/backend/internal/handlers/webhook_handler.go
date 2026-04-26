package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payroute/backend/internal/services"
)

func HandleWebhook(c *gin.Context) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	signature := c.GetHeader("X-Webhook-Signature")
	if signature == "" || !services.VerifyWebhookSignature(rawBody, signature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook signature"})
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	providerRef, _ := payload["provider_reference"].(string)
	if providerRef == "" {
		if id, ok := payload["id"].(string); ok {
			providerRef = id
		}
	}
	if providerRef == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider_reference is required"})
		return
	}

	headers := make(map[string]interface{})
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if err := services.ProcessWebhook(providerRef, payload, headers); err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "received", "processing_error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}

func SimulateWebhook(c *gin.Context) {
	var req struct {
		ProviderReference string `json:"provider_reference" binding:"required"`
		Status            string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload := map[string]interface{}{
		"provider_reference": req.ProviderReference,
		"status":             req.Status,
	}
	headers := map[string]interface{}{"X-Simulated": "true"}

	if err := services.ProcessWebhook(req.ProviderReference, payload, headers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "simulated", "result": req.Status})
}
