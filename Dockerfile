FROM golang:1.25-alpine AS build

ARG GOPROXY
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org,direct}
ARG GO_BUILD_PARALLEL=2
ENV GOFLAGS=-p=${GO_BUILD_PARALLEL}

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=overmindv-gomod,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,id=overmindv-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=overmindv-gobuild,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api-gateway ./cmd/api-gateway

FROM alpine:3.22
RUN addgroup -S api-gateway && adduser -S -G api-gateway api-gateway && \
    mkdir -p /var/log/api-gateway && chown -R api-gateway:api-gateway /var/log/api-gateway
USER api-gateway
COPY --from=build /out/api-gateway /usr/local/bin/api-gateway
EXPOSE 8081
ENTRYPOINT ["api-gateway"]
