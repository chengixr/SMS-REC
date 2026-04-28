# SMS 转发系统实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建多用户 SMS 转发系统，Android 手机接收短信 → Go Server 实时转发 → Windows 桌面弹出通知。

**Architecture:** Go Server 为中枢，REST API 处理注册/登录/查询，WebSocket Hub 管理客户端连接并按 user_id 路由消息。Android 通过 BroadcastReceiver 监听 SMS 并通过 WebSocket 上传，Windows 通过 WebSocket 接收并弹出 Toast 通知。MySQL 存储用户、设备、短信和连接日志。

**Tech Stack:** Go (net/http + gorilla/websocket + jwt), MySQL, Kotlin (Jetpack Compose + Retrofit + OkHttp), C# WPF (.NET ClientWebSocket)

---

## 文件结构

### Server (Go) — `server/`
```
server/
├── main.go              — 入口：init DB, 启动 HTTP Server
├── go.mod
├── go.sum
├── config/
│   └── config.go        — 从环境变量读配置，单例
├── store/
│   └── store.go         — MySQL CRUD，DB 连接池初始化
├── auth/
│   └── auth.go          — bcrypt 密码哈希，JWT 签发/验证
├── api/
│   └── api.go           — REST Handler 函数
├── hub/
│   └── hub.go           — WS 连接池，消息路由，Client 结构
├── logger/
│   └── logger.go        — 结构化日志输出
└── middleware/
    └── middleware.go     — JWT 认证中间件，CORS
```

### Android (Kotlin) — `android/`
```
android/
├── build.gradle.kts              — 项目级
├── app/
│   ├── build.gradle.kts          — 模块级，依赖声明
│   └── src/main/
│       ├── AndroidManifest.xml   — 权限 + Service 声明
│       ├── java/com/smsnotifier/
│       │   ├── SmsApp.kt               — Application 类
│       │   ├── ui/
│       │   │   ├── LoginScreen.kt      — 登录/注册 Compose
│       │   │   ├── MainScreen.kt       — 短信列表 + 状态
│       │   │   └── theme/Theme.kt      — Material3 主题
│       │   ├── service/
│       │   │   ├── SmsReceiver.kt      — BroadcastReceiver
│       │   │   └── WebSocketService.kt — 前台 Service
│       │   ├── data/
│       │   │   ├── ApiClient.kt        — Retrofit 接口
│       │   │   └── WebSocketClient.kt  — OkHttp WS 封装
│       │   └── model/
│       │       └── Models.kt           — 数据类
│       └── res/...
```

### Windows (WPF C#) — `windows/SmsNotifier/`
```
windows/SmsNotifier/
├── SmsNotifier.csproj
├── App.xaml / App.xaml.cs
├── Models/
│   └── Models.cs                   — 数据类
├── Services/
│   ├── ApiService.cs               — REST 客户端
│   └── WebSocketService.cs         — WS 客户端（断线重连）
├── Views/
│   ├── LoginWindow.xaml / .cs
│   ├── MainWindow.xaml / .cs
│   └── StatusIndicator.xaml / .cs
└── TrayIcon.cs                     — 系统托盘 + Toast
```

---

### Task 1: Go 项目脚手架 + 配置 + 数据库迁移

**Files:**
- Create: `server/go.mod`
- Create: `server/main.go`
- Create: `server/config/config.go`
- Create: `server/store/store.go`

- [ ] **Step 1: 初始化 Go module**

```bash
cd server && go mod init sms-server
```

- [ ] **Step 2: 创建配置模块 `server/config/config.go`**

```go
package config

import "os"

type Config struct {
    Port      string
    DBHost    string
    DBPort    string
    DBUser    string
    DBPass    string
    DBName    string
    JWTSecret string
}

func Load() *Config {
    return &Config{
        Port:      getEnv("PORT", "8080"),
        DBHost:    getEnv("DB_HOST", "127.0.0.1"),
        DBPort:    getEnv("DB_PORT", "3306"),
        DBUser:    getEnv("DB_USER", "smsuser"),
        DBPass:    getEnv("DB_PASS", "smspass"),
        DBName:    getEnv("DB_NAME", "sms_rec"),
        JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
    }
}

func (c *Config) DSN() string {
    return c.DBUser + ":" + c.DBPass + "@tcp(" + c.DBHost + ":" + c.DBPort + ")/" + c.DBName + "?parseTime=true"
}

func getEnv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
```

- [ ] **Step 3: 创建数据库访问层 `server/store/store.go`**

```go
package store

import (
    "database/sql"
    "fmt"
    "time"

    _ "github.com/go-sql-driver/mysql"
)

type Store struct {
    DB *sql.DB
}

func New(dsn string) (*Store, error) {
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("ping db: %w", err)
    }
    return &Store{DB: db}, nil
}

func (s *Store) Migrate() error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS users (
            id         INT AUTO_INCREMENT PRIMARY KEY,
            username   VARCHAR(64) UNIQUE NOT NULL,
            password   VARCHAR(256) NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
        )`,
        `CREATE TABLE IF NOT EXISTS devices (
            id          INT AUTO_INCREMENT PRIMARY KEY,
            user_id     INT NOT NULL,
            device_type ENUM('android', 'windows') NOT NULL,
            device_name VARCHAR(128),
            created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
            last_seen   DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id)
        )`,
        `CREATE TABLE IF NOT EXISTS sms_logs (
            id           INT AUTO_INCREMENT PRIMARY KEY,
            user_id      INT NOT NULL,
            sender       VARCHAR(32),
            content      TEXT,
            received_at  DATETIME,
            delivered    BOOLEAN DEFAULT FALSE,
            delivered_at DATETIME,
            FOREIGN KEY (user_id) REFERENCES users(id)
        )`,
        `CREATE TABLE IF NOT EXISTS connection_logs (
            id          INT AUTO_INCREMENT PRIMARY KEY,
            user_id     INT NOT NULL,
            device_type ENUM('android', 'windows') NOT NULL,
            event       ENUM('connect', 'disconnect') NOT NULL,
            timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP,
            detail      VARCHAR(256),
            FOREIGN KEY (user_id) REFERENCES users(id)
        )`,
    }
    for _, q := range queries {
        if _, err := s.DB.Exec(q); err != nil {
            return fmt.Errorf("migrate: %w", err)
        }
    }
    return nil
}
```

- [ ] **Step 4: 添加依赖**

```bash
cd server && go get github.com/go-sql-driver/mysql
```

- [ ] **Step 5: Commit**

---

### Task 2: Auth 模块

**Files:**
- Create: `server/auth/auth.go`

- [ ] **Step 1: 创建 `server/auth/auth.go`**

```go
package auth

import (
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"
)

type Claims struct {
    UserID   int    `json:"user_id"`
    Username string `json:"username"`
    jwt.RegisteredClaims
}

