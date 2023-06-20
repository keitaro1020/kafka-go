FROM golang:1.20-alpine

ENV ROOT=/app

WORKDIR ${ROOT}
COPY . .

RUN go mod download

RUN apk add --no-cache alpine-sdk
