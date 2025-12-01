import argparse
import json
import sys
from pathlib import Path
from typing import Any

from google_auth_oauthlib.flow import InstalledAppFlow

DEFAULT_SCOPE = "https://www.googleapis.com/auth/gmail.readonly"


def parse_args():
    parser = argparse.ArgumentParser(
        description="Obtain a Gmail access/refresh token using an OAuth client secrets JSON.",
    )
    parser.add_argument(
        "--client-secret",
        default="client_secret.json",
        help="Path to the OAuth client secrets JSON (Desktop application).",
    )
    parser.add_argument(
        "--scope",
        default=DEFAULT_SCOPE,
        help=f"OAuth scope to request (default: {DEFAULT_SCOPE}).",
    )
    parser.add_argument(
        "--manual",
        action="store_true",
        help="Use copy/paste flow instead of starting a local redirect listener.",
    )
    parser.add_argument(
        "--output-env",
        type=Path,
        help="Optional path to write export statements for env vars.",
    )
    parser.add_argument(
        "--gmail-address",
        help="Optional Gmail inbox to include in the output env file (e.g., kongctle2e@gmail.com).",
    )
    return parser.parse_args()


def mask_token(value: str | None) -> str:
    if not value:
        return ""
    if len(value) <= 8:
        return "***"
    return f"{value[:4]}...{value[-4:]}"


def load_client_meta(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as fh:
        data = json.load(fh)
    # client_secret.json for desktop apps uses the "installed" key
    return data.get("installed") or {}


def run_local(flow: InstalledAppFlow):
    # Starts a loopback listener on a random port and opens the browser.
    return flow.run_local_server(port=0, prompt="consent", access_type="offline")


def run_manual(flow: InstalledAppFlow):
    # Generates an auth URL and asks the user to paste back the code.
    auth_url, _ = flow.authorization_url(prompt="consent", access_type="offline")
    print("Open this URL in a browser, authorize with the test Gmail account, and paste the code below:")
    print(auth_url)
    code = input("Code: ").strip()
    flow.fetch_token(code=code)
    return flow.credentials


def write_exports(path: Path, client_meta: dict[str, Any], refresh: str, access: str, gmail_addr: str | None):
    lines = [
        f"export KONGCTL_E2E_GMAIL_REFRESH_TOKEN={refresh}",
        f"export KONGCTL_E2E_GMAIL_ACCESS_TOKEN={access}",
    ]
    client_id = client_meta.get("client_id", "")
    client_secret = client_meta.get("client_secret", "")
    if client_id:
        lines.append(f"export KONGCTL_E2E_GMAIL_CLIENT_ID={client_id}")
    if client_secret:
        lines.append(f"export KONGCTL_E2E_GMAIL_CLIENT_SECRET={client_secret}")

    if gmail_addr:
        lines.append(f"export KONGCTL_E2E_GMAIL_ADDRESS={gmail_addr}")
    else:
        lines.append("# export KONGCTL_E2E_GMAIL_ADDRESS=<your-test-inbox>")

    content = "\n".join(lines) + "\n"
    path.write_text(content, encoding="utf-8")
    print(f"Wrote env exports to {path}")


def main():
    args = parse_args()
    client_secret_path = Path(args.client_secret)

    try:
        client_meta = load_client_meta(client_secret_path)
        flow = InstalledAppFlow.from_client_secrets_file(
            args.client_secret,
            scopes=[args.scope],
        )
    except Exception as exc:  # pragma: no cover - helper script
        print(f"Failed to load client secrets from {args.client_secret}: {exc}", file=sys.stderr)
        return 1

    if args.manual:
        creds = run_manual(flow)
    else:
        creds = run_local(flow)

    print("Access token:", creds.token)
    print("Access token (masked):", mask_token(creds.token))
    print("Refresh token (masked):", mask_token(creds.refresh_token))
    if not args.output_env:
        print("Full tokens not printed. Use --output-env to write export statements to a file.", file=sys.stderr)

    if args.output_env:
        write_exports(
            args.output_env,
            client_meta,
            refresh=creds.refresh_token,
            access=creds.token,
            gmail_addr=args.gmail_address,
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
