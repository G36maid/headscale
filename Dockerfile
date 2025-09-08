# Builder image
FROM docker.io/golang:1.23-bookworm AS build
ARG VERSION=0.23.0
ARG BUF_VERSION=1.57.0
ENV GOPATH=/go
WORKDIR /go/src/headscale

COPY go.mod go.sum /go/src/headscale/
RUN go mod download

COPY . .

## Install protoc plugins required for buf generate
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest

## Generate grpc protocols
RUN curl -fsSL https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(uname -s)-$(uname -m) -o /bin/buf
RUN chmod +x /bin/buf
RUN buf generate proto

RUN CGO_ENABLED=0 GOOS=linux go install -ldflags="-s -w -X github.com/juanfont/headscale/cmd/headscale/cli.Version=$VERSION" -a ./cmd/headscale && test -e /go/bin/headscale

# Production image
FROM docker.io/debian:bookworm-slim

RUN apt-get update \
    && apt-get install --no-install-recommends --yes less jq \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

COPY --from=build /go/bin/headscale /bin/headscale
ENV TZ=UTC

RUN mkdir -p /var/run/headscale

EXPOSE 8080/tcp
CMD ["headscale"]
