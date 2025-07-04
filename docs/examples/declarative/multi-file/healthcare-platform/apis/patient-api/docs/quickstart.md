# Patient API Quick Start Guide

## Prerequisites

Before you begin, ensure you have:
- A verified HealthConnect developer account
- OAuth 2.0 client credentials or mTLS certificate
- Understanding of FHIR standards (recommended)

## Authentication

### OAuth 2.0 Flow

1. **Obtain authorization code:**
```
https://auth.healthconnect.io/authorize?
  client_id=YOUR_CLIENT_ID&
  redirect_uri=YOUR_REDIRECT_URI&
  response_type=code&
  scope=patient/*.read&
  state=RANDOM_STATE
```

2. **Exchange for access token:**
```bash
curl -X POST https://auth.healthconnect.io/token \
  -d "grant_type=authorization_code" \
  -d "code=AUTHORIZATION_CODE" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET"
```

3. **Use access token:**
```bash
curl https://api.healthconnect.io/patient/v1/patients/P1234567890 \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Accept: application/fhir+json"
```

## Your First Request

Get patient demographics:

```bash
curl -X GET https://api.healthconnect.io/patient/v1/patients/P1234567890 \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Accept: application/fhir+json"
```

Response:
```json
{
  "resourceType": "Patient",
  "id": "P1234567890",
  "meta": {
    "versionId": "1",
    "lastUpdated": "2024-01-15T10:30:00Z"
  },
  "name": [{
    "use": "official",
    "family": "Smith",
    "given": ["John", "Michael"]
  }],
  "gender": "male",
  "birthDate": "1980-05-15"
}
```

## Common Use Cases

### 1. List Patient Appointments
```bash
curl https://api.healthconnect.io/patient/v1/patients/P1234567890/appointments?status=scheduled
```

### 2. Get Lab Results
```bash
curl https://api.healthconnect.io/patient/v1/patients/P1234567890/records?type=lab-results
```

### 3. Search Patients (Provider Access)
```bash
curl https://api.healthconnect.io/patient/v1/patients?family=Smith&birthdate=1980-05-15
```

## Rate Limits

- Standard: 100 requests/minute
- Bulk operations: 10 requests/minute
- Search operations: 20 requests/minute

## Error Handling

All errors follow FHIR OperationOutcome format:

```json
{
  "resourceType": "OperationOutcome",
  "issue": [{
    "severity": "error",
    "code": "not-found",
    "details": {
      "text": "Patient P9999999999 not found"
    }
  }]
}
```

## Next Steps

1. Review our [Security Guide](./security)
2. Explore the [API Reference](https://api.healthconnect.io/patient/v1/docs)
3. Join our developer community
4. Subscribe to API updates

## Support

- Email: patient-api-support@healthconnect.io
- Slack: healthconnect-dev.slack.com
- Office Hours: Wednesdays 3-4 PM EST