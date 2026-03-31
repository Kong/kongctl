import os
import pathlib
import subprocess
import sys
import tempfile


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit("usage: load_custom_cert_env.py <command> [args...]")

    with tempfile.TemporaryDirectory() as tmpdir:
        tmp_path = pathlib.Path(tmpdir)
        cert_path = tmp_path / "portal-custom-domain.crt"
        key_path = tmp_path / "portal-custom-domain.key"

        subprocess.run(
            [
                "openssl",
                "req",
                "-x509",
                "-nodes",
                "-newkey",
                "rsa:2048",
                "-keyout",
                str(key_path),
                "-out",
                str(cert_path),
                "-days",
                "3650",
                "-subj",
                "/CN=*.kongctl-e2e.io",
                "-addext",
                "subjectAltName=DNS:*.kongctl-e2e.io",
                "-addext",
                "basicConstraints=critical,CA:FALSE",
                "-addext",
                "keyUsage=critical,digitalSignature,keyEncipherment",
                "-addext",
                "extendedKeyUsage=serverAuth",
            ],
            check=True,
            capture_output=True,
            text=True,
        )

        env = os.environ.copy()
        env["KONGCTL_E2E_PORTAL_CUSTOM_CERT"] = cert_path.read_text(encoding="utf-8")
        env["KONGCTL_E2E_PORTAL_CUSTOM_KEY"] = key_path.read_text(encoding="utf-8")

        os.execvpe(sys.argv[1], sys.argv[1:], env)


if __name__ == "__main__":
    main()
