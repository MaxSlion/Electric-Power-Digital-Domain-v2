# Backend Service

Golang 后端控制层（Gin + gRPC + Redis + MySQL），负责算法能力发现、任务提交、进度同步、结果回调与前端 API。支持高并发、实时通信和大数据传输。

## 功能概览

### 核心功能
- **gRPC Client**：调用算法服务 `GetAvailableSchemes`、`SubmitTask`、`WatchTaskProgress`，支持重试、连接池、并发控制
- **gRPC Server**：接收算法服务 `ReportResult` 回调，支持大结果传输
- **HTTP API**：RESTful API，支持 Swagger 文档、限流、幂等性控制
- **WebSocket**：实时进度推送，支持心跳、自动重连、多客户端广播

### 高可用特性
- gRPC 连接池 & Keep-Alive
- 指数退避重试策略 (Exponential Backoff)
- 请求幂等性控制 (X-Request-ID)
- 限流中间件 (Rate Limiter)
- 请求超时控制
- 结构化日志 (Zap)
- 定时健康检查 & 僵尸任务清理
- 优雅关闭 (Graceful Shutdown)

### 大数据支持
- ReportResult 大结果上报（单条消息最大 100MB）
- 超大结果建议落盘/对象存储，仅回传摘要与索引

## 目录结构

```
backend-service/
├── cmd/server/           # 启动入口
├── docs/                 # Swagger 文档
├── internal/
│   ├── config/           # 环境配置
│   ├── grpcclient/       # 算法服务 gRPC Client（含连接池、重试）
│   ├── grpcserver/       # 结果回调 gRPC Server
│   ├── http/             # HTTP API（含 Swagger 注解）
│   ├── middleware/       # 中间件（限流、幂等性、日志、CORS）
│   ├── models/           # 业务模型
│   ├── scheduler/        # 定时任务（健康检查、僵尸清理）
│   ├── services/         # 业务服务层
│   ├── storage/          # MySQL/Redis（含连接池）
│   └── ws/               # WebSocket Hub（含心跳、广播）
└── proto/                # gRPC proto 定义
```

## 快速开始

### 1. 生成 gRPC 代码

```bash
# 安装 protoc 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成 Go 代码
protoc --go_out=./proto --go-grpc_out=./proto proto/algorithm.proto
```

### 2. 配置环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HTTP_ADDR` | `:8080` | HTTP 服务监听地址 |
| `ALGO_GRPC_ADDR` | `127.0.0.1:50051` | 算法服务 gRPC 地址 |
| `RESULT_GRPC_ADDR` | `:9090` | 结果回调 gRPC 监听地址 |
| `MYSQL_DSN` | `root:root@tcp(127.0.0.1:3306)/algo?parseTime=true` | MySQL 连接串 |
| `REDIS_ADDR` | `127.0.0.1:6379` | Redis 地址 |
| `REDIS_PASSWORD` | `` | Redis 密码 |
| `RATE_LIMIT_RPS` | `100` | 每分钟请求限制 |
| `REQUEST_TIMEOUT_SEC` | `30` | 请求超时秒数 |
| `ENABLE_SWAGGER` | `true` | 是否启用 Swagger UI |

### 3. 安装依赖并启动

```bash
cd backend-service
go mod tidy
go run ./cmd/server
```

### 4. 访问 Swagger UI

启动后访问: http://localhost:8080/swagger/index.html

## HTTP API

### 算法方案

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/algorithms/schemes` | 获取可用算法方案列表 |

### 任务管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/jobs` | 提交新任务（支持幂等性） |
| GET | `/api/v1/jobs` | 分页查询任务列表 |
| GET | `/api/v1/jobs/:id` | 获取任务详情 |
| GET | `/api/v1/jobs/:id/result` | 获取任务结果 |
| POST | `/api/v1/jobs/:id/cancel` | 取消任务 |

### 系统管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/system/health` | 健康检查 |
| GET | `/api/v1/system/stats` | 系统统计 |
| GET | `/health` | 简单健康探针（K8s） |

### 请求示例

