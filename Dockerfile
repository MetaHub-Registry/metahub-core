FROM alpine:3.23

COPY pkg /pkg
COPY go.mod go.sum /