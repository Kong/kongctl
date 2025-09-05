# Platform Core API Code Examples

Practical code examples for integrating with the Platform Core API across different programming languages and frameworks.

## JavaScript/Node.js Examples

### Basic API Client

```javascript
class PlatformClient {
  constructor(apiKey, baseUrl = 'https://api.company.com/v2') {
    this.apiKey = apiKey;
    this.baseUrl = baseUrl;
  }

  async request(method, endpoint, data = null) {
    const url = `${this.baseUrl}${endpoint}`;
    
    const options = {
      method,
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json'
      }
    };

    if (data) {
      options.body = JSON.stringify(data);
    }

    const response = await fetch(url, options);
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(`API Error ${response.status}: ${error.error_description}`);
    }

    return await response.json();
  }

  // User methods
  async getCurrentUser() {
    return this.request('GET', '/users/me');
  }

  async getUser(userId) {
    return this.request('GET', `/users/${userId}`);
  }

  // Auth methods
  async login(username, password) {
    return this.request('POST', '/auth/login', { username, password });
  }

  // Config methods
  async getConfig() {
    return this.request('GET', '/config');
  }

  // Health check
  async getHealth() {
    return this.request('GET', '/health');
  }
}

// Usage
const client = new PlatformClient('your_api_key_here');

try {
  const user = await client.getCurrentUser();
  console.log('Current user:', user);
} catch (error) {
  console.error('Error:', error.message);
}
```

### Authentication Flow

```javascript
// OAuth 2.0 Authorization Code Flow
class OAuthClient {
  constructor(clientId, clientSecret, redirectUri) {
    this.clientId = clientId;
    this.clientSecret = clientSecret;
    this.redirectUri = redirectUri;
    this.authUrl = 'https://auth.company.com/oauth';
  }

  getAuthorizationUrl(state) {
    const params = new URLSearchParams({
      response_type: 'code',
      client_id: this.clientId,
      redirect_uri: this.redirectUri,
      scope: 'read write',
      state: state
    });

    return `${this.authUrl}/authorize?${params}`;
  }

  async exchangeCodeForToken(code) {
    const response = await fetch(`${this.authUrl}/token`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded'
      },
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        code: code,
        client_id: this.clientId,
        client_secret: this.clientSecret,
        redirect_uri: this.redirectUri
      })
    });

    return await response.json();
  }

  async refreshToken(refreshToken) {
    const response = await fetch(`${this.authUrl}/token`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded'
      },
      body: new URLSearchParams({
        grant_type: 'refresh_token',
        refresh_token: refreshToken,
        client_id: this.clientId,
        client_secret: this.clientSecret
      })
    });

    return await response.json();
  }
}
```

## Python Examples

### Basic Client

```python
import requests
import json
from typing import Optional, Dict, Any

class PlatformClient:
    def __init__(self, api_key: str, base_url: str = "https://api.company.com/v2"):
        self.api_key = api_key
        self.base_url = base_url
        self.session = requests.Session()
        self.session.headers.update({
            'Authorization': f'Bearer {api_key}',
            'Content-Type': 'application/json'
        })

    def _request(self, method: str, endpoint: str, data: Optional[Dict] = None) -> Dict[str, Any]:
        url = f"{self.base_url}{endpoint}"
        
        kwargs = {'method': method, 'url': url}
        if data:
            kwargs['json'] = data

        response = self.session.request(**kwargs)
        
        if not response.ok:
            error_data = response.json()
            raise Exception(f"API Error {response.status_code}: {error_data.get('error_description')}")
        
        return response.json()

    def get_current_user(self) -> Dict[str, Any]:
        return self._request('GET', '/users/me')

    def get_user(self, user_id: str) -> Dict[str, Any]:
        return self._request('GET', f'/users/{user_id}')

    def login(self, username: str, password: str) -> Dict[str, Any]:
        return self._request('POST', '/auth/login', {
            'username': username,
            'password': password
        })

    def get_config(self) -> Dict[str, Any]:
        return self._request('GET', '/config')

    def get_health(self) -> Dict[str, Any]:
        return self._request('GET', '/health')

# Usage
client = PlatformClient('your_api_key_here')

try:
    user = client.get_current_user()
    print(f"Current user: {user}")
except Exception as e:
    print(f"Error: {e}")
```

