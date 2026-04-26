package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/payroute/backend/internal/database"
	"github.com/payroute/backend/internal/services"
)

type CreatePaymentRequest struct {
	SenderAccountID     string  `json:"sender_account_id" binding:"required"`
	RecipientName       string  `json:"recipient_name" binding:"required"`
	RecipientCountry    string  `json:"recipient_country" binding:"required"`
	DestinationCurrency string  `json:"destination_currency" binding:"required"`
	Amount              float64 `json:"amount" binding:"required,gt=0"`
}

func CreatePayment(c *gin.Context) {
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header is required"})
		return
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	senderAccountID, err := uuid.Parse(req.SenderAccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sender_account_id"})
		return
	}

	result, err := services.CreatePayment(services.CreatePaymentInput{
		SenderAccountID:     senderAccountID,
		RecipientName:       req.RecipientName,
		RecipientCountry:    req.RecipientCountry,
		DestinationCurrency: req.DestinationCurrency,
		Amount:              req.Amount,
		IdempotencyKey:      idempotencyKey,
	})

	if err != nil {
		switch err.Error() {
		case "insufficient funds":
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"transaction":    result.Transaction,
		"fx_quote":       result.FxQuote,
		"ledger_entries": result.LedgerEntries,
		"status_history": result.StatusHistory,
	})
}

func GetPayment(c *gin.Context) {
	id := c.Param("id")
	result, err := services.GetPaymentByID(id)
	if err != nil {
		if err.Error() == "transaction not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"transaction":    result.Transaction,
		"fx_quote":       result.FxQuote,
		"ledger_entries": result.LedgerEntries,
		"status_history": result.StatusHistory,
	})
}

func ListPayments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	result, err := services.ListPayments(services.ListPaymentsInput{
		Status:    c.Query("status"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func GetFXQuote(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	amountStr := c.Query("amount")

	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to currencies are required"})
		return
	}

	quote, err := services.GenerateFXQuote(from, to)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{"quote": quote}
	if amountStr != "" {
		if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
			response["destination_amount"] = amount * quote.Rate
			response["source_amount"] = amount
		}
	}
	c.JSON(http.StatusOK, response)
}

func GetAccounts(c *gin.Context) {
	type AccountRow struct {
		ID           string  `gorm:"column:id" json:"id"`
		BusinessName string  `gorm:"column:business_name" json:"business_name"`
		Currency     string  `gorm:"column:currency" json:"currency"`
		Balance      float64 `gorm:"column:balance" json:"balance"`
	}
	var rows []AccountRow
	sql := `
		SELECT a.id, b.name as business_name, a.currency, a.balance
		FROM accounts a
		JOIN businesses b ON a.business_id = b.id
		WHERE b.name != 'PayRoute Platform'
		ORDER BY b.name, a.currency
	`
	if err := database.DB.Raw(sql).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": rows})
}
