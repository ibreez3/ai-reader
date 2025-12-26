## ---- Build stage ----
FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git build-base ca-certificates && update-ca-certificates

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
ENV GO111MODULE=on
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go env -w GOPROXY=${GOPROXY} && go mod download || (echo "retry with direct" && go env -w GOPROXY=direct && go mod download)

# Copy source
COPY . .

# Build server binary (static)
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o /app/server ./cmd/server

## ---- Runtime stage ----
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

# Copy binary and config
COPY --from=builder /app/server /app/server
COPY config/ /app/config/

# Create output dir (mounted as volume in compose)
RUN mkdir -p /app/output

EXPOSE 8080
ENTRYPOINT ["/app/server"]
