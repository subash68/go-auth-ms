# ─── Stage 1: Build ──────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

# Install git so `go mod download` can fetch VCS deps
RUN apk add --no-cache git

WORKDIR /app

# Cache dependency downloads separately from the source build
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source tree
COPY . .

# Build the server binary; CGO disabled for a fully-static binary
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=mod go build -o /bin/server ./cmd/server

# ─── Stage 2: Run ────────────────────────────────────────────────────────────
FROM alpine:3.18

# ca-certificates are needed for any TLS outbound calls
RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/server /bin/server

EXPOSE 9090 8080

ENTRYPOINT ["/bin/server"]
