# Voice API: Authentication Guide

The Voice API uses JWT (JSON Web Token) authentication to secure access to your voice applications.

## Overview

Unlike the SMS API, the Voice API requires JWT bearer tokens for authentication. This provides:
- Enhanced security with cryptographic signatures
- Fine-grained access control
- Token expiration for additional security

## Creating a JWT

### Method 1: Using the Dashboard

The easiest way to get started:

1. Log in to your Vonage Dashboard
2. Navigate to Voice → Applications
3. Select your application or create a new one
4. Click "Generate JWT"
5. Copy the generated token

### Method 2: Programmatic Generation

For production use, generate JWTs programmatically:

#### Prerequisites
- Your Application ID
- Your Private Key (downloaded when creating the application)

#### Python Example

```python
import jwt
import time
import uuid

def generate_jwt(application_id, private_key_path):
    # Read private key
    with open(private_key_path, 'r') as f:
        private_key = f.read()
    
    # JWT claims
    claims = {
        'iat': int(time.time()),  # Issued at
        'exp': int(time.time()) + 3600,  # Expires in 1 hour
        'jti': str(uuid.uuid4()),  # Unique token ID
        'application_id': application_id
    }
    
    # Generate JWT
    token = jwt.encode(
        claims,
        private_key,
        algorithm='RS256'
    )
    
    return token

# Usage
app_id = 'aaaaaaaa-bbbb-cccc-dddd-0123456789ab'
token = generate_jwt(app_id, 'private.key')
print(f"Bearer {token}")
```

#### Node.js Example

```javascript
const jwt = require('jsonwebtoken');
const fs = require('fs');
const { v4: uuidv4 } = require('uuid');

function generateJWT(applicationId, privateKeyPath) {
    // Read private key
    const privateKey = fs.readFileSync(privateKeyPath);
    
    // JWT claims
    const claims = {
        iat: Math.floor(Date.now() / 1000),
        exp: Math.floor(Date.now() / 1000) + 3600,
        jti: uuidv4(),
        application_id: applicationId
    };
    
    // Generate JWT
    const token = jwt.sign(claims, privateKey, { algorithm: 'RS256' });
    
    return token;
}

// Usage
const appId = 'aaaaaaaa-bbbb-cccc-dddd-0123456789ab';
const token = generateJWT(appId, 'private.key');
console.log(`Bearer ${token}`);
```

## JWT Structure

A Voice API JWT contains these claims:

```json
{
  "iat": 1234567890,
  "exp": 1234571490,
  "jti": "abc123-def456-ghi789",
  "application_id": "aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "sub": "user@example.com",
  "acl": {
    "paths": {
      "/*/calls/**": {},
      "/*/recordings/**": {}
    }
  }
}
```

### Required Claims

| Claim | Description | Example |
|-------|-------------|---------|
| `iat` | Issued at (Unix timestamp) | `1234567890` |
| `exp` | Expiration time (Unix timestamp) | `1234571490` |
| `jti` | Unique token identifier | `"abc123"` |
| `application_id` | Your Vonage application ID | `"aaaa-bbbb-cccc"` |

### Optional Claims

| Claim | Description | Use Case |
|-------|-------------|----------|
| `sub` | Subject (user identifier) | User-specific tokens |
| `acl` | Access Control List | Restrict token permissions |
| `nbf` | Not valid before | Delayed activation |

## Using JWTs in API Calls

Include the JWT in the Authorization header:

```bash
curl -X POST https://api.nexmo.com/v1/calls \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "to": [{
      "type": "phone",
      "number": "447700900000"
    }],
    "from": {
      "type": "phone",
      "number": "447700900001"
    },
    "ncco": [{
      "action": "talk",
      "text": "Hello from Vonage"
    }]
  }'
```

## Managing Applications and Keys

### Creating an Application

