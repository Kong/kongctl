# Platform Core API Authentication

Comprehensive guide to authenticating with the Platform Core API.

## Authentication Methods

The Platform Core API supports multiple authentication methods:

1. **API Key Authentication** - For server-to-server communication
2. **OAuth 2.0** - For user-facing applications
3. **JWT Tokens** - For session-based authentication

## API Key Authentication

### Getting Your API Key

1. Sign up for a developer account
2. Navigate to the API Keys section
3. Generate a new API key
4. Copy and store securely

### Using API Keys

Include your API key in the `Authorization` header:

```http
GET /v2/users/me HTTP/1.1
Host: api.company.com
Authorization: Bearer your_api_key_here
Content-Type: application/json
```

### Example Request

```javascript
const response = await fetch('https://api.company.com/v2/users/me', {
  headers: {
    'Authorization': 'Bearer your_api_key_here',
    'Content-Type': 'application/json'
  }
});

const user = await response.json();
console.log(user);
```

## OAuth 2.0 Authentication

### Authorization Code Flow

1. **Redirect to Authorization Server**
```
https://auth.company.com/oauth/authorize?
  response_type=code&
  client_id=your_client_id&
  redirect_uri=your_redirect_uri&
  scope=read write&
  state=random_state_value
```

2. **Exchange Code for Token**
```javascript
const tokenResponse = await fetch('https://auth.company.com/oauth/token', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/x-www-form-urlencoded'
  },
  body: new URLSearchParams({
    grant_type: 'authorization_code',
    code: authorization_code,
    client_id: your_client_id,
    client_secret: your_client_secret,
    redirect_uri: your_redirect_uri
  })
});

const tokens = await tokenResponse.json();
// { access_token, refresh_token, token_type, expires_in }
```

3. **Use Access Token**
```javascript
const apiResponse = await fetch('https://api.company.com/v2/users/me', {
  headers: {
    'Authorization': `Bearer ${tokens.access_token}`
  }
});
```

### Token Refresh

```javascript
async function refreshToken(refreshToken) {
  const response = await fetch('https://auth.company.com/oauth/token', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded'
    },
    body: new URLSearchParams({
      grant_type: 'refresh_token',
      refresh_token: refreshToken,
      client_id: your_client_id,
      client_secret: your_client_secret
    })
  });
  
  return await response.json();
}
```

## JWT Tokens

### Login Endpoint

```javascript
const loginResponse = await fetch('https://api.company.com/v2/auth/login', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    username: 'user@example.com',
    password: 'secure_password'
  })
});

const { access_token, refresh_token } = await loginResponse.json();
```

### Token Validation

```javascript
// Server-side token validation
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

### Authentication Errors

```json
{
  "error": "invalid_token",
  "error_description": "The access token expired",
  "error_code": 4011,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

Common authentication error codes:
- `4010` - Missing authentication
- `4011` - Invalid or expired token
- `4012` - Malformed token
- `4030` - Insufficient permissions

### Handling Auth Errors

```javascript
async function makeAuthenticatedRequest(url, token) {
  const response = await fetch(url, {
    headers: {
      'Authorization': `Bearer ${token}`
    }
  });
  
  if (response.status === 401) {
    const error = await response.json();
    
    if (error.error_code === 4011) {
      // Token expired - refresh or re-authenticate
      console.log('Token expired, refreshing...');
      // Handle token refresh
    } else {
      // Other auth error
      console.error('Authentication failed:', error.error_description);
    }
  }
  
  return response;
}
```

## Security Best Practices

### Token Storage
- **Web Apps:** Use secure, httpOnly cookies
- **Mobile Apps:** Use secure keychain/keystore
- **Server Apps:** Environment variables or secret management

### Token Transmission
- Always use HTTPS
- Use Authorization header, not URL parameters
- Implement proper CORS policies

### Token Lifecycle
- Set appropriate expiration times
- Implement automatic refresh
- Revoke tokens on logout
- Monitor for suspicious activity

## Testing Authentication

### Health Check with Auth
```bash
curl -X GET "https://api.company.com/v2/health" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Auth Test Endpoint
```bash
curl -X GET "https://api.company.com/v2/auth/test" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Expected response:
```json
{
  "authenticated": true,
  "token_type": "api_key",
  "user_id": "user-123",
  "scopes": ["read", "write"],
  "expires_at": "2024-01-15T12:30:00Z"
}
```

## Migration Guide

### From v1 to v2

**v1 Authentication (Legacy):**
```javascript
// Old way
const response = await fetch('/v1/auth', {
  method: 'POST',
  body: JSON.stringify({ user: 'email', pass: 'password' })
});
```

**v2 Authentication (Current):**
```javascript
// New way
const response = await fetch('/v2/auth/login', {
  method: 'POST',
  body: JSON.stringify({ username: 'email', password: 'password' })
});
```

## Need Help?

- **Integration Issues** - Check our troubleshooting guide
- **OAuth Setup** - Contact developer support
- **Security Questions** - Reach out to our security team
- **Rate Limits** - Review our rate limiting guide

---

*Last updated: January 15, 2024*