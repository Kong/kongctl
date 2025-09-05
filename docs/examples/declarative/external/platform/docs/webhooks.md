# Platform Core API Webhooks

Set up webhooks to receive real-time notifications about events in your application.

## Overview

Webhooks allow you to receive HTTP POST requests when specific events occur in your account. This enables you to build real-time integrations and automate workflows.

## Supported Events

### User Events
- `user.created` - New user registration
- `user.updated` - User profile changes
- `user.deleted` - User account deletion
- `user.login` - User authentication
- `user.logout` - User session end

### Authentication Events
- `token.created` - New access token issued
- `token.refreshed` - Access token refreshed
- `token.revoked` - Token manually revoked
- `token.expired` - Token expired

### System Events
- `api.rate_limit_exceeded` - Rate limit violation
- `api.error` - API error occurred
- `system.maintenance` - Scheduled maintenance

## Setting Up Webhooks

### 1. Configure Webhook Endpoint

Create an endpoint in your application to receive webhooks:

```javascript
// Express.js example
app.post('/webhooks/platform', (req, res) => {
  const event = req.body;
  
  // Verify webhook signature
  if (!verifyWebhookSignature(req)) {
    return res.status(401).send('Invalid signature');
  }
  
  // Process the event
  handleWebhookEvent(event);
  
  // Return success response
  res.status(200).send('OK');
});
```

### 2. Register Your Webhook

Register your webhook endpoint through the developer dashboard or API:

```bash
curl -X POST "https://api.company.com/v2/webhooks" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-app.com/webhooks/platform",
    "events": ["user.created", "user.updated", "token.created"],
    "active": true
  }'
```

## Webhook Format

### Request Structure

All webhooks are sent as HTTP POST requests with the following structure:

```json
{
  "id": "webhook_123456",
  "event": "user.created",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "user": {
      "id": "user_789",
      "username": "new.user@example.com",
      "name": "New User",
      "created_at": "2024-01-15T10:30:00Z"
    }
  },
  "api_version": "2.1.0"
}
```

### Headers

```http
POST /webhooks/platform HTTP/1.1
Host: your-app.com
Content-Type: application/json
X-Platform-Event: user.created
X-Platform-Signature: sha256=abc123...
X-Platform-Delivery: delivery_456
User-Agent: Platform-Webhooks/1.0
```

## Event Examples

### User Created
```json
{
  "id": "webhook_001",
  "event": "user.created",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "user": {
      "id": "user_789",
      "username": "john.doe@example.com",
      "name": "John Doe",
      "created_at": "2024-01-15T10:30:00Z",
      "permissions": ["read", "write"]
    }
  }
}
```

### Token Refreshed
```json
{
  "id": "webhook_002",
  "event": "token.refreshed",
  "timestamp": "2024-01-15T11:00:00Z",
  "data": {
    "token": {
      "user_id": "user_789",
      "token_type": "access_token",
      "expires_at": "2024-01-15T12:00:00Z",
      "scopes": ["read", "write"]
    }
  }
}
```

### Rate Limit Exceeded
```json
{
  "id": "webhook_003",
  "event": "api.rate_limit_exceeded",
  "timestamp": "2024-01-15T11:15:00Z",
  "data": {
    "rate_limit": {
      "user_id": "user_789",
      "endpoint": "/v2/users/me",
      "limit": 1000,
      "window": 3600,
      "exceeded_by": 50
    }
  }
}
```

## Security

### Signature Verification

All webhooks include an HMAC signature in the `X-Platform-Signature` header:

```javascript
const crypto = require('crypto');

function verifyWebhookSignature(request) {
  const signature = request.headers['x-platform-signature'];
  const body = JSON.stringify(request.body);
  const secret = process.env.WEBHOOK_SECRET;
  
  const expectedSignature = 'sha256=' + 
    crypto.createHmac('sha256', secret)
          .update(body)
          .digest('hex');
  
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expectedSignature)
  );
}
```

### Best Practices

1. **Always verify signatures** before processing webhooks
2. **Use HTTPS endpoints** for webhook URLs
3. **Implement idempotency** using the webhook ID
4. **Return 2xx status codes** quickly to avoid retries
5. **Process webhooks asynchronously** for better performance

## Handling Failures

### Retry Logic

If your endpoint returns a non-2xx status code, we'll retry the webhook:

- **1st retry:** After 15 seconds
- **2nd retry:** After 60 seconds  
- **3rd retry:** After 300 seconds (5 minutes)
- **4th retry:** After 900 seconds (15 minutes)
- **5th retry:** After 3600 seconds (1 hour)

After 5 failed attempts, the webhook is marked as failed and won't be retried.

### Monitoring Failures

```javascript
app.post('/webhooks/platform', async (req, res) => {
  try {
    // Process webhook
    await processWebhook(req.body);
    res.status(200).send('OK');
  } catch (error) {
    console.error('Webhook processing failed:', error);
    
    // Return appropriate error code
    if (error.code === 'TEMPORARY_ERROR') {
      res.status(500).send('Temporary error, please retry');
    } else {
      res.status(400).send('Permanent error, do not retry');
    }
  }
});
```

## Testing Webhooks

### Webhook Test Tool

Test your webhook endpoint with our testing tool:

```bash
curl -X POST "https://api.company.com/v2/webhooks/test" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "webhook_id": "webhook_123",
    "event": "user.created"
  }'
```

### Local Development

For local testing, use tools like ngrok to expose your local server:

```bash
# Expose local port 3000
ngrok http 3000

# Use the ngrok URL in your webhook configuration
# https://abc123.ngrok.io/webhooks/platform
```

## Managing Webhooks

### List Webhooks
```bash
curl -X GET "https://api.company.com/v2/webhooks" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### Update Webhook
```bash
curl -X PUT "https://api.company.com/v2/webhooks/webhook_123" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "events": ["user.created", "user.updated", "user.deleted"],
    "active": true
  }'
```

### Delete Webhook
```bash
curl -X DELETE "https://api.company.com/v2/webhooks/webhook_123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## Troubleshooting

### Common Issues

**Webhooks not being delivered:**
- Check that your endpoint returns 2xx status codes
- Verify your endpoint is accessible from the internet
- Ensure HTTPS is properly configured

**Signature verification failing:**
- Verify you're using the correct webhook secret
- Ensure you're comparing the raw request body
- Check for encoding issues with the signature

**Missing events:**
- Verify the events are enabled for your webhook
- Check that your webhook is marked as active
- Review webhook delivery logs in the dashboard

### Debug Mode

Enable debug logging to troubleshoot webhook issues:

```javascript
app.post('/webhooks/platform', (req, res) => {
  console.log('Webhook received:', {
    headers: req.headers,
    body: req.body,
    signature: req.headers['x-platform-signature']
  });
  
  // Process webhook...
});
```

## Rate Limits

Webhook endpoints are subject to rate limits:
- **Maximum:** 1000 webhooks per minute per endpoint
- **Burst:** Up to 100 webhooks in 10 seconds

If rate limits are exceeded, webhooks will be queued and delivered when the limit resets.

---

*Last updated: January 15, 2024*