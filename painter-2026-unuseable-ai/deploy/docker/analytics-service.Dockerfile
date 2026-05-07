FROM golang:1.23 AS builder
WORKDIR /app
COPY services /app/services
WORKDIR /app/services/analytics-service
RUN go build -o /out/analytics-service ./cmd/analytics

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/analytics-service /analytics-service
EXPOSE 18084
ENTRYPOINT ["/analytics-service"]
