# Multi-stage Dockerfile for HardSendHub (Frontend + Backend)

# Stage 1: Build React Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go Backend
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app/backend
RUN apk add --no-cache git ca-certificates
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# CGO_ENABLED=0 uses modernc.org/sqlite pure Go
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hardsend-server main.go

# Stage 3: Minimal Final Runtime Image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

# Copy binary, freshly compiled frontend static assets, and pre-seeded reputation DB
COPY --from=backend-builder /app/backend/hardsend-server ./hardsend-server
COPY --from=frontend-builder /app/backend/static ./static
COPY --from=backend-builder /app/backend/hardsend_metrics.db /app/seed/hardsend_metrics.db

# Ensure directory for uploads and SQLite DB
RUN mkdir -p /app/tmp /app/data /app/seed

ENV PORT=8080
ENV DB_PATH=/app/data/hardsend_metrics.db
ENV TEMP_DIR=/app/tmp
ENV WORKER_COUNT=2
ENV CF_ACCOUNT_ID=974a2eb57b6467c57ede7c991e7b6f81
ENV CF_FROM_ADDRESS=facturas@facturasvideodigital.com
ENV CF_RATE_LIMIT=15
ENV ADMIN_USERNAME=hardsendvideodigital
ENV CF_WORKER_URL=https://app.facturasvideodigital.com
# Runtime secrets (CF_API_TOKEN, ADMIN_PASSWORD, JWT_SECRET) are injected by Cloudflare Worker envVars.

EXPOSE 8080

CMD ["./hardsend-server"]
