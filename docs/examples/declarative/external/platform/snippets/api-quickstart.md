# Quick Start Guide

Get up and running with our APIs in under 5 minutes:

## 1. Get Your API Key
```bash
# Get your free API key at: https://developer.company.com/keys
export API_KEY="your_api_key_here"
```

## 2. Make Your First Call
```bash
curl -X GET "https://api.company.com/v2/health" \
  -H "Authorization: Bearer $API_KEY"
```

## 3. Explore the Response
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "2.1.0"
}
```

**Ready to dive deeper?** Check out our [Getting Started Guide](/getting-started) →