# Etapa 1: Build com Go oficial
FROM golang:1.24.4 AS builder

WORKDIR /app

# Copia os arquivos do server
COPY server/go.mod server/go.sum ./
COPY proto/ ../proto/      

RUN go mod download

COPY server/ ./
COPY proto/ ../proto/

RUN go build -o server

# Imagem final
FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /app/server .
ENTRYPOINT ["/app/server"]