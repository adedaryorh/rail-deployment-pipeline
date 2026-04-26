package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/payroute/backend/internal/database"
	"github.com/payroute/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func VerifyWebhookSignature(rawBody []byte, signature string) bool {
	secret := os.Getenv("WEBHOOK_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	// Constant-time comparison prevents timing attacks
	return hmac.Equal([]byte(expected), []byte(signature))
}

func ProcessWebhook(providerRef string, payload map[string]interface{}, headers map[string]interface{}) error {
	// Always store raw event first, before any processing
	event := &models.WebhookEvent{
		ProviderReference: providerRef,
		Payload:           payload,
		Headers:           headers,
		ReceivedAt:        time.Now(),
		Processed:         false,
	}
	if err := database.DB.Create(event).Error; err != nil {
		return fmt.Errorf("failed to store webhook event: %w", err)
	}

	var tx models.Transaction
	if err := database.DB.Where("provider_reference = ?", providerRef).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Unknown reference — return 200 to prevent provider retry storms.
			// The raw event is stored; a background job can retry later.
			log.Printf("Webhook for unknown provider_reference=%s — stored for review", providerRef)
			database.DB.Model(event).Update("processed", true)
			return nil
		}
		return err
	}

	// Idempotency: skip if already in a terminal state
	if tx.Status == models.StatusCompleted || tx.Status == models.StatusFailed || tx.Status == models.StatusReversed {
		log.Printf("Transaction %s already in terminal state %s — skipping duplicate webhook", tx.ID, tx.Status)
		database.DB.Model(event).Update("processed", true)
		return nil
	}

	status, _ := payload["status"].(string)

	err := database.DB.Transaction(func(dbTx *gorm.DB) error {
		switch strings.ToLower(status) {
		case "completed":
			return handleCompleted(dbTx, &tx)
		case "failed":
			return handleFailed(dbTx, &tx)
		default:
			log.Printf("Unrecognised webhook status=%q for tx %s", status, tx.ID)
			return nil
		}
	})

	if err != nil {
		return err
	}

	database.DB.Model(event).Update("processed", true)
	return nil
}

func handleCompleted(dbTx *gorm.DB, tx *models.Transaction) error {
	now := time.Now()
	prevStatus := tx.Status

	if err := dbTx.Model(tx).Updates(map[string]interface{}{
		"status":       models.StatusCompleted,
		"completed_at": now,
	}).Error; err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	// Record status transition
	if err := dbTx.Create(&models.TransactionStatusHistory{
		TransactionID: tx.ID,
		FromStatus:    &prevStatus,
		ToStatus:      models.StatusCompleted,
		Reason:        "provider confirmed payment completed",
	}).Error; err != nil {
		return err
	}

	// Lock platform NGN holding
	var platformNGN models.Account
	if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("business_id = ? AND currency = ?", "00000000-0000-0000-0000-000000000001", "NGN").
		First(&platformNGN).Error; err != nil {
		return fmt.Errorf("platform NGN account not found: %w", err)
	}

	// Find sender's business destination-currency account for settlement
	var senderAccount models.Account
	if err := dbTx.First(&senderAccount, "id = ?", tx.SenderAccountID).Error; err != nil {
		return fmt.Errorf("sender account not found: %w", err)
	}

	var recipientAccount models.Account
	if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("business_id = ? AND currency = ?", senderAccount.BusinessID, tx.DestinationCurrency).
		First(&recipientAccount).Error; err != nil {
		return fmt.Errorf("recipient %s account not found: %w", tx.DestinationCurrency, err)
	}

	// Debit platform holding (release the locked NGN)
	if err := dbTx.Model(&platformNGN).Update("balance", gorm.Expr("balance - ?", tx.SourceAmount)).Error; err != nil {
		return fmt.Errorf("failed to debit platform holding: %w", err)
	}
	// Credit recipient in destination currency
	if err := dbTx.Model(&recipientAccount).Update("balance", gorm.Expr("balance + ?", tx.DestinationAmount)).Error; err != nil {
		return fmt.Errorf("failed to credit recipient: %w", err)
	}

	// Ledger entries for settlement
	if err := dbTx.Create(&models.LedgerEntry{
		TransactionID: tx.ID,
		AccountID:     platformNGN.ID,
		Amount:        tx.SourceAmount,
		Currency:      "NGN",
		EntryType:     models.EntryDebit,
	}).Error; err != nil {
		return err
	}
	if err := dbTx.Create(&models.LedgerEntry{
		TransactionID: tx.ID,
		AccountID:     recipientAccount.ID,
		Amount:        tx.DestinationAmount,
		Currency:      tx.DestinationCurrency,
		EntryType:     models.EntryCredit,
	}).Error; err != nil {
		return err
	}

	log.Printf("Settled: %s — ₦%.2f → %.2f %s", tx.Reference, tx.SourceAmount, tx.DestinationAmount, tx.DestinationCurrency)
	return nil
}

func handleFailed(dbTx *gorm.DB, tx *models.Transaction) error {
	prevStatus := tx.Status

	if err := dbTx.Model(tx).Update("status", models.StatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if err := dbTx.Create(&models.TransactionStatusHistory{
		TransactionID: tx.ID,
		FromStatus:    &prevStatus,
		ToStatus:      models.StatusFailed,
		Reason:        "provider reported payment failed — reversing debit",
	}).Error; err != nil {
		return err
	}

	// Lock both accounts for reversal
	var platformNGN models.Account
	if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("business_id = ? AND currency = ?", "00000000-0000-0000-0000-000000000001", "NGN").
		First(&platformNGN).Error; err != nil {
		return err
	}

	var senderAccount models.Account
	if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&senderAccount, "id = ?", tx.SenderAccountID).Error; err != nil {
		return err
	}

	// Reverse: return NGN from platform holding to sender
	if err := dbTx.Model(&platformNGN).Update("balance", gorm.Expr("balance - ?", tx.SourceAmount)).Error; err != nil {
		return err
	}
	if err := dbTx.Model(&senderAccount).Update("balance", gorm.Expr("balance + ?", tx.SourceAmount)).Error; err != nil {
		return err
	}

	// Compensating ledger entries (reversal — note the swap of debit/credit)
	if err := dbTx.Create(&models.LedgerEntry{
		TransactionID: tx.ID,
		AccountID:     platformNGN.ID,
		Amount:        tx.SourceAmount,
		Currency:      "NGN",
		EntryType:     models.EntryDebit,
	}).Error; err != nil {
		return err
	}
	if err := dbTx.Create(&models.LedgerEntry{
		TransactionID: tx.ID,
		AccountID:     senderAccount.ID,
		Amount:        tx.SourceAmount,
		Currency:      "NGN",
		EntryType:     models.EntryCredit,
	}).Error; err != nil {
		return err
	}

	log.Printf("Reversed: %s — ₦%.2f returned to sender", tx.Reference, tx.SourceAmount)
	return nil
}