1. **Via Dashboard**:
   - Go to Voice → Applications
   - Click "Create Application"
   - Set capabilities (Voice, Messages, etc.)
   - Download private key (IMPORTANT: Can't be retrieved later)

2. **Via API**:
   ```bash
   curl -X POST https://api.nexmo.com/v2/applications \
     -H "Authorization: Basic <base64(api_key:api_secret)>" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "My Voice App",
       "capabilities": {
         "voice": {
           "webhooks": {
             "answer_url": {
               "address": "https://example.com/answer",
               "http_method": "POST"
             },
             "event_url": {
               "address": "https://example.com/events",
               "http_method": "POST"
             }
           }
         }
       }
     }'
   ```

### Key Management Best Practices

1. **Secure Storage**:
   ```python
   # Store private key securely
   import os
   from cryptography.fernet import Fernet
   
   def encrypt_private_key(key_path, password):
       # Read key
       with open(key_path, 'rb') as f:
           private_key = f.read()
       
       # Encrypt
       cipher = Fernet(password)
       encrypted = cipher.encrypt(private_key)
       
       # Store encrypted
       with open(f"{key_path}.enc", 'wb') as f:
           f.write(encrypted)
   ```

2. **Key Rotation**:
   - Generate new keys periodically
   - Update application with new public key
   - Phase out old keys gradually

3. **Environment Variables**:
   ```bash
   export VONAGE_APPLICATION_ID="aaaa-bbbb-cccc"
   export VONAGE_PRIVATE_KEY_PATH="/secure/path/private.key"
   ```

## Token Security

### 1. Short Expiration Times

Keep tokens short-lived:

```python
def generate_short_lived_jwt(application_id, private_key, duration=300):
    """Generate JWT with 5-minute expiration"""
    claims = {
        'iat': int(time.time()),
        'exp': int(time.time()) + duration,  # 5 minutes
        'jti': str(uuid.uuid4()),
        'application_id': application_id
    }
    # ...
```

### 2. Token Refresh

Implement automatic token refresh:

```python
class VoiceClient:
    def __init__(self, app_id, private_key_path):
        self.app_id = app_id
        self.private_key_path = private_key_path
        self._token = None
        self._token_expiry = 0
    
    def get_token(self):
        # Check if token needs refresh
        if time.time() >= self._token_expiry - 60:  # Refresh 1 min early
            self._token = generate_jwt(self.app_id, self.private_key_path)
            self._token_expiry = time.time() + 3600
        
        return self._token
```

### 3. ACL Restrictions

Limit token permissions:

```python
def generate_restricted_jwt(application_id, private_key):
    claims = {
        'iat': int(time.time()),
        'exp': int(time.time()) + 3600,
        'jti': str(uuid.uuid4()),
        'application_id': application_id,
        'acl': {
            'paths': {
                '/*/calls': {
                    'methods': ['POST']  # Only create calls
                }
            }
        }
    }
    # ...
```

## Troubleshooting Authentication

### Common Errors

#### 401 Unauthorized
```json
{
  "type": "UNAUTHORIZED",
  "error_title": "Unauthorized",
  "detail": "Invalid token"
}
```

**Causes**:
- Expired token
- Malformed JWT
- Wrong private key
- Missing Bearer prefix

**Solution**:
```python
# Verify token locally
def verify_jwt(token, public_key):
    try:
        decoded = jwt.decode(
            token,
            public_key,
            algorithms=['RS256']
        )
        print("Token valid:", decoded)
    except jwt.ExpiredSignatureError:
        print("Token expired")
    except jwt.InvalidTokenError as e:
        print(f"Invalid token: {e}")
```

#### 403 Forbidden
```json
{
  "type": "FORBIDDEN",
  "error_title": "Forbidden",
  "detail": "Insufficient permissions"
}
```

**Cause**: Token lacks required permissions

**Solution**: Check ACL claims or use full-permission token

### Debugging Tips

1. **Decode Token**:
   ```bash
   # Decode JWT header and payload (without verification)
   echo "YOUR_JWT" | cut -d. -f1,2 | base64 -d
   ```

2. **Check Expiration**:
   ```python
   import jwt
   
   # Decode without verification
   decoded = jwt.decode(token, options={"verify_signature": False})
   exp_time = datetime.fromtimestamp(decoded['exp'])
   print(f"Token expires at: {exp_time}")
   ```

3. **Validate Locally**:
   Use https://jwt.io to inspect token structure

## Next Steps

- [Getting Started](./getting-started) - Make your first call
- [Error Handling](./errors) - Handle authentication errors
- [NCCO Reference](./ncco) - Build call flows