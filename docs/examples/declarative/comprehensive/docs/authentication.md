# SecureBank API Authentication Guide

## Overview

The SecureBank API supports two authentication methods:
- **OAuth 2.0** - For user-authorized applications
- **API Key** - For machine-to-machine integrations

## OAuth 2.0 Authentication

### Authorization Flow

1. **Redirect users to authorization endpoint:**
   ```
   https://auth.securebank.com/oauth/authorize?
     client_id=YOUR_CLIENT_ID&
     redirect_uri=YOUR_REDIRECT_URI&
     response_type=code&
     scope=read write&
     state=RANDOM_STATE
   ```

2. **Exchange authorization code for access token:**
   ```bash
   curl -X POST https://auth.securebank.com/oauth/token \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "grant_type=authorization_code" \
     -d "code=AUTHORIZATION_CODE" \
     -d "client_id=YOUR_CLIENT_ID" \
     -d "client_secret=YOUR_CLIENT_SECRET" \
     -d "redirect_uri=YOUR_REDIRECT_URI"
   ```

3. **Use access token in API requests:**
   ```bash
   curl https://api.securebank.com/v2/accounts \
     -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
   ```

### Available Scopes

- `read` - Read access to accounts and transactions
- `write` - Write access for payments and transfers

### Token Expiration

- Access tokens expire after 1 hour
- Refresh tokens expire after 30 days
- Use refresh token to obtain new access token without re-authorization

## API Key Authentication

### Obtaining an API Key

1. Log in to the SecureBank Developer Portal
2. Navigate to Applications > API Keys
3. Click "Generate New API Key"
4. Store the key securely - it won't be shown again

### Using API Keys

Include the API key in the `X-API-Key` header:

```bash
curl https://api.securebank.com/v2/accounts \
  -H "X-API-Key: YOUR_API_KEY"
```

### API Key Limitations

- API keys have read-only access by default
- Contact support to enable write permissions
- Keys can be scoped to specific IP addresses
- Maximum 5 active keys per application

## Security Best Practices

1. **Never expose credentials in client-side code**
2. **Use OAuth 2.0 for user-facing applications**
3. **Rotate API keys regularly**
4. **Implement proper token storage**
5. **Use HTTPS for all API requests**
6. **Validate SSL certificates**

## Error Handling

### 401 Unauthorized

- Invalid or expired credentials
- Missing authentication headers
- Insufficient scopes

### 403 Forbidden

- Valid credentials but insufficient permissions
- IP address restrictions
- Account suspended

## Testing Authentication

Use our sandbox environment for testing:
- Base URL: `https://sandbox.securebank.com/v2`
- Test credentials available in developer portal
- No real transactions processed