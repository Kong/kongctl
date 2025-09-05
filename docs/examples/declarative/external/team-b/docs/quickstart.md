# Payment Processing API Quick Start

Get started with payment processing in under 10 minutes.

## Step 1: Create a Customer

```bash
curl -X POST "https://api.company.com/payments/v2/customers" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "name": "John Doe"
  }'
```

## Step 2: Add a Payment Method

```bash
curl -X POST "https://api.company.com/payments/v2/customers/customer_123/payment-methods" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "card",
    "card": {
      "number": "4242424242424242",
      "exp_month": 12,
      "exp_year": 2025,
      "cvc": "123"
    }
  }'
```

## Step 3: Process a Payment

```bash
curl -X POST "https://api.company.com/payments/v2/payments" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 2999,
    "currency": "USD",
    "customer_id": "customer_123",
    "payment_method_id": "pm_456"
  }'
```

## Next Steps

- **Set up webhooks** for payment notifications
- **Create subscriptions** for recurring billing
- **Generate invoices** for manual billing
- **Implement refunds** for customer service

🚀 You're ready to start processing payments securely!