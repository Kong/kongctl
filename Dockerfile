ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM alpine:3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS basefs

RUN addgroup -S kongctl && adduser -S kongctl -G kongctl && apk add --no-cache ca-certificates

FROM alpine:3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

ARG TARGETPLATFORM

COPY --from=basefs /etc/passwd /etc/passwd
COPY --from=basefs /etc/group /etc/group
COPY --from=basefs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY $TARGETPLATFORM/kongctl /kongctl

USER kongctl
ENTRYPOINT ["/kongctl"]
