# Migration Guide: SecureBank API v1 to v2

## Overview

SecureBank API v2 introduces enhanced security, improved performance, and new features. This guide helps you migrate from v1 to v2.

## Key Changes

### 1. Base URL Update

- **v1**: `https://api.securebank.com/v1`
- **v2**: `https://api.securebank.com/v2`

### 2. Authentication Changes

#### Deprecated: Basic Authentication
v1 supported HTTP Basic Authentication. This is removed in v2.

```diff
- curl -u username:password https://api.securebank.com/v1/accounts
+ curl -H "X-API-Key: YOUR_API_KEY" https://api.securebank.com/v2/accounts
```

#### Enhanced OAuth Scopes
New granular scopes in v2:

```diff
- scope=api
+ scope=read write
```

### 3. Response Format Changes

#### Standardized Error Responses

v1 error response:
```json
{
  "error": "Invalid request",
  "error_code": 400
}
```

v2 error response:
```json
{
  "code": "INVALID_REQUEST",
  "message": "Invalid request parameters",
  "details": {
    "field": "account_id",
    "reason": "Invalid format"
  }
}
```

#### Consistent Date Formats

All dates now use ISO 8601 format:

```diff
- "created": "2024-01-15 10:30:00"
+ "created_at": "2024-01-15T10:30:00Z"
```

### 4. Endpoint Changes

#### Renamed Endpoints

| v1 Endpoint | v2 Endpoint | Notes |
|------------|-------------|-------|
| `/account/{id}` | `/accounts/{accountId}` | Pluralized, clearer parameter |
| `/transfer` | `/payments/transfer` | Grouped under payments |
| `/transaction/list` | `/accounts/{accountId}/transactions` | RESTful nesting |

#### Removed Endpoints

- `/account/balance` - Use `/accounts/{accountId}` instead
- `/transaction/search` - Use query parameters on `/transactions`

#### New Endpoints

- `/payments/batch` - Batch payment processing
- `/accounts/{accountId}/statements` - Account statements
- `/webhooks` - Webhook management

### 5. Request Parameter Changes

#### Account Creation

v1:
```json
{
  "type": "checking",
  "currency": "USD",
  "initial_deposit": 100
}
```

v2:
```json
{
  "account_type": "checking",
  "currency": "USD",
  "initial_balance": {
    "amount": 100,
    "currency": "USD"
  }
}
```

#### Transaction Filters

v1: `GET /transactions?from=2024-01-01&to=2024-01-31`

v2: `GET /accounts/{accountId}/transactions?from_date=2024-01-01&to_date=2024-01-31`

### 6. Field Name Changes

| v1 Field | v2 Field | Type Change |
|----------|----------|-------------|
| `id` | `account_id` | No |
| `type` | `account_type` | No |
| `balance` | `balance.amount` | Nested object |
| `created` | `created_at` | ISO 8601 |
| `modified` | `updated_at` | ISO 8601 |

## Migration Strategy

### Phase 1: Preparation (Week 1-2)
1. Review API changes
2. Update authentication method
3. Test in sandbox environment

### Phase 2: Implementation (Week 3-4)
1. Update base URLs
2. Modify request/response handlers
3. Update field mappings

### Phase 3: Testing (Week 5)
1. Run parallel tests (v1 and v2)
2. Compare responses
3. Verify data consistency

### Phase 4: Rollout (Week 6)
1. Deploy to staging
2. Monitor error rates
3. Gradual production rollout

## Code Examples

### Before (v1)

```python
import requests
from requests.auth import HTTPBasicAuth

# Authentication
auth = HTTPBasicAuth('username', 'password')

# Get account
response = requests.get(
    'https://api.securebank.com/v1/account/12345',
    auth=auth
)

account = response.json()
balance = account['balance']
```

### After (v2)

```python
import requests

# Authentication
headers = {
    'X-API-Key': 'YOUR_API_KEY',
    'Content-Type': 'application/json'
}

# Get account
response = requests.get(
    'https://api.securebank.com/v2/accounts/12345',
    headers=headers
)

account = response.json()
balance = account['balance']['amount']
```

## Deprecation Timeline

- **January 2024**: v2 released
- **March 2024**: v1 deprecation announced
- **June 2024**: v1 enters maintenance mode (critical fixes only)
- **December 2024**: v1 End of Life

## Support

### Migration Assistance

- Technical documentation: https://developer.securebank.com/migration
- Support email: api-migration@securebank.com
- Office hours: Tuesdays 2-4 PM EST

### Tools

- **Migration validator**: https://tools.securebank.com/api-validator
- **Response comparator**: Compare v1 and v2 responses
- **SDK updates**: Latest SDKs support both versions during transition