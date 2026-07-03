FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm install
COPY web ./
RUN npm run build

FROM golang:1.22-alpine AS api
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 go build -o /out/scorecard ./cmd/scorecard

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=api /out/scorecard /app/scorecard
COPY --from=web /src/web/dist /app/web/dist
RUN mkdir -p /app/data
ENV PORT=3003 DB_PATH=/app/data/scorecard.db STATIC_DIR=/app/web/dist
EXPOSE 3003
CMD ["/app/scorecard"]
