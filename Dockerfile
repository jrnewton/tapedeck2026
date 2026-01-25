FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY cmd/ ./cmd/
RUN go build -o tapedeck ./cmd/tapedeck
RUN go build -o tapedeck-cli ./cmd/tapedeck-cli

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/tapedeck .
COPY --from=builder /app/tapedeck-cli .
EXPOSE 8080
CMD ["./tapedeck"]
