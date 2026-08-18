# syntax=docker/dockerfile:1
# ai-forum api 镜像（#8：Go + Gin 单进程容器，非 root 运行）

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21
# app 固定 UID=1000（#12：宿主机 /opt/ai-forum/uploads 按此 chown，容器非 root 才能写入上传目录）
RUN addgroup -S app && adduser -S -u 1000 -G app app
WORKDIR /app
COPY --from=build /out/api /app/api
# 以非 root 运行；健康检查用 busybox wget（#8 §3.1 healthcheck）
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
