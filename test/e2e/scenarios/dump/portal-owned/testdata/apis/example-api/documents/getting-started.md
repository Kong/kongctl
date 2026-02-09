# Voice API: Getting Started

Welcome to the Voice API! This guide will help you make your first programmable voice call.

## Overview

The Voice API enables you to:
- Make outbound calls to phone numbers worldwide
- Control calls in real-time (mute, transfer, hang up)
- Play audio files or text-to-speech
- Handle inbound calls to your virtual numbers
- Build complex call flows with NCCO

## Prerequisites

Before you begin, you'll need:
1. A Vonage account with Voice API enabled
2. A JWT for authentication (or create one in Dashboard)
3. A virtual number for making/receiving calls
4. An application with voice capabilities

## Quick Start

### 1. Create Your First Call

Here's how to make a simple outbound call that plays text-to-speech:

```bash
curl -X POST https://api.nexmo.com/v1/calls \
  -H "Authorization: Bearer YOUR_JWT" \
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
      "text": "Hello, this is a call from the Voice API"
    }]
  }'
```

### 2. Understanding the Response

A successful response includes:

```json
{
  "uuid": "63f61863-4a51-4f6b-86e1-46edebcf9356",
  "status": "started",
  "direction": "outbound",
  "conversation_uuid": "CON-f972836a-550f-45fa-956c-12a2ab5b7d22"
}
```

Key fields:
- `uuid`: Unique identifier for this call leg
- `status`: Current state of the call
- `conversation_uuid`: Groups related call legs

## NCCO - Call Control Objects

NCCO (Nexmo Call Control Objects) define what happens during a call:

### Basic Actions

#### Talk - Text to Speech
```json
{
  "action": "talk",
  "text": "Welcome to our service. Press 1 for sales, 2 for support.",
  "language": "en-US",
  "style": 0
}
```

#### Stream - Play Audio
```json
{
  "action": "stream",
  "streamUrl": ["https://example.com/welcome.mp3"],
  "loop": 1
}
```

#### Input - Collect DTMF
```json
{
  "action": "input",
  "maxDigits": 1,
  "timeOut": 10,
  "eventUrl": ["https://example.com/webhooks/dtmf"]
}
```

### Example: Interactive Voice Response (IVR)

```json
[
  {
    "action": "talk",
    "text": "Welcome to ACME company. Press 1 for sales, 2 for support.",
    "bargeIn": true
  },
  {
    "action": "input",
    "maxDigits": 1,
    "timeOut": 5,
    "eventUrl": ["https://example.com/webhooks/ivr"]
  }
]
```

## Call Flow Example

Here's a complete example of an outbound call with an answer URL:

```bash
curl -X POST https://api.nexmo.com/v1/calls \
  -H "Authorization: Bearer YOUR_JWT" \
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
    "answer_url": ["https://example.com/webhooks/answer"],
    "event_url": ["https://example.com/webhooks/events"]
  }'
```

Your answer webhook should return NCCO:

```python
from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/webhooks/answer', methods=['POST'])
def answer_webhook():
    ncco = [
        {
            "action": "talk",
            "text": "Hello, you have reached ACME company."
        },
        {
            "action": "stream",
            "streamUrl": ["https://example.com/hold-music.mp3"]
        }
    ]
    return jsonify(ncco)
```

## Call States

Understanding call states helps you build robust applications:

| State | Description |
|-------|-------------|
| `started` | Call initiated |
| `ringing` | Destination is ringing |
| `answered` | Call was answered |
| `completed` | Call ended normally |
| `failed` | Call could not connect |
| `rejected` | Call was rejected |
| `busy` | Destination was busy |
| `timeout` | No answer within timeout |

## Making Different Types of Calls

### Call a Phone Number
```json
{
  "to": [{
    "type": "phone",
    "number": "447700900000"
  }]
}
```

### Call a SIP Endpoint
```json
{
  "to": [{
    "type": "sip",
    "uri": "sip:user@example.com"
  }]
}
```

### Call with DTMF on Answer
```json
{
  "to": [{
    "type": "phone",
    "number": "447700900000",
    "dtmfAnswer": "p*123#"
  }]
}
```

## Best Practices

1. **Use Answer URLs**: More flexible than inline NCCO
2. **Handle All Events**: Monitor call progress via event webhooks
3. **Set Timeouts**: Configure appropriate ringing and length timers
4. **Test Locally**: Use ngrok for webhook development
5. **Handle Errors**: Implement fallback behavior

## Testing Your Implementation

### 1. Test Numbers

Use these numbers for testing different scenarios:
- `447700900001`: Always answers
- `447700900002`: Always busy  
- `447700900003`: Always fails

### 2. Using the Playground

The Voice API Playground lets you test NCCO without coding:
1. Visit https://dashboard.nexmo.com/voice/playground
2. Build your NCCO visually
3. Make test calls

## Next Steps

- [Authentication Guide](./authentication) - Set up JWT authentication
- [NCCO Reference](./ncco) - Build complex call flows
- [Webhooks](./webhooks) - Handle call events
- [Error Handling](./errors) - Troubleshoot common issues

## Need Help?

- Email: devrel@vonage.com
- Documentation: https://developer.nexmo.com/voice/voice-api/overview
- Community: https://developer.vonage.com/community