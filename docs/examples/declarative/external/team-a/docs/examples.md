# Customer Analytics API Examples

Practical examples for common use cases.

## JavaScript/Node.js

```javascript
const CustomerAnalytics = require('@company/customer-analytics');
const analytics = new CustomerAnalytics('YOUR_API_KEY');

// Track events
await analytics.trackEvent({
  customer_id: 'customer_123',
  event_type: 'purchase',
  properties: { amount: 99.99 }
});

// Get customer details
const customer = await analytics.getCustomer('customer_123');
```

## Python

```python
from customer_analytics import AnalyticsClient

client = AnalyticsClient(api_key='YOUR_API_KEY')

# Create segment
segment = client.create_segment({
    'name': 'VIP Customers',
    'criteria': {'lifetime_value_min': 1000}
})

# Get analytics report
report = client.get_analytics_report(
    report_type='retention',
    start_date='2024-01-01',
    end_date='2024-01-31'
)
```

## Common Patterns

### Real-time Event Tracking
Track user actions as they happen for immediate insights.

### Batch Analytics Processing  
Process large volumes of historical data efficiently.

### Customer Journey Analysis
Understand the complete customer lifecycle and touchpoints.