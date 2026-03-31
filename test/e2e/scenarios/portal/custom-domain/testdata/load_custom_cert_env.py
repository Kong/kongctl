import os
import re
import sys


def extract_field(text: str, field: str) -> str:
    pattern = rf'{field}: "((?:[^"\\\\]|\\\\.)*)"'
    match = re.search(pattern, text, re.DOTALL)
    if match is None:
        raise SystemExit(f"failed to locate {field} in source fixture")
    return bytes(match.group(1), "utf-8").decode("unicode_escape")


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit("usage: load_custom_cert_env.py <fixture> <command> [args...]")

    fixture_path = sys.argv[1]
    with open(fixture_path, encoding="utf-8") as fixture:
        text = fixture.read()

    env = os.environ.copy()
    env["KONGCTL_E2E_PORTAL_CUSTOM_CERT"] = extract_field(text, "certificate")
    env["KONGCTL_E2E_PORTAL_CUSTOM_KEY"] = extract_field(text, "key")

    os.execvpe(sys.argv[2], sys.argv[2:], env)


if __name__ == "__main__":
    main()
