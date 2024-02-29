FROM golang:1.22 AS build

LABEL maintainer="Taota"
LABEL description="Find me http://www.callmetaota.fun/"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o painter-server-new .

FROM debian:buster-slim

WORKDIR /root/

RUN mkdir -p "/root/conf"
RUN mkdir -p "/root/tolog/logs"

COPY --from=build /app/painter-server-new .
COPY --from=build /app/conf/conf.json ./conf

RUN chmod +x ./painter-server-new

EXPOSE 3003

CMD ["./painter-server-new"]
