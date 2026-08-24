# syntax=docker/dockerfile:1
# ai-forum api 镜像（#8：Go + Gin 单进程容器，非 root 运行）

FROM golang:1.26-alpine AS build
# #43 服务器本地构建（v3.2）：服务器 proxy.golang.org 被墙 → 部署脚本 build-arg 覆盖为 goproxy.cn；
#   CI（GitHub runner）可访问默认 proxy.golang.org，无需覆盖
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
ARG GOSUMDB=sum.golang.org
ENV GOSUMDB=${GOSUMDB}
WORKDIR /src
COPY go.mod go.sum ./
# BuildKit cache mount：go.mod/go.sum 不变 → download 层 cache hit（服务器越部署越快，避免冷下全量 modules）
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21
# app 固定 UID=1000（#12：宿主机 /opt/ai-forum/uploads 按此 chown，容器非 root 才能写入上传目录）
RUN addgroup -S app && adduser -S -u 1000 -G app app
WORKDIR /app
COPY --from=build /out/api /app/api
# 以非 root 运行；健康检查用 busybox wget（#8 §3.1 healthcheck）
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
