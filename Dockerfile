# One container: the Go backend also serves the built dashboard.
#
#   docker build -t chaukanna .
#   docker run -p 8080:8080 -v chaukanna-data:/data --env-file backend/.env chaukanna
#
# The browser only allows the microphone on https or localhost, so a hosted copy
# needs a TLS front (Render, Fly and Railway all provide one).

FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /chaukanna ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -h /app chaukanna && mkdir -p /data && chown chaukanna /data
WORKDIR /app
COPY --from=backend /chaukanna ./chaukanna
COPY --from=web /web/dist ./web
COPY benchmark/cases ./cases
USER chaukanna
ENV PORT=8080 WEB_DIST=/app/web BENCHMARK_DIR=/app/cases SQLITE_PATH=/data/chaukanna.db
EXPOSE 8080
CMD ["./chaukanna"]
