FROM golang:1.24.3-alpine as builder

WORKDIR /build
COPY . /build/

RUN go build -o server dialogs.go

FROM scratch as runner
WORKDIR /app

COPY --from=builder /build/server /app/server

ENTRYPOINT ["/app/server"]
