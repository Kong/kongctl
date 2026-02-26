# check=skip=InvalidDefaultArgInFrom
ARG GO_VERSION

FROM golang:${GO_VERSION}-alpine AS builder

ARG TAG=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
      -X main.version=${TAG} \
      -X main.commit=${COMMIT} \
      -X main.date=${BUILD_DATE}" \
    -o kongctl .

FROM alpine:3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

RUN addgroup -S kongctl && adduser -S kongctl -G kongctl && apk add --no-cache ca-certificates

COPY --from=builder /workspace/kongctl /kongctl

USER kongctl
ENTRYPOINT ["/kongctl"]
