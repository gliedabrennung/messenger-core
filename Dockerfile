FROM golang:1.26.1-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o messenger-server ./cmd/server/main.go

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/messenger-server .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./messenger-server"]