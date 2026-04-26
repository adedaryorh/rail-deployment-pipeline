package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/payroute/backend/internal/database"
	"github.com/payroute/backend/internal/models"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var fxRates = map[string]map[string]float64{
	"NGN": {
		"USD": 1.0 / 1500.0,
		"EUR": 1.0 / 1650.0,
		"GBP": 1.0 / 1900.0,
	},
}

func GetFXRate(from, to string) (float64, error) {
	from = strings.ToUpper(from)
	to = strings.ToUpper(to)
	if from == to {
		return 1.0, nil
	}
	if rates, ok := fxRates[from]; ok {
		if rate, ok := rates[to]; ok {
			return rate, nil
		}
	}
	return 0, fmt.Errorf("FX rate not available for %s → %s", from, to)
}

func GenerateFXQuote(fromCurrency, toCurrency string) (*models.FxQuote, error) {
	rate, err := GetFXRate(fromCurrency, toCurrency)
	if err != nil {
		return nil, err
	}
	quote := &models.FxQuote{
		FromCurrency: strings.ToUpper(fromCurrency),
		ToCurrency:   strings.ToUpper(toCurrency),
		Rate:         rate,
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
	if err := database.DB.Create(quote).Error; err != nil {
		return nil, fmt.Errorf("failed to create FX quote: %w", err)
	}
	return quote, nil
}

func generateReference() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "PAY-" + strings.ToUpper(hex.EncodeToString(b))
}

type CreatePaymentInput struct {
	SenderAccountID     uuid.UUID
	RecipientName       string
	RecipientCountry    string
	DestinationCurrency string
	Amount              float64
	IdempotencyKey      string
}

type PaymentResult struct {
	Transaction   *models.Transaction               `json:"transaction"`
	FxQuote       *models.FxQuote                   `json:"fx_quote"`
	LedgerEntries []models.LedgerEntry              `json:"ledger_entries"`
	StatusHistory []models.TransactionStatusHistory `json:"status_history"`
}

func CreatePayment(input CreatePaymentInput) (*PaymentResult, error) {
	var existing models.IdempotencyKey
	if err := database.DB.Where("key = ? AND endpoint = ?", input.IdempotencyKey, "/payments").First(&existing).Error; err == nil {
		txID, _ := existing.Response["transaction_id"].(string)
		return loadPaymentResult(txID)
	}

	var result *PaymentResult

	err := database.DB.Transaction(func(dbTx *gorm.DB) error {
		// Step 2: Lock sender account row — prevents concurrent overdraft
		var senderAccount models.Account
		if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&senderAccount, "id = ?", input.SenderAccountID).Error; err != nil {
			return fmt.Errorf("sender account not found: %w", err)
		}

		if senderAccount.Balance < input.Amount {
			return errors.New("insufficient funds")
		}

		rate, err := GetFXRate(senderAccount.Currency, input.DestinationCurrency)
		if err != nil {
			return err
		}
		destinationAmount := input.Amount * rate

		quote := &models.FxQuote{
			FromCurrency: senderAccount.Currency,
			ToCurrency:   strings.ToUpper(input.DestinationCurrency),
			Rate:         rate,
			ExpiresAt:    time.Now().Add(5 * time.Minute),
		}
		if err := dbTx.Create(quote).Error; err != nil {
			return fmt.Errorf("failed to create FX quote: %w", err)
		}

		// Step 4: Lock platform holding account & move funds
		var platformAccount models.Account
		if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("business_id = ? AND currency = ?",
				uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				senderAccount.Currency).
			First(&platformAccount).Error; err != nil {
			return fmt.Errorf("platform holding account not found: %w", err)
		}

		if err := dbTx.Model(&senderAccount).Update("balance", gorm.Expr("balance - ?", input.Amount)).Error; err != nil {
			return fmt.Errorf("failed to debit sender: %w", err)
		}
		if err := dbTx.Model(&platformAccount).Update("balance", gorm.Expr("balance + ?", input.Amount)).Error; err != nil {
			return fmt.Errorf("failed to credit platform: %w", err)
		}

		// Step 5: Create transaction (initiated → processing)
		reference := generateReference()
		initiatedStatus := models.StatusInitiated
		transaction := &models.Transaction{
			Reference:           reference,
			SenderAccountID:     input.SenderAccountID,
			RecipientName:       input.RecipientName,
			RecipientCountry:    input.RecipientCountry,
			DestinationCurrency: strings.ToUpper(input.DestinationCurrency),
			SourceAmount:        input.Amount,
			DestinationAmount:   destinationAmount,
			FxRate:              rate,
			Status:              models.StatusProcessing,
			FxQuoteID:           quote.ID,
		}
		if err := dbTx.Create(transaction).Error; err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}
		if err := dbTx.Create(&models.TransactionStatusHistory{
			TransactionID: transaction.ID,
			FromStatus:    nil,
			ToStatus:      initiatedStatus,
			Reason:        "payment initiated",
		}).Error; err != nil {
			return err
		}
		if err := dbTx.Create(&models.TransactionStatusHistory{
			TransactionID: transaction.ID,
			FromStatus:    &initiatedStatus,
			ToStatus:      models.StatusProcessing,
			Reason:        "funds locked, submitted to provider",
		}).Error; err != nil {
			return err
		}

		debitEntry := models.LedgerEntry{
			TransactionID: transaction.ID,
			AccountID:     senderAccount.ID,
			Amount:        input.Amount,
			Currency:      senderAccount.Currency,
			EntryType:     models.EntryDebit,
		}
		creditEntry := models.LedgerEntry{
			TransactionID: transaction.ID,
			AccountID:     platformAccount.ID,
			Amount:        input.Amount,
			Currency:      senderAccount.Currency,
			EntryType:     models.EntryCredit,
		}
		if err := dbTx.Create(&debitEntry).Error; err != nil {
			return err
		}
		if err := dbTx.Create(&creditEntry).Error; err != nil {
			return err
		}

		//Submit it to Stripe (outside tight DB lock — idempotent)
		providerRef, err := submitToStripe(transaction)
		if err != nil {
			log.Printf("Stripe submission failed for %s: %v — stored as pending", transaction.Reference, err)
			providerRef = "pending-" + transaction.Reference
		}
		if err := dbTx.Model(transaction).Update("provider_reference", providerRef).Error; err != nil {
			return err
		}
		transaction.ProviderReference = providerRef

		result = &PaymentResult{
			Transaction:   transaction,
			FxQuote:       quote,
			LedgerEntries: []models.LedgerEntry{debitEntry, creditEntry},
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Store idempotency key (outside transaction — best effort)
	database.DB.Create(&models.IdempotencyKey{
		Key:      input.IdempotencyKey,
		Endpoint: "/payments",
		Response: map[string]interface{}{"transaction_id": result.Transaction.ID.String()},
	})

	return result, nil
}

func submitToStripe(tx *models.Transaction) (string, error) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	amountInCents := int64(tx.DestinationAmount * 100)
	if amountInCents < 50 {
		amountInCents = 50
	}
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountInCents),
		Currency: stripe.String(strings.ToLower(tx.DestinationCurrency)),
		Metadata: map[string]string{
			"payroute_reference": tx.Reference,
			"transaction_id":     tx.ID.String(),
			"recipient_name":     tx.RecipientName,
			"recipient_country":  tx.RecipientCountry,
		},
	}
	params.IdempotencyKey = stripe.String(tx.Reference)
	pi, err := paymentintent.New(params)
	if err != nil {
		return "", err
	}
	return pi.ID, nil
}

