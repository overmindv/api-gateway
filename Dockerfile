FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/laserbeak ./cmd/laserbeak

FROM alpine:3.22
RUN addgroup -S laserbeak && adduser -S -G laserbeak laserbeak
USER laserbeak
COPY --from=build /out/laserbeak /usr/local/bin/laserbeak
EXPOSE 8081
ENTRYPOINT ["laserbeak"]
