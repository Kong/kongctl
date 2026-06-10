ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM alpine:3@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4 AS basefs

RUN addgroup -S kongctl && adduser -S kongctl -G kongctl \
    && mkdir -p /home/kongctl && chown kongctl:kongctl /home/kongctl \
    && apk add --no-cache ca-certificates

FROM alpine:3@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4

ARG TARGETPLATFORM

COPY --from=basefs /etc/passwd /etc/passwd
COPY --from=basefs /etc/group /etc/group
COPY --from=basefs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=basefs --chown=kongctl:kongctl /home/kongctl /home/kongctl

COPY $TARGETPLATFORM/kongctl /kongctl

USER kongctl
ENTRYPOINT ["/kongctl"]
