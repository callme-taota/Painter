FROM golang:1.23 AS builder
WORKDIR /app
COPY services /app/services
WORKDIR /app/services/api-gateway
RUN go build -o /out/api-gateway ./cmd/gateway

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/api-gateway /api-gateway
EXPOSE 18080
ENTRYPOINT ["/api-gateway"]
