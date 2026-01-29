# Codebreakers API: Error Handling

Errors follow a consistent JSON shape:

```
{
  "code": "machine_readable",
  "message": "human readable",
  "requestId": "optional"
}
```

## Error codes
- `unauthorized`: missing or invalid credentials
- `not_found`: game not found or not owned by the caller
- `game_finished`: guesses submitted after game completed
- `invalid_guess`: guess does not match `^[1-6]{4}$`
- `rate_limited`: too many requests
