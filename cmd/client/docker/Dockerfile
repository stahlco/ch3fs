FROM golang:latest AS build
WORKDIR /app

COPY ../.. .


# Statische, portable Binary für Linux ARM64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /client ./cmd/client/main.go

FROM alpine:latest
COPY --from=build ./client /bin/client
RUN chmod +x /bin/client

ENTRYPOINT ["/bin/client"]