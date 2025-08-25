FROM golang:1.23 AS builder
WORKDIR /src
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache docker-cli
COPY --from=builder /src/server ./server
COPY schema.sql ./
ENV GIN_MODE=release
CMD ["./server"]