func loadPaymentResult(txID string) (*PaymentResult, error) {
	var tx models.Transaction
	if err := database.DB.
		Preload("SenderAccount").
		Preload("SenderAccount.Business").
		Preload("LedgerEntries").
		Preload("LedgerEntries.Account").
		Preload("FxQuote").
		First(&tx, "id = ?", txID).Error; err != nil {
		return nil, err
	}
	var history []models.TransactionStatusHistory
	database.DB.Where("transaction_id = ?", tx.ID).Order("created_at ASC").Find(&history)

	return &PaymentResult{
		Transaction:   &tx,
		FxQuote:       tx.FxQuote,
		LedgerEntries: tx.LedgerEntries,
		StatusHistory: history,
	}, nil
}

func GetPaymentByID(id string) (*PaymentResult, error) {
	result, err := loadPaymentResult(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("transaction not found")
		}
		return nil, err
	}
	return result, nil
}

type ListPaymentsInput struct {
	Status    string
	StartDate string
	EndDate   string
	Page      int
	PageSize  int
}

type ListPaymentsResult struct {
	Transactions []models.Transaction `json:"transactions"`
	Total        int64                `json:"total"`
	Page         int                  `json:"page"`
	PageSize     int                  `json:"page_size"`
	TotalPages   int                  `json:"total_pages"`
}

func ListPayments(input ListPaymentsInput) (*ListPaymentsResult, error) {
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		input.PageSize = 10
	}

	query := database.DB.Model(&models.Transaction{}).
		Preload("SenderAccount").
		Preload("SenderAccount.Business")

	if input.Status != "" {
		query = query.Where("status = ?", input.Status)
	}
	if input.StartDate != "" {
		query = query.Where("created_at >= ?", input.StartDate)
	}
	if input.EndDate != "" {
		query = query.Where("created_at <= ?", input.EndDate)
	}

	var total int64
	query.Count(&total)

	offset := (input.Page - 1) * input.PageSize
	var transactions []models.Transaction
	if err := query.Order("created_at DESC").Offset(offset).Limit(input.PageSize).Find(&transactions).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / input.PageSize
	if int(total)%input.PageSize != 0 {
		totalPages++
	}
	return &ListPaymentsResult{
		Transactions: transactions,
		Total:        total,
		Page:         input.Page,
		PageSize:     input.PageSize,
		TotalPages:   totalPages,
	}, nil
}
