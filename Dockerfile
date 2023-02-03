FROM golang:1.19 as build
COPY . .
RUN GOPATH="" CGO_ENABLED=0 go build -o /switchbot_exporter

FROM alpine:latest
COPY --from=build /switchbot_exporter /switchbot_exporter
RUN apk add --no-cache ca-certificates && update-ca-certificates
ENTRYPOINT [ "/switchbot_exporter" ]
