# Cursor2API Go版本

一个将 Cursor IDE 客户端 API 转换为 OpenAI 兼容 API 的 Go 服务。使用 Cursor IDE 的 gRPC-Web 协议，完全兼容 OpenAI API 格式。

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 功能特性

- ✅ 完全兼容 OpenAI API 格式
- ✅ 支持流式和非流式响应
- ✅ 支持多种先进 AI 模型
- ✅ 高性能 Go 语言实现
- ✅ 使用 Cursor IDE 客户端 API（gRPC-Web 协议）
- ✅ 无需 Node.js，纯 Go 实现
- ✅ 简洁的 Web 界面

## 支持的模型

- claude-3.5-sonnet, claude-3.5-haiku
- claude-4-sonnet, claude-4.5-sonnet, claude-4-opus
- gpt-4o, gpt-4-turbo
- 以及 Cursor IDE 支持的所有模型

## 快速开始

### 环境要求

- Go 1.21+
- Cursor IDE 账户和有效的 JWT Token

### 获取 Cursor Token

你需要获取 Cursor 的 Session Token（`WorkosCursorSessionToken`）：

1. **方法一：通过 Cursor 网站（推荐）**
   - 访问 [www.cursor.com](https://www.cursor.com) 并登录账户
   - 按 `F12` 打开浏览器开发者工具
   - 转到 `应用（Application）` -> `Cookies` -> `https://www.cursor.com`
   - 找到名为 `WorkosCursorSessionToken` 的 Cookie，复制其值
   
   > 注意：Token 格式类似 `user_01JXXXXXX...` 或包含 `%3A%3A` 分隔符

2. **方法二：通过网络抓包**
   - 使用 Fiddler、Charles 或 Wireshark
   - 捕获 Cursor IDE 发送到 `api2.cursor.sh` 的请求
   - 查看 `Authorization: Bearer <token>` 头中的值

3. **方法三：从 Cursor IDE 配置文件**
   - Windows: `%APPDATA%\Cursor\User\globalStorage\storage.json`
   - macOS: `~/Library/Application Support/Cursor/User/globalStorage/storage.json`
   - 搜索文件中的 `accessToken` 或 `cursorAuth` 字段

### 安装和运行

1. **克隆项目**：
   ```bash
   git clone https://github.com/your-repo/cursor2api-go.git
   cd cursor2api-go
   ```

2. **安装依赖**：
   ```bash
   go mod download
   ```

3. **配置环境变量**：
   ```bash
   # 复制示例配置
   cp env.sample .env
   
   # 编辑 .env 文件，填入你的 Cursor Token
   nano .env
   ```

   必须配置的变量：
   ```env
   CURSOR_TOKEN=你的Cursor_JWT_Token
   API_KEY=你的API访问密钥
   ```

4. **运行服务**：
   ```bash
   # 方式1：直接运行
   go run main.go

   # 方式2：构建后运行
   go build -o cursor2api
   ./cursor2api
   ```

服务将在 http://localhost:8002 启动

## 配置说明

### 必需配置

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `CURSOR_TOKEN` | Cursor IDE JWT Token | `eyJhbGciOiJIUzI1...` |
| `API_KEY` | 访问本 API 的密钥 | `your-api-key` |

### 可选配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PORT` | 服务端口 | `8002` |
| `DEBUG` | 调试模式 | `false` |
| `MODELS` | 支持的模型列表（逗号分隔） | `claude-3.5-sonnet,gpt-4o` |
| `TIMEOUT` | 请求超时时间（秒） | `120` |
| `MAX_INPUT_LENGTH` | 最大输入长度 | `200000` |
| `CURSOR_API_URL` | Cursor API 地址 | `https://api2.cursor.sh` |
| `CURSOR_VERSION` | 客户端版本号 | `0.48.6` |
| `CURSOR_TIMEZONE` | 时区 | `Asia/Shanghai` |
| `CURSOR_GHOST_MODE` | 隐私模式 | `true` |
| `CURSOR_CLIENT_KEY` | 客户端密钥（可选） | - |
| `CURSOR_CHECKSUM` | 校验和（可选） | - |

## API 使用

### 接口信息

- **服务地址**: http://localhost:8002
- **认证方式**: Bearer Token

### 支持的接口

- `GET /` - API 文档页面
- `GET /v1/models` - 获取模型列表
- `POST /v1/chat/completions` - 聊天完成
- `GET /health` - 健康检查

### 使用示例

#### 获取模型列表

```bash
curl -X GET "http://localhost:8002/v1/models" \
  -H "Authorization: Bearer your-api-key"
```

#### 非流式聊天

```bash
curl -X POST "http://localhost:8002/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "claude-3.5-sonnet",
    "messages": [
      {
        "role": "user",
        "content": "你好，请简单介绍一下你自己"
      }
    ],
    "stream": false
  }'
```

#### 流式聊天

```bash
curl -X POST "http://localhost:8002/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "claude-3.5-sonnet",
    "messages": [
      {
        "role": "user",
        "content": "你好"
      }
    ],
    "stream": true
  }'
```

## 项目结构

```
cursor2api-go/
├── main.go              # 主程序入口
├── config/              # 配置管理
│   └── config.go
├── handlers/            # HTTP 处理器
│   └── handler.go
├── services/            # 业务服务层
│   └── cursor.go        # Cursor gRPC-Web 客户端
├── models/              # 数据模型
│   └── models.go
├── utils/               # 工具函数
│   └── utils.go
├── middleware/          # 中间件
│   ├── auth.go
│   ├── cors.go
│   └── error.go
├── proto/               # Protobuf 定义
│   └── cursor.proto
├── static/              # 静态文件
│   └── docs.html
├── env.sample           # 环境变量示例
├── go.mod               # Go 模块文件
└── README.md            # 项目说明
```

## 技术实现

### Cursor IDE API 协议

本项目通过逆向工程 Cursor IDE 客户端，实现了对其 API 的调用：

- **协议**: gRPC-Web over HTTP/1.1
- **端点**: `https://api2.cursor.sh/aiserver.v1.AiService/StreamChat`
- **认证**: JWT Bearer Token
- **数据格式**: Protocol Buffers

### 关键 Headers

```
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/grpc-web+proto
connect-protocol-version: 1
x-cursor-client-version: 0.48.6
x-cursor-timezone: Asia/Shanghai
x-ghost-mode: true
x-request-id: <UUID>
```

## 故障排除

### 常见问题

1. **认证失败 (401)**
   - 检查 `CURSOR_TOKEN` 是否正确配置
   - Token 可能已过期，需要重新获取

2. **请求超时**
   - 增加 `TIMEOUT` 配置值
   - 检查网络连接

3. **模型不可用**
   - 确认模型名称拼写正确
   - 检查你的 Cursor 账户是否有该模型的访问权限

## 部署

### Docker 部署

```bash
docker build -t cursor2api .
docker run -d -p 8002:8002 \
  -e CURSOR_TOKEN=your_token \
  -e API_KEY=your_api_key \
  cursor2api
```

### Docker Compose

```yaml
version: '3'
services:
  cursor2api:
    build: .
    ports:
      - "8002:8002"
    environment:
      - CURSOR_TOKEN=${CURSOR_TOKEN}
      - API_KEY=${API_KEY}
```

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 免责声明

本项目仅供学习和研究使用，请勿用于商业用途。使用本项目时请遵守 Cursor 的使用条款。

---

⭐ 如果这个项目对您有帮助，请给我们一个 Star！
