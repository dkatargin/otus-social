FROM golang:1.24.3-alpine as builder

WORKDIR /build
COPY . /build/

RUN go build -o server dialogs.go

FROM alpine:latest as runner
WORKDIR /app

RUN apk --no-cache add ca-certificates zabbix-agent2

COPY --from=builder /build/server /app/server
COPY zabbix_agent2.conf /etc/zabbix/zabbix_agent2.conf

EXPOSE 8080 10050

CMD ["sh", "-c", "zabbix_agent2 -c /etc/zabbix/zabbix_agent2.conf & exec /app/server"]
