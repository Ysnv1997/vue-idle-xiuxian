FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /app/migrations /app/migrations

ENV MIGRATIONS_DIR=/app/migrations
EXPOSE 8081

CMD ["/app/api"]
