# SMS API: Getting Started

Welcome to the SMS API! This guide will help you send your first SMS message in just a few minutes.

## Overview

The SMS API allows you to:
- Send SMS messages to phone numbers worldwide
- Receive delivery receipts for sent messages
- Handle inbound SMS messages to your virtual numbers
- Send messages in multiple formats (text, binary, Unicode)

## Prerequisites

Before you begin, you'll need:
1. An API key and secret
2. A sender ID or virtual number (depending on your region)
3. The recipient's phone number in E.164 format

## Quick Start

### 1. Send Your First SMS

Here's a simple example to send an SMS message:

```bash
curl -X POST https://rest.nexmo.com/sms/json \
  -d "api_key=YOUR_API_KEY" \
  -d "api_secret=YOUR_API_SECRET" \
  -d "to=447700900000" \
  -d "from=AcmeInc" \
  -d "text=Hello from the SMS API!"
```

### 2. Understanding the Response

A successful response looks like this:

```json
{
  "message-count": "1",
  "messages": [{
    "to": "447700900000",
    "message-id": "0A0000000123ABCD1",
    "status": "0",
    "remaining-balance": "3.14159265",
    "message-price": "0.03330000",
    "network": "23430"
  }]
}
```

Key fields:
- `status`: "0" means success
- `message-id`: Unique identifier for tracking
- `remaining-balance`: Your account balance after sending

## Supported Regions

The SMS API supports sending to most countries worldwide. However, some regions have specific requirements:

- **USA/Canada**: Requires a Vonage virtual number as sender
- **India**: Requires pre-registered sender IDs and templates
- **Europe**: Alphanumeric sender IDs are generally supported

## Message Types

### Text Messages
Standard SMS messages up to 160 characters:

```bash
curl -X POST https://rest.nexmo.com/sms/json \
  -d "api_key=YOUR_API_KEY" \
  -d "api_secret=YOUR_API_SECRET" \
  -d "to=447700900000" \
  -d "from=AcmeInc" \
  -d "text=Your verification code is: 123456"
```

### Unicode Messages
For non-Latin characters (Arabic, Chinese, etc.):

```bash
curl -X POST https://rest.nexmo.com/sms/json \
  -d "api_key=YOUR_API_KEY" \
  -d "api_secret=YOUR_API_SECRET" \
  -d "to=447700900000" \
  -d "from=AcmeInc" \
  -d "text=你好世界" \
  -d "type=unicode"
```

## Best Practices

1. **Use E.164 Format**: Always format phone numbers with country code (e.g., 447700900000)
2. **Handle Errors**: Check the status field in responses
3. **Monitor Balance**: Track your remaining balance to avoid failed sends
4. **Request Delivery Receipts**: Enable callbacks to track message delivery
5. **Respect Rate Limits**: Default limit is 30 messages per second

## Next Steps

- [Authentication Guide](./authentication) - Learn about API keys and signatures
- [Error Handling](./errors) - Understand error codes and troubleshooting
- [Webhooks](./webhooks) - Set up delivery receipts and inbound SMS
- [API Reference](https://rest.nexmo.com/sms) - Full API documentation

## Need Help?

- Email: devrel@vonage.com
- Documentation: https://developer.nexmo.com/messaging/sms/overview
- Status: https://api.vonage.com/status