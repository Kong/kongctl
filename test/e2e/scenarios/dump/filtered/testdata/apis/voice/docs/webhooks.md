# Voice API: Webhooks and Events

Webhooks are essential for building interactive voice applications. They allow your application to receive real-time notifications about call events and control call flow dynamically.

## Overview

The Voice API uses two types of webhooks:

1. **Answer Webhook** - Triggered when a call is answered
2. **Event Webhook** - Triggered for all call state changes

## Answer Webhook

The answer webhook is called when Vonage needs instructions for handling a call. It must return an NCCO (Nexmo Call Control Object) array.

### Setting Up Answer Webhooks

#### 1. Application Level (Recommended)
Configure in your application settings:

```python
# When creating an application
app_config = {
    "name": "My Voice App",
    "capabilities": {
        "voice": {
            "webhooks": {
                "answer_url": {
                    "address": "https://example.com/webhooks/answer",
                    "http_method": "POST"
                }
            }
        }
    }
}
```

#### 2. Per Call
Override application settings for specific calls:

```python
call_request = {
    "to": [{"type": "phone", "number": "447700900000"}],
    "from": {"type": "phone", "number": "447700900001"},
    "answer_url": ["https://example.com/custom-answer"],
    "answer_method": "POST"
}
```

### Answer Webhook Request

Vonage sends this data to your answer webhook:

```json
{
  "from": "447700900001",
  "to": "447700900000",
  "uuid": "aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "conversation_uuid": "CON-aaaaaaaa-bbbb-cccc-dddd-0123456789ab"
}
```

### Answer Webhook Response

Your webhook must return an NCCO array:

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/webhooks/answer', methods=['POST'])
def answer_webhook():
    # Get call details
    call_data = request.get_json()
    from_number = call_data.get('from')
    
    # Return NCCO based on caller
    if from_number in vip_numbers:
        ncco = [
            {
                "action": "talk",
                "text": "Welcome VIP customer. Connecting you to priority support."
            },
            {
                "action": "connect",
                "endpoint": [{
                    "type": "phone",
                    "number": "447700900123"
                }]
            }
        ]
    else:
        ncco = [
            {
                "action": "talk",
                "text": "Welcome. Please hold while we connect you."
            },
            {
                "action": "stream",
                "streamUrl": ["https://example.com/hold-music.mp3"]
            }
        ]
    
    return jsonify(ncco)
```

## Event Webhook

The event webhook receives notifications for all call state changes and events.

### Setting Up Event Webhooks

Configure alongside answer webhooks:

```python
app_config = {
    "name": "My Voice App",
    "capabilities": {
        "voice": {
            "webhooks": {
                "answer_url": {
                    "address": "https://example.com/webhooks/answer",
                    "http_method": "POST"
                },
                "event_url": {
                    "address": "https://example.com/webhooks/events",
                    "http_method": "POST"
                }
            }
        }
    }
}
```

### Event Types

#### Call State Events

```json
{
  "from": "447700900001",
  "to": "447700900000",
  "uuid": "aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "conversation_uuid": "CON-aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "status": "started",
  "direction": "outbound",
  "timestamp": "2020-01-01T12:00:00.000Z"
}
```

Status values:
- `started` - Call initiated
- `ringing` - Destination ringing
- `answered` - Call answered
- `machine` - Answering machine detected
- `complete` - Call ended normally
- `busy` - Line busy
- `cancelled` - Call cancelled
- `failed` - Call failed
- `rejected` - Call rejected
- `timeout` - No answer
- `unanswered` - Call not answered

#### NCCO Action Events

##### Input Events
Triggered by DTMF collection:

```json
{
  "dtmf": "1234",
  "timed_out": false,
  "uuid": "aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "conversation_uuid": "CON-aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "timestamp": "2020-01-01T12:00:00.000Z"
}
```

##### Record Events
Triggered by recording actions:

```json
{
  "start_time": "2020-01-01T12:00:00Z",
  "recording_url": "https://api.nexmo.com/v1/files/aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "size": 12345,
  "recording_uuid": "aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "end_time": "2020-01-01T12:01:00Z",
  "conversation_uuid": "CON-aaaaaaaa-bbbb-cccc-dddd-0123456789ab",
  "timestamp": "2020-01-01T12:01:00.000Z"
}
```

### Handling Events

```python
@app.route('/webhooks/events', methods=['POST'])
def event_webhook():
    event = request.get_json()
    
    # Log all events
    logging.info(f"Voice event: {event}")
    
    # Handle specific events
    if event.get('status') == 'answered':
        handle_call_answered(event)
    elif event.get('status') == 'completed':
        handle_call_completed(event)
    elif event.get('dtmf'):
        handle_dtmf_input(event)
    elif event.get('recording_url'):
        handle_recording_complete(event)
    
    return '', 204

