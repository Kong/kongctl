# SMS API: Webhooks

Webhooks allow you to receive real-time notifications about SMS events, including delivery receipts and inbound messages.

## Overview

The SMS API supports two types of webhooks:

1. **Delivery Receipts** - Status updates for sent messages
2. **Inbound SMS** - Messages received on your virtual numbers

## Delivery Receipts

### Setting Up Delivery Receipts

You can configure delivery receipts at two levels:

#### 1. Account Level (Dashboard)
Set a default webhook URL for all SMS:
1. Log in to Dashboard
2. Go to Settings → Account Settings
3. Set "Delivery Receipt Webhook URL"

#### 2. Per Message
Override the default for specific messages:

```bash
curl -X POST https://rest.nexmo.com/sms/json \
  -d "api_key=YOUR_API_KEY" \
  -d "api_secret=YOUR_API_SECRET" \
  -d "to=447700900000" \
  -d "from=AcmeInc" \
  -d "text=Hello World" \
  -d "status-report-req=true" \
  -d "callback=https://example.com/webhooks/delivery"
```

### Delivery Receipt Format

```json
{
  "msisdn": "447700900000",
  "to": "AcmeInc",
  "network-code": "23430",
  "messageId": "0A0000001234567B",
  "price": "0.03330000",
  "status": "delivered",
  "scts": "2001011400",
  "err-code": "0",
  "api-key": "abcd1234",
  "client-ref": "my-reference",
  "message-timestamp": "2020-01-01 12:00:00",
  "timestamp": "1582650446",
  "nonce": "ec11dd3e-1e7f-4db5-9467-82b02cd223b9",
  "sig": "1A20E4E2069B609FDA6CECA9DE18D5CAFE99720DDB628BD6BE8B19942A336E1C"
}
```

### Status Values

| Status | Description | Final? |
|--------|-------------|--------|
| `delivered` | Message delivered to handset | Yes |
| `expired` | Delivery attempt expired | Yes |
| `failed` | Delivery failed | Yes |
| `rejected` | Message rejected by carrier | Yes |
| `accepted` | Message accepted by carrier | No |
| `buffered` | Message queued by carrier | No |
| `unknown` | Status unknown | No |

### Error Codes

The `err-code` field provides additional details:

| Code | Description |
|------|-------------|
| `0` | Delivered successfully |
| `1` | Unknown subscriber |
| `2` | Subscriber busy |
| `3` | Subscriber absent |
| `4` | Subscriber memory full |
| `5` | Routing error |
| `6` | Network error |
| `99` | General error |

## Inbound SMS

### Setting Up Inbound SMS

1. **Purchase a Virtual Number**
   - Use Dashboard or Number API
   - Choose number with SMS capability

2. **Configure Webhook URL**
   - In Dashboard: Numbers → Your Number → SMS Webhook URL
   - Or via API when purchasing

### Inbound SMS Format

```json
{
  "api-key": "abcd1234",
  "msisdn": "447700900001",
  "to": "447700900000",
  "messageId": "0A0000000123ABCD1",
  "text": "Hello world",
  "type": "text",
  "keyword": "HELLO",
  "message-timestamp": "2020-01-01 12:00:00",
  "timestamp": "1578787200",
  "nonce": "aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "concat": "true",
  "concat-ref": "1",
  "concat-total": "3",
  "concat-part": "2"
}
```

### Handling Concatenated Messages

Long messages are split into parts:

```python
# Store message parts
message_parts = {}

def handle_inbound_sms(webhook_data):
    if webhook_data.get('concat') == 'true':
        # Part of a multi-part message
        ref = webhook_data['concat-ref']
        part = int(webhook_data['concat-part'])
        total = int(webhook_data['concat-total'])
        
        if ref not in message_parts:
            message_parts[ref] = {}
        
        message_parts[ref][part] = webhook_data['text']
        
        # Check if all parts received
        if len(message_parts[ref]) == total:
            # Reconstruct full message
            full_text = ''
            for i in range(1, total + 1):
                full_text += message_parts[ref][i]
            
            # Process complete message
            process_message(webhook_data['msisdn'], full_text)
            
            # Clean up
            del message_parts[ref]
    else:
        # Single part message
        process_message(webhook_data['msisdn'], webhook_data['text'])
```

