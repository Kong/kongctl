# Customer Analytics API Quick Start

Get up and running with the Customer Analytics API in under 10 minutes.

## Step 1: Get Your API Key

1. Sign up for a developer account
2. Navigate to the API Keys section  
3. Generate a new API key for the Customer Analytics API
4. Copy and store your API key securely

## Step 2: Test Your Connection

```bash
curl -X GET "https://api.company.com/customer-analytics/v1/health" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

Expected response:
```json
{
  "status": "healthy",
  "version": "1.2.0",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Step 3: Create Your First Customer

```bash
curl -X POST "https://api.company.com/customer-analytics/v1/customers" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "name": "John Doe"
  }'
```

## Step 4: Track Customer Events

```bash
curl -X POST "https://api.company.com/customer-analytics/v1/events" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "customer_123",
    "event_type": "page_view",
    "properties": {
      "page_url": "https://example.com/product/123",
      "page_title": "Product Page"
    }
  }'
```

## Step 5: Query Customer Data

```bash
# Get all customers
curl -X GET "https://api.company.com/customer-analytics/v1/customers" \
  -H "Authorization: Bearer YOUR_API_KEY"

# Get specific customer details
curl -X GET "https://api.company.com/customer-analytics/v1/customers/customer_123" \
  -H "Authorization: Bearer YOUR_API_KEY"

# Get customer events
curl -X GET "https://api.company.com/customer-analytics/v1/customers/customer_123/events" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## Next Steps

- **Create customer segments** for targeted analysis
- **Set up analytics reports** for business insights
- **Implement event tracking** in your application
- **Explore the full API reference** for advanced features

🚀 You're ready to start building with customer analytics!