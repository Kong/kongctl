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
    alt_cert_path = cert_dir / "runtime-alt.pem"
    alt_key_path = cert_dir / "runtime-alt.key"
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
    subprocess.run(
        [
            "openssl",
            "req",
            "-x509",
            "-nodes",
            "-newkey",
            "ec",
            "-pkeyopt",
            "ec_paramgen_curve:P-256",
            "-keyout",
            str(alt_key_path),
            "-out",
            str(alt_cert_path),
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
    command = sys.argv[2:]
    result = subprocess.run(command, check=False)
    if result.returncode == 0 and "--output-file" in command:
        output_index = command.index("--output-file") + 1
        sys.stdout.write(pathlib.Path(command[output_index]).read_text())
    sys.exit(result.returncode)


if __name__ == "__main__":
    main()
