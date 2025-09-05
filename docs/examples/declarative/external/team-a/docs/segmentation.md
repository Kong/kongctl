# Customer Segmentation Guide

Create and manage customer segments for targeted analytics and marketing.

## Creating Segments

```bash
curl -X POST "https://api.company.com/customer-analytics/v1/segments" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "High Value Customers",
    "description": "Customers with lifetime value > $500",
    "criteria": {
      "lifetime_value_min": 500,
      "purchase_count_min": 3
    }
  }'
```

## Segment Criteria

Available criteria for customer segmentation:
- **age_min/age_max** - Age range
- **location** - Geographic location
- **purchase_count_min** - Minimum purchases
- **lifetime_value_min** - Minimum lifetime value
- **last_active_days** - Recent activity window

## Managing Segments

- List all segments
- Update segment criteria
- Get customers in a segment
- Delete unused segments

## Best Practices

- Keep segment names descriptive
- Regularly review and update criteria
- Monitor segment size for statistical significance
- Use segments for personalized experiences