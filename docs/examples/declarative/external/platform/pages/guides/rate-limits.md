# Rate Limits Guide

Understanding and working with API rate limits to ensure optimal performance and reliability.

## Overview

Our APIs implement rate limiting to ensure fair usage and system stability. Rate limits vary by authentication method and endpoint.

## Rate Limit Tiers

### API Key Authentication
- **Tier 1 (Free):** 1,000 requests per hour
- **Tier 2 (Pro):** 10,000 requests per hour  
- **Tier 3 (Enterprise):** 100,000 requests per hour

### OAuth 2.0 Authentication
- **Per User:** 5,000 requests per hour
- **Per Application:** 50,000 requests per hour

## Rate Limit Headers

Every API response includes rate limit information in the headers:

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1642678800
X-RateLimit-Window: 3600
```

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Total requests allowed in the time window |
| `X-RateLimit-Remaining` | Requests remaining in current window |
| `X-RateLimit-Reset` | Unix timestamp when the window resets |
| `X-RateLimit-Window` | Window duration in seconds |

## Rate Limit Exceeded Response

When you exceed the rate limit, you'll receive a `429 Too Many Requests` response:

```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1642678800
Retry-After: 3600

{
  "error": "rate_limit_exceeded",
  "error_description": "API rate limit exceeded",
  "error_code": 4290,
  "retry_after": 3600,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Handling Rate Limits

### 1. Monitor Headers

Always check rate limit headers in your application:

```javascript
async function makeApiCall(url, options) {
  const response = await fetch(url, options);
  
  // Check rate limit headers
  const remaining = parseInt(response.headers.get('X-RateLimit-Remaining'));
  const resetTime = parseInt(response.headers.get('X-RateLimit-Reset'));
  
  console.log(`Requests remaining: ${remaining}`);
  console.log(`Reset time: ${new Date(resetTime * 1000)}`);
  
  if (response.status === 429) {
    const retryAfter = parseInt(response.headers.get('Retry-After'));
    throw new RateLimitError(`Rate limit exceeded. Retry after ${retryAfter} seconds`);
  }
  
  return response;
}
```

### 2. Implement Exponential Backoff

When you hit a rate limit, implement exponential backoff:

```javascript
async function apiCallWithRetry(url, options, maxRetries = 3) {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await makeApiCall(url, options);
    } catch (error) {
      if (error instanceof RateLimitError && attempt < maxRetries) {
        const delay = Math.pow(2, attempt) * 1000; // Exponential backoff
        console.log(`Rate limited. Retrying in ${delay}ms...`);
        await new Promise(resolve => setTimeout(resolve, delay));
        continue;
      }
      throw error;
    }
  }
}
```

### 3. Implement Request Queuing

For high-volume applications, implement a request queue:

```javascript
class ApiQueue {
  constructor(requestsPerSecond = 1) {
    this.queue = [];
    this.interval = 1000 / requestsPerSecond;
    this.processing = false;
  }
  
  async addRequest(url, options) {
    return new Promise((resolve, reject) => {
      this.queue.push({ url, options, resolve, reject });
      this.processQueue();
    });
  }
  
  async processQueue() {
    if (this.processing || this.queue.length === 0) return;
    
    this.processing = true;
    
    while (this.queue.length > 0) {
      const { url, options, resolve, reject } = this.queue.shift();
      
      try {
        const result = await makeApiCall(url, options);
        resolve(result);
      } catch (error) {
        reject(error);
      }
      
      // Wait before processing next request
      if (this.queue.length > 0) {
        await new Promise(resolve => setTimeout(resolve, this.interval));
      }
    }
    
    this.processing = false;
  }
}

// Usage
const apiQueue = new ApiQueue(10); // 10 requests per second
const response = await apiQueue.addRequest('/api/endpoint', { method: 'GET' });
```

## Optimization Strategies

### 1. Batch Requests

Some endpoints support batch operations:

```javascript
// Instead of multiple individual requests
// const user1 = await fetch('/v2/users/123');
// const user2 = await fetch('/v2/users/456');
// const user3 = await fetch('/v2/users/789');

// Use batch endpoint
const users = await fetch('/v2/users/batch', {
  method: 'POST',
  body: JSON.stringify({
    user_ids: ['123', '456', '789']
  })
});
```

### 2. Use Caching

Implement caching to reduce API calls:

```javascript
const cache = new Map();
const CACHE_TTL = 5 * 60 * 1000; // 5 minutes

async function getCachedData(key, fetchFunction) {
  const cached = cache.get(key);
  
  if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
    return cached.data;
  }
  
  const data = await fetchFunction();
  cache.set(key, {
    data,
    timestamp: Date.now()
  });
  
  return data;
}

// Usage
const userData = await getCachedData(
  `user_${userId}`,
  () => fetch(`/v2/users/${userId}`)
);
```

### 3. Request Deduplication

Avoid duplicate requests for the same resource:

```javascript
const pendingRequests = new Map();

async function deduplicatedRequest(url, options) {
  const key = `${options.method || 'GET'}:${url}`;
  
  if (pendingRequests.has(key)) {
    return pendingRequests.get(key);
  }
  
  const promise = fetch(url, options);
  pendingRequests.set(key, promise);
  
  try {
    const result = await promise;
    return result;
  } finally {
    pendingRequests.delete(key);
  }
}
```

## Rate Limit Monitoring

### Dashboard Metrics

Monitor your API usage through the developer dashboard:

- **Current Usage** - Real-time request counts
- **Historical Usage** - Usage patterns over time
- **Rate Limit Hits** - When and how often you hit limits
- **Performance Metrics** - Request latency and error rates

### Alerting

Set up alerts for approaching rate limits:

```javascript
function checkRateLimit(response) {
  const remaining = parseInt(response.headers.get('X-RateLimit-Remaining'));
  const limit = parseInt(response.headers.get('X-RateLimit-Limit'));
  const utilization = ((limit - remaining) / limit) * 100;
  
  if (utilization > 80) {
    console.warn(`High rate limit utilization: ${utilization.toFixed(1)}%`);
    // Send alert to monitoring system
    sendAlert('rate_limit_warning', { utilization });
  }
}
```

## Increasing Rate Limits

Need higher rate limits? Here are your options:

### Upgrade Your Plan
- **Pro Plan** - 10x higher limits
- **Enterprise Plan** - 100x higher limits + custom limits available

### Request Temporary Increases
For special events or migrations, contact support for temporary limit increases.

### Optimize Your Usage
- Review your API usage patterns
- Implement caching and batching
- Remove unnecessary API calls

## Best Practices Summary

1. **Always monitor** rate limit headers
2. **Implement exponential backoff** for rate limit errors
3. **Use caching** to reduce API calls
4. **Batch requests** when possible
5. **Queue requests** for high-volume applications
6. **Set up monitoring** and alerting
7. **Plan for peak usage** by upgrading your limits

## Troubleshooting

### Common Issues

**Q: Why am I hitting rate limits with low request volumes?**
A: Check if you're making requests from multiple IP addresses or using multiple API keys for the same account.

**Q: Rate limit headers show incorrect values**
A: Rate limit headers are eventually consistent. During high load, there may be slight delays in header updates.

**Q: How do burst limits work?**
A: Our rate limits allow short bursts above the sustained rate, but extended high usage will trigger limits.

---

*Last updated: January 15, 2024*