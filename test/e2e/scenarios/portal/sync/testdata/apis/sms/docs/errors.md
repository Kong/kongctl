# SMS API: Error Handling

This guide helps you understand and handle errors when using the SMS API.

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

**Example**:
```json
{
  "status": "2",
  "error-text": "Missing to param"
}
```

**Solution**: Ensure all required parameters are included:
- `api_key` - Your API key
- `api_secret` or `sig` - Authentication
- `from` - Sender ID or number
- `to` - Recipient number
- `text` - Message content

#### Status 3: Invalid Parameters
**Description**: One or more parameter values are invalid.

**Common Causes**:
- Phone number not in E.164 format
- Message text too long
- Invalid sender ID for destination

**Solution**: Validate parameters before sending:
```python
import re

def validate_phone_number(number):
    # E.164 format: 7-15 digits
    pattern = r'^\d{7,15}$'
    return re.match(pattern, number) is not None

def validate_message_length(text, encoding='text'):
    if encoding == 'text':
        return len(text) <= 160
    elif encoding == 'unicode':
        return len(text) <= 70
    return False
```

#### Status 4: Invalid Credentials
**Description**: Your API key and/or secret are incorrect, invalid, or disabled.

**Solution**:
- Verify credentials in Dashboard
- Check for typos or extra spaces
- Ensure account is active
- Regenerate credentials if needed

#### Status 5: Internal Error
**Description**: An error occurred in the platform while processing.

**Solution**:
- Retry with exponential backoff
- If persistent, contact support@nexmo.com
- Include request ID in support ticket

### Billing and Account Errors (8-9)

#### Status 8: Partner Account Barred
**Description**: Your Vonage account has been suspended.

**Solution**: Contact support@nexmo.com immediately

#### Status 9: Partner Quota Violation
**Description**: Insufficient credit to send the message.

**Solution**:
- Check account balance
- Top up your account
- Implement balance monitoring

### Destination Errors (6-7, 29, 33, 40)

#### Status 6: Invalid Message
**Description**: Platform unable to process the message.

**Common Causes**:
- Unrecognized number prefix
- Invalid destination country

#### Status 7: Number Barred
**Description**: The recipient number is blacklisted.

**Solution**:
- Verify the number is correct
- Check if recipient has opted out
- Contact support for unblocking

#### Status 29: Non-Whitelisted Destination
**Description**: Account is in demo mode with destination restrictions.

**Solution**:
- Add number to whitelist in Dashboard
- Or top up account to remove restrictions

### Authentication Errors (14, 32)

#### Status 14: Invalid Signature
**Description**: The signature supplied could not be verified.

**Solution**: Review signature generation:
```python
# Correct signature generation
def generate_signature(params, secret):
    # Sort parameters alphabetically
    sorted_params = sorted(params.items())
    # Exclude 'sig' parameter
    filtered = [(k, v) for k, v in sorted_params if k != 'sig']
    # Create string
    param_string = '&'.join([f"{k}={v}" for k, v in filtered])
    # Add secret and hash
    return hashlib.md5(f"{param_string}{secret}".encode()).hexdigest()
```

### Regional Errors (15, 22)

#### Status 15: Invalid Sender Address
**Description**: Using non-authorized sender ID.

**Common in**:
- North America (requires long number)
- Countries with sender ID registration

**Solution**:
- Use approved sender IDs only
- Purchase virtual number for region
- Check country-specific requirements

## Error Handling Best Practices

### 1. Implement Retry Logic

```python
import time
import random

def send_sms_with_retry(params, max_retries=3):
    retryable_errors = [1, 5]  # Throttled, Internal Error
    
    for attempt in range(max_retries):
        response = send_sms(params)
        status = int(response['messages'][0]['status'])
        
        if status == 0:  # Success
            return response
        
        if status not in retryable_errors:
            # Non-retryable error
            raise Exception(f"SMS failed: {response}")
        
        # Exponential backoff with jitter
        wait_time = (2 ** attempt) + random.uniform(0, 1)
        time.sleep(wait_time)
    
    raise Exception("Max retries exceeded")
```

### 2. Log Errors for Analysis

```python
import logging

def handle_sms_error(response):
    for message in response['messages']:
        if message['status'] != '0':
            logging.error(
                "SMS failed",
                extra={
                    'status': message['status'],
                    'error_text': message.get('error-text'),
                    'to': message.get('to'),
                    'message_id': message.get('message-id')
                }
            )
```

### 3. Monitor Error Rates

Track error patterns to identify issues:
- Sudden increase in auth errors → credential issue
- Regional error patterns → compliance issue
- Consistent throttling → need rate limit adjustment

## Testing Error Scenarios

Use these test numbers to simulate errors:

| Number | Error Simulated |
|--------|----------------|
| 447700900001 | Throttled (Status 1) |
| 447700900002 | Missing Parameters (Status 2) |
| 447700900009 | Insufficient Credit (Status 9) |

## Getting Help

If you encounter persistent errors:

1. Check the [API Status Page](https://api.vonage.com/status)
2. Review your Dashboard for account issues
3. Contact support with:
   - Error message and status code
   - Request parameters (excluding credentials)
   - Message IDs if available
   - Time of occurrence

## Next Steps

- [Getting Started](./getting-started) - Send your first SMS
- [Authentication](./authentication) - Secure your requests
- [Webhooks](./webhooks) - Handle delivery receipts