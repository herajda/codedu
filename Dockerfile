# Build frontend
FROM node:20 AS frontend-builder
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend .
RUN npm run build

# Build backend
FROM golang:1.23 AS backend-builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /app/backend
ENV GOTOOLCHAIN=auto
RUN go mod download
COPY backend .
# copy frontend build into expected path
COPY --from=frontend-builder /frontend/build ../frontend/build
RUN CGO_ENABLED=0 GOOS=linux go build -o server

# Final image
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=backend-builder /app/backend/server .
COPY --from=backend-builder /app/backend/schema.sql .
COPY --from=frontend-builder /frontend/build /frontend/build
EXPOSE 22946
CMD ["./server"]
