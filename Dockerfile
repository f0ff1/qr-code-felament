FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates postgresql-client redis && \
    rm -rf /var/cache/apk/*
WORKDIR /app
COPY --from=builder /app /app/app
COPY web ./web
EXPOSE 8080
CMD ["/app/app"]
