FROM golang:1.25-alpine AS build

ARG GOPROXY
ENV GOPROXY=${GOPROXY:-https://proxy.golang.org,direct}

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api-gateway ./cmd/api-gateway

FROM alpine:3.22
RUN addgroup -S api-gateway && adduser -S -G api-gateway api-gateway && \
    mkdir -p /var/log/api-gateway && chown -R api-gateway:api-gateway /var/log/api-gateway
USER api-gateway
COPY --from=build /out/api-gateway /usr/local/bin/api-gateway
EXPOSE 8081
ENTRYPOINT ["api-gateway"]
