# Provider API Integration Guide

## Overview

The Provider API enables healthcare organizations to access comprehensive provider data, manage schedules, and integrate with practice management systems.

## Getting Started

### 1. Authentication Setup

Choose your authentication method based on use case:

**OAuth 2.0 Client Credentials** (Recommended for server-to-server):
```bash
curl -X POST https://auth.healthconnect.io/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "scope=provider:read schedule:read"
```

**Mutual TLS** (For high-security integrations):
- Request mTLS certificate from security team
- Configure your client with certificate and private key
- No additional authentication headers required

### 2. Basic Integration

Search for cardiologists in New York:

```python
import requests

# Get access token (implement proper token caching)
token_response = requests.post(
    "https://auth.healthconnect.io/token",
    data={
        "grant_type": "client_credentials",
        "client_id": CLIENT_ID,
        "client_secret": CLIENT_SECRET,
        "scope": "provider:read"
    }
)
access_token = token_response.json()["access_token"]

# Search providers
headers = {"Authorization": f"Bearer {access_token}"}
response = requests.get(
    "https://api.healthconnect.io/provider/v2/providers",
    headers=headers,
    params={
        "specialty": "cardiology",
        "location": "New York, NY",
        "accepting_new_patients": True
    }
)

providers = response.json()["providers"]
```

### 3. Schedule Integration

Get available appointment slots:

```python
def get_available_slots(provider_id, start_date, end_date):
    response = requests.get(
        f"https://api.healthconnect.io/provider/v2/providers/{provider_id}/schedule",
        headers=headers,
        params={
            "start_date": start_date,
            "end_date": end_date
        }
    )
    
    schedule = response.json()["schedule"]
    available_slots = [slot for slot in schedule if slot["available"]]
    return available_slots
```

## Common Integration Patterns

### 1. Provider Directory

Build a searchable provider directory:

```javascript
class ProviderDirectory {
    constructor(apiClient) {
        this.api = apiClient;
        this.cache = new Map();
    }
    
    async searchProviders(criteria) {
        const cacheKey = JSON.stringify(criteria);
        
        // Check cache (5 minute TTL)
        if (this.cache.has(cacheKey)) {
            const cached = this.cache.get(cacheKey);
            if (Date.now() - cached.timestamp < 300000) {
                return cached.data;
            }
        }
        
        // Fetch from API
        const providers = await this.api.get('/providers', { params: criteria });
        
        // Enrich with additional data
        const enriched = await Promise.all(
            providers.map(async (provider) => ({
                ...provider,
                locations: await this.getProviderLocations(provider.id),
                nextAvailable: await this.getNextAvailableSlot(provider.id)
            }))
        );
        
        // Cache results
        this.cache.set(cacheKey, {
            data: enriched,
            timestamp: Date.now()
        });
        
        return enriched;
    }
}
```

### 2. Appointment Booking

Integrate with scheduling systems:

```python
class AppointmentBooking:
    def __init__(self, api_client, webhook_handler):
        self.api = api_client
        self.webhooks = webhook_handler
        
    def find_appointment(self, criteria):
        """Find available appointment slots matching criteria"""
        providers = self.api.search_providers(
            specialty=criteria['specialty'],
            location=criteria['location'],
            insurance=criteria['insurance']
        )
        
        available_slots = []
        for provider in providers[:10]:  # Limit to top 10
            schedule = self.api.get_schedule(
                provider['id'],
                criteria['start_date'],
                criteria['end_date']
            )
            
            for slot in schedule:
                if slot['available'] and self.matches_criteria(slot, criteria):
                    available_slots.append({
                        'provider': provider,
                        'slot': slot
                    })
        
        return sorted(available_slots, key=lambda x: x['slot']['date'])
    
    def book_appointment(self, provider_id, slot, patient_info):
        """Book appointment through partner system"""
        # This would integrate with your booking system
        booking_result = self.partner_api.create_booking(
            provider_id=provider_id,
            datetime=f"{slot['date']}T{slot['time']}",
            patient=patient_info
        )
        
        # Register for updates
        self.webhooks.subscribe(
            event='schedule.updated',
            provider_id=provider_id,
            callback=self.handle_schedule_change
        )
        
        return booking_result
```

### 3. Data Synchronization

Keep provider data in sync:

```javascript
class ProviderSync {
    async fullSync() {
        console.log('Starting full provider sync...');
        
        let page = 1;
        let hasMore = true;
        
        while (hasMore) {
            const response = await this.api.get('/providers', {
                params: { page, per_page: 100 }
            });
            
            await this.processProviders(response.providers);
            
            hasMore = response.has_next_page;
            page++;
        }
        
        await this.updateSyncTimestamp();
    }
    
    async incrementalSync() {
        const lastSync = await this.getLastSyncTimestamp();
        
        // Use webhooks for real-time updates
        const updates = await this.api.get('/providers/changes', {
            params: { since: lastSync }
        });
        
        for (const change of updates) {
            switch (change.type) {
                case 'created':
                case 'updated':
                    await this.upsertProvider(change.provider);
                    break;
                case 'deactivated':
                    await this.deactivateProvider(change.provider_id);
                    break;
            }
        }
        
        await this.updateSyncTimestamp();
    }
}
```

## Performance Optimization

### 1. Implement Caching

```python
from functools import lru_cache
from datetime import datetime, timedelta

class CachedProviderAPI:
    def __init__(self, api_client):
        self.api = api_client
        
    @lru_cache(maxsize=1000)
    def get_provider(self, provider_id):
        """Cache provider details for 1 hour"""
        return self.api.get(f'/providers/{provider_id}')
    
    def get_schedule(self, provider_id, start_date, end_date):
        """Schedule data should not be cached"""
        return self.api.get(
            f'/providers/{provider_id}/schedule',
            params={'start_date': start_date, 'end_date': end_date}
        )
```

### 2. Batch Requests

```python
async def get_multiple_providers(provider_ids):
    """Fetch multiple providers efficiently"""
    # Use asyncio for concurrent requests
    tasks = [
        get_provider(pid) for pid in provider_ids
    ]
    
    providers = await asyncio.gather(*tasks)
    return providers
```

### 3. Use Field Filtering

```bash
# Only request needed fields
curl "https://api.healthconnect.io/provider/v2/providers?fields=id,name,specialties,locations"
```

## Error Handling

Implement robust error handling:

```python
class ProviderAPIError(Exception):
    pass

def handle_api_request(func):
    def wrapper(*args, **kwargs):
        try:
            return func(*args, **kwargs)
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 429:
                # Rate limited - implement backoff
                retry_after = int(e.response.headers.get('Retry-After', 60))
                time.sleep(retry_after)
                return func(*args, **kwargs)
            elif e.response.status_code == 404:
                return None
            else:
                raise ProviderAPIError(f"API error: {e.response.text}")
        except requests.exceptions.ConnectionError:
            raise ProviderAPIError("Connection failed")
    return wrapper
```

## Testing

### 1. Use Sandbox Environment

```python
# Configure for sandbox
API_BASE_URL = "https://sandbox.healthconnect.io/provider/v2"

# Test data available
TEST_PROVIDERS = [
    "PRV0000000001",  # Dr. Test Smith - Cardiology
    "PRV0000000002",  # Dr. Demo Jones - Pediatrics
]
```

### 2. Mock Responses

```python
# Use for unit testing
MOCK_PROVIDER = {
    "id": "PRV1234567890",
    "npi": "1234567890",
    "name": {
        "first": "John",
        "last": "Smith",
        "title": "MD"
    },
    "specialties": ["cardiology"],
    "accepting_new_patients": True
}
```

## Support Resources

- API Reference: https://api.healthconnect.io/provider/v2/docs
- Postman Collection: https://healthconnect.io/postman/provider-api
- Status Page: https://status.healthconnect.io
- Developer Forum: https://forum.healthconnect.io/provider-api