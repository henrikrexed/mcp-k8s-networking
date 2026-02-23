FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /mcp-k8s-networking ./cmd/server/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /mcp-k8s-networking /mcp-k8s-networking
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/mcp-k8s-networking"]
