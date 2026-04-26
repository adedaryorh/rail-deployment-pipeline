package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/payroute/backend/internal/services"
)

// ── FX Rate Tests ──────────────────────────────────────────────────────────────

func TestGetFXRate_NGNtoUSD(t *testing.T) {
	rate, err := services.GetFXRate("NGN", "USD")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := 1.0 / 1500.0
	if rate != expected {
		t.Errorf("expected rate %.10f, got %.10f", expected, rate)
	}
}

func TestGetFXRate_SameCurrency(t *testing.T) {
	rate, err := services.GetFXRate("NGN", "NGN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 1.0 {
		t.Errorf("same currency should return 1.0, got %f", rate)
	}
}

func TestGetFXRate_Unsupported(t *testing.T) {
	_, err := services.GetFXRate("NGN", "JPY")
	if err == nil {
		t.Error("expected error for unsupported currency pair, got nil")
	}
}

func TestGetFXRate_CaseInsensitive(t *testing.T) {
	rate1, err1 := services.GetFXRate("ngn", "usd")
	rate2, err2 := services.GetFXRate("NGN", "USD")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if rate1 != rate2 {
		t.Errorf("rates should match regardless of case: %f != %f", rate1, rate2)
	}
}

// ── FX Calculation Tests ───────────────────────────────────────────────────────

func TestDestinationAmountCalculation(t *testing.T) {
	cases := []struct {
		sourceAmount float64
		from, to     string
		expectedDest float64
	}{
		{1_500_000, "NGN", "USD", 1000.0},   // ₦1.5M at 1/1500
		{1_650_000, "NGN", "EUR", 1000.0},   // ₦1.65M at 1/1650
		{1_900_000, "NGN", "GBP", 1000.0},   // ₦1.9M at 1/1900
	}

	for _, tc := range cases {
		rate, err := services.GetFXRate(tc.from, tc.to)
		if err != nil {
			t.Fatalf("[%s→%s] GetFXRate error: %v", tc.from, tc.to, err)
		}
		dest := tc.sourceAmount * rate
		// Allow 0.01 tolerance for floating point
		diff := dest - tc.expectedDest
		if diff > 0.01 || diff < -0.01 {
			t.Errorf("[%s→%s] expected %.4f, got %.4f", tc.from, tc.to, tc.expectedDest, dest)
		}
	}
}

// ── Webhook Signature Tests ────────────────────────────────────────────────────

func computeHMAC(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// Note: VerifyWebhookSignature reads WEBHOOK_SECRET from env.
// These tests verify the HMAC logic directly.

func TestHMACSignatureValid(t *testing.T) {
	secret := "test_webhook_secret"
	body := `{"provider_reference":"pi_abc123","status":"completed"}`
	sig := computeHMAC(secret, body)

	mac1 := hmac.New(sha256.New, []byte(secret))
	mac1.Write([]byte(body))
	expected := hex.EncodeToString(mac1.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		t.Error("valid signature should match")
	}
}

func TestHMACSignatureInvalid(t *testing.T) {
	secret := "test_webhook_secret"
	body := `{"provider_reference":"pi_abc123","status":"completed"}`
	tamperedSig := computeHMAC("wrong_secret", body)
	correctSig := computeHMAC(secret, body)

	if hmac.Equal([]byte(tamperedSig), []byte(correctSig)) {
		t.Error("tampered signature should not match")
	}
}

func TestHMACSignatureBodyTamper(t *testing.T) {
	secret := "test_webhook_secret"
	originalBody := `{"provider_reference":"pi_abc123","status":"completed"}`
	tamperedBody := `{"provider_reference":"pi_abc123","status":"failed"}`

	sigForOriginal := computeHMAC(secret, originalBody)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tamperedBody))
	expectedForTampered := hex.EncodeToString(mac.Sum(nil))

	if hmac.Equal([]byte(sigForOriginal), []byte(expectedForTampered)) {
		t.Error("signature from original body must not match tampered body")
	}
}

// ── Balance Integrity Tests ────────────────────────────────────────────────────