## Webhook Security

### 1. Validate Signatures

Verify webhooks are from Vonage:

```python
import hashlib
import hmac

def validate_webhook_signature(params, signature_secret):
    # Get signature from params
    provided_sig = params.get('sig')
    if not provided_sig:
        return False
    
    # Sort parameters (excluding sig)
    sorted_params = sorted(
        [(k, v) for k, v in params.items() if k != 'sig']
    )
    
    # Build string
    param_string = '&'.join([f"{k}={v}" for k, v in sorted_params])
    
    # Calculate expected signature
    expected_sig = hashlib.md5(
        f"{param_string}{signature_secret}".encode()
    ).hexdigest()
    
    # Compare signatures
    return hmac.compare_digest(provided_sig, expected_sig)
```

### 2. Implement Idempotency

Prevent duplicate processing:

```python
processed_messages = set()

def handle_webhook(data):
    message_id = data.get('messageId')
    
    if message_id in processed_messages:
        # Already processed
        return {'status': 'duplicate'}
    
    # Process webhook
    process_webhook(data)
    
    # Mark as processed
    processed_messages.add(message_id)
    
    return {'status': 'success'}
```

### 3. Use HTTPS

Always use HTTPS endpoints for webhooks:
- Encrypts data in transit
- Prevents tampering
- Required for signature validation

## Webhook Best Practices

### 1. Respond Quickly

- Return 2xx status code immediately
- Process webhooks asynchronously
- Timeout is 5 seconds

```python
from flask import Flask, request
from queue import Queue
import threading

app = Flask(__name__)
webhook_queue = Queue()

@app.route('/webhooks/sms', methods=['POST'])
def webhook_handler():
    # Quick validation
    data = request.get_json()
    if not data:
        return '', 400
    
    # Queue for processing
    webhook_queue.put(data)
    
    # Return immediately
    return '', 200

def process_webhooks():
    while True:
        data = webhook_queue.get()
        # Process webhook
        handle_webhook_data(data)
```

### 2. Handle Retries

Vonage retries failed webhooks:
- Retries for 24 hours
- Exponential backoff
- Must return 2xx to stop retries

### 3. Log Everything

```python
import logging

def log_webhook(data):
    logging.info(
        "Webhook received",
        extra={
            'type': 'inbound' if 'text' in data else 'delivery',
            'message_id': data.get('messageId'),
            'from': data.get('msisdn'),
            'to': data.get('to'),
            'status': data.get('status'),
            'timestamp': data.get('message-timestamp')
        }
    )
```

## Testing Webhooks

### Using ngrok for Local Development

1. Install ngrok: `brew install ngrok`
2. Start local server: `python app.py`
3. Create tunnel: `ngrok http 5000`
4. Use ngrok URL in Dashboard

### Webhook Testing Service

Use webhook.site for quick testing:
1. Visit https://webhook.site
2. Copy unique URL
3. Use as webhook URL
4. View incoming webhooks

## Common Issues

### Not Receiving Webhooks

1. **Check URL is accessible**
   ```bash
   curl -X POST https://your-webhook-url.com/sms \
     -H "Content-Type: application/json" \
     -d '{"test": "data"}'
   ```

2. **Verify webhook configuration**
   - Check Dashboard settings
   - Ensure URL is HTTPS
   - No authentication required

3. **Check firewall/security groups**
   - Allow Vonage IP ranges
   - No rate limiting on webhook endpoint

### Duplicate Webhooks

- Implement idempotency checking
- Store processed message IDs
- Return 200 quickly to prevent retries

## Next Steps

- [Getting Started](./getting-started) - Send your first SMS
- [Authentication](./authentication) - Secure your API requests
- [Error Handling](./errors) - Handle errors gracefully