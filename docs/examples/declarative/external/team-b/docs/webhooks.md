# Payment Webhooks

Real-time notifications for payment events.

## Webhook Events

- `payment.succeeded` - Payment completed successfully
- `payment.failed` - Payment failed or was declined
- `subscription.created` - New subscription started
- `subscription.cancelled` - Subscription was cancelled
- `invoice.paid` - Invoice payment received

## Setting Up Webhooks

```bash
curl -X POST "https://api.company.com/payments/v2/webhooks" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-app.com/webhooks/payments",
    "events": ["payment.succeeded", "payment.failed"]
  }'
```

## Webhook Security

- Verify webhook signatures
- Use HTTPS endpoints only
- Implement idempotency
- Return 200 status codes promptly