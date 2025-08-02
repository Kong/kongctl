# Voice API: NCCO Reference

NCCO (Nexmo Call Control Objects) define the flow of a voice call. This guide covers all NCCO actions and how to build complex call flows.

## Overview

An NCCO is a JSON array of actions that are executed sequentially during a call:

```json
[
  {
    "action": "talk",
    "text": "Welcome to our service"
  },
  {
    "action": "input",
    "eventUrl": ["https://example.com/ivr"]
  }
]
```

## NCCO Actions

### Talk - Text to Speech

Converts text to speech using advanced TTS engines.

```json
{
  "action": "talk",
  "text": "Welcome to ACME corporation. How can we help you today?",
  "language": "en-US",
  "style": 0,
  "level": 0,
  "loop": 1,
  "bargeIn": true
}
```

#### Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `text` | string | **Required** Text to speak (max 1500 chars) | - |
| `language` | string | Language code | `en-US` |
| `style` | integer | Voice style (0-11) | `0` |
| `level` | string | Volume (-1 to 1) | `0` |
| `loop` | integer | Repeat count (0 = infinite) | `1` |
| `bargeIn` | boolean | Allow interruption by DTMF | `false` |

#### Language Support

Common languages:
- `en-US` - English (US)
- `en-GB` - English (UK)
- `es-ES` - Spanish (Spain)
- `fr-FR` - French (France)
- `de-DE` - German
- `it-IT` - Italian
- `ja-JP` - Japanese
- `ko-KR` - Korean
- `pt-BR` - Portuguese (Brazil)
- `zh-CN` - Chinese (Mandarin)

### Stream - Play Audio

Plays audio files into the call.

```json
{
  "action": "stream",
  "streamUrl": [
    "https://example.com/welcome.mp3",
    "https://example.com/fallback.mp3"
  ],
  "level": 0.5,
  "loop": 3,
  "bargeIn": true
}
```

#### Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `streamUrl` | array | **Required** URLs of audio files | - |
| `level` | string | Volume (-1 to 1) | `0` |
| `loop` | integer | Repeat count (0 = infinite) | `1` |
| `bargeIn` | boolean | Allow interruption | `false` |

#### Audio Requirements
- Format: MP3, WAV, OGG
- Sampling rate: 8kHz or 16kHz
- Channels: Mono recommended
- Maximum file size: 100MB

### Input - Collect User Input

Collects DTMF digits or speech from the caller.

```json
{
  "action": "input",
  "type": ["dtmf", "speech"],
  "dtmf": {
    "maxDigits": 4,
    "timeOut": 10,
    "submitOnHash": true
  },
  "speech": {
    "language": "en-US",
    "context": ["sales", "support", "billing"],
    "startTimeout": 5,
    "endOnSilence": 2
  },
  "eventUrl": ["https://example.com/webhooks/input"]
}
```

#### Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `type` | array | Input types: `["dtmf"]` or `["speech"]` or both | `["dtmf"]` |
| `eventUrl` | array | **Required** Webhook for results | - |
| `dtmf.maxDigits` | integer | Maximum digits to collect | `4` |
| `dtmf.timeOut` | integer | Seconds to wait for input | `5` |
| `dtmf.submitOnHash` | boolean | Submit on # key | `false` |
| `speech.language` | string | Speech recognition language | `en-US` |
| `speech.context` | array | Expected words/phrases | - |
| `speech.startTimeout` | integer | Seconds before timeout | `10` |
| `speech.endOnSilence` | number | Seconds of silence to end | `2` |

### Record - Record Audio

Records the call audio.

```json
{
  "action": "record",
  "format": "mp3",
  "split": "conversation",
  "channels": 2,
  "endOnSilence": 3,
  "endOnKey": "#",
  "timeOut": 3600,
  "beepStart": true,
  "eventUrl": ["https://example.com/webhooks/recording"],
  "transcription": {
    "language": "en-US",
    "eventUrl": ["https://example.com/webhooks/transcription"]
  }
}
```

#### Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `format` | string | Audio format: `mp3`, `wav`, `ogg` | `mp3` |
| `split` | string | Recording mode: `conversation`, `legs` | - |
| `channels` | integer | Number of channels (1-32) | `1` |
| `endOnSilence` | integer | Seconds of silence to stop | `0` |
| `endOnKey` | string | DTMF key to stop recording | - |
| `timeOut` | integer | Maximum duration (seconds) | `7200` |
| `beepStart` | boolean | Play beep before recording | `false` |
| `eventUrl` | array | Webhook for recording complete | - |
| `transcription` | object | Enable speech-to-text | - |

### Connect - Connect to Endpoint

Connects the call to another endpoint (phone, SIP, WebSocket).

```json
{
  "action": "connect",
  "endpoint": [
    {
      "type": "phone",
      "number": "447700900000",
      "onAnswer": {
        "url": "https://example.com/agent-whisper",
        "ringbackTone": "https://example.com/ringback.mp3"
      }
    }
  ],
  "from": "447700900001",
  "limit": 7200,
  "machineDetection": "continue",
  "advanced_machine_detection": {
    "behavior": "continue",
    "mode": "detect",
    "beep_timeout": 45
  },
  "eventUrl": ["https://example.com/webhooks/connect-status"],
  "eventType": "synchronous",
  "ringbackTone": "https://example.com/custom-ringback.mp3"
}
```

#### Endpoint Types

##### Phone Endpoint
```json
{
  "type": "phone",
  "number": "447700900000",
  "dtmfAnswer": "1234#"
}
```

##### SIP Endpoint
```json
{
  "type": "sip",
  "uri": "sip:user@example.com",
  "headers": {
    "X-Custom-Header": "value"
  }
}
```

##### WebSocket Endpoint
```json
{
  "type": "websocket",
  "uri": "wss://example.com/socket",
  "content-type": "audio/l16;rate=16000",
  "headers": {
    "Authorization": "Bearer token"
  }
}
```

### Conversation - Create Named Conversation

Creates a reusable conversation for conference calls.

```json
{
  "action": "conversation",
  "name": "team-standup-" + new Date().toISOString().split('T')[0],
  "startOnEnter": true,
  "endOnExit": false,
  "record": true,
  "mute": false,
  "musicOnHoldUrl": ["https://example.com/hold-music.mp3"],
  "canSpeak": ["everyone"],
  "canHear": ["everyone"]
}
```

#### Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `name` | string | **Required** Unique conversation name | - |
| `startOnEnter` | boolean | Start when moderator joins | `false` |
| `endOnExit` | boolean | End when moderator leaves | `false` |
| `record` | boolean | Record conversation | `false` |
| `mute` | boolean | Join muted | `false` |
| `musicOnHoldUrl` | array | Music while alone | - |
| `canSpeak` | array | UUIDs that can speak | `["everyone"]` |
| `canHear` | array | UUIDs that can hear | `["everyone"]` |

### Notify - Send Event

Sends a custom event to your webhook during the call.

```json
{
  "action": "notify",
  "payload": {
    "event": "customer_verified",
    "customer_id": "12345",
    "verified_at": "2024-01-01T12:00:00Z"
  },
  "eventUrl": ["https://example.com/webhooks/notifications"],
  "eventMethod": "POST"
}
```

## Building Complex Call Flows

### Multi-Level IVR System

```json
[
  {
    "action": "talk",
    "text": "Welcome to ACME Bank. For account balance, press 1. For transactions, press 2. For customer service, press 3.",
    "bargeIn": true
  },
  {
    "action": "input",
    "maxDigits": 1,
    "timeOut": 10,
    "eventUrl": ["https://example.com/ivr/main-menu"]
  }
]
```

Handle the selection:

```python
@app.route('/ivr/main-menu', methods=['POST'])
def main_menu():
    selection = request.json['dtmf']
    
    if selection == '1':
        # Account Balance
        ncco = [
            {
                "action": "talk",
                "text": "Please enter your 4-digit PIN."
            },
            {
                "action": "input",
                "maxDigits": 4,
                "timeOut": 10,
                "eventUrl": ["https://example.com/ivr/verify-pin"]
            }
        ]
    elif selection == '2':
        # Transactions
        ncco = [
            {
                "action": "talk",
                "text": "For recent transactions, press 1. For pending transactions, press 2."
            },
            {
                "action": "input",
                "maxDigits": 1,
                "eventUrl": ["https://example.com/ivr/transactions"]
            }
        ]
    elif selection == '3':
        # Customer Service
        ncco = [
            {
                "action": "talk",
                "text": "Connecting you to the next available agent."
            },
            {
                "action": "stream",
                "streamUrl": ["https://example.com/hold-music.mp3"],
                "loop": 0
            },
            {
                "action": "connect",
                "endpoint": [{
                    "type": "phone",
                    "number": "447700900123"
                }],
                "eventUrl": ["https://example.com/ivr/agent-connected"]
            }
        ]
    else:
        # Invalid selection
        ncco = [
            {
                "action": "talk",
                "text": "Invalid selection. Please try again."
            },
            # Loop back to main menu
            {
                "action": "input",
                "maxDigits": 1,
                "eventUrl": ["https://example.com/ivr/main-menu"]
            }
        ]
    
    return jsonify(ncco)
```

### Call Center with Queue

