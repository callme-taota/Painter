# syntax=docker/dockerfile:1
FROM node:24.14.0-bookworm-slim@sha256:d8e448a56fc63242f70026718378bd4b00f8c82e78d20eefb199224a4d8e33d8 AS frontend
WORKDIR /src/web/frontend
COPY web/frontend/package.json web/frontend/package-lock.json ./
RUN npm ci
COPY web/frontend/ ./
RUN npm run build

FROM golang:1.26.11-bookworm@sha256:dc40a13b61304647ed7e5d581d246b86413dfc29d63c6d018310feb93cd98fc0 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/assets ./web/assets
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/painter ./cmd/painter

# Shared base layers with the database include matching pg_dump/pg_restore clients.
# This image starts only the Go application; it does not start a PostgreSQL server.
FROM postgres:18.6-bookworm@sha256:bbf83594798e144bff365a9c191c5d779f181fb3135902064ab8ff11918020fc AS runtime
RUN mkdir -p /var/lib/painter/media /var/backups/painter && chown -R postgres:postgres /var/lib/painter /var/backups/painter
COPY --from=builder /out/painter /usr/local/bin/painter
USER postgres
WORKDIR /var/lib/painter
ENV LISTEN_ADDR=:8080 MEDIA_DIR=/var/lib/painter/media
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=3 CMD ["painter", "healthcheck"]
ENTRYPOINT ["painter"]
CMD ["serve"]
