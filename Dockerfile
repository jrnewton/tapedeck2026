FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
RUN go build -o tapedeck ./cmd/tapedeck
RUN go build -o tapedeck-cli ./cmd/tapedeck-cli

FROM alpine:latest
RUN apk add --no-cache tzdata
ENV TZ=America/New_York
WORKDIR /app
COPY --from=builder /app/tapedeck .
COPY --from=builder /app/tapedeck-cli .
COPY cmd/tapedeck/web/ ./web/
EXPOSE 8080
CMD ["./tapedeck"]
