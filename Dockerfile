# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/magnit_bot ./cmd/main

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/magnit_bot /app/magnit_bot
COPY telegram.yml checklist.txt /app/

RUN adduser -D -H -u 10001 appuser \
	&& mkdir -p /app/logs \
	&& chown -R appuser:appuser /app

USER appuser

ENV CHECKLIST_PATH=/app/checklist.txt \
	LOGS_DIR=/app/logs \
	TIMEZONE=Europe/Moscow

CMD ["/app/magnit_bot"]
