FROM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go build -o /out/subforme ./cmd/subforme

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /out/subforme /app/
COPY --from=frontend /src/dist /app/web

# Default config - override via env vars or volume mount
COPY backend/config /app/config
# Backup defaults for first-run initialization
COPY backend/config /app/defaults/config

EXPOSE 8080
CMD ["/app/subforme"]
