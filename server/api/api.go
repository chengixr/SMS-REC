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
	Store     *store.Store
	Hub       *hub.Hub
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