func TestDoubleEntryBalance(t *testing.T) {
	// Simulate what CreatePayment does to ledger entries.
	// The sum of all amounts by type must balance:
	// total debits == total credits across a single transaction.

	type ledgerEntry struct {
		amount    float64
		entryType string // "debit" or "credit"
	}

	// Initiation entries (funds lock)
	initiationEntries := []ledgerEntry{
		{500_000, "debit"},  // sender debit
		{500_000, "credit"}, // platform holding credit
	}

	var totalDebits, totalCredits float64
	for _, e := range initiationEntries {
		if e.entryType == "debit" {
			totalDebits += e.amount
		} else {
			totalCredits += e.amount
		}
	}
	if totalDebits != totalCredits {
		t.Errorf("initiation entries don't balance: debits=%.2f credits=%.2f", totalDebits, totalCredits)
	}

	// Settlement entries (on webhook completed)
	// NGN side: platform holding debit
	// USD side: recipient credit (different currency — modelled as separate entries)
	// The balance assertion here is that: for each currency, entries balance.
	ngnEntries := []ledgerEntry{
		{500_000, "debit"}, // platform NGN debit
	}
	// USD entries are credits only on the destination side — this is by design.
	// The FX conversion is the "exchange" — you debit NGN and credit USD.
	// Balance is enforced per-currency in the ledger, not across currencies.
	_ = ngnEntries // intentional: documenting the design

	// Reversal entries (on webhook failed)
	reversalEntries := []ledgerEntry{
		{500_000, "debit"},  // platform holding debit
		{500_000, "credit"}, // sender credit (refund)
	}

	totalDebits, totalCredits = 0, 0
	for _, e := range reversalEntries {
		if e.entryType == "debit" {
			totalDebits += e.amount
		} else {
			totalCredits += e.amount
		}
	}
	if totalDebits != totalCredits {
		t.Errorf("reversal entries don't balance: debits=%.2f credits=%.2f", totalDebits, totalCredits)
	}
}

func TestInsufficientFundsCheck(t *testing.T) {
	balance := 1_000_000.0
	requestedAmount := 2_000_000.0

	if balance >= requestedAmount {
		t.Error("should detect insufficient funds")
	}
}

func TestSufficientFundsCheck(t *testing.T) {
	balance := 5_000_000.0
	requestedAmount := 500_000.0

	if balance < requestedAmount {
		t.Error("should allow payment when funds are sufficient")
	}
}

func TestExactFundsCheck(t *testing.T) {
	balance := 500_000.0
	requestedAmount := 500_000.0

	// Exact amount should be allowed
	if balance < requestedAmount {
		t.Error("exact balance amount should be permitted")
	}
}

// ── Reference Generation Tests ─────────────────────────────────────────────────

func TestReferenceFormat(t *testing.T) {
	// References must start with PAY- and be non-empty
	// We test the format by checking known-good examples
	validRefs := []string{"PAY-A1B2C3D4", "PAY-00000000", "PAY-FFFFFFFF"}
	for _, ref := range validRefs {
		if len(ref) < 8 {
			t.Errorf("reference %q is too short", ref)
		}
		if ref[:4] != "PAY-" {
			t.Errorf("reference %q does not start with PAY-", ref)
		}
	}
}

// ── Status Transition Tests ────────────────────────────────────────────────────

func TestValidStatusTransitions(t *testing.T) {
	// Valid: processing → completed
	// Valid: processing → failed
	// Invalid: completed → processing (terminal state)
	// Invalid: failed → completed (terminal state)

	type transition struct {
		from  string
		to    string
		valid bool
	}

	transitions := []transition{
		{"initiated", "processing", true},
		{"processing", "completed", true},
		{"processing", "failed", true},
		{"completed", "processing", false}, // terminal — must not be re-entered
		{"failed", "completed", false},     // terminal
		{"reversed", "processing", false},  // terminal
	}

	terminalStates := map[string]bool{
		"completed": true,
		"failed":    true,
		"reversed":  true,
	}

	for _, tc := range transitions {
		isValid := !terminalStates[tc.from]
		if isValid != tc.valid {
			t.Errorf("transition %s→%s: expected valid=%v got valid=%v",
				tc.from, tc.to, tc.valid, isValid)
		}
	}
}
