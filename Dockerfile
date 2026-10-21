# 1) Frontend estático
FROM node:22-alpine AS web
WORKDIR /web
COPY Frontend/package*.json ./
RUN npm ci
COPY Frontend/ ./
RUN npm run build

# 2) API en Go
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY Backend/go/go.mod Backend/go/go.sum ./
RUN go mod download
COPY Backend/go/ ./
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

# 3) Imagen final: una sola app que sirve la API y el frontend
FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=api /out/server ./server
COPY --from=web /web/build ./public
ENV STATIC_DIR=/app/public GO_API_PORT=8080
USER app
EXPOSE 8080
CMD ["./server"]
