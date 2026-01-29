# Codebreakers API: Authentication

All requests must be authenticated with OIDC at the Kong gateway.

## How it works
- Your IdP issues an access token.
- Kong validates the token and maps the user to a Kong Consumer.
- The upstream service trusts the gateway and uses the injected consumer headers.

## Client guidance
- Send the access token in the `Authorization` header.
- Do not set `X-Consumer-Username`; the gateway injects this header.

## Errors
Missing or invalid credentials will return `401` with code `unauthorized`.
