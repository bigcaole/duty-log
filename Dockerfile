FROM golang:1.23-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM postgres:18-alpine

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget

RUN addgroup -S app && adduser -S -G app app

COPY --from=builder /out/server /app/server
COPY templates /app/templates
COPY static /app/static
COPY fonts /app/fonts

RUN mkdir -p /app/backups && chown -R app:app /app

USER app

ENV PORT=5001

EXPOSE 5001

HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD wget -q -O- http://127.0.0.1:5001/healthz >/dev/null || exit 1

CMD ["/app/server"]