### Error Handling

```python
import requests
from requests.adapters import HTTPAdapter
from requests.packages.urllib3.util.retry import Retry

class PlatformClientWithRetry(PlatformClient):
    def __init__(self, api_key: str, base_url: str = "https://api.company.com/v2"):
        super().__init__(api_key, base_url)
        
        # Configure retry strategy
        retry_strategy = Retry(
            total=3,
            backoff_factor=1,
            status_forcelist=[429, 500, 502, 503, 504],
        )
        
        adapter = HTTPAdapter(max_retries=retry_strategy)
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)

    def _request(self, method: str, endpoint: str, data: Optional[Dict] = None) -> Dict[str, Any]:
        try:
            return super()._request(method, endpoint, data)
        except requests.exceptions.RequestException as e:
            if hasattr(e.response, 'status_code'):
                if e.response.status_code == 401:
                    raise AuthenticationError("Authentication failed")
                elif e.response.status_code == 429:
                    raise RateLimitError("Rate limit exceeded")
                elif e.response.status_code >= 500:
                    raise ServerError("Server error")
            raise APIError(f"Request failed: {str(e)}")

class APIError(Exception):
    pass

class AuthenticationError(APIError):
    pass

class RateLimitError(APIError):
    pass

class ServerError(APIError):
    pass
```

## Go Examples

### Basic Client

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type PlatformClient struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

type User struct {
    ID        string    `json:"id"`
    Username  string    `json:"username"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type LoginResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    TokenType    string `json:"token_type"`
    ExpiresIn    int    `json:"expires_in"`
}

func NewPlatformClient(apiKey string) *PlatformClient {
    return &PlatformClient{
        apiKey:  apiKey,
        baseURL: "https://api.company.com/v2",
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *PlatformClient) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
    url := c.baseURL + endpoint
    
    var reqBody io.Reader
    if body != nil {
        jsonData, err := json.Marshal(body)
        if err != nil {
            return nil, err
        }
        reqBody = bytes.NewBuffer(jsonData)
    }

    req, err := http.NewRequest(method, url, reqBody)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    return c.client.Do(req)
}

func (c *PlatformClient) GetCurrentUser() (*User, error) {
    resp, err := c.makeRequest("GET", "/users/me", nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API error: %d", resp.StatusCode)
    }

    var user User
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, err
    }

    return &user, nil
}

func (c *PlatformClient) Login(username, password string) (*LoginResponse, error) {
    loginReq := LoginRequest{
        Username: username,
        Password: password,
    }

    resp, err := c.makeRequest("POST", "/auth/login", loginReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("login failed: %d", resp.StatusCode)
    }

    var loginResp LoginResponse
    if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
        return nil, err
    }

    return &loginResp, nil
}

// Usage
func main() {
    client := NewPlatformClient("your_api_key_here")

    user, err := client.GetCurrentUser()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Current user: %+v\n", user)
}
```

## cURL Examples

### Authentication

```bash
# Login with username/password
curl -X POST "https://api.company.com/v2/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user@example.com",
    "password": "secure_password"
  }'

# Use API key
curl -X GET "https://api.company.com/v2/users/me" \
  -H "Authorization: Bearer YOUR_API_KEY"

# OAuth token exchange
curl -X POST "https://auth.company.com/oauth/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code&code=AUTH_CODE&client_id=CLIENT_ID&client_secret=CLIENT_SECRET&redirect_uri=REDIRECT_URI"
```

### User Management

```bash
# Get current user
curl -X GET "https://api.company.com/v2/users/me" \
  -H "Authorization: Bearer YOUR_TOKEN"

# Get user by ID
curl -X GET "https://api.company.com/v2/users/user_123" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Configuration

```bash
# Get configuration
curl -X GET "https://api.company.com/v2/config" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Health Check

```bash
# Check API health
curl -X GET "https://api.company.com/v2/health" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Webhook Examples

### Express.js Webhook Handler

```javascript
const express = require('express');
const crypto = require('crypto');
const app = express();

app.use(express.raw({ type: 'application/json' }));

app.post('/webhooks/platform', (req, res) => {
  const signature = req.headers['x-platform-signature'];
  const body = req.body;
  
  // Verify signature
  const expectedSignature = 'sha256=' + 
    crypto.createHmac('sha256', process.env.WEBHOOK_SECRET)
          .update(body)
          .digest('hex');
  
  if (!crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expectedSignature))) {
    return res.status(401).send('Invalid signature');
  }
  
  // Parse webhook data
  const event = JSON.parse(body);
  
  console.log('Received webhook:', event);
  
  // Handle different event types
  switch (event.event) {
    case 'user.created':
      handleUserCreated(event.data.user);
      break;
    case 'user.updated':
      handleUserUpdated(event.data.user);
      break;
    case 'token.refreshed':
      handleTokenRefreshed(event.data.token);
      break;
    default:
      console.log('Unknown event type:', event.event);
  }
  
  res.status(200).send('OK');
});

