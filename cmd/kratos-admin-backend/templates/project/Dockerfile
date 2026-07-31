FROM golang:1.26.3-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./internal/cmd/server

FROM alpine:3.23

WORKDIR /app
COPY --from=builder /out/server ./server
COPY configs ./configs

EXPOSE 7001 6001
ENTRYPOINT ["./server", "--conf", "./configs"]
