FROM node:22-alpine AS frontend-builder

WORKDIR /frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY frontend/ ./
RUN npm run build

FROM golang:1.26.1-alpine AS backend-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o messenger-server ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata wget \
    && adduser -D -u 10001 -h /app app

WORKDIR /app

COPY --from=backend-builder /app/messenger-server .
COPY --from=frontend-builder /frontend/dist ./frontend/dist

USER app

EXPOSE 8080

CMD ["./messenger-server"]
