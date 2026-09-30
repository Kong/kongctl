ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM alpine:3@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS basefs

RUN addgroup -S kongctl && adduser -S kongctl -G kongctl \
    && mkdir -p /home/kongctl && chown kongctl:kongctl /home/kongctl \
    && apk add --no-cache ca-certificates

FROM alpine:3@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

ARG TARGETPLATFORM

# Upgrades libssl3/libcrypto3 to the latest patched versions from the Alpine
# repo index at build time. Intentionally left unpinned: the index moves, so a
# literal version pin would go stale and hard-fail `apk add` once the branch
# stops serving it. The shipped binary is static (CGO_ENABLED=0), so these
# libs affect scanner output, not runtime, and build-to-build drift in them
# is an accepted tradeoff.
RUN apk add --no-cache --upgrade libssl3 libcrypto3

COPY --from=basefs /etc/passwd /etc/passwd
COPY --from=basefs /etc/group /etc/group
COPY --from=basefs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=basefs --chown=kongctl:kongctl /home/kongctl /home/kongctl

COPY $TARGETPLATFORM/kongctl /kongctl

USER kongctl
ENTRYPOINT ["/kongctl"]
