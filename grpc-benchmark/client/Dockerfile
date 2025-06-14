# Etapa 1: Build com Go oficial
FROM golang:1.24.4 AS builder

WORKDIR /app

# Copia os arquivos do server
COPY server/go.mod client/go.sum ./
COPY proto/ ../proto/       

RUN go mod download

COPY client/ ./
COPY proto/ ../proto/

RUN go build -o client

# Imagem final
FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /app/client .
ENTRYPOINT ["/app/client"]
