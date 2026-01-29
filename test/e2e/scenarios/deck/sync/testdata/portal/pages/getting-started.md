# Getting Started

The Codebreakers API is protected with OIDC at the Kong gateway. Obtain a token from your identity provider and include it in each request.

## Quick start
1. Authenticate with your IdP and obtain an access token.
2. Start a new game with `POST /games`.
3. Submit guesses with `POST /games/{gameId}/guesses` until you win or run out of attempts.

## Notes
- The gateway injects consumer identity headers; do not send `X-Consumer-Username` yourself.
- The API returns `401` for missing or invalid credentials.
