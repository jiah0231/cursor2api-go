# 构建阶段
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cursor2api-go .

# 运行阶段
FROM node:20-alpine

# 安装必要的工具
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/cursor2api-go .

# 复制jscode文件（必须）
COPY --from=builder /app/jscode ./jscode

# 复制静态文件
COPY --from=builder /app/static ./static

# 复制环境变量示例
COPY --from=builder /app/.env.example ./.env.example

# 暴露端口
EXPOSE 8002

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8002/health || exit 1

# 启动应用
CMD ["./cursor2api-go"]