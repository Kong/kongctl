# Team B Configuration - Payment Processing API

This configuration demonstrates how Team B manages their Payment Processing API while publishing to an external portal managed by the platform team.

## Overview

Team B is responsible for:
- Payment Processing API development and maintenance
- Secure financial transaction handling
- PCI compliance and security standards
- API documentation and integration support
- Publishing to the shared developer portal (external reference)

## Files Structure

```
team-b/
├── api.yaml                            # API + external portal reference
├── specs/                             # OpenAPI specifications  
│   ├── payment-processing-v2.3.yaml   # Current version
│   └── payment-processing-v2.2.yaml   # Legacy version
├── docs/                             # API documentation
│   ├── overview.md                   # API overview and features
│   ├── quickstart.md                # Getting started guide
│   ├── security.md                  # Security and compliance
│   ├── webhooks.md                  # Payment event notifications
│   └── examples.md                  # Code examples
└── README.md                        # This file
```

## API Configuration

The Payment Processing API provides:

### Core Features (v2.3.0)
- **Payment Processing**: Secure one-time and recurring payments
- **Subscription Management**: Complete recurring billing lifecycle
- **Invoice Generation**: Automated and manual invoice creation
- **Customer Management**: Billing profiles and payment methods
- **Multi-currency Support**: Process payments globally
- **Webhook Notifications**: Real-time payment event updates

### Key Endpoints
- `POST /payments` - Process one-time payments
- `GET /payments` - List and search payments
- `POST /customers` - Create billing customer profiles
- `POST /subscriptions` - Create recurring subscriptions
- `POST /invoices` - Generate invoices
- `POST /webhooks` - Set up event notifications

## External Portal Reference

The configuration includes an external portal reference:

```yaml
# External portal definition
portals:
  - ref: shared-developer-portal
    _external:
      selector:
        matchFields:
          name: "Shared Developer Portal"

# API publication to external portal
publications:
  - ref: payment-processing-to-shared-portal
    portal_id: !ref shared-developer-portal#id
    visibility: public
```

This approach allows Team B to:
1. Reference the platform team's portal without managing it
2. Publish their API to the shared portal automatically
3. Focus on payment processing rather than portal management
4. Maintain security focus on financial operations

## Dependencies

### External Dependencies
- **Shared Developer Portal**: Managed by platform team
- **Platform Core API**: Authentication and user management
- **Payment Processors**: Stripe, PayPal integrations

### Deployment Order
1. Platform team must deploy the shared portal first
2. Team B can then deploy their API configuration
3. API will be automatically published to the shared portal

## Deployment

```bash
# Ensure platform portal exists first
cd ../platform/
kongctl apply portal.yaml api.yaml

# Deploy Team B configuration  
cd ../team-b/
kongctl apply api.yaml
```

## API Features

### Payment Processing
```bash
# Process a payment
curl -X POST "https://api.company.com/payments/v2/payments" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "amount": 2999,
    "currency": "USD", 
    "customer_id": "customer_123",
    "payment_method_id": "pm_456"
  }'
```

### Subscription Billing
```bash
# Create a subscription
curl -X POST "https://api.company.com/payments/v2/subscriptions" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "customer_id": "customer_123",
    "plan_id": "plan_monthly",
    "payment_method_id": "pm_456"
  }'
```

### Invoice Management
```bash
# Generate an invoice
curl -X POST "https://api.company.com/payments/v2/invoices" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "customer_id": "customer_123",
    "line_items": [{
      "description": "Service Fee",
      "quantity": 1,
      "unit_amount": 5000
    }]
  }'
```

### Webhook Setup
```bash
# Set up payment webhooks
curl -X POST "https://api.company.com/payments/v2/webhooks" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "url": "https://your-app.com/webhooks/payments",
    "events": ["payment.succeeded", "payment.failed"]
  }'
```

## Use Cases

### E-commerce Platforms
- Online store checkout and payment processing
- Subscription product billing
- Multi-vendor marketplace payments
- Refund and dispute management

### SaaS Applications
- Monthly/annual subscription billing
- Usage-based pricing models
- Trial period management
- Dunning and payment recovery

### Service Businesses
- Invoice generation and payment collection
- One-time service payments
- Recurring service billing
- Payment method management

## Security & Compliance

### PCI DSS Compliance
- **Level 1 PCI DSS**: Highest level certification
- **Data Security**: All payment data encrypted
- **Tokenization**: Secure payment method storage
- **Access Controls**: Strict permission management

### Security Features
- **Fraud Detection**: Advanced fraud prevention
- **3D Secure**: Additional authentication layer
- **Address Verification**: AVS checks for cards
- **Velocity Limits**: Transaction frequency controls

### Best Practices
- Never store raw payment card data
- Use tokenized payment methods only
- Implement proper SSL/TLS encryption
- Monitor for suspicious transaction patterns

## Version Management

### Current Version (v2.3.0)
- Enhanced subscription management
- Multi-currency support improvements
- Advanced webhook event types
- Improved fraud detection capabilities

### Legacy Version (v2.2.0)
- Basic payment processing
- Simple subscription billing
- Core webhook events
- Maintained for backward compatibility

## Rate Limits

- **Sandbox**: 100 requests/minute
- **Production**: 1000 requests/minute
- **Enterprise**: Custom limits based on agreement
- **Burst**: Up to 2x rate limit for short periods

## Webhook Events

- `payment.succeeded` - Payment completed successfully
- `payment.failed` - Payment failed or declined
- `subscription.created` - New subscription started
- `subscription.cancelled` - Subscription cancelled
- `invoice.paid` - Invoice payment received
- `customer.created` - New customer profile created

## Error Handling

### Common Error Codes
- `4020` - Payment declined by processor
- `4021` - Insufficient funds
- `4022` - Invalid payment method
- `4023` - Fraud detection triggered
- `4024` - Processing limit exceeded

### Error Response Format
```json
{
  "error": "payment_failed",
  "error_description": "The payment method was declined",
  "error_code": 4020,
  "timestamp": "2024-01-15T10:30:00Z",
  "payment_id": "payment_123"
}
```

## Support

- **Team**: Team B
- **Email**: team-b@company.com
- **Payment Issues**: Financial transaction support
- **Security**: PCI compliance and security questions
- **Portal**: Questions about shared portal handled by platform team
- **Emergency**: 24/7 support for payment processing issues

## Monitoring

Team B monitors:
- Payment success/failure rates
- Transaction processing latency
- Fraud detection accuracy
- Subscription billing health
- Webhook delivery success
- PCI compliance status
- Portal publication status

## Testing

### Test Cards
- **Visa**: 4242424242424242 (succeeds)
- **Visa Declined**: 4000000000000002 (declined)
- **Fraud**: 4100000000000019 (fraud detected)

### Test Environment
- Use sandbox API keys for testing
- Test webhooks with ngrok or similar tools
- Verify PCI compliance in test scenarios