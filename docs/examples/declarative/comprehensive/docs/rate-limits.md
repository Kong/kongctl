# SecureBank API Rate Limiting

## Overview

Rate limiting protects our API infrastructure and ensures fair usage across all consumers. Different tiers have different limits based on your subscription level.

## Rate Limit Tiers

### Free Tier
- **100 requests per hour** per API key
- **10 requests per minute** burst limit
- Ideal for development and testing

### Standard Tier
- **1,000 requests per hour** per API key
- **50 requests per minute** burst limit
- Suitable for small to medium applications

### Premium Tier
- **10,000 requests per hour** per API key
- **200 requests per minute** burst limit
- Designed for production applications

### Enterprise Tier
- **Custom limits** based on requirements
- Dedicated infrastructure available
- Contact sales for pricing

## Rate Limit Headers

Every API response includes rate limit information:

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
```

- `X-RateLimit-Limit` - Maximum requests allowed in current window
- `X-RateLimit-Remaining` - Requests remaining in current window
- `X-RateLimit-Reset` - Unix timestamp when the window resets

## Endpoint-Specific Limits

Some endpoints have additional restrictions:

### Payment Endpoints
- `/payments/transfer` - Maximum 10 requests per minute
- `/payments/batch` - Maximum 5 requests per hour

### Bulk Operations
- `/accounts/bulk` - Maximum 100 accounts per request
- `/transactions/export` - Maximum 1 request per hour

## Handling Rate Limits

### 429 Too Many Requests

When rate limited, you'll receive:

```json
{
  "code": "RATE_LIMIT_EXCEEDED",
  "message": "Rate limit exceeded. Please retry after 1640995200",
  "details": {
    "limit": 1000,
    "remaining": 0,
    "reset_at": "2024-01-01T00:00:00Z"
  }
}
```

### Best Practices

1. **Implement exponential backoff**
   ```python
   def make_request_with_retry(url, max_retries=3):
       for attempt in range(max_retries):
           response = requests.get(url)
           if response.status_code == 429:
               wait_time = 2 ** attempt
               time.sleep(wait_time)
           else:
               return response
   ```

2. **Cache responses when possible**
   - Account details rarely change
   - Transaction history can be cached for minutes

3. **Use webhooks for real-time updates**
   - Reduces polling frequency
   - More efficient for both parties

4. **Batch operations**
   - Use bulk endpoints when available
   - Combine multiple operations into single requests

## Monitoring Usage

### Developer Portal Dashboard

View your current usage:
1. Log in to developer portal
2. Navigate to Analytics > API Usage
3. View hourly, daily, and monthly trends

### Programmatic Monitoring

```bash
curl https://api.securebank.com/v2/rate-limit/status \
  -H "X-API-Key: YOUR_API_KEY"
```

Response:
```json
{
  "tier": "standard",
  "current_usage": {
    "hour": 245,
    "minute": 12
  },
  "limits": {
    "hour": 1000,
    "minute": 50
  }
}
```

## Requesting Higher Limits

If you need higher limits:

1. **Optimize your integration first**
   - Implement caching
   - Use batch operations
   - Remove unnecessary requests

2. **Upgrade your tier**
   - Self-service upgrade in developer portal
   - Immediate limit increase

3. **Contact support for custom limits**
   - Email: api-support@securebank.com
   - Include usage patterns and projections

## Rate Limit Exemptions

Certain operations are exempt from rate limiting:
- OAuth token refresh
- Rate limit status endpoint
- Health check endpoint

## Testing Rate Limits

In sandbox environment:
- Use header `X-Force-Rate-Limit: true` to simulate rate limiting
- Test your retry logic without hitting actual limits