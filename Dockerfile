FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/log-parser ./cmd/app

FROM alpine:3.20

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/log-parser /app/log-parser
COPY migrations /app/migrations

RUN mkdir -p /app/data && chown -R app:app /app

USER app

ENV PORT=8080
ENV DATA_DIR=/app/data
ENV MIGRATIONS_DIR=/app/migrations
ENV LOG_LEVEL=info

EXPOSE 8080

ENTRYPOINT ["/app/log-parser"]
