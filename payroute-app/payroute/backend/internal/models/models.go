package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	EntryDebit  EntryType = "debit"
	EntryCredit EntryType = "credit"
)

type Business struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Accounts  []Account `gorm:"foreignKey:BusinessID" json:"accounts,omitempty"`
}

func (b *Business) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

type Account struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	BusinessID uuid.UUID `gorm:"type:uuid;not null" json:"business_id"`
	Currency   string    `gorm:"not null" json:"currency"`
	Balance    float64   `gorm:"type:numeric;not null;default:0" json:"balance"`
	CreatedAt  time.Time `json:"created_at"`
	Business   *Business `gorm:"foreignKey:BusinessID" json:"business,omitempty"`
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

type TransactionStatus string

const (
	StatusInitiated  TransactionStatus = "initiated"
	StatusProcessing TransactionStatus = "processing"
	StatusCompleted  TransactionStatus = "completed"
	StatusFailed     TransactionStatus = "failed"
	StatusReversed   TransactionStatus = "reversed"
)

// Transaction represents a cross-border payment
type Transaction struct {
	ID                  uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	Reference           string            `gorm:"uniqueIndex;not null" json:"reference"`
	SenderAccountID     uuid.UUID         `gorm:"type:uuid;not null" json:"sender_account_id"`
	RecipientName       string            `gorm:"not null" json:"recipient_name"`
	RecipientCountry    string            `gorm:"not null" json:"recipient_country"`
	DestinationCurrency string            `gorm:"not null" json:"destination_currency"`
	SourceAmount        float64           `gorm:"type:numeric;not null" json:"source_amount"`
	DestinationAmount   float64           `gorm:"type:numeric;not null" json:"destination_amount"`
	FxRate              float64           `gorm:"type:numeric;not null" json:"fx_rate"`
	Status              TransactionStatus `gorm:"not null;default:'initiated'" json:"status"`
	ProviderReference   string            `json:"provider_reference,omitempty"`
	FxQuoteID           uuid.UUID         `gorm:"type:uuid" json:"fx_quote_id"`
	CreatedAt           time.Time         `json:"created_at"`
	CompletedAt         *time.Time        `json:"completed_at,omitempty"`
	SenderAccount       *Account          `gorm:"foreignKey:SenderAccountID" json:"sender_account,omitempty"`
	LedgerEntries       []LedgerEntry     `gorm:"foreignKey:TransactionID" json:"ledger_entries,omitempty"`
	FxQuote             *FxQuote          `gorm:"foreignKey:FxQuoteID" json:"fx_quote,omitempty"`
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

type EntryType string

type LedgerEntry struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TransactionID uuid.UUID `gorm:"type:uuid;not null" json:"transaction_id"`
	AccountID     uuid.UUID `gorm:"type:uuid;not null" json:"account_id"`
	Amount        float64   `gorm:"type:numeric;not null" json:"amount"`
	Currency      string    `gorm:"not null" json:"currency"`
	EntryType     EntryType `gorm:"not null" json:"entry_type"`
	CreatedAt     time.Time `json:"created_at"`
	Account       *Account  `gorm:"foreignKey:AccountID" json:"account,omitempty"`
}

func (l *LedgerEntry) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// FxQuote stores exchange rate quotes
type FxQuote struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	FromCurrency string    `gorm:"not null" json:"from_currency"`
	ToCurrency   string    `gorm:"not null" json:"to_currency"`
	Rate         float64   `gorm:"type:numeric;not null" json:"rate"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (f *FxQuote) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

func (f *FxQuote) IsExpired() bool {
	return time.Now().After(f.ExpiresAt)
}

// WebhookEvent stores raw webhook payloads
type WebhookEvent struct {
	ID                uuid.UUID              `gorm:"type:uuid;primaryKey" json:"id"`
	ProviderReference string                 `gorm:"not null" json:"provider_reference"`
	Payload           map[string]interface{} `gorm:"type:jsonb;serializer:json" json:"payload"`
	Headers           map[string]interface{} `gorm:"type:jsonb;serializer:json" json:"headers"`
	ReceivedAt        time.Time              `json:"received_at"`
	Processed         bool                   `gorm:"default:false" json:"processed"`
}

func (w *WebhookEvent) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

type IdempotencyKey struct {
	ID        uuid.UUID              `gorm:"type:uuid;primaryKey" json:"id"`
	Key       string                 `gorm:"uniqueIndex;not null" json:"key"`
	Endpoint  string                 `gorm:"not null" json:"endpoint"`
	Response  map[string]interface{} `gorm:"type:jsonb;serializer:json" json:"response"`
	CreatedAt time.Time              `json:"created_at"`
}

func (i *IdempotencyKey) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

type TransactionStatusHistory struct {
	ID            uuid.UUID          `gorm:"type:uuid;primaryKey" json:"id"`
	TransactionID uuid.UUID          `gorm:"type:uuid;not null" json:"transaction_id"`
	FromStatus    *TransactionStatus `gorm:"type:text" json:"from_status,omitempty"`
	ToStatus      TransactionStatus  `gorm:"type:text;not null" json:"to_status"`
	Reason        string             `json:"reason,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

func (h *TransactionStatusHistory) BeforeCreate(tx *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	return nil
}
