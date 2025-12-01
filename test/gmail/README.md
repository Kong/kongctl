# Gmail token helper

This folder contains a small helper for minting a Gmail access token and refresh token for the portal e2e tests.

The script expects a **Desktop application** OAuth client JSON (not a Web client). You can download this from Google Cloud Console (APIs & Services → Credentials).

## Usage

1) Place your client secrets JSON (Desktop client) somewhere locally.
2) Run the helper, pointing at that file:

### Example client_secret.json file
```json
{
  "installed": {
    "client_id": "903989593872-xxxxxxxxx.apps.googleusercontent.com",
    "client_secret": "GOCSPX-xxxxxxxxxxx",
    "redirect_uris": [
      "http://localhost",
      "http://localhost:8080/",
      "http://localhost:8080/Callback"
    ],
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token"
  }
}
```

```bash
python test/gmail/refresh-token.py --client-secret /path/to/client_secret.json
```

By default it starts a local loopback listener and opens the browser. If that fails or you prefer copy/paste, use the manual flow:

```bash
python test/gmail/refresh-token.py \
  --client-secret /path/to/client_secret.json \
  --manual
```

To capture the exports into a file for reuse, add `--output-env`:

```bash
python test/gmail/refresh-token.py \
  --client-secret /path/to/client_secret.json \
  --output-env /tmp/gmail-env.sh \
  --gmail-address "<your-test-gmail@example.com>"
```

The output file will contain `export` statements for the refresh token, access token, client id/secret from the JSON, and the Gmail address if provided (otherwise a commented placeholder).

Both flows request the Gmail readonly scope. After you authorize with the test Gmail account, the script prints:
- Masked access and refresh tokens for confirmation
- Use `--output-env` to get the full values written to disk (refresh token is long-lived; set this as `KONGCTL_E2E_GMAIL_REFRESH_TOKEN`)

### Push the generated secrets to GitHub

You can push the values from the generated env file into GitHub repo secrets using the helper script (requires `gh` CLI logged in with repo secret write access):

```bash
# Make the script executable once
chmod +x test/gmail/push-secrets.sh

# Push to the current repo detected by gh repo view
./test/gmail/push-secrets.sh --env-file /tmp/gmail-env.sh

# Or push to an explicit repo
./test/gmail/push-secrets.sh --env-file /tmp/gmail-env.sh --repo kong/kongctl
```

## Notes

- Use an incognito/private browser window if multiple Google accounts are signed in so you can pick the correct inbox.
- If you see `redirect_uri_mismatch`, make sure the client is a Desktop client; Web clients will not work with these flows.
