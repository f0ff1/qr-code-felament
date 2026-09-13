FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate

FROM alpine:3.20 AS app
RUN apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /app
COPY --from=builder /out/app /app/app
COPY web ./web
EXPOSE 8080
CMD ["/app/app"]

FROM alpine:3.20 AS migrate
RUN apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /app
COPY --from=builder /out/migrate /app/migrate
COPY db/migrations /migrations
ENV MIGRATIONS_DIR=/migrations
CMD ["/app/migrate"]
