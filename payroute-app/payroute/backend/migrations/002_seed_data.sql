
-- Seeds the system with initial businesses and accounts.

-- Platform holding business (internal — not a real client)
INSERT INTO businesses (id, name, created_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'PayRoute Platform', NOW())
ON CONFLICT (id) DO NOTHING;

-- Platform NGN holding account (receives locked funds at initiation)
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'NGN', 0)
ON CONFLICT (id) DO NOTHING;

-- Platform USD holding account (debited at settlement)
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'USD', 0)
ON CONFLICT (id) DO NOTHING;

-- Platform EUR holding account
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'EUR', 0)
ON CONFLICT (id) DO NOTHING;

-- Platform GBP holding account
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000013', '00000000-0000-0000-0000-000000000001', 'GBP', 0)
ON CONFLICT (id) DO NOTHING;

-- ABC Imports — primary test business
INSERT INTO businesses (id, name, created_at)
VALUES ('00000000-0000-0000-0000-000000000002', 'ABC Imports', NOW())
ON CONFLICT (id) DO NOTHING;

-- ABC Imports NGN account — seeded with ₦5,000,000
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000020', '00000000-0000-0000-0000-000000000002', 'NGN', 5000000)
ON CONFLICT (id) DO NOTHING;

-- ABC Imports USD account — empty, receives settlement
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000002', 'USD', 0)
ON CONFLICT (id) DO NOTHING;

-- ABC Imports EUR account
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000022', '00000000-0000-0000-0000-000000000002', 'EUR', 0)
ON CONFLICT (id) DO NOTHING;

-- ABC Imports GBP account
INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000023', '00000000-0000-0000-0000-000000000002', 'GBP', 0)
ON CONFLICT (id) DO NOTHING;

-- Second test business
INSERT INTO businesses (id, name, created_at)
VALUES ('00000000-0000-0000-0000-000000000003', 'Global Traders Ltd', NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000030', '00000000-0000-0000-0000-000000000003', 'NGN', 2000000)
ON CONFLICT (id) DO NOTHING;

INSERT INTO accounts (id, business_id, currency, balance)
VALUES ('00000000-0000-0000-0000-000000000031', '00000000-0000-0000-0000-000000000003', 'USD', 0)
ON CONFLICT (id) DO NOTHING;
