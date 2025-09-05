# Authentication Guide

This guide covers all authentication methods supported by our platform APIs.

## API Key Authentication

The simplest authentication method for server-to-server communication.

### Getting Your API Key

1. Log in to your developer account
2. Navigate to "API Keys" section
3. Click "Generate New Key"
4. Copy and securely store your key

### Using API Keys

Include your API key in the `Authorization` header:

```http
GET /v2/users/me HTTP/1.1
Host: api.company.com
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

### Best Practices

- **Never expose API keys** in client-side code
- **Rotate keys regularly** (every 90 days recommended)
- **Use different keys** for different environments
- **Monitor key usage** through the developer dashboard

## OAuth 2.0 Authentication

For applications that need to act on behalf of users.

### Authorization Code Flow

1. **Redirect user to authorization server:**
```
https://auth.company.com/oauth/authorize?
  response_type=code&
  client_id=YOUR_CLIENT_ID&
  redirect_uri=YOUR_REDIRECT_URI&
  scope=read+write&
  state=RANDOM_STATE
```

2. **Exchange authorization code for access token:**
```http
POST /oauth/token HTTP/1.1
Host: auth.company.com
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&
code=AUTHORIZATION_CODE&
client_id=YOUR_CLIENT_ID&
client_secret=YOUR_CLIENT_SECRET&
redirect_uri=YOUR_REDIRECT_URI
```

3. **Use access token for API calls:**
```http
GET /v2/users/me HTTP/1.1
Host: api.company.com
Authorization: Bearer ACCESS_TOKEN
```

### Token Refresh

Access tokens expire after 1 hour. Use the refresh token to get a new access token:

```http
POST /oauth/token HTTP/1.1
Host: auth.company.com
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token&
refresh_token=REFRESH_TOKEN&
client_id=YOUR_CLIENT_ID&
client_secret=YOUR_CLIENT_SECRET
```

## JWT Tokens

For session-based authentication in web applications.

### Token Structure

JWT tokens contain three parts separated by dots:
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.
SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

### Validating JWT Tokens

Always validate JWT tokens on the server side:

```javascript
const jwt = require('jsonwebtoken');

function validateToken(token) {
  try {
    const decoded = jwt.verify(token, process.env.JWT_SECRET);
    return { valid: true, user: decoded };
  } catch (error) {
    return { valid: false, error: error.message };
  }
}
```

## Error Handling

### Common Authentication Errors

| Status Code | Error | Description |
|-------------|-------|-------------|
| 401 | `invalid_token` | Token is malformed or expired |
| 401 | `missing_token` | No authentication token provided |
| 403 | `insufficient_scope` | Token doesn't have required permissions |
| 429 | `rate_limit_exceeded` | Too many authentication attempts |

### Example Error Response

```json
{
  "error": "invalid_token",
  "error_description": "The access token expired",
  "error_code": 4011,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Security Recommendations

### Token Storage

- **Web Applications:** Store tokens in secure, httpOnly cookies
- **Mobile Apps:** Use secure keystore/keychain
- **Server Applications:** Use environment variables or secret management

### Token Transmission

- Always use HTTPS for token transmission
- Include tokens in Authorization header, not URL parameters
- Implement proper CORS policies for web applications

### Token Lifecycle

- Set appropriate token expiration times
- Implement automatic token refresh
- Revoke tokens when users log out
- Monitor for suspicious token usage

## Testing Authentication

Use our test endpoints to verify your authentication implementation:

```bash
# Test API key authentication
curl -X GET "https://api.company.com/v2/auth/test" \
  -H "Authorization: Bearer YOUR_API_KEY"

# Expected response for valid token
{
  "authenticated": true,
  "token_type": "api_key",
  "expires_at": null,
  "scopes": ["read", "write"]
}
```

## Need Help?

- **Authentication Issues** - Check our troubleshooting guide
- **Integration Questions** - Contact our developer support
- **Security Concerns** - Reach out to our security team

---

*Last updated: January 15, 2024*