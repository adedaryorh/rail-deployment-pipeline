app.post('/webhooks/payment-provider', express.raw({ type: 'application/json' }), async (req, res) => {
    const signature = req.headers['x-webhook-signature'];
    const webhookSecret = process.env.WEBHOOK_SECRET;
    const crypto = require('crypto');

    try {
        if (!signature) {
            return res.status(401).send('Missing signature');
        }

        const rawBody = req.body;
        const computedSignature = crypto
            .createHmac('sha256', webhookSecret)
            .update(rawBody)
            .digest('hex');

        if (!crypto.timingSafeEqual(
            Buffer.from(signature),
            Buffer.from(computedSignature)
        )) {
            return res.status(401).send('Invalid signature');
        }

        const payload = JSON.parse(rawBody.toString());

        // 2. Store raw event BEFORE any processing
        await db.query(
            `INSERT INTO webhook_events 
       (provider_reference, payload, headers, received_at, processed) 
       VALUES ($1, $2, $3, NOW(), false)`,
            [payload.reference, JSON.stringify(payload), JSON.stringify(req.headers)]
        );

        // 3. Unknown transaction — return 200 to prevent retries
        const result = await db.query(
            'SELECT * FROM transactions WHERE provider_reference = $1',
            [payload.reference]
        );

        if (!result.rows[0]) {
            return res.status(200).send('OK');
        }

        const tx = result.rows[0];

        // 4. Idempotency — skip if already terminal
        if (tx.status === 'completed' || tx.status === 'failed' || tx.status === 'reversed') {
            return res.status(200).send('OK');
        }

        // 5. Process inside a DB transaction — atomicity guaranteed
        await db.transaction(async (client) => {
            if (payload.status === 'completed') {
                await client.query(
                    'UPDATE transactions SET status = $1, completed_at = NOW() WHERE id = $2',
                    ['completed', tx.id]
                );
                // Use destination_amount — NOT source amount, NOT payload amount
                await client.query(
                    'UPDATE accounts SET balance = balance + $1 WHERE id = $2',
                    [tx.destination_amount, tx.recipient_account_id]
                );
            } else if (payload.status === 'failed') {
                await client.query(
                    'UPDATE transactions SET status = $1 WHERE id = $2',
                    ['failed', tx.id]
                );
                // Reversal uses source amount — returning NGN to sender
                await client.query(
                    'UPDATE accounts SET balance = balance + $1 WHERE id = $2',
                    [tx.source_amount, tx.sender_account_id]
                );
            }
        });

        return res.status(200).send('OK');

    } catch (error) {
        // 6. Always return 200 — log internally, never expose to provider
        console.error('Webhook processing error:', error);
        return res.status(200).send('OK');
    }
});