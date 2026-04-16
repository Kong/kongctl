ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM alpine:3@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS basefs

RUN addgroup -S kongctl && adduser -S kongctl -G kongctl \
    && mkdir -p /home/kongctl && chown kongctl:kongctl /home/kongctl \
    && apk add --no-cache ca-certificates

FROM alpine:3@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

ARG TARGETPLATFORM

COPY --from=basefs /etc/passwd /etc/passwd
COPY --from=basefs /etc/group /etc/group
COPY --from=basefs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=basefs --chown=kongctl:kongctl /home/kongctl /home/kongctl

COPY $TARGETPLATFORM/kongctl /kongctl

USER kongctl
ENTRYPOINT ["/kongctl"]
