# ---------- Stage 1: Builder ----------
FROM golang:1.22-alpine AS builder

# git é necessário para o go resolver módulos remotos (Prometheus).
RUN apk add --no-cache git

WORKDIR /app

# Copia primeiro o manifesto de módulos para aproveitar o cache de camadas.
COPY go.mod ./

# Copia o código-fonte.
COPY *.go ./

# Resolve dependências (gera/atualiza go.sum) e compila um binário estático.
# CGO_ENABLED=0 garante um binário sem dependências de libc, ideal para Alpine.
# -ldflags "-s -w" remove a tabela de símbolos e o DWARF, reduzindo o tamanho.
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# ---------- Stage 2: Runtime ----------
FROM alpine:3.20

# Certificados de CA para conexões TLS de saída, se necessário.
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

# Copia apenas o binário compilado do estágio anterior.
COPY --from=builder /app/server ./server

# Executa como usuário não-root por segurança.
USER app

EXPOSE 8080

ENTRYPOINT ["/app/server"]
