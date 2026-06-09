# SMS API: Error Handling (Updated)

This guide helps you understand and handle errors when using the SMS API.

> **Updated**: This version includes improved error descriptions and new
> troubleshooting tips.

## Error Response Format

When an error occurs, the API returns a response with a non-zero status code:

```json
{
  "message-count": "1",
  "messages": [{
    "status": "2",
    "error-text": "Missing to param"
  }]
}
```

## Common Error Codes

### Request Errors (1-9)

#### Status 1: Throttled

**Description**: You are sending SMS faster than the account limit.

**Solution**:
- Implement rate limiting in your application
- Default limit is 30 messages per second
- Contact support to increase limits if needed

#### Status 2: Missing Parameters

**Description**: Your request is missing required parameters.

**Solution**: Ensure all required parameters are included:
- `api_key` - Your API key
- `api_secret` or `sig` - Authentication
- `from` - Sender ID or number
- `to` - Recipient number
- `text` - Message content

#### Status 4: Invalid Credentials

**Description**: Your API key and/or secret are incorrect, invalid, or
disabled.

**Solution**:
- Verify credentials in Dashboard
- Check for typos or extra spaces
- Ensure account is active

## Error Handling Best Practices

Implement retry logic with exponential backoff for transient errors (status
codes 1 and 5). Log all errors with relevant context for later analysis.

## Next Steps

- [Getting Started](./getting-started) - Send your first SMS
- [Authentication](./authentication) - Secure your requests
- [Webhooks](./webhooks) - Handle delivery receipts