func HashPassword(pw string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPassword(hash, pw string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func GenerateToken(userID int, username, secret string) (string, error) {
    claims := Claims{
        UserID:   userID,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func ParseToken(tokenStr, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{},
        func(t *jwt.Token) (interface{}, error) {
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
            }
            return []byte(secret), nil
        })
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }
    return claims, nil
}
```

- [ ] **Step 2: 添加依赖**

```bash
cd server && go get github.com/golang-jwt/jwt/v5 golang.org/x/crypto/bcrypt
```

- [ ] **Step 3: Commit**

---

### Task 3: Store 数据访问方法

**Files:**
- Modify: `server/store/store.go`

- [ ] **Step 1: 在 `server/store/store.go` 末尾追加 User CRUD**

```go
func (s *Store) CreateUser(username, hashedPW string) (int64, error) {
    r, err := s.DB.Exec("INSERT INTO users (username, password) VALUES (?, ?)", username, hashedPW)
    if err != nil {
        return 0, err
    }
    return r.LastInsertId()
}

func (s *Store) GetUserByUsername(username string) (id int, hashedPW string, err error) {
    err = s.DB.QueryRow("SELECT id, password FROM users WHERE username = ?", username).Scan(&id, &hashedPW)
    return
}
```

- [ ] **Step 2: 追加 Device CRUD**

```go
func (s *Store) UpsertDevice(userID int, deviceType, deviceName string) error {
    _, err := s.DB.Exec(`
        INSERT INTO devices (user_id, device_type, device_name, last_seen)
        VALUES (?, ?, ?, NOW())
        ON DUPLICATE KEY UPDATE device_name = VALUES(device_name), last_seen = NOW()
    `, userID, deviceType, deviceName)
    return err
}

func (s *Store) GetDevices(userID int) ([]map[string]interface{}, error) {
    rows, err := s.DB.Query(
        "SELECT id, device_type, device_name, last_seen FROM devices WHERE user_id = ? ORDER BY last_seen DESC",
        userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var devices []map[string]interface{}
    for rows.Next() {
        var id int
        var dtype, dname string
        var lastSeen time.Time
        rows.Scan(&id, &dtype, &dname, &lastSeen)
        devices = append(devices, map[string]interface{}{
            "id": id, "device_type": dtype, "device_name": dname, "last_seen": lastSeen,
        })
    }
    return devices, nil
}
```

- [ ] **Step 3: 追加 SMS Log CRUD**

```go
func (s *Store) InsertSMSLog(userID int, sender, content string, receivedAt time.Time) (int64, error) {
    r, err := s.DB.Exec(
        "INSERT INTO sms_logs (user_id, sender, content, received_at) VALUES (?, ?, ?, ?)",
        userID, sender, content, receivedAt)
    if err != nil {
        return 0, err
    }
    return r.LastInsertId()
}

func (s *Store) MarkSMSDelivered(smsID int64) error {
    _, err := s.DB.Exec("UPDATE sms_logs SET delivered = TRUE, delivered_at = NOW() WHERE id = ?", smsID)
    return err
}

func (s *Store) GetSMSHistory(userID, page, size int) ([]map[string]interface{}, error) {
    offset := (page - 1) * size
    rows, err := s.DB.Query(
        `SELECT id, sender, content, received_at, delivered, delivered_at
         FROM sms_logs WHERE user_id = ? ORDER BY received_at DESC LIMIT ? OFFSET ?`,
        userID, size, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var logs []map[string]interface{}
    for rows.Next() {
        var id int
        var sender, content string
        var receivedAt time.Time
        var delivered bool
        var deliveredAt sql.NullTime
        rows.Scan(&id, &sender, &content, &receivedAt, &delivered, &deliveredAt)
        item := map[string]interface{}{
            "id": id, "sender": sender, "content": content,
            "received_at": receivedAt, "delivered": delivered,
        }
        if deliveredAt.Valid {
            item["delivered_at"] = deliveredAt.Time
        }
        logs = append(logs, item)
    }
    return logs, nil
}
```

- [ ] **Step 4: 追加 Connection Log CRUD**

```go
func (s *Store) InsertConnectionLog(userID int, deviceType, event, detail string) error {
    _, err := s.DB.Exec(
        "INSERT INTO connection_logs (user_id, device_type, event, detail) VALUES (?, ?, ?, ?)",
        userID, deviceType, event, detail)
    return err
}
```

- [ ] **Step 5: Commit**

---

### Task 4: Logger 模块

**Files:**
- Create: `server/logger/logger.go`

- [ ] **Step 1: 创建 `server/logger/logger.go`**

```go
package logger

import (
    "log"
    "time"
)

func ServerStart(port string) {
    log.Printf("[INFO] %s server start listening on :%s", time.Now().Format(time.RFC3339), port)
}

func ClientConnect(userID int, deviceType, ip string) {
    log.Printf("[INFO] %s client connect user_id=%d device=%s ip=%s",
        time.Now().Format(time.RFC3339), userID, deviceType, ip)
}

func ClientDisconnect(userID int, deviceType string) {
    log.Printf("[INFO] %s client disconnect user_id=%d device=%s",
        time.Now().Format(time.RFC3339), userID, deviceType)
}

func SMSDeliver(smsID int64, fromUser int, toDevice string, success bool) {
    status := "OK"
    if !success {
        status = "FAIL"
    }
    log.Printf("[INFO] %s sms_deliver sms_id=%d from_user=%d to_device=%s result=%s",
        time.Now().Format(time.RFC3339), smsID, fromUser, toDevice, status)
}
```

- [ ] **Step 2: Commit**

---

### Task 5: Hub (WebSocket 连接池)

**Files:**
- Create: `server/hub/hub.go`

- [ ] **Step 1: 创建 `server/hub/hub.go`**

```go
package hub

import (
    "encoding/json"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

type Message struct {
    Type string          `json:"type"`
    Data json.RawMessage `json:"data"`
}

type Client struct {
    UserID     int
    DeviceType string
    DeviceName string
    Conn       *websocket.Conn
    Send       chan []byte
}

type Hub struct {
    mu        sync.RWMutex
    clients   map[int]map[*Client]bool
    Register  chan *Client
    Unregister chan *Client
    Broadcast chan *userMessage
}

type userMessage struct {
    userID  int
    message []byte
}

func New() *Hub {
    return &Hub{
        clients:    make(map[int]map[*Client]bool),
        Register:   make(chan *Client),
        Unregister: make(chan *Client),
        Broadcast:  make(chan *userMessage),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case c := <-h.Register:
            h.mu.Lock()
            if h.clients[c.UserID] == nil {
                h.clients[c.UserID] = make(map[*Client]bool)
            }
            h.clients[c.UserID][c] = true
            h.mu.Unlock()

        case c := <-h.Unregister:
            h.mu.Lock()
            if clients, ok := h.clients[c.UserID]; ok {
                if _, exists := clients[c]; exists {
                    close(c.Send)
                    delete(clients, c)
                }
                if len(clients) == 0 {
                    delete(h.clients, c.UserID)
                }
            }
            h.mu.Unlock()

        case um := <-h.Broadcast:
            h.mu.RLock()
            if clients, ok := h.clients[um.userID]; ok {
                for c := range clients {
                    select {
                    case c.Send <- um.message:
                    default:
                    }
                }
            }
            h.mu.RUnlock()
        }
    }
}

func (h *Hub) SendToUser(userID int, msg Message) {
    data, _ := json.Marshal(msg)
    h.Broadcast <- &userMessage{userID: userID, message: data}
}

func (h *Hub) WritePump(c *Client) {
    ticker := time.NewTicker(30 * time.Second)
    defer func() {
        ticker.Stop()
        c.Conn.Close()
    }()
    for {
        select {
        case msg, ok := <-c.Send:
            c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if !ok {
                c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
                return
            }
        case <-ticker.C:
            c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

- [ ] **Step 2: 添加依赖**

```bash
cd server && go get github.com/gorilla/websocket
```

- [ ] **Step 3: Commit**

---

### Task 6: API Handler

**Files:**
- Create: `server/api/api.go`

- [ ] **Step 1: 创建 `server/api/api.go`**

```go
package api

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "sms-server/auth"
    "sms-server/hub"
    "sms-server/logger"
    "sms-server/store"
)

type API struct {
    Store *store.Store
    Hub   *hub.Hub
    JWTSecret string
}

type registerReq struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type loginReq struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

func (a *API) Register(w http.ResponseWriter, r *http.Request) {
    var req registerReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid body"}`, 400)
        return
    }
    if req.Username == "" || req.Password == "" {
        http.Error(w, `{"error":"username and password required"}`, 400)
        return
    }
    hashed, err := auth.HashPassword(req.Password)
    if err != nil {
        http.Error(w, `{"error":"internal error"}`, 500)
        return
    }
    _, err = a.Store.CreateUser(req.Username, hashed)
    if err != nil {
        http.Error(w, `{"error":"username taken"}`, 409)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"ok":true}`))
}

func (a *API) Login(w http.ResponseWriter, r *http.Request) {
    var req loginReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid body"}`, 400)
        return
    }
    id, hashed, err := a.Store.GetUserByUsername(req.Username)
    if err != nil || !auth.CheckPassword(hashed, req.Password) {
        http.Error(w, `{"error":"invalid credentials"}`, 401)
        return
    }
    token, err := auth.GenerateToken(id, req.Username, a.JWTSecret)
    if err != nil {
        http.Error(w, `{"error":"internal error"}`, 500)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (a *API) GetDevices(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(int)
    devices, err := a.Store.GetDevices(userID)
    if err != nil {
        http.Error(w, `{"error":"internal error"}`, 500)
        return
    }
    if devices == nil {
        devices = []map[string]interface{}{}
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(devices)
}

func (a *API) GetSMSHistory(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(int)
    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page < 1 {
        page = 1
    }
    size, _ := strconv.Atoi(r.URL.Query().Get("size"))
    if size < 1 || size > 100 {
        size = 20
    }
    logs, err := a.Store.GetSMSHistory(userID, page, size)
    if err != nil {
        http.Error(w, `{"error":"internal error"}`, 500)
        return
    }
    if logs == nil {
        logs = []map[string]interface{}{}
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(logs)
}

func (a *API) GetConnectionStatus(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(int)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "connected", "user_id": strconv.Itoa(userID)})
}

func (a *API) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    claims, err := auth.ParseToken(token, a.JWTSecret)
    if err != nil {
        http.Error(w, "unauthorized", 401)
        return
    }
    deviceType := r.URL.Query().Get("device")
    deviceName := r.URL.Query().Get("name")

    upgrader := hub.WebsocketUpgrader()
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    client := &hub.Client{
        UserID:     claims.UserID,
        DeviceType: deviceType,
        DeviceName: deviceName,
        Conn:       conn,
        Send:       make(chan []byte, 256),
    }
    a.Hub.Register <- client

    a.Store.UpsertDevice(claims.UserID, deviceType, deviceName)
    a.Store.InsertConnectionLog(claims.UserID, deviceType, "connect", r.RemoteAddr)
    logger.ClientConnect(claims.UserID, deviceType, r.RemoteAddr)

    a.Hub.SendToUser(claims.UserID, hub.Message{
        Type: "connection_status",
        Data: json.RawMessage(mustMarshal(map[string]interface{}{
            "device_type": deviceType,
            "device_name": deviceName,
            "online":      true,
        })),
    })

    go a.Hub.WritePump(client)

    a.readPump(client)
}

func (a *API) readPump(c *hub.Client) {
    defer func() {
        a.Hub.Unregister <- c
        a.Store.InsertConnectionLog(c.UserID, c.DeviceType, "disconnect", "")
        logger.ClientDisconnect(c.UserID, c.DeviceType)
        c.Conn.Close()
    }()
    c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.Conn.SetPongHandler(func(string) error {
        c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })
    for {
        _, msgBytes, err := c.Conn.ReadMessage()
        if err != nil {
            break
        }
        var msg hub.Message
        if err := json.Unmarshal(msgBytes, &msg); err != nil {
            continue
        }
        switch msg.Type {
        case "sms_received":
            var data struct {
                Sender     string `json:"sender"`
                Content    string `json:"content"`
                ReceivedAt string `json:"received_at"`
            }
            json.Unmarshal(msg.Data, &data)
            receivedAt, _ := time.Parse(time.RFC3339, data.ReceivedAt)
            if receivedAt.IsZero() {
                receivedAt = time.Now()
            }
            smsID, err := a.Store.InsertSMSLog(c.UserID, data.Sender, data.Content, receivedAt)
            if err != nil {
                a.Hub.SendToUser(c.UserID, hub.Message{
                    Type: "error",
                    Data: json.RawMessage(`{"msg":"store error"}`),
                })
                continue
            }
            a.Store.MarkSMSDelivered(smsID)
            logger.SMSDeliver(smsID, c.UserID, "windows", true)

            deliverData, _ := json.Marshal(map[string]interface{}{
                "id":          smsID,
                "sender":      data.Sender,
                "content":     data.Content,
                "received_at": receivedAt.Format(time.RFC3339),
            })
            a.Hub.SendToUser(c.UserID, hub.Message{
                Type: "sms_deliver",
                Data: deliverData,
            })
            a.Hub.SendToUser(c.UserID, hub.Message{
                Type: "ack",
                Data: json.RawMessage(`{"ok":true}`),
            })
        case "ping":
            c.Send <- []byte(`{"type":"pong","data":{}}`)
        }
    }
}

func mustMarshal(v interface{}) string {
    b, _ := json.Marshal(v)
    return string(b)
}
```

- [ ] **Step 2: 在 `server/hub/hub.go` 中添加 upgrader 导出函数**

```go
func WebsocketUpgrader() websocket.Upgrader {
    return websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
}
```

- [ ] **Step 3: Commit**

---

### Task 7: Middleware

**Files:**
- Create: `server/middleware/middleware.go`

- [ ] **Step 1: 创建 `server/middleware/middleware.go`**

```go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "sms-server/auth"
)

func Auth(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            header := r.Header.Get("Authorization")
            token := strings.TrimPrefix(header, "Bearer ")
            if token == "" {
                http.Error(w, `{"error":"unauthorized"}`, 401)
                return
            }
            claims, err := auth.ParseToken(token, jwtSecret)
            if err != nil {
                http.Error(w, `{"error":"unauthorized"}`, 401)
                return
            }
            ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
            ctx = context.WithValue(ctx, "username", claims.Username)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func CORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if r.Method == "OPTIONS" {
            w.WriteHeader(204)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

- [ ] **Step 2: Commit**

---

### Task 8: main.go 入口

**Files:**
- Create: `server/main.go`

- [ ] **Step 1: 创建 `server/main.go`**

```go
package main

import (
    "log"
    "net/http"

    "sms-server/api"
    "sms-server/config"
    "sms-server/hub"
    "sms-server/logger"
    "sms-server/middleware"
    "sms-server/store"
)

func main() {
    cfg := config.Load()

    s, err := store.New(cfg.DSN())
    if err != nil {
        log.Fatalf("db connect failed: %v", err)
    }
    if err := s.Migrate(); err != nil {
        log.Fatalf("db migrate failed: %v", err)
    }
    logger.ServerStart(cfg.Port)

    h := hub.New()
    go h.Run()

    api := &api.API{Store: s, Hub: h, JWTSecret: cfg.JWTSecret}

    mux := http.NewServeMux()
    mux.HandleFunc("POST /api/register", api.Register)
    mux.HandleFunc("POST /api/login", api.Login)

    authMux := http.NewServeMux()
    authMux.HandleFunc("GET /api/devices", api.GetDevices)
    authMux.HandleFunc("GET /api/sms/history", api.GetSMSHistory)
    authMux.HandleFunc("GET /api/connection/status", api.GetConnectionStatus)

    mux.Handle("/api/", middleware.Auth(cfg.JWTSecret)(authMux))
    mux.HandleFunc("/ws", api.HandleWebSocket)

    handler := middleware.CORS(mux)

    log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
```

- [ ] **Step 2: 编译验证**

```bash
cd server && go build -o sms-server ./...
```

- [ ] **Step 3: Commit**

---

### Task 9: Android 项目脚手架 + Models

**Files:**
- Create: `android/build.gradle.kts`
- Create: `android/app/build.gradle.kts`
- Create: `android/app/src/main/AndroidManifest.xml`
- Create: `android/app/src/main/java/com/smsnotifier/model/Models.kt`

- [ ] **Step 1: 创建项目级 `android/build.gradle.kts`**

```kotlin
plugins {
    id("com.android.application") version "8.2.0" apply false
    id("org.jetbrains.kotlin.android") version "1.9.20" apply false
}
```

- [ ] **Step 2: 创建模块级 `android/app/build.gradle.kts`**

```kotlin
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.smsnotifier"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.smsnotifier"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"
    }

    buildFeatures {
        compose = true
    }
    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.5"
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2024.01.00"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.activity:activity-compose:1.8.2")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.7.0")
    implementation("com.squareup.retrofit2:retrofit:2.9.0")
    implementation("com.squareup.retrofit2:converter-gson:2.9.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.google.code.gson:gson:2.10.1")
}
```

- [ ] **Step 3: 创建 `AndroidManifest.xml`**

```xml
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-permission android:name="android.permission.RECEIVE_SMS" />
    <uses-permission android:name="android.permission.READ_SMS" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />

    <application
        android:name=".SmsApp"
        android:allowBackup="true"
        android:label="SMS Notifier"
        android:supportsRtl="true"
        android:theme="@style/Theme.Material3.DayNight.NoActionBar">
        <activity
            android:name=".MainActivity"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
        <receiver
            android:name=".service.SmsReceiver"
            android:exported="true">
            <intent-filter>
                <action android:name="android.provider.Telephony.SMS_RECEIVED" />
            </intent-filter>
        </receiver>
        <service
            android:name=".service.WebSocketService"
            android:foregroundServiceType="dataSync" />
    </application>
</manifest>
```

- [ ] **Step 4: 创建 `Models.kt`**

```kotlin
package com.smsnotifier.model

data class RegisterRequest(val username: String, val password: String)
data class LoginRequest(val username: String, val password: String)
data class LoginResponse(val token: String)

data class WsMessage(
    val type: String,
    val data: Map<String, Any>? = null
)

data class SmsItem(
    val id: Long,
    val sender: String,
    val content: String,
    val received_at: String,
    val delivered: Boolean = false
)

data class DeviceInfo(
    val id: Int,
    val device_type: String,
    val device_name: String,
    val last_seen: String
)

enum class ConnectionStatus {
    CONNECTED, DISCONNECTED, CONNECTING
}
```

- [ ] **Step 5: Commit**

---

### Task 10: Android REST 客户端

**Files:**
- Create: `android/app/src/main/java/com/smsnotifier/data/ApiClient.kt`

- [ ] **Step 1: 创建 `ApiClient.kt`**

```kotlin
package com.smsnotifier.data

import com.google.gson.Gson
import com.smsnotifier.model.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

class ApiClient(private val baseUrl: String) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()
    private val gson = Gson()
    private var token: String? = null

    fun setToken(t: String) { token = t }

    suspend fun register(username: String, password: String): Boolean = withContext(Dispatchers.IO) {
        val body = gson.toJson(RegisterRequest(username, password))
            .toRequestBody("application/json".toMediaType())
        val req = Request.Builder().url("$baseUrl/api/register").post(body).build()
        val resp = client.newCall(req).execute()
        resp.isSuccessful
    }

    suspend fun login(username: String, password: String): String? = withContext(Dispatchers.IO) {
        val body = gson.toJson(LoginRequest(username, password))
            .toRequestBody("application/json".toMediaType())
        val req = Request.Builder().url("$baseUrl/api/login").post(body).build()
        val resp = client.newCall(req).execute()
        if (resp.isSuccessful) {
            val loginResp = gson.fromJson(resp.body?.string(), LoginResponse::class.java)
            token = loginResp.token
            loginResp.token
        } else null
    }

    suspend fun getDevices(): List<DeviceInfo> = withContext(Dispatchers.IO) {
        val req = Request.Builder().url("$baseUrl/api/devices")
            .header("Authorization", "Bearer $token").build()
        val resp = client.newCall(req).execute()
        gson.fromJson(resp.body?.string(), Array<DeviceInfo>::class.java).toList()
    }

    suspend fun getSmsHistory(page: Int = 1, size: Int = 20): List<SmsItem> = withContext(Dispatchers.IO) {
        val req = Request.Builder().url("$baseUrl/api/sms/history?page=$page&size=$size")
            .header("Authorization", "Bearer $token").build()
        val resp = client.newCall(req).execute()
        gson.fromJson(resp.body?.string(), Array<SmsItem>::class.java).toList()
    }
}
```

- [ ] **Step 2: Commit**

---

### Task 11: Android WebSocket 客户端

**Files:**
- Create: `android/app/src/main/java/com/smsnotifier/data/WebSocketClient.kt`

- [ ] **Step 1: 创建 `WebSocketClient.kt`**

```kotlin
package com.smsnotifier.data

import com.google.gson.Gson
import com.smsnotifier.model.WsMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import okhttp3.*

class WebSocketClient(
    private val serverUrl: String,
    private val token: String,
    private val deviceType: String,
    private val deviceName: String
) {
    private val client = OkHttpClient()
    private var ws: WebSocket? = null
    private val gson = Gson()

    private val _connectionStatus = MutableStateFlow(false)
    val connectionStatus: StateFlow<Boolean> = _connectionStatus

    private val _incomingSms = MutableStateFlow<SmsItem?>(null)
    val incomingSms: StateFlow<SmsItem?> = _incomingSms

    fun connect() {
        val url = "${serverUrl}/ws?token=$token&device=$deviceType&name=$deviceName"
            .replace("http://", "ws://").replace("https://", "wss://")
        val req = Request.Builder().url(url).build()
        ws = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                _connectionStatus.value = true
            }
            override fun onMessage(webSocket: WebSocket, text: String) {
                val msg = gson.fromJson(text, WsMessage::class.java)
                when (msg.type) {
                    "sms_deliver" -> {
                        val data = msg.data as? Map<*, *> ?: return
                        _incomingSms.value = SmsItem(
                            id = (data["id"] as Double).toLong(),
                            sender = data["sender"] as? String ?: "",
                            content = data["content"] as? String ?: "",
                            received_at = data["received_at"] as? String ?: ""
                        )
                    }
                    "connection_status" -> { /* handled by MainScreen */ }
                }
            }
            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                _connectionStatus.value = false
            }
            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                _connectionStatus.value = false
            }
        })
    }

    fun send(type: String, data: Map<String, Any>) {
        val msg = mapOf("type" to type, "data" to data)
        ws?.send(gson.toJson(msg))
    }

    fun disconnect() {
        ws?.close(1000, "user close")
        _connectionStatus.value = false
    }
}
```

- [ ] **Step 2: Commit**

---

### Task 12: Android SMS Receiver

**Files:**
- Create: `android/app/src/main/java/com/smsnotifier/service/SmsReceiver.kt`

- [ ] **Step 1: 创建 `SmsReceiver.kt`**

```kotlin
package com.smsnotifier.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Telephony
import java.time.Instant
import java.time.ZoneOffset

class SmsReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Telephony.Sms.Intents.SMS_RECEIVED_ACTION) return

        val messages = Telephony.Sms.Intents.getMessagesFromIntent(intent)
        for (sms in messages) {
            val data = mapOf(
                "sender" to sms.originatingAddress,
                "content" to sms.messageBody,
                "received_at" to Instant.now().atOffset(ZoneOffset.UTC).toString()
            )
            WebSocketService.sendSms(data)
        }
    }
}
```

- [ ] **Step 2: Commit**

---

### Task 13: Android WebSocket Foreground Service

**Files:**
- Create: `android/app/src/main/java/com/smsnotifier/service/WebSocketService.kt`
- Create: `android/app/src/main/java/com/smsnotifier/SmsApp.kt`

- [ ] **Step 1: 创建 `WebSocketService.kt`**

```kotlin
package com.smsnotifier.service

import android.app.*
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import com.smsnotifier.data.WebSocketClient

class WebSocketService : Service() {
    companion object {
        private var wsClient: WebSocketClient? = null
        const val CHANNEL_ID = "sms_ws_channel"
        const val NOTIFICATION_ID = 1

        fun sendSms(data: Map<String, Any>) {
            wsClient?.send("sms_received", data)
        }

        fun isConnected(): Boolean = wsClient?.connectionStatus?.value ?: false
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("SMS Notifier")
            .setContentText("短信转发服务运行中")
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setOngoing(true)
            .build()
        startForeground(NOTIFICATION_ID, notification, if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
        } else 0)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val prefs = getSharedPreferences("sms_prefs", Context.MODE_PRIVATE)
        val token = prefs.getString("token", "") ?: ""
        val serverUrl = prefs.getString("server_url", "http://10.0.2.2:8080") ?: ""
        val deviceName = Build.MODEL

        wsClient = WebSocketClient(serverUrl, token, "android", deviceName)
        wsClient?.connect()
        return START_STICKY
    }

    override fun onDestroy() {
        wsClient?.disconnect()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID, "SMS 转发服务",
                NotificationManager.IMPORTANCE_LOW
            )
            getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
    }
}
```

- [ ] **Step 2: 创建 `SmsApp.kt`**

```kotlin
package com.smsnotifier

import android.app.Application

class SmsApp : Application()
```

- [ ] **Step 3: Commit**

---

### Task 14: Android UI — Login Screen

**Files:**
- Create: `android/app/src/main/java/com/smsnotifier/ui/theme/Theme.kt`
- Create: `android/app/src/main/java/com/smsnotifier/ui/LoginScreen.kt`
- Create: `android/app/src/main/java/com/smsnotifier/MainActivity.kt`

- [ ] **Step 1: 创建 `Theme.kt`**

```kotlin
package com.smsnotifier.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable

private val LightColors = lightColorScheme()
@Composable
fun SmsNotifierTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = LightColors, content = content)
}
```

- [ ] **Step 2: 创建 `LoginScreen.kt`**

```kotlin
package com.smsnotifier.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.smsnotifier.data.ApiClient
import kotlinx.coroutines.launch

@Composable
fun LoginScreen(
    apiClient: ApiClient,
    onLoginSuccess: (String) -> Unit
) {
    var username by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var error by remember { mutableStateOf("") }
    val scope = rememberCoroutineScope()

    Column(
        modifier = Modifier.fillMaxSize().padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text("SMS Notifier", style = MaterialTheme.typography.headlineMedium)
        Spacer(modifier = Modifier.height(32.dp))
        OutlinedTextField(value = username, onValueChange = { username = it },
            label = { Text("用户名") }, modifier = Modifier.fillMaxWidth())
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(value = password, onValueChange = { password = it },
            label = { Text("密码") }, modifier = Modifier.fillMaxWidth())
        if (error.isNotEmpty()) {
            Text(error, color = MaterialTheme.colorScheme.error)
            Spacer(modifier = Modifier.height(8.dp))
        }
        Spacer(modifier = Modifier.height(16.dp))
        Button(onClick = {
            scope.launch {
                val token = apiClient.login(username, password)
                if (token != null) {
                    onLoginSuccess(token)
                } else {
                    error = "登录失败"
                }
            }
        }, modifier = Modifier.fillMaxWidth()) { Text("登录") }

        Spacer(modifier = Modifier.height(8.dp))
        OutlinedButton(onClick = {
            scope.launch {
                if (apiClient.register(username, password)) {
                    val token = apiClient.login(username, password)
                    if (token != null) onLoginSuccess(token)
                    else error = "注册成功，但登录失败"
                } else {
                    error = "注册失败（用户名可能已被占用）"
                }
            }
        }, modifier = Modifier.fillMaxWidth()) { Text("注册") }
    }
}
```

- [ ] **Step 3: 创建 `MainActivity.kt`**

```kotlin
package com.smsnotifier

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.core.app.ActivityCompat
import com.smsnotifier.data.ApiClient
import com.smsnotifier.service.WebSocketService
import com.smsnotifier.ui.LoginScreen
import com.smsnotifier.ui.MainScreen
import com.smsnotifier.ui.theme.SmsNotifierTheme

class MainActivity : ComponentActivity() {
    private val apiClient = ApiClient("http://10.0.2.2:8080")

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestPermissions()

        val prefs = getSharedPreferences("sms_prefs", 0)
        val savedToken = prefs.getString("token", "")

        setContent {
            SmsNotifierTheme {
                if (savedToken.isNullOrEmpty()) {
                    LoginScreen(apiClient) { token ->
                        prefs.edit().putString("token", token).apply()
                        apiClient.setToken(token)
                        startService()
                        setContent {
                            SmsNotifierTheme { MainScreen(apiClient) }
                        }
                    }
                } else {
                    apiClient.setToken(savedToken)
                    startService()
                    MainScreen(apiClient)
                }
            }
        }
    }

    private fun startService() {
        val intent = Intent(this, WebSocketService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun requestPermissions() {
        val perms = mutableListOf<String>()
        if (ActivityCompat.checkSelfPermission(this, Manifest.permission.RECEIVE_SMS)
            != PackageManager.PERMISSION_GRANTED) perms.add(Manifest.permission.RECEIVE_SMS)
        if (ActivityCompat.checkSelfPermission(this, Manifest.permission.READ_SMS)
            != PackageManager.PERMISSION_GRANTED) perms.add(Manifest.permission.READ_SMS)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ActivityCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
            != PackageManager.PERMISSION_GRANTED) perms.add(Manifest.permission.POST_NOTIFICATIONS)
        if (perms.isNotEmpty()) {
            ActivityCompat.requestPermissions(this, perms.toTypedArray(), 100)
        }
    }
}
```

- [ ] **Step 4: Commit**

---

### Task 15: Android UI — Main Screen + 连接状态

**Files:**
- Create: `android/app/src/main/java/com/smsnotifier/ui/MainScreen.kt`

- [ ] **Step 1: 创建 `MainScreen.kt`**

```kotlin
package com.smsnotifier.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.smsnotifier.data.ApiClient
import com.smsnotifier.model.ConnectionStatus
import com.smsnotifier.model.SmsItem
import com.smsnotifier.service.WebSocketService
import kotlinx.coroutines.delay

@Composable
fun MainScreen(apiClient: ApiClient) {
    var status by remember { mutableStateOf(ConnectionStatus.CONNECTING) }
    var smsList by remember { mutableStateOf(listOf<SmsItem>()) }

    LaunchedEffect(Unit) {
        while (true) {
            status = if (WebSocketService.isConnected())
                ConnectionStatus.CONNECTED else ConnectionStatus.DISCONNECTED
            try {
                smsList = apiClient.getSmsHistory(1, 50)
            } catch (_: Exception) {}
            delay(5000)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("SMS Notifier") },
                actions = {
                    Row(verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.padding(end = 12.dp)) {
                        Box(modifier = Modifier.size(10.dp).clip(CircleShape)
                            .background(if (status == ConnectionStatus.CONNECTED)
                                Color(0xFF4CAF50) else Color(0xFFF44336)))
                        Spacer(modifier = Modifier.width(6.dp))
                        Text(if (status == ConnectionStatus.CONNECTED) "已连接" else "已断开",
                            style = MaterialTheme.typography.bodySmall)
                    }
                }
            )
        }
    ) { padding ->
        Column(modifier = Modifier.padding(padding)) {
            LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                isIndeterminate = status == ConnectionStatus.CONNECTING
            )
            LazyColumn(modifier = Modifier.fillMaxSize()) {
                items(smsList) { sms ->
                    Card(modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 4.dp)) {
                        Column(modifier = Modifier.padding(12.dp)) {
                            Text(sms.sender, style = MaterialTheme.typography.titleSmall)
                            Text(sms.content, style = MaterialTheme.typography.bodyMedium)
                            Text(sms.received_at, style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.outline)
                        }
                    }
                }
            }
        }
    }
}
```

- [ ] **Step 2: Commit**

---

### Task 16: Windows 项目脚手架 + Models

**Files:**
- Create: `windows/SmsNotifier/SmsNotifier.csproj`
- Create: `windows/SmsNotifier/App.xaml`
- Create: `windows/SmsNotifier/App.xaml.cs`
- Create: `windows/SmsNotifier/Models/Models.cs`

- [ ] **Step 1: 创建 `SmsNotifier.csproj`**

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>WinExe</OutputType>
    <TargetFramework>net8.0-windows</TargetFramework>
    <UseWPF>true</UseWPF>
    <Nullable>enable</Nullable>
  </PropertyGroup>
</Project>
```

- [ ] **Step 2: 创建 `App.xaml`**

```xml
<Application x:Class="SmsNotifier.App"
    xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
    xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
    StartupUri="Views/MainWindow.xaml" />
```

- [ ] **Step 3: 创建 `App.xaml.cs`**

```csharp
using System.Windows;

namespace SmsNotifier;

public partial class App : Application { }
```

- [ ] **Step 4: 创建 `Models/Models.cs`**

```csharp
using System.Text.Json.Serialization;

namespace SmsNotifier.Models;

public class RegisterRequest
{
    [JsonPropertyName("username")] public string Username { get; set; } = "";
    [JsonPropertyName("password")] public string Password { get; set; } = "";
}

public class LoginRequest
{
    [JsonPropertyName("username")] public string Username { get; set; } = "";
    [JsonPropertyName("password")] public string Password { get; set; } = "";
}

public class LoginResponse
{
    [JsonPropertyName("token")] public string Token { get; set; } = "";
}

public class SmsItem
{
    [JsonPropertyName("id")] public long Id { get; set; }
    [JsonPropertyName("sender")] public string Sender { get; set; } = "";
    [JsonPropertyName("content")] public string Content { get; set; } = "";
    [JsonPropertyName("received_at")] public string ReceivedAt { get; set; } = "";
}

public class WsMessage
{
    [JsonPropertyName("type")] public string Type { get; set; } = "";
    [JsonPropertyName("data")] public System.Text.Json.JsonElement? Data { get; set; }
}

public enum ConnectionStatus { Connected, Disconnected, Connecting }
```

- [ ] **Step 5: Commit**

---

### Task 17: Windows REST API Service

**Files:**
- Create: `windows/SmsNotifier/Services/ApiService.cs`

- [ ] **Step 1: 创建 `ApiService.cs`**

```csharp
using System.Net.Http.Json;
using SmsNotifier.Models;

namespace SmsNotifier.Services;

public class ApiService
{
    private readonly HttpClient _client;
    private string? _token;

    public ApiService(string baseUrl)
    {
        _client = new HttpClient { BaseAddress = new Uri(baseUrl) };
    }

    public void SetToken(string t) => _token = t;

    public async Task<bool> Register(string username, string password)
    {
        var resp = await _client.PostAsJsonAsync("/api/register",
            new RegisterRequest { Username = username, Password = password });
        return resp.IsSuccessStatusCode;
    }

    public async Task<string?> Login(string username, string password)
    {
        var resp = await _client.PostAsJsonAsync("/api/login",
            new LoginRequest { Username = username, Password = password });
        if (!resp.IsSuccessStatusCode) return null;
        var result = await resp.Content.ReadFromJsonAsync<LoginResponse>();
        _token = result?.Token;
        return _token;
    }

    public async Task<List<SmsItem>> GetSmsHistory(int page = 1, int size = 50)
    {
        var req = new HttpRequestMessage(HttpMethod.Get, $"/api/sms/history?page={page}&size={size}");
        req.Headers.Add("Authorization", $"Bearer {_token}");
        var resp = await _client.SendAsync(req);
        return await resp.Content.ReadFromJsonAsync<List<SmsItem>>() ?? new List<SmsItem>();
    }
}
```

- [ ] **Step 2: Commit**

---

### Task 18: Windows WebSocket Service

**Files:**
- Create: `windows/SmsNotifier/Services/WebSocketService.cs`

- [ ] **Step 1: 创建 `WebSocketService.cs`**

```csharp
using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using SmsNotifier.Models;

namespace SmsNotifier.Services;

public class WebSocketService
{
    private readonly string _serverUrl;
    private readonly string _token;
    private readonly string _deviceType;
    private readonly string _deviceName;
    private ClientWebSocket? _ws;
    private CancellationTokenSource? _cts;

    public event Action<SmsItem>? OnSmsReceived;
    public event Action<ConnectionStatus>? OnStatusChanged;

    public WebSocketService(string serverUrl, string token, string deviceType, string deviceName)
    {
        _serverUrl = serverUrl; _token = token;
        _deviceType = deviceType; _deviceName = deviceName;
    }

    public async Task ConnectAsync()
    {
        OnStatusChanged?.Invoke(ConnectionStatus.Connecting);
        _cts = new CancellationTokenSource();
        _ws = new ClientWebSocket();

        var url = _serverUrl.Replace("http://", "ws://").Replace("https://", "wss://");
        var uri = new Uri($"{url}/ws?token={_token}&device={_deviceType}&name={_deviceName}");

        try
        {
            await _ws.ConnectAsync(uri, _cts.Token);
            OnStatusChanged?.Invoke(ConnectionStatus.Connected);
            _ = ReceiveLoop();
        }
        catch
        {
            OnStatusChanged?.Invoke(ConnectionStatus.Disconnected);
        }
    }

    private async Task ReceiveLoop()
    {
        var buffer = new byte[4096];
        try
        {
            while (_ws?.State == WebSocketState.Open)
            {
                var result = await _ws.ReceiveAsync(buffer, _cts!.Token);
                if (result.MessageType == WebSocketMessageType.Close) break;

                var text = Encoding.UTF8.GetString(buffer, 0, result.Count);
                var msg = JsonSerializer.Deserialize<WsMessage>(text);
                if (msg?.Type == "sms_deliver" && msg.Data.HasValue)
                {
                    var sms = JsonSerializer.Deserialize<SmsItem>(msg.Data.Value.GetRawText());
                    if (sms != null) OnSmsReceived?.Invoke(sms);
                }
            }
        }
        catch { }
        finally
        {
            OnStatusChanged?.Invoke(ConnectionStatus.Disconnected);
        }
    }

    public async Task DisconnectAsync()
    {
        _cts?.Cancel();
        if (_ws?.State == WebSocketState.Open)
            await _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None);
        OnStatusChanged?.Invoke(ConnectionStatus.Disconnected);
    }
}
```

- [ ] **Step 2: Commit**

---

### Task 19: Windows Login Window

**Files:**
- Create: `windows/SmsNotifier/Views/LoginWindow.xaml`
- Create: `windows/SmsNotifier/Views/LoginWindow.xaml.cs`

- [ ] **Step 1: 创建 `LoginWindow.xaml`**

```xml
<Window x:Class="SmsNotifier.Views.LoginWindow"
        xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Title="SMS Notifier - 登录" Width="360" Height="320"
        WindowStartupLocation="CenterScreen" ResizeMode="NoResize">
    <StackPanel Margin="24" VerticalAlignment="Center">
        <TextBlock Text="SMS Notifier" FontSize="20" FontWeight="Bold"
                   HorizontalAlignment="Center" Margin="0,0,0,24"/>
        <TextBlock Text="用户名" Margin="0,0,0,4"/>
        <TextBox x:Name="UsernameBox" Margin="0,0,0,12" Height="28"/>
        <TextBlock Text="密码" Margin="0,0,0,4"/>
        <PasswordBox x:Name="PasswordBox" Margin="0,0,0,16" Height="28"/>
        <TextBlock x:Name="ErrorText" Foreground="Red" Margin="0,0,0,8"
                   Visibility="Collapsed"/>
        <Button x:Name="LoginBtn" Content="登录" Height="32"
                Click="LoginBtn_Click" Margin="0,0,0,8"/>
        <Button x:Name="RegisterBtn" Content="注册" Height="32"
                Click="RegisterBtn_Click"/>
    </StackPanel>
</Window>
```

- [ ] **Step 2: 创建 `LoginWindow.xaml.cs`**

```csharp
using System.Windows;
using SmsNotifier.Services;

namespace SmsNotifier.Views;

public partial class LoginWindow : Window
{
    private readonly ApiService _api;
    public string? Token { get; private set; }

    public LoginWindow(ApiService api)
    {
        InitializeComponent();
        _api = api;
    }

    private async void LoginBtn_Click(object sender, RoutedEventArgs e)
    {
        ErrorText.Visibility = Visibility.Collapsed;
        var token = await _api.Login(UsernameBox.Text, PasswordBox.Password);
        if (token != null)
        {
            Token = token;
            DialogResult = true;
            Close();
        }
        else
        {
            ErrorText.Text = "登录失败";
            ErrorText.Visibility = Visibility.Visible;
        }
    }

    private async void RegisterBtn_Click(object sender, RoutedEventArgs e)
    {
        ErrorText.Visibility = Visibility.Collapsed;
        if (await _api.Register(UsernameBox.Text, PasswordBox.Password))
        {
            var token = await _api.Login(UsernameBox.Text, PasswordBox.Password);
            if (token != null)
            {
                Token = token;
                DialogResult = true;
                Close();
                return;
            }
        }
        ErrorText.Text = "注册失败（用户名可能已被占用）";
        ErrorText.Visibility = Visibility.Visible;
    }
}
```

- [ ] **Step 3: Commit**

---

### Task 20: Windows Main Window + 连接状态

**Files:**
- Create: `windows/SmsNotifier/Views/MainWindow.xaml`
- Create: `windows/SmsNotifier/Views/MainWindow.xaml.cs`

- [ ] **Step 1: 创建 `MainWindow.xaml`**

```xml
<Window x:Class="SmsNotifier.Views.MainWindow"
        xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Title="SMS Notifier" Width="500" Height="600"
        WindowStartupLocation="CenterScreen">
    <DockPanel>
        <StatusBar DockPanel.Dock="Bottom">
            <StatusBarItem>
                <StackPanel Orientation="Horizontal">
                    <Ellipse x:Name="StatusDot" Width="10" Height="10" Margin="4,0,6,0"
                             Fill="Gray"/>
                    <TextBlock x:Name="StatusText" Text="未连接"/>
                </StackPanel>
            </StatusBarItem>
        </StatusBar>
        <ListView x:Name="SmsListView" Margin="8">
            <ListView.View>
                <GridView>
                    <GridViewColumn Header="发送者" Width="120"
                        DisplayMemberBinding="{Binding Sender}"/>
                    <GridViewColumn Header="内容" Width="280"
                        DisplayMemberBinding="{Binding Content}"/>
                    <GridViewColumn Header="时间" Width="80"
                        DisplayMemberBinding="{Binding ReceivedAt}"/>
                </GridView>
            </ListView.View>
        </ListView>
    </DockPanel>
</Window>
```

- [ ] **Step 2: 创建 `MainWindow.xaml.cs`**

```csharp
using System.Collections.ObjectModel;
using System.Windows;
using System.Windows.Media;
using SmsNotifier.Models;
using SmsNotifier.Services;

namespace SmsNotifier.Views;

public partial class MainWindow : Window
{
    private readonly ApiService _api;
    private readonly WebSocketService _ws;
    private readonly TrayIcon _trayIcon;
    public ObservableCollection<SmsItem> SmsItems { get; } = new();

    public MainWindow(ApiService api, string token)
    {
        InitializeComponent();
        _api = api;
        _api.SetToken(token);
        SmsListView.ItemsSource = SmsItems;

        _ws = new WebSocketService("http://localhost:8080", token, "windows",
            Environment.MachineName);
        _ws.OnSmsReceived += sms =>
        {
            Dispatcher.Invoke(() =>
            {
                SmsItems.Insert(0, sms);
                _trayIcon.ShowNotification(sms.Sender, sms.Content);
            });
        };
        _ws.OnStatusChanged += status =>
        {
            Dispatcher.Invoke(() => UpdateStatus(status));
        };

        _trayIcon = new TrayIcon(this);
        Loaded += async (_, _) =>
        {
            await LoadHistory();
            await _ws.ConnectAsync();
        };
        Closing += async (_, _) => await _ws.DisconnectAsync();
    }

    private async Task LoadHistory()
    {
        try
        {
            var items = await _api.GetSmsHistory();
            foreach (var item in items) SmsItems.Add(item);
        }
        catch { }
    }

    private void UpdateStatus(ConnectionStatus status)
    {
        StatusDot.Fill = status switch
        {
            ConnectionStatus.Connected => Brushes.Green,
            ConnectionStatus.Disconnected => Brushes.Red,
            _ => Brushes.Orange
        };
        StatusText.Text = status switch
        {
            ConnectionStatus.Connected => "已连接",
            ConnectionStatus.Disconnected => "已断开",
            _ => "连接中..."
        };
    }
}
```

- [ ] **Step 3: Commit**

---

### Task 21: Windows 系统托盘 + Toast 通知

**Files:**
- Create: `windows/SmsNotifier/TrayIcon.cs`
- Modify: `windows/SmsNotifier/App.xaml` — 去掉 StartupUri

- [ ] **Step 1: 创建 `TrayIcon.cs`**

```csharp
using System.Drawing;
using System.Windows;

namespace SmsNotifier;

public class TrayIcon
{
    private readonly NotifyIcon _notifyIcon;
    private readonly Window _window;

    public TrayIcon(Window window)
    {
        _window = window;
        _notifyIcon = new NotifyIcon
        {
            Text = "SMS Notifier",
            Icon = SystemIcons.Application, // 实际项目中替换为自定义图标
            Visible = true
        };
        _notifyIcon.DoubleClick += (_, _) =>
        {
            _window.Show();
            _window.WindowState = WindowState.Normal;
        };
        _window.StateChanged += (_, _) =>
        {
            if (_window.WindowState == WindowState.Minimized)
                _window.Hide();
        };
    }

    public void ShowNotification(string title, string text)
    {
        _notifyIcon.ShowBalloonTip(3000, title, text, ToolTipIcon.Info);
    }
}
```

- [ ] **Step 2: 修改 `App.xaml` 去掉自动 StartupUri**

```xml
<Application x:Class="SmsNotifier.App"
    xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
    xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" />
```

- [ ] **Step 3: 修改 `App.xaml.cs` 添加启动逻辑**

```csharp
using System.IO;
using System.Text.Json;
using System.Windows;
using SmsNotifier.Services;
using SmsNotifier.Views;

namespace SmsNotifier;

public partial class App
{
    private const string ConfigPath = "config.json";

    protected override void OnStartup(StartupEventArgs e)
    {
        var api = new ApiService("http://localhost:8080");
        string? token = null;

        if (File.Exists(ConfigPath))
        {
            var json = File.ReadAllText(ConfigPath);
            token = JsonSerializer.Deserialize<Dictionary<string, string>>(json)?.GetValueOrDefault("token");
            if (token != null) api.SetToken(token);
        }

        if (token == null)
        {
            var loginWindow = new LoginWindow(api);
            if (loginWindow.ShowDialog() == true)
            {
                token = loginWindow.Token;
                File.WriteAllText(ConfigPath,
                    JsonSerializer.Serialize(new Dictionary<string, string> { ["token"] = token! }));
            }
            else
            {
                Shutdown();
                return;
            }
        }

        var mainWindow = new MainWindow(api, token!);
        mainWindow.Show();
    }
}
```

- [ ] **Step 4: 在 `SmsNotifier.csproj` 中启用隐式 using**

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>WinExe</OutputType>
    <TargetFramework>net8.0-windows</TargetFramework>
    <UseWPF>true</UseWPF>
    <UseWindowsForms>true</UseWindowsForms>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
  </PropertyGroup>
</Project>
```

- [ ] **Step 5: Commit**

---

## 实现顺序

按依赖关系推荐：

1. **Task 1-8**：Go Server（核心依赖，先完成）
2. **Task 16-21**：Windows 客户端（可与 Android 并行）
3. **Task 9-15**：Android 客户端（可与 Windows 并行）

Server → 两个客户端可并行开发。
