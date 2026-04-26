# PayRoute — Cross-Border Payment System

A simplified cross-border payment processing service where Nigerian businesses can send money to international suppliers.

## Tech Stack

- **Backend**: Go + Gin + GORM
- **Database**: PostgreSQL
- **Payment Provider**: Stripe
- **Frontend**: React + Axios
- **Infrastructure**: Docker + docker-compose

## Quick Start

```bash
# 1. Clone and enter project
git clone <repo> && cd payroute

# 2. Copy environment file
cp .env.example .env
# Edit .env with your Stripe test key (optional — system works in simulation mode)

# 3. Start everything
docker-compose up --build

# Frontend: http://localhost:3000
# Backend:  http://localhost:8080
# DB:       localhost:5432
```

## Seed Data

The system auto-seeds on first run:

| Entity | Details |
|--------|---------|
| **ABC Imports** | NGN account: ₦5,000,000 / USD account: $0 |
| **PayRoute Platform** | Holding accounts for ledger balancing |



## API Endpoints

### Payments

```
POST   /payments          Create a new cross-border payment
GET    /payments          List payments (pagination, status filter)
GET    /payments/:id      Get payment details with ledger & FX quote
```

### Webhooks

```
POST   /webhooks/provider   Real provider webhook
POST   /webhooks/simulate   Dev tool: simulate webhook outcome
```

### Utilities

```
GET    /accounts            List business accounts
GET    /fx/quote            Get live FX quote
GET    /health              Health check
```

### Create Payment Example

```bash
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{
    "sender_account_id": "00000000-0000-0000-0000-000000000020",
    "recipient_name": "Acme Corp",
    "recipient_country": "United States",
    "destination_currency": "USD",
    "amount": 500000
  }'
```

### Simulate Webhook (dev)

```bash
curl -X POST http://localhost:8080/webhooks/simulate \
  -H "Content-Type: application/json" \
  -d '{
    "provider_reference": "pi_xxx",
    "status": "completed"
  }'
```

## Payment Lifecycle

```
POST /payments
  → Idempotency check
  → Validate balance (SELECT FOR UPDATE)
  → Generate FX quote (5 min expiry)
  → Lock funds: debit sender → credit platform holding
  → Create transaction (status: processing)
  → Submit to Stripe
  → Return response

POST /webhooks/provider (async)
  → Verify HMAC-SHA256 signature
  → Store raw event
  → completed: credit recipient, debit platform holding
  → failed: reverse debit back to sender
```

## FX Rates (Simulated)

| Pair | Rate |
|------|------|
| NGN → USD | 1/1500 |
| NGN → EUR | 1/1650 |
| NGN → GBP | 1/1900 |

## Account IDs (Seed)

| Account | UUID |
|---------|------|
| ABC Imports NGN | `00000000-0000-0000-0000-000000000020` |
| ABC Imports USD | `00000000-0000-0000-0000-000000000021` |
| Platform NGN Holding | `00000000-0000-0000-0000-000000000010` |
| Platform USD Holding | `00000000-0000-0000-0000-000000000011` |