function handleUserCreated(user) {
  console.log('New user created:', user);
  // Send welcome email, update database, etc.
}

function handleUserUpdated(user) {
  console.log('User updated:', user);
  // Update local user cache, sync data, etc.
}

function handleTokenRefreshed(token) {
  console.log('Token refreshed for user:', token.user_id);
  // Update token storage, log activity, etc.
}

app.listen(3000, () => {
  console.log('Webhook server running on port 3000');
});
```

## Error Handling Examples

### JavaScript with Retry Logic

```javascript
async function apiCallWithRetry(apiCall, maxRetries = 3, delay = 1000) {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await apiCall();
    } catch (error) {
      if (attempt === maxRetries) {
        throw error;
      }
      
      // Check if it's a retryable error
      if (error.status === 429 || error.status >= 500) {
        console.log(`Attempt ${attempt} failed, retrying in ${delay}ms...`);
        await new Promise(resolve => setTimeout(resolve, delay));
        delay *= 2; // Exponential backoff
      } else {
        throw error; // Don't retry client errors
      }
    }
  }
}

// Usage
try {
  const user = await apiCallWithRetry(() => client.getCurrentUser());
  console.log('User:', user);
} catch (error) {
  console.error('Failed after all retries:', error);
}
```

## Testing Examples

### Jest Test Suite

```javascript
const PlatformClient = require('./platform-client');

// Mock fetch
global.fetch = jest.fn();

describe('PlatformClient', () => {
  let client;
  
  beforeEach(() => {
    client = new PlatformClient('test-api-key');
    fetch.mockClear();
  });

  test('getCurrentUser returns user data', async () => {
    const mockUser = {
      id: 'user_123',
      username: 'test@example.com',
      name: 'Test User'
    };

    fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockUser
    });

    const user = await client.getCurrentUser();
    
    expect(user).toEqual(mockUser);
    expect(fetch).toHaveBeenCalledWith(
      'https://api.company.com/v2/users/me',
      expect.objectContaining({
        method: 'GET',
        headers: expect.objectContaining({
          'Authorization': 'Bearer test-api-key'
        })
      })
    );
  });

  test('handles API errors correctly', async () => {
    fetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({
        error: 'unauthorized',
        error_description: 'Invalid API key'
      })
    });

    await expect(client.getCurrentUser()).rejects.toThrow('API Error 401: Invalid API key');
  });
});
```

---

*Last updated: January 15, 2024*