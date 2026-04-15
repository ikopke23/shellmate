FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o shellmate-server ./cmd/shellmate-server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates                                                                                                                                                
RUN addgroup -S shellmate && adduser -S shellmate -G shellmate
WORKDIR /app
COPY --from=builder /app/shellmate-server .
COPY --from=builder /app/migrations ./migrations
EXPOSE 2222
ENV DATABASE_URL="" \
    INVITE_CODE="" \
    SSH_PORT=":2222"
USER shellmate
CMD ["./shellmate-server"]
