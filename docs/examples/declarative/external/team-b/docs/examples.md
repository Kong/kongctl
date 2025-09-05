# Payment Processing API Examples

Code examples for common payment operations.

## JavaScript/Node.js

```javascript
const PaymentAPI = require('@company/payments');
const payments = new PaymentAPI('YOUR_API_KEY');

// Process a payment
const payment = await payments.processPayment({
  amount: 2999,
  currency: 'USD',
  customer_id: 'customer_123',
  payment_method_id: 'pm_456'
});

// Create subscription
const subscription = await payments.createSubscription({
  customer_id: 'customer_123',
  plan_id: 'plan_monthly'
});
```

## Python

```python
from payments import PaymentClient

client = PaymentClient(api_key='YOUR_API_KEY')

# Issue refund
refund = client.refund_payment(
    payment_id='payment_123',
    amount=1000,
    reason='requested_by_customer'
)

# Generate invoice
invoice = client.create_invoice(
    customer_id='customer_456',
    line_items=[{
        'description': 'Service Fee',
        'quantity': 1,
        'unit_amount': 5000
    }]
)
```

## Common Patterns

### Subscription Billing
Automated recurring payments with flexible pricing and trial periods.

### Split Payments
Distribute payments across multiple recipients in marketplace scenarios.

### Payment Recovery
Handle failed payments with retry logic and dunning management.