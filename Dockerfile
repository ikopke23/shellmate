FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o shellmate-server ./cmd/shellmate-server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/shellmate-server .
COPY --from=builder /app/migrations ./migrations
EXPOSE 2222
ENV DATABASE_URL="" \
    INVITE_CODE="" \
    SSH_PORT=":2222"
CMD ["./shellmate-server"]