```bash
# 提交任务
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: unique-request-123" \
  -d '{
    "scheme": "KBM-WF01",
    "data_id": "sample_001",
    "params": {"threshold": 0.9},
    "user_id": "user_001"
  }'

# 查询任务列表
curl "http://localhost:8080/api/v1/jobs?page=1&page_size=20&status=SUCCESS"

# 获取任务结果
curl http://localhost:8080/api/v1/jobs/{job_id}/result
```

## WebSocket

### 连接

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?job_id=<task_id>&user_id=<user_id>');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log(`Progress: ${msg.percentage}% - ${msg.message}`);
};

ws.onclose = () => {
  // 自动重连逻辑
};
```

### 消息格式

```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "percentage": 50,
  "message": "正在执行潮流计算...",
  "timestamp": 1707033600000,
  "stage": "power_flow",
  "metrics": {
    "iterations": "5",
    "convergence": "0.001"
  }
}
```

## 架构图

```
┌─────────────┐       ┌──────────────────────────────────────────────────┐
│  Frontend   │       │                 Backend Service                   │
│  (Vue/React)│       │                                                   │
└──────┬──────┘       │  ┌─────────────┐    ┌──────────────────────────┐│
       │              │  │   Gin HTTP  │    │    WebSocket Hub          ││
       │ HTTP/WS      │  │   + Swagger │    │  (Heartbeat, Broadcast)   ││
       ├──────────────┼──┤             │    │                          ││
       │              │  └──────┬──────┘    └──────────────────────────┘│
       │              │         │                                        │
       │              │  ┌──────▼──────┐    ┌──────────────────────────┐│
       │              │  │  Middleware │    │       Scheduler           ││
       │              │  │ Rate Limit  │    │  (Health, Zombie Cleanup) ││
       │              │  │ Idempotency │    │                          ││
       │              │  └──────┬──────┘    └──────────────────────────┘│
       │              │         │                                        │
       │              │  ┌──────▼──────┐                                 │
       │              │  │  JobService │                                 │
       │              │  └──────┬──────┘                                 │
       │              │         │                                        │
       │              │  ┌──────┴──────┐                                 │
       │              │  │             │                                 │
       │              │  ▼             ▼                                 │
       │              │  ┌───────┐  ┌───────┐                            │
       │              │  │ MySQL │  │ Redis │                            │
       │              │  │(Jobs) │  │(Cache)│                            │
       │              │  └───────┘  └───────┘                            │
       │              │                                                   │
       │              │  ┌─────────────────────────────────────────────┐ │
       │              │  │           gRPC Layer                         │ │
       │              │  │  ┌─────────────┐    ┌─────────────────────┐ │ │
       │              │  │  │ AlgoClient  │    │   ResultServer      │ │ │
       │              │  │  │ (Retry/Pool)│◄───┤  (Callback/Stream)  │ │ │
       │              │  │  └──────┬──────┘    └─────────────────────┘ │ │
       │              │  └─────────┼───────────────────────────────────┘ │
       └──────────────┴────────────┼─────────────────────────────────────┘
                                   │
                                   ▼
                      ┌────────────────────────────┐
                      │    Algorithm Service       │
                      │   (Python + gRPC:50051)    │
                      └────────────────────────────┘
```

## 高并发设计

### gRPC 连接管理
- Keep-Alive: 10秒间隔心跳
- 并发控制: 最大100个并发调用
- 指数退避: 100ms-5s 重试间隔
- 消息大小: 支持 100MB

### Redis 连接池
- 连接池大小: 100
- 最小空闲连接: 10
- 读写超时: 3秒

### MySQL 连接池
- 最大连接: 100
- 最大空闲: 20
- 连接存活: 5分钟

## 定时任务

| 任务 | 周期 | 说明 |
|------|------|------|
| 僵尸任务清理 | 5分钟 | 标记运行超过30分钟的任务为失败 |
| 健康检查 | 30秒 | 检查算法服务可用性 |
| 方案缓存刷新 | 1分钟 | 从算法服务刷新方案列表 |

## 部署

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server

FROM alpine:3.19
COPY --from=builder /app/server /server
EXPOSE 8080 9090
CMD ["/server"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend-service
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: backend
        image: backend-service:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
        readinessProbe:
          httpGet:
            path: /api/v1/system/health
            port: 8080
```
