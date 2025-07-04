# Known Issues with Declarative Examples

## Current Limitations

1. **Authentication Strategy Types**
   - Only `key_auth` and `openid_connect` are supported
   - `mtls` (Mutual TLS) is not yet supported

2. **Control Plane Configuration**
   - The `cluster_type` field should not be specified (it's not supported in the current SDK)

3. **API Version Gateway Service**
   - The `gateway_service` field in API versions may cause circular dependency issues
   - This feature is still under development

4. **Multi-file Loading**
   - Loading entire resource definitions using `!file` tags (e.g., `apis: !file apis/api.yaml`) may have path resolution issues
   - Workaround: Define resources directly in the main file and use `!file` tags only for specific field values

5. **Label Values**
   - All label values must be strings (not booleans or numbers)
   - Example: Use `user_facing: "true"` not `user_facing: true`

## Testing Examples

To test examples, use:
```bash
./kongctl plan -f <example-file> --pat "$(cat ~/.konnect/<pat-file>)"
```

## Simplified Examples

For testing purposes, simplified versions without problematic fields are recommended:
- Remove `gateway_service` from API versions
- Remove `cluster_type` from control planes
- Use only supported authentication strategy types