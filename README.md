# Cursor2API

<div align="center">

![Python](https://img.shields.io/badge/Python-3.11+-blue?logo=python&logoColor=white)
![FastAPI](https://img.shields.io/badge/FastAPI-0.109+-green?logo=fastapi&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-yellow)
![Docker](https://img.shields.io/badge/Docker-Ready-blue?logo=docker&logoColor=white)

**将 Cursor IDE API 转换为 OpenAI 兼容 API 的代理服务**

[功能特性](#功能特性) • [快速开始](#快速开始) • [API 文档](#api-使用) • [配置说明](#配置说明)

</div>

---

## ✨ 功能特性

- 🚀 **完全兼容 OpenAI API** - 支持 `/v1/chat/completions` 和 `/v1/models` 接口
- 🌊 **流式响应支持** - 实时 SSE 流式输出
- 🤖 **多模型支持** - GPT-4o、Claude、Gemini、DeepSeek 等
- 🎨 **精美 Web UI** - 内置聊天测试界面
- 🐳 **Docker 支持** - 一键部署
- ⚡ **高性能** - 基于 FastAPI 异步框架
- 🔒 **安全** - API 密钥认证

## 📋 支持的模型

| 厂商 | 模型 |
|------|------|
| OpenAI | gpt-4o, gpt-4-turbo, o3, o4-mini |
| Anthropic | claude-3.5-sonnet, claude-4-sonnet, claude-4-opus |
| Google | gemini-2.5-pro, gemini-2.5-flash |
| DeepSeek | deepseek-r1, deepseek-v3.1 |
| xAI | grok-3, grok-4 |
| Moonshot | kimi-k2-instruct |

## 🚀 快速开始

### 环境要求

- Python 3.11+
- Cursor 账户

### 获取 Cursor Token

1. 访问 [www.cursor.com](https://www.cursor.com) 并登录
2. 按 `F12` 打开浏览器开发者工具
3. 转到 `Application` → `Cookies` → `https://www.cursor.com`
4. 找到 `WorkosCursorSessionToken` 并复制其值

> Token 格式可能是 `user_01JXXXXXX...` 或包含 `%3A%3A` 分隔符，程序会自动处理

### 安装运行

```bash
# 克隆项目
git clone https://github.com/jiah0231/cursor2api-go.git
cd cursor2api-go

# 安装依赖
pip install -r requirements.txt

# 配置环境变量
cp env.sample .env
# 编辑 .env 文件，填入 CURSOR_TOKEN

# 运行服务
python main.py
```

服务启动后访问 http://localhost:8002 查看 Web UI

### Docker 部署

```bash
# 使用 docker-compose
docker-compose up -d

# 或者直接运行
docker run -d -p 8002:8002 \
  -e CURSOR_TOKEN=your_token \
  -e API_KEY=your_api_key \
  cursor2api
```

## 📡 API 使用

### 接口信息

| 项目 | 值 |
|------|------|
| 服务地址 | http://localhost:8002 |
| 认证方式 | Bearer Token |
| 默认密钥 | sk-cursor2api |

### 获取模型列表

```bash
curl -X GET "http://localhost:8002/v1/models" \
  -H "Authorization: Bearer sk-cursor2api"
```

### 聊天完成（非流式）

```bash
curl -X POST "http://localhost:8002/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-cursor2api" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": false
  }'
```

### 聊天完成（流式）

```bash
curl -X POST "http://localhost:8002/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-cursor2api" \
  -d '{
    "model": "claude-3.5-sonnet",
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": true
  }'
```

### 健康检查

```bash
curl http://localhost:8002/health
```

## ⚙️ 配置说明

### 必需配置

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `CURSOR_TOKEN` | Cursor Session Token | `user_01JXXX...` |
| `API_KEY` | 访问本 API 的密钥 | `sk-cursor2api` |

### 可选配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PORT` | 服务端口 | `8002` |
| `DEBUG` | 调试模式 | `false` |
| `MODELS` | 支持的模型列表 | `gpt-4o,claude-3.5-sonnet,...` |
| `TIMEOUT` | 请求超时（秒） | `120` |
| `CURSOR_VERSION` | 客户端版本 | `0.48.6` |
| `CURSOR_TIMEZONE` | 时区 | `Asia/Shanghai` |
| `CURSOR_GHOST_MODE` | 隐私模式 | `true` |

## 📁 项目结构

```
cursor2api/
├── main.py              # 主程序入口
├── app/
│   ├── __init__.py
│   ├── config.py        # 配置管理
│   ├── models.py        # 数据模型
│   ├── routes.py        # API 路由
│   └── cursor_client.py # Cursor gRPC-Web 客户端
├── static/
│   └── index.html       # Web UI
├── requirements.txt     # Python 依赖
├── Dockerfile          # Docker 配置
├── docker-compose.yml  # Docker Compose 配置
└── env.sample          # 环境变量示例
```

## 🔧 技术实现

本项目通过逆向工程 Cursor IDE 客户端，实现了对其 API 的调用：

- **协议**: gRPC-Web over HTTP/1.1
- **端点**: `https://api2.cursor.sh/aiserver.v1.AiService/StreamChat`
- **认证**: WorkosCursorSessionToken
- **数据格式**: Protocol Buffers（手动编码，无需 protoc）

### 关键 Headers

```
Authorization: Bearer <TOKEN>
Content-Type: application/connect+proto
connect-protocol-version: 1
x-cursor-client-version: 0.48.6
x-cursor-timezone: Asia/Shanghai
x-ghost-mode: true
```

## 🐛 故障排除

### 认证失败 (401)
- 检查 `CURSOR_TOKEN` 是否正确配置
- Token 可能已过期，需要重新获取

### 请求超时
- 增加 `TIMEOUT` 配置值
- 检查网络连接

### 模型不可用
- 确认模型名称拼写正确
- 检查 Cursor 账户是否有该模型的访问权限

## 📜 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## ⚠️ 免责声明

本项目仅供学习和研究使用，请勿用于商业用途。使用本项目时请遵守 Cursor 的使用条款。

---

<div align="center">
⭐ 如果这个项目对您有帮助，请给我们一个 Star！
</div>
