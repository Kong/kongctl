# SMS API: Authentication Guide

The SMS API supports two methods of authentication to ensure secure access to your account.

## Authentication Methods

### 1. API Key and Secret

The simplest authentication method uses your API key and secret as request parameters.

#### How to Authenticate

Include these parameters in every request:

```bash
curl -X POST https://rest.nexmo.com/sms/json \
  -d "api_key=abcd1234" \
  -d "api_secret=abcdef0123456789" \
  -d "to=447700900000" \
  -d "from=AcmeInc" \
  -d "text=Hello World"
```

#### Security Considerations

- **Never expose credentials**: Don't commit API secrets to version control
- **Use environment variables**: Store credentials securely
- **Rotate regularly**: Change your API secret periodically
- **Use HTTPS**: Always use encrypted connections

### 2. Signature Authentication

For enhanced security, you can use signature-based authentication instead of sending your API secret directly.

#### How It Works

1. Create a signature using your request parameters
2. Include the signature instead of your API secret
3. The server validates the signature using your shared secret

#### Generating a Signature

The signature is an MD5 hash of:
- All request parameters (except `sig`) in alphabetical order
- A timestamp
- Your signature secret

Example in Python:

```python
import hashlib
import time

def generate_signature(params, signature_secret):
    # Remove sig parameter if present
    params = {k: v for k, v in params.items() if k != 'sig'}
    
    # Sort parameters alphabetically
    sorted_params = sorted(params.items())
    
    # Create parameter string
    param_string = '&'.join([f"{k}={v}" for k, v in sorted_params])
    
    # Add signature secret
    signature_string = param_string + signature_secret
    
    # Generate MD5 hash
    return hashlib.md5(signature_string.encode()).hexdigest()

# Example usage
params = {
    'api_key': 'abcd1234',
    'to': '447700900000',
    'from': 'AcmeInc',
    'text': 'Hello World',
    'timestamp': str(int(time.time()))
}

signature = generate_signature(params, 'your_signature_secret')
params['sig'] = signature
```

#### Using Signature Authentication

```bash
curl -X POST https://rest.nexmo.com/sms/json \
  -d "api_key=abcd1234" \
  -d "to=447700900000" \
  -d "from=AcmeInc" \
  -d "text=Hello World" \
  -d "timestamp=1234567890" \
  -d "sig=7a8c1f3d4e5b6a9c8d7e6f5a4b3c2d1e"
```

## Getting Your Credentials

### API Key and Secret

1. Log in to your Vonage Dashboard
2. Navigate to Settings → API Settings
3. Your API key is displayed
4. Click "Show API Secret" to reveal your secret

### Signature Secret

1. In the Dashboard, go to Settings → API Settings
2. Click "Signature Secret"
3. Generate or update your signature secret

## Security Best Practices

### 1. Environment Variables

Store credentials in environment variables:

```bash
export VONAGE_API_KEY="abcd1234"
export VONAGE_API_SECRET="abcdef0123456789"
```

Then use them in your code:

```python
import os

api_key = os.environ.get('VONAGE_API_KEY')
api_secret = os.environ.get('VONAGE_API_SECRET')
```

### 2. Configuration Files

If using configuration files:
- Never commit them to version control
- Use `.gitignore` to exclude them
- Set appropriate file permissions (e.g., 600)

### 3. Key Rotation

Regularly rotate your credentials:
1. Generate new API secret in Dashboard
2. Update your applications
3. Monitor for any failures
4. Remove old credentials

### 4. IP Whitelisting

For additional security:
1. Go to Dashboard → API Settings
2. Add trusted IP addresses
3. Only requests from these IPs will be accepted

## Troubleshooting Authentication

### Common Errors

#### Invalid Credentials (Status 4)
```json
{
  "message-count": "1",
  "messages": [{
    "status": "4",
    "error-text": "Invalid credentials"
  }]
}
```

**Solution**: Verify your API key and secret are correct

#### Invalid Signature (Status 14)
```json
{
  "message-count": "1",
  "messages": [{
    "status": "14",
    "error-text": "Invalid signature"
  }]
}
```

**Solution**: Check your signature generation algorithm

#### Missing Parameters (Status 2)
```json
{
  "message-count": "1",
  "messages": [{
    "status": "2",
    "error-text": "Missing api_key"
  }]
}
```

**Solution**: Ensure all required parameters are included

## Next Steps

- [Error Handling](./errors) - Learn about error codes
- [Getting Started](./getting-started) - Send your first SMS
- [Webhooks](./webhooks) - Set up callbacks