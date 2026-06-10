# kongctl Installer

`scripts/install.sh` is the canonical source for the shell installer published
at:

```shell
https://get.konghq.com/kongctl
```

The hosted file in `Kong/get.konghq.com` is a reviewed copy, not the source of
truth. Update `scripts/install.sh` in this repository first. After changes land
on `main`, the `Sync get.konghq.com installer` workflow opens or updates a pull
request in `Kong/get.konghq.com` that copies the script to the root-level
`kongctl` path.

The sync workflow requires a `GET_KONGHQ_COM_TOKEN` secret with permission to
push branches and open pull requests in `Kong/get.konghq.com`. It must not
auto-merge the hosted-script pull request.

Before merging installer changes, run:

```shell
make test-installer
```

The installer verifies GitHub release archive checksums by default. Stronger
release provenance, such as signed checksums or artifact attestations, should be
added as a future hardening step without weakening checksum verification.
