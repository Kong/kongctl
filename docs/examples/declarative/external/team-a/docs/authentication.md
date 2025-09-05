# Customer Analytics API Authentication

Authentication methods for the Customer Analytics API.

## API Key Authentication

Include your API key in the Authorization header:

```http
Authorization: Bearer YOUR_API_KEY
```

## OAuth 2.0 Authentication

For user-facing applications, use OAuth 2.0 with the following scopes:
- `read` - Read customer data and analytics
- `write` - Track events and modify customer data

## Security Best Practices

- Store API keys securely
- Use HTTPS for all requests  
- Rotate keys regularly
- Monitor API usage for anomalies