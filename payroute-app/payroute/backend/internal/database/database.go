package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/payroute/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	var err error
	for i := 0; i < 10; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err == nil {
			break
		}
		log.Printf("Waiting for database... attempt %d/10", i+1)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected successfully")
	Migrate()
	Seed()
}

func Migrate() {
	err := DB.AutoMigrate(
		&models.Business{},
		&models.Account{},
		&models.FxQuote{},
		&models.Transaction{},
		&models.TransactionStatusHistory{},
		&models.LedgerEntry{},
		&models.WebhookEvent{},
		&models.IdempotencyKey{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Database migrated successfully")
}

func Seed() {
	var count int64
	DB.Model(&models.Business{}).Count(&count)
	if count > 0 {
		log.Println("Database already seeded, skipping")
		return
	}

	log.Println("Seeding database...")

	platformBusiness := models.Business{
		ID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name: "PayRoute Platform",
	}
	DB.Create(&platformBusiness)

	platformNGN := models.Account{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		BusinessID: platformBusiness.ID,
		Currency:   "NGN",
		Balance:    0,
	}
	DB.Create(&platformNGN)

	platformUSD := models.Account{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000011"),
		BusinessID: platformBusiness.ID,
		Currency:   "USD",
		Balance:    0,
	}
	DB.Create(&platformUSD)

	abcBusiness := models.Business{
		ID:   uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Name: "ABC Imports",
	}
	DB.Create(&abcBusiness)

	abcNGN := models.Account{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000020"),
		BusinessID: abcBusiness.ID,
		Currency:   "NGN",
		Balance:    5000000,
	}
	DB.Create(&abcNGN)

	// ABC Imports USD account
	abcUSD := models.Account{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000021"),
		BusinessID: abcBusiness.ID,
		Currency:   "USD",
		Balance:    0,
	}
	DB.Create(&abcUSD)

	log.Println("Database seeded successfully")
}

// GetPlatformHoldingAccount returns the platform NGN holding account
func GetPlatformHoldingAccount(currency string) (*models.Account, error) {
	var account models.Account
	businessID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	result := DB.Where("business_id = ? AND currency = ?", businessID, currency).First(&account)
	return &account, result.Error
}

// RecordStatusTransition writes an immutable audit entry for a status change
func RecordStatusTransition(db *gorm.DB, txID uuid.UUID, from *models.TransactionStatus, to models.TransactionStatus, reason string) error {
	entry := models.TransactionStatusHistory{
		TransactionID: txID,
		FromStatus:    from,
		ToStatus:      to,
		Reason:        reason,
	}
	return db.Create(&entry).Error
}
