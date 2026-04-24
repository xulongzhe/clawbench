# === Stage 1: Build frontend ===
FROM node:22-alpine AS frontend-builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY web/ web/
COPY vite.config.ts ./
COPY assets/ assets/

RUN npm run build
# Output: /app/public/

# === Stage 2: Build backend ===
FROM golang:1.21-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o clawbench ./cmd/server

# === Stage 3: Runtime ===
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /app/clawbench ./clawbench
COPY --from=frontend-builder /app/public ./public/
COPY assets/ ./assets/
COPY agents/ ./agents/
COPY config.docker.yaml ./config.yaml

RUN chmod +x ./clawbench

ENV PORT=20000
EXPOSE 20000

VOLUME ["/data"]

ENTRYPOINT ["./clawbench"]
