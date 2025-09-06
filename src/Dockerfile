FROM golang:1.24.3-alpine as builder

WORKDIR /build
COPY . /build/

RUN go build -o server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /build/server /app/server

ENTRYPOINT ["/app/server"]
