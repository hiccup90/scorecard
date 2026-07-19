# --- frontend ---
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web ./
RUN npm run build

# --- backend (SQLite via CGO) ---
FROM golang:1.22-alpine AS api
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/scorecard ./cmd/scorecard

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata su-exec \
  && adduser -D -H -u 10001 appuser \
  && mkdir -p /app/data /app/web/dist \
  && chown -R appuser:appuser /app
WORKDIR /app
COPY --from=api /out/scorecard /app/scorecard
COPY --from=web /src/web/dist /app/web/dist
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh
# Start as root so entrypoint can chown mounted ./data, then drop to appuser.
USER root
ENV PORT=3003 \
    DB_PATH=/app/data/scorecard.db \
    STATIC_DIR=/app/web/dist \
    TZ=Asia/Shanghai \
    LOG_LEVEL=info
EXPOSE 3003
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:3003/healthz >/dev/null || exit 1
ENTRYPOINT ["/app/docker-entrypoint.sh"]
