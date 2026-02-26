# Build stage
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /monstera-fed ./cmd/monstera-fed

# Runtime stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
USER nobody
EXPOSE 8080
ENTRYPOINT ["/monstera-fed", "serve"]
COPY --from=builder /monstera-fed /monstera-fed
