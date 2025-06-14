# Etapa 1: Build com Go oficial
FROM golang:1.24.4 AS builder

WORKDIR /app
COPY worker/go.mod worker/go.sum ./
RUN go mod download

COPY worker/*.go ./
RUN go build -o worker

# # Imagem final
# FROM debian:bullseye-slim
# WORKDIR /app
# COPY --from=builder /app/worker .
# CMD ["./worker"]

# Imagem final
FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /app/worker .
CMD ["./worker"]


