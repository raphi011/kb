# ── Stage 1: build kb ─────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache curl nodejs npm

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Download vendored JS deps and bundle JS + CSS
RUN curl -fsSL https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js \
      -o internal/server/static/htmx.min.js && \
    curl -fsSL https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js \
      -o internal/server/static/mermaid.min.js && \
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js && \
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css

# Generate chroma.css and fingerprint every asset referenced by HTML so the
# served URL changes whenever its content changes — letting the binary set
# `Cache-Control: immutable` without serving stale assets after a release.
RUN go run ./cmd/genchroma -out internal/server/static/chroma.css && \
    go run ./cmd/genassets \
      -dir internal/server/static \
      -out internal/server/static/dist \
      -manifest internal/server/static/dist/manifest.json \
      -files app.min.js,style.min.css,chroma.css,htmx.min.js,mermaid.min.js,marp-core.min.js,marp-browser.min.js

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/kb ./cmd/kb

# ── Stage 2: minimal runtime ─────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates git && \
    adduser -D -u 1000 kb

COPY --from=builder /bin/kb /usr/local/bin/kb

VOLUME ["/repo"]
EXPOSE 8080

USER 1000
ENTRYPOINT ["kb", "serve", "--addr", ":8080", "--repo", "/repo"]