def handle_call_answered(event):
    # Track call duration
    call_id = event['uuid']
    answered_time = event['timestamp']
    cache.set(f"call:{call_id}:answered", answered_time)

def handle_call_completed(event):
    # Calculate call duration
    call_id = event['uuid']
    answered_time = cache.get(f"call:{call_id}:answered")
    if answered_time:
        duration = calculate_duration(answered_time, event['timestamp'])
        save_call_record(call_id, duration, event)
```

## Building Interactive Voice Response (IVR)

Here's a complete IVR example using webhooks:

### Answer Webhook - Initial Menu

```python
@app.route('/webhooks/answer', methods=['POST'])
def answer():
    ncco = [
        {
            "action": "talk",
            "text": "Welcome to ACME company. Press 1 for sales, 2 for support, or 3 for billing.",
            "bargeIn": True  # Allow interruption
        },
        {
            "action": "input",
            "type": ["dtmf"],
            "maxDigits": 1,
            "timeOut": 10,
            "eventUrl": ["https://example.com/webhooks/ivr-input"]
        }
    ]
    return jsonify(ncco)
```

### Input Event Webhook - Handle Menu Selection

```python
@app.route('/webhooks/ivr-input', methods=['POST'])
def handle_ivr_input():
    data = request.get_json()
    selection = data.get('dtmf')
    
    if selection == '1':
        # Sales
        ncco = [
            {
                "action": "talk",
                "text": "Connecting you to our sales team."
            },
            {
                "action": "connect",
                "endpoint": [{
                    "type": "phone",
                    "number": "447700900100",
                    "onAnswer": {
                        "url": "https://example.com/webhooks/agent-whisper",
                        "ringbackTone": "https://example.com/ringback.mp3"
                    }
                }],
                "eventUrl": ["https://example.com/webhooks/connect-events"]
            }
        ]
    elif selection == '2':
        # Support
        ncco = [
            {
                "action": "talk",
                "text": "For technical support, press 1. For billing support, press 2."
            },
            {
                "action": "input",
                "maxDigits": 1,
                "timeOut": 10,
                "eventUrl": ["https://example.com/webhooks/support-menu"]
            }
        ]
    elif selection == '3':
        # Billing
        ncco = handle_billing_request(data['uuid'])
    else:
        # Invalid selection
        ncco = [
            {
                "action": "talk",
                "text": "Invalid selection. Please try again."
            },
            {
                "action": "input",
                "maxDigits": 1,
                "timeOut": 10,
                "eventUrl": ["https://example.com/webhooks/ivr-input"]
            }
        ]
    
    return jsonify(ncco)
```

## Webhook Security

### 1. Validate Webhook Origin

Verify requests are from Vonage:

```python
import hmac
import hashlib

def validate_webhook(request, secret):
    # Get signature from header
    signature = request.headers.get('Authorization')
    if not signature:
        return False
    
    # Remove 'Bearer ' prefix
    signature = signature.replace('Bearer ', '')
    
    # Calculate expected signature
    payload = request.get_data()
    expected = hmac.new(
        secret.encode(),
        payload,
        hashlib.sha256
    ).hexdigest()
    
    # Compare signatures
    return hmac.compare_digest(signature, expected)

@app.route('/webhooks/answer', methods=['POST'])
def secure_answer():
    if not validate_webhook(request, WEBHOOK_SECRET):
        return '', 401
    
    # Process webhook...
```

### 2. Use HTTPS

Always use HTTPS for webhook URLs:
- Encrypts data in transit
- Prevents tampering
- Required for production

### 3. Implement Rate Limiting

Protect against webhook floods:

```python
from flask_limiter import Limiter

limiter = Limiter(
    app,
    key_func=lambda: request.remote_addr,
    default_limits=["100 per minute"]
)

@app.route('/webhooks/events', methods=['POST'])
@limiter.limit("100 per minute")
def rate_limited_webhook():
    # Process webhook...
```

## Webhook Best Practices

### 1. Respond Quickly

Return response within 5 seconds:

```python
import threading
from queue import Queue

webhook_queue = Queue()

@app.route('/webhooks/events', methods=['POST'])
def quick_webhook():
    # Quick validation
    data = request.get_json()
    if not data:
        return '', 400
    
    # Queue for async processing
    webhook_queue.put(data)
    
    # Return immediately
    return '', 204

def process_webhooks():
    while True:
        data = webhook_queue.get()
        try:
            handle_webhook_data(data)
        except Exception as e:
            logging.error(f"Webhook processing error: {e}")
```

### 2. Handle Failures Gracefully

Provide fallback NCCO for errors:

```python
@app.route('/webhooks/answer', methods=['POST'])
def resilient_answer():
    try:
        # Your logic here
        ncco = generate_ncco(request.get_json())
    except Exception as e:
        logging.error(f"Answer webhook error: {e}")
        # Fallback NCCO
        ncco = [
            {
                "action": "talk",
                "text": "We're experiencing technical difficulties. Please try again later."
            }
        ]
    
    return jsonify(ncco)
```

### 3. Log Everything

Comprehensive logging for debugging:

```python
import json
import time

def log_webhook(webhook_type, data):
    log_entry = {
        'timestamp': time.time(),
        'type': webhook_type,
        'uuid': data.get('uuid'),
        'conversation_uuid': data.get('conversation_uuid'),
        'status': data.get('status'),
        'from': data.get('from'),
        'to': data.get('to'),
        'data': json.dumps(data)
    }
    
    logging.info("Webhook received", extra=log_entry)
```

### 4. Test Webhooks Locally

Use ngrok for local development:

```bash
# Install ngrok
brew install ngrok

# Start your local server
python app.py

# Create public tunnel
ngrok http 5000

# Use ngrok URL in Dashboard
# https://abc123.ngrok.io/webhooks/answer
```

## Common Webhook Patterns

### Call Recording with Transcription

```python
@app.route('/webhooks/answer', methods=['POST'])
def record_call():
    ncco = [
        {
            "action": "talk",
            "text": "This call is being recorded for quality purposes."
        },
        {
            "action": "record",
            "eventUrl": ["https://example.com/webhooks/recording"],
            "transcription": {
                "language": "en-US",
                "eventUrl": ["https://example.com/webhooks/transcription"]
            }
        },
        {
            "action": "connect",
            "endpoint": [{"type": "phone", "number": "447700900100"}]
        }
    ]
    return jsonify(ncco)

@app.route('/webhooks/recording', methods=['POST'])
def handle_recording():
    data = request.get_json()
    recording_url = data['recording_url']
    
    # Download and store recording
    store_recording(recording_url, data['recording_uuid'])
    
    return '', 204
```

### Call Transfer with Whisper

```python
@app.route('/webhooks/transfer', methods=['POST'])
def transfer_call():
    data = request.get_json()
    
    ncco = [
        {
            "action": "talk",
            "text": "Please wait while we transfer your call."
        },
        {
            "action": "connect",
            "endpoint": [{
                "type": "phone",
                "number": "447700900200",
                "onAnswer": {
                    "url": "https://example.com/webhooks/whisper"
                }
            }]
        }
    ]
    return jsonify(ncco)

@app.route('/webhooks/whisper', methods=['POST'])
def agent_whisper():
    # Message only the agent hears
    ncco = [
        {
            "action": "talk",
            "text": "Incoming transfer from support queue."
        }
    ]
    return jsonify(ncco)
```

## Next Steps

- [Getting Started](./getting-started) - Make your first call
- [NCCO Reference](./ncco) - Build complex call flows
- [Authentication](./authentication) - Secure your webhooks