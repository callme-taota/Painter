FROM golang:1.23 AS builder
WORKDIR /app
COPY services /app/services
WORKDIR /app/services/identity-service
RUN go build -o /out/identity-service ./cmd/identity

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/identity-service /identity-service
EXPOSE 18081
ENTRYPOINT ["/identity-service"]
