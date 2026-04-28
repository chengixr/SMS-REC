# SMS 转发系统设计规范

## 概述

手机端（Android）接收短信 → 通过 Go Server 实时转发 → Windows 桌面端弹出通知。支持多用户注册，各端独立管理。

## 技术栈

| 组件 | 技术 |
|------|------|
| Server | Go (net/http + gorilla/websocket) |
| 数据库 | MySQL |
| Android | Kotlin + Jetpack Compose |
| Windows | WPF (C# .NET) |
| 通信 | REST (HTTPS) + WebSocket (WSS) |
| 认证 | JWT Token |

## 架构

```
Android ──REST/WS──→ Go Server ←──REST/WS── Windows
                          │
                       MySQL
```

- **REST API**：注册、登录、历史查询等请求-响应操作
- **WebSocket**：短信实时转发、连接状态推送
- Server 按 user_id 管理 WS 连接池，同用户下的 Android 和 Windows 通过 Server 完成中转

## 数据库模型

### users
```sql
CREATE TABLE users (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    username   VARCHAR(64) UNIQUE NOT NULL,
    password   VARCHAR(256) NOT NULL,  -- bcrypt hash
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### devices
```sql
CREATE TABLE devices (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    user_id     INT NOT NULL,
    device_type ENUM('android', 'windows') NOT NULL,
    device_name VARCHAR(128),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen   DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### sms_logs
```sql
CREATE TABLE sms_logs (
    id           INT AUTO_INCREMENT PRIMARY KEY,
    user_id      INT NOT NULL,
    sender       VARCHAR(32),
    content      TEXT,
    received_at  DATETIME,
    delivered    BOOLEAN DEFAULT FALSE,
    delivered_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX idx_sms_logs_user_time ON sms_logs(user_id, received_at DESC);
```

### connection_logs
```sql
CREATE TABLE connection_logs (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    user_id     INT NOT NULL,
    device_type ENUM('android', 'windows') NOT NULL,
    event       ENUM('connect', 'disconnect') NOT NULL,
    timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP,
    detail      VARCHAR(256),
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

## REST API

| Method | Path | 认证 | 说明 |
|--------|------|------|------|
| POST | /api/register | 无 | 注册，body: {username, password} |
| POST | /api/login | 无 | 登录，返回 JWT |
| GET | /api/devices | JWT | 当前用户设备列表及在线状态 |
| GET | /api/sms/history | JWT | SMS 历史分页，query: page, size |
| GET | /api/connection/status | JWT | 当前用户各设备连接状态 |

## WebSocket 协议

- 连接路径：`ws://host:port/ws?token=<JWT>`
- 连接后 Server 根据 JWT 识别用户，注册到 Hub
- 消息格式统一为：

```json
{ "type": "消息类型", "data": { ... } }
```

### 消息类型

| type | 方向 | 说明 |
|------|------|------|
| sms_received | Android → Server | 手机收到新短信 |
| sms_deliver | Server → Windows | 转发短信给桌面 |
| ack | Server → Android | Server 确认收到短信 |
| connection_status | Server → All | 用户某设备连接/断开 |
| ping/pong | 双向 | 心跳保活 |

### sms_received / sms_deliver data 结构
```json
{
    "id": 123,
    "sender": "10690088",
    "content": "您的验证码是887766",
    "received_at": "2026-04-27T15:30:00Z"
}
```

### connection_status data 结构
```json
{
    "device_type": "android",
    "device_name": "Pixel 8",
    "online": true
}
```

## Server 模块

```
server/
├── main.go           — 入口，初始化 DB、启动 HTTP Server
├── config/
│   └── config.go     — 配置（监听端口、DB连接、JWT密钥）
├── auth/
│   └── auth.go       — JWT 生成/验证、bcrypt 密码处理
├── store/
│   └── store.go      — MySQL 数据访问（user/device/sms/connection_log CRUD）
├── api/
│   └── api.go        — REST Handler（register/login/devices/history）
├── hub/
│   └── hub.go        — WebSocket 连接池，按 user_id 索引，消息路由
├── logger/
│   └── logger.go     — 结构化日志（服务启动、客户端连接/断开、短信分发）
└── middleware/
    └── middleware.go  — JWT 验证中间件、CORS、请求日志
```

### Hub 设计

- `map[userID]map[deviceID]*Client` 二级索引
- `Register` / `Unregister` channel 控制并发
- 收到 `sms_received` → 存 sms_logs → 写 connection_logs（分发记录）→ 查找同用户的 Windows 连接 → 发送 `sms_deliver`
- 连接断开 → 写 connection_logs → 广播 `connection_status` 给同用户其他设备

### Logger 设计

三种日志事件，统一格式：`[level] time component message`

- **服务启动**：Server 启动时记录监听端口、DB 连接状态
- **客户端连接**：WS 连接建立/断开时记录 user_id, device_type, ip
- **短信分发**：记录 sms_id, user_id, 来源设备, 目标设备, 是否成功

## Android 客户端模块

```
app/
├── ui/
│   ├── LoginScreen.kt      — 登录/注册
│   ├── MainScreen.kt       — 主界面（短信列表 + 连接状态）
│   └── StatusBar.kt        — 连接状态指示器
├── service/
│   ├── SmsReceiver.kt      — BroadcastReceiver 监听新短信
│   └── WebSocketService.kt — 前台 Service，维持 WS 长连接
├── data/
│   ├── ApiClient.kt        — Retrofit REST 客户端
│   └── WebSocketClient.kt  — OkHttp WebSocket 封装
└── model/
    └── Models.kt           — 数据类
```

### 权限要求
- `RECEIVE_SMS` / `READ_SMS`
- `FOREGROUND_SERVICE`（持久 WebSocket 连接）
- `POST_NOTIFICATIONS`（Android 13+）

### 核心流程
1. 启动 → 注册/登录 → 获取 JWT → 保存本地
2. WebSocketService 启动，携带 JWT 连接 Server
3. SmsReceiver 监听新短信 → 通过 WebSocketClient 发送 `sms_received`
4. 收到 `ack` → 确认转发成功
5. 收到 `connection_status` → 更新 UI 状态指示器

## Windows 客户端模块

```
SmsNotifier/
├── Views/
│   ├── LoginWindow.xaml      — 登录/注册
│   ├── MainWindow.xaml       — 主窗口（短信历史）
│   └── StatusIndicator.xaml  — 连接状态控件
├── Services/
│   ├── ApiService.cs         — HttpClient REST 封装
│   └── WebSocketService.cs   — ClientWebSocket 封装
├── Models/
│   └── Models.cs             — 数据类
├── App.xaml.cs               — 启动入口
└── TrayIcon.cs               — 系统托盘 + 通知弹窗
```

### 核心流程
1. 启动 → 注册/登录 → 获取 JWT → 保存本地
2. 连接 WebSocket，最小化到系统托盘
3. 收到 `sms_deliver` → 弹出 Windows Toast 通知
4. 断开时自动重连，重连期间托盘图标显示离线状态

## 部署

- Go Server 编译为单一二进制，监听 `:8080`
- MySQL 环境变量配置：`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`
- JWT Secret 环境变量：`JWT_SECRET`
- Android APK 直接安装，Windows 发布为单文件 .exe

## 非功能需求

- **实时性**：短信到达 → Windows 通知，延迟 < 3 秒（正常网络）
- **可靠性**：WebSocket 断线自动重连，增量退避
- **安全性**：密码 bcrypt 存储，API 使用 HTTPS + JWT，WebSocket 通过 token 认证
- **日志**：Server 结构化日志输出到 stdout，可按需重定向到文件
