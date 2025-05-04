# syntax=docker/dockerfile:1
FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/dark-detector

FROM scratch
COPY --from=builder /app/dark-detector /app/dark-detector
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT [ "/app/dark-detector" ]
