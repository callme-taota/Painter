FROM golang:1.23 AS builder
WORKDIR /app
COPY services /app/services
WORKDIR /app/services/content-service
RUN go build -o /out/content-service ./cmd/content

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/content-service /content-service
EXPOSE 18082
ENTRYPOINT ["/content-service"]
