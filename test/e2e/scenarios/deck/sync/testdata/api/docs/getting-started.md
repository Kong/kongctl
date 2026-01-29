# Codebreakers API: Getting Started

The Codebreakers API is a simple Mastermind-style game with three endpoints.

## Base URL
Use the base URL provided by your Konnect environment, for example:

```
https://api.example.com
```

## Authentication
This API uses OIDC enforced at the gateway. Include your access token:

```
Authorization: Bearer $TOKEN
```

## Start a game
```
curl -X POST https://api.example.com/games \
  -H "Authorization: Bearer $TOKEN"
```

## Get game state
```
curl https://api.example.com/games/1 \
  -H "Authorization: Bearer $TOKEN"
```

## Submit a guess
```
curl -X POST https://api.example.com/games/1/guesses \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"guess":"1234"}'
```
