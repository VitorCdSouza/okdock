FROM node:22-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS api
WORKDIR /src
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
RUN rm -rf internal/webui/dist
COPY --from=web /src/dist/web/browser/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/okdock ./cmd/okdock

FROM alpine:3.20
RUN apk add --no-cache docker-cli docker-cli-compose ca-certificates tzdata
COPY --from=api /out/okdock /usr/local/bin/okdock

ENV OKDOCK_ADDR=:8080 \
    OKDOCK_CONFIG=/config \
    OKDOCK_TEMPLATES=/templates
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health || exit 1

ENTRYPOINT ["/usr/local/bin/okdock"]
