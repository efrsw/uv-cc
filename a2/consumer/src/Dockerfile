# Use Debian because of libc and librdkafka compat
FROM golang:1.25-bookworm AS builder

ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o /app .

FROM debian:bookworm-slim AS final

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app /bin/app

EXPOSE 80

CMD ["/bin/app"]
