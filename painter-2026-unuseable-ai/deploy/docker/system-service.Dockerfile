FROM golang:1.23 AS builder
WORKDIR /app
COPY services /app/services
WORKDIR /app/services/system-service
RUN go build -o /out/system-service ./cmd/system

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/system-service /system-service
EXPOSE 18083
ENTRYPOINT ["/system-service"]
