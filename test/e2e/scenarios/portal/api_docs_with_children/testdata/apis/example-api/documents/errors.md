# Voice API: Error Handling

This guide helps you understand and handle errors when using the Voice API.

## Error Response Format

The Voice API returns errors in a consistent JSON format:

```json
{
  "type": "https://developer.nexmo.com/api-errors#invalid-request",
  "title": "Invalid Request",
  "detail": "The request body did not contain valid JSON",
  "instance": "798b8f199c45014ab7b08bfe9cc1c12c"
}
```

## HTTP Status Codes

### Success Codes

| Code | Description | Used When |
|------|-------------|-----------|
| 200 | OK | GET requests succeed |
| 201 | Created | Call created successfully |
| 204 | No Content | Call updated/deleted successfully |

### Client Error Codes (4xx)

#### 400 Bad Request
Invalid request format or parameters.

**Common Causes**:
- Malformed JSON
- Missing required fields
- Invalid parameter values

**Example**:
```json
{
  "type": "BAD_REQUEST",
  "title": "Bad Request",
  "detail": "Invalid 'to' type. Must be one of: phone, sip, websocket, vbc"
}
```

**Solution**:
```python
# Validate request before sending
def validate_call_request(request):
    required_fields = ['to', 'from']
    
    for field in required_fields:
        if field not in request:
            raise ValueError(f"Missing required field: {field}")
    
    # Validate 'to' format
    for endpoint in request['to']:
        if endpoint['type'] not in ['phone', 'sip', 'websocket', 'vbc']:
            raise ValueError(f"Invalid endpoint type: {endpoint['type']}")
```

#### 401 Unauthorized
Authentication failed.

**Common Causes**:
- Missing Authorization header
- Invalid JWT token
- Expired token

**Example**:
```json
{
  "type": "UNAUTHORIZED",
  "title": "Unauthorized",
  "detail": "Token signature verification failed"
}
```

**Solution**:
```python
import time
import jwt

class TokenManager:
    def __init__(self, app_id, private_key):
        self.app_id = app_id
        self.private_key = private_key
        self.token = None
        self.expiry = 0
    
    def get_valid_token(self):
        # Refresh if expired or about to expire
        if time.time() >= self.expiry - 60:
            self.token = self.generate_token()
            self.expiry = time.time() + 3600
        
        return self.token
    
    def generate_token(self):
        # Generate new JWT
        # ... (implementation)
```

#### 403 Forbidden
Request not allowed.

**Common Causes**:
- Insufficient permissions
- Feature not enabled for account
- Restricted destination

**Example**:
```json
{
  "type": "FORBIDDEN",
  "title": "Forbidden",
  "detail": "Calls to premium numbers are not allowed"
}
```

#### 404 Not Found
Resource doesn't exist.

**Common Causes**:
- Invalid call UUID
- Call already ended
- Wrong endpoint URL

**Example**:
```json
{
  "type": "NOT_FOUND",
  "title": "Not Found",
  "detail": "Call 63f61863-4a51-4f6b-86e1-46edebcf9356 not found"
}
```

### Server Error Codes (5xx)

#### 500 Internal Server Error
Unexpected server error.

**Solution**: Retry with exponential backoff

#### 503 Service Unavailable
Temporary service issue.

**Solution**: Retry after delay

## Common Voice API Errors

### Call Creation Errors

#### Invalid Number Format
```json
{
  "type": "INVALID_REQUEST",
  "title": "Invalid Request",
  "detail": "Number must be in E.164 format"
}
```

**Solution**:
```python
import re

def format_e164(number, country_code=None):
    # Remove all non-digits
    number = re.sub(r'\D', '', number)
    
    # Add country code if needed
    if country_code and not number.startswith(country_code):
        number = country_code + number
    
    # Validate length (7-15 digits)
    if not 7 <= len(number) <= 15:
        raise ValueError(f"Invalid number length: {len(number)}")
    
    return number
```

#### No Answer URL or NCCO
```json
{
  "type": "INVALID_REQUEST",
  "title": "Invalid Request",
  "detail": "Either 'answer_url' or 'ncco' must be provided"
}
```

**Solution**: Always provide either answer_url or ncco:
```python
# Option 1: Using answer_url
call_request = {
    "to": [{"type": "phone", "number": "447700900000"}],
    "from": {"type": "phone", "number": "447700900001"},
    "answer_url": ["https://example.com/answer"]
}

# Option 2: Using inline NCCO
call_request = {
    "to": [{"type": "phone", "number": "447700900000"}],
    "from": {"type": "phone", "number": "447700900001"},
    "ncco": [{"action": "talk", "text": "Hello"}]
}
```

### Call Control Errors

#### Call Not In Progress
```json
{
  "type": "INVALID_REQUEST",
  "title": "Invalid Request",
  "detail": "Call is not in progress"
}
```

**Solution**: Check call status before modifying:
```python
def modify_call_safely(call_uuid, action):
    # Get current call status
    call = get_call_details(call_uuid)
    
    if call['status'] not in ['started', 'ringing', 'answered']:
        raise Exception(f"Cannot modify call in status: {call['status']}")
    
    # Proceed with modification
    update_call(call_uuid, action)
```

### Webhook Errors

#### Invalid NCCO Response
Your answer webhook must return valid NCCO:

**Bad Response**:
```python
@app.route('/answer', methods=['POST'])
def answer():
    return "Hello"  # Wrong - returns string
```

**Good Response**:
```python
@app.route('/answer', methods=['POST'])
def answer():
    ncco = [
        {
            "action": "talk",
            "text": "Hello"
        }
    ]
    return jsonify(ncco)  # Correct - returns JSON array
```

## Error Handling Best Practices

### 1. Implement Retry Logic

```python
import time
import random
from functools import wraps

def retry_with_backoff(max_retries=3, base_delay=1):
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            for attempt in range(max_retries):
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    if attempt == max_retries - 1:
                        raise
                    
                    # Check if error is retryable
                    if hasattr(e, 'status_code'):
                        if e.status_code in [429, 500, 502, 503, 504]:
                            # Exponential backoff with jitter
                            delay = base_delay * (2 ** attempt)
                            jitter = random.uniform(0, delay * 0.1)
                            time.sleep(delay + jitter)
                        else:
                            raise  # Don't retry client errors
                    else:
                        raise
            
        return wrapper
    return decorator

@retry_with_backoff(max_retries=3)
def create_call(call_data):
    # API call implementation
    pass
```

### 2. Log Errors for Analysis

```python
import logging
import json

def log_api_error(error_response, request_data):
    logging.error(
        "Voice API Error",
        extra={
            'error_type': error_response.get('type'),
            'error_title': error_response.get('title'),
            'error_detail': error_response.get('detail'),
            'instance': error_response.get('instance'),
            'request_data': json.dumps(request_data),
            'timestamp': time.time()
        }
    )
```

### 3. Handle Specific Errors

```python
class VoiceAPIClient:
    def handle_api_error(self, response):
        error_data = response.json()
        error_type = error_data.get('type', '')
        
        if 'invalid-request' in error_type:
            raise InvalidRequestError(error_data['detail'])
        elif 'unauthorized' in error_type:
            # Refresh token and retry
            self.refresh_token()
            raise RetryableError("Token refreshed, retry request")
        elif 'forbidden' in error_type:
            raise ForbiddenError(error_data['detail'])
        elif 'not-found' in error_type:
            raise NotFoundError(error_data['detail'])
        else:
            raise VoiceAPIError(error_data)
```

### 4. Graceful Degradation

```python
def make_call_with_fallback(primary_number, fallback_number, message):
    try:
        # Try primary number
        return create_call({
            "to": [{"type": "phone", "number": primary_number}],
            "ncco": [{"action": "talk", "text": message}]
        })
    except Exception as e:
        logging.warning(f"Primary call failed: {e}")
        
        # Try fallback number
        try:
            return create_call({
                "to": [{"type": "phone", "number": fallback_number}],
                "ncco": [{"action": "talk", "text": message}]
            })
        except Exception as e2:
            logging.error(f"Fallback call also failed: {e2}")
            raise
```

## Debugging Tips

### 1. Enable Detailed Logging

```python
import requests
import logging

# Enable debug logging for requests
logging.basicConfig(level=logging.DEBUG)

# Log request/response details
class LoggingSession(requests.Session):
    def request(self, *args, **kwargs):
        logging.debug(f"Request: {args} {kwargs}")
        response = super().request(*args, **kwargs)
        logging.debug(f"Response: {response.status_code} {response.text}")
        return response
```

### 2. Use Request IDs

Track requests with unique IDs:

```python
import uuid

def make_api_call(endpoint, data):
    request_id = str(uuid.uuid4())
    
    headers = {
        'Authorization': f'Bearer {token}',
        'Content-Type': 'application/json',
        'X-Request-ID': request_id
    }
    
    logging.info(f"Making request {request_id} to {endpoint}")
    
    try:
        response = requests.post(endpoint, json=data, headers=headers)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        logging.error(f"Request {request_id} failed: {e}")
        raise
```

### 3. Test Error Scenarios

```python
# Test different error scenarios
test_cases = [
    {
        "name": "Invalid number format",
        "to": [{"type": "phone", "number": "invalid"}],
        "expected_error": "INVALID_REQUEST"
    },
    {
        "name": "Missing required field",
        "to": [],  # Empty 'to' array
        "expected_error": "BAD_REQUEST"
    },
    {
        "name": "Expired token",
        "token": "expired.jwt.token",
        "expected_error": "UNAUTHORIZED"
    }
]

for test in test_cases:
    try:
        result = test_api_call(test)
        print(f"Test '{test['name']}' unexpectedly succeeded")
    except APIError as e:
        if test['expected_error'] in str(e):
            print(f"Test '{test['name']}' passed")
        else:
            print(f"Test '{test['name']}' failed: {e}")
```

## Getting Help

When contacting support, provide:

1. **Request details**:
   - Full request (excluding auth tokens)
   - Response received
   - Timestamp

2. **Error information**:
   - Error type and message
   - Instance ID from error response
   - Call UUID if applicable

3. **Context**:
   - What you were trying to achieve
   - Any recent changes
   - Frequency of the issue

## Next Steps

- [Getting Started](./getting-started) - Make your first call
- [Authentication](./authentication) - Set up JWT tokens
- [Webhooks](./webhooks) - Handle call events