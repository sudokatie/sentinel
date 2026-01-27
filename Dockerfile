FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o sentinel ./cmd/sentinel

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/sentinel .

EXPOSE 3000

ENV SENTINEL_DB_PATH=/data/sentinel.db

VOLUME ["/data"]

CMD ["./sentinel"]