```json
[
  {
    "action": "talk",
    "text": "All agents are busy. You are number"
  },
  {
    "action": "talk",
    "text": "5"  // Dynamic queue position
  },
  {
    "action": "talk",
    "text": "in the queue."
  },
  {
    "action": "notify",
    "payload": {
      "event": "caller_queued",
      "position": 5
    },
    "eventUrl": ["https://example.com/queue/notify"]
  },
  {
    "action": "stream",
    "streamUrl": ["https://example.com/queue-music.mp3"],
    "loop": 0,
    "bargeIn": false
  }
]
```

### Voice Broadcasting

```python
def create_broadcast_ncco(message, recipient_name=None):
    ncco = []
    
    # Personalized greeting if name provided
    if recipient_name:
        ncco.append({
            "action": "talk",
            "text": f"Hello {recipient_name},"
        })
    
    # Main message
    ncco.append({
        "action": "talk",
        "text": message
    })
    
    # Interactive option
    ncco.append({
        "action": "talk",
        "text": "Press 1 to hear this message again, or 2 to speak with a representative."
    })
    
    ncco.append({
        "action": "input",
        "maxDigits": 1,
        "eventUrl": ["https://example.com/broadcast/response"]
    })
    
    return ncco
```

### Advanced Machine Detection

Handle answering machines differently:

```json
[
  {
    "action": "connect",
    "endpoint": [{
      "type": "phone",
      "number": "447700900000"
    }],
    "advanced_machine_detection": {
      "behavior": "continue",
      "mode": "detect_beep"
    },
    "eventUrl": ["https://example.com/amd-result"]
  }
]
```

Handle the detection result:

```python
@app.route('/amd-result', methods=['POST'])
def handle_amd():
    data = request.json
    
    if data.get('machine_detection_behavior') == 'machine':
        # Detected answering machine
        ncco = [
            {
                "action": "talk",
                "text": "Hello, this is ACME company calling. Please call us back at 555-1234."
            }
        ]
    else:
        # Human answered
        ncco = [
            {
                "action": "talk",
                "text": "Hello, may I speak with John Smith?"
            },
            {
                "action": "input",
                "speech": {
                    "language": "en-US"
                },
                "eventUrl": ["https://example.com/verify-person"]
            }
        ]
    
    return jsonify(ncco)
```

## Best Practices

### 1. Use Conditional Logic

Build dynamic flows based on caller data:

```python
def get_personalized_ncco(caller_number):
    customer = lookup_customer(caller_number)
    
    if customer and customer.is_vip:
        return [
            {
                "action": "talk",
                "text": f"Welcome back, {customer.name}. Connecting you to your dedicated agent."
            },
            {
                "action": "connect",
                "endpoint": [{
                    "type": "phone",
                    "number": customer.dedicated_agent_number
                }]
            }
        ]
    else:
        return get_standard_ivr_ncco()
```

### 2. Handle Errors Gracefully

Always provide fallback options:

```python
def safe_ncco_handler(func):
    def wrapper(*args, **kwargs):
        try:
            return func(*args, **kwargs)
        except Exception as e:
            logging.error(f"NCCO generation error: {e}")
            # Return safe fallback
            return jsonify([
                {
                    "action": "talk",
                    "text": "We're experiencing technical difficulties. Please call back later."
                }
            ])
    return wrapper
```

### 3. Optimize for User Experience

- Keep messages concise
- Allow barge-in for experienced users
- Provide clear instructions
- Offer escape options

```json
[
  {
    "action": "talk",
    "text": "For sales, press 1. Support, press 2. Or press 0 to speak with an operator.",
    "bargeIn": true
  },
  {
    "action": "input",
    "maxDigits": 1,
    "timeOut": 5,
    "eventUrl": ["https://example.com/menu"]
  }
]
```

## Testing NCCOs

### NCCO Validator

Validate your NCCO before using:

```python
def validate_ncco(ncco):
    valid_actions = ['talk', 'stream', 'input', 'record', 
                     'connect', 'conversation', 'notify']
    
    if not isinstance(ncco, list):
        raise ValueError("NCCO must be an array")
    
    for i, action in enumerate(ncco):
        if 'action' not in action:
            raise ValueError(f"Action {i} missing 'action' field")
        
        if action['action'] not in valid_actions:
            raise ValueError(f"Invalid action: {action['action']}")
        
        # Validate required fields per action type
        validate_action_fields(action)
    
    return True
```

### Voice Playground

Test NCCOs without coding:
1. Visit Dashboard → Voice → Playground
2. Build NCCO visually
3. Test with real calls
4. Export as JSON

## Next Steps

- [Getting Started](./getting-started) - Make your first call
- [Webhooks](./webhooks) - Handle call events
- [Error Handling](./errors) - Handle NCCO errors