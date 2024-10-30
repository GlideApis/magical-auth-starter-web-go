# syntax=docker/dockerfile:1
# Build the image with the following command
FROM golang:latest AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod tidy

COPY ./ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o quickstart -a ./main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/quickstart /app/quickstart
COPY ./static /app/static


RUN chmod +x /app/quickstart

EXPOSE 4567

ENTRYPOINT ["./quickstart"]
