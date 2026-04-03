# Portal Custom Domain Example

This example shows how to configure a portal custom domain with
`domain_verification_method: custom_certificate`.

The certificate and private key are loaded from environment variables with
`!env` so they do not need to be stored in plaintext in the declarative
configuration.

Before running `apply`, replace `portal-dev.your-domain.com` in
`portal-custom-domain.yaml` and in the commands below with a hostname you
control. Konnect rejects hostnames ending in `example.com`, `example.net`, and
`example.org`.

## Files

- `portal-custom-domain.yaml` - creates a portal and configures a custom
  domain. The `ssl.custom_certificate` and `ssl.custom_private_key` fields use
  `!env`.

## Usage

Generate a temporary self-signed PEM certificate and private key for local
testing:

```bash
CUSTOM_DOMAIN="portal-dev.your-domain.com"

openssl req -x509 -nodes -newkey rsa:2048 \
  -keyout portal-custom-domain.key \
  -out portal-custom-domain.crt \
  -days 365 \
  -subj "/CN=${CUSTOM_DOMAIN}" \
  -addext "subjectAltName=DNS:${CUSTOM_DOMAIN}" \
  -addext 'basicConstraints=critical,CA:FALSE' \
  -addext 'keyUsage=critical,digitalSignature,keyEncipherment' \
  -addext 'extendedKeyUsage=serverAuth'
```

Inspect the generated files:

```bash
openssl x509 -in portal-custom-domain.crt -noout -subject -issuer -dates
openssl pkey -in portal-custom-domain.key -noout -check
```

Preview the change:

```bash
PORTAL_CUSTOM_CERT="$(cat portal-custom-domain.crt)" \
PORTAL_CUSTOM_KEY="$(cat portal-custom-domain.key)" \
kongctl diff -f portal-custom-domain.yaml
```

Apply the configuration:

```bash
PORTAL_CUSTOM_CERT="$(cat portal-custom-domain.crt)" \
PORTAL_CUSTOM_KEY="$(cat portal-custom-domain.key)" \
kongctl apply -f portal-custom-domain.yaml --auto-approve
```

Check the portal after apply:

```bash
kongctl get portal portal-custom-domain-example -o yaml
```

Delete the example when you are done:

```bash
PORTAL_CUSTOM_CERT="$(cat portal-custom-domain.crt)" \
PORTAL_CUSTOM_KEY="$(cat portal-custom-domain.key)" \
kongctl delete -f portal-custom-domain.yaml --auto-approve
```
