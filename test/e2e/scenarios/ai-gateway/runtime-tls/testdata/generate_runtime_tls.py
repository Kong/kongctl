import pathlib
import subprocess
import sys


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit("usage: generate_runtime_tls.py <workdir> <command> [args...]")

    cert_dir = pathlib.Path(sys.argv[1]) / "certs"
    cert_dir.mkdir(parents=True, exist_ok=True)
    cert_path = cert_dir / "runtime.pem"
    key_path = cert_dir / "runtime.key"
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
            "1",
            "-subj",
            "/CN=runtime-tls.kongctl-e2e.io",
            "-addext",
            "subjectAltName=DNS:runtime-tls.kongctl-e2e.io",
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    sys.exit(subprocess.run(sys.argv[2:], check=False).returncode)


if __name__ == "__main__":
    main()
