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

	a := &api.API{Store: s, Hub: h, JWTSecret: cfg.JWTSecret}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", a.Register)
	mux.HandleFunc("POST /api/login", a.Login)

	authMux := http.NewServeMux()
	authMux.HandleFunc("GET /api/devices", a.GetDevices)
	authMux.HandleFunc("GET /api/sms/history", a.GetSMSHistory)
	authMux.HandleFunc("GET /api/connection/status", a.GetConnectionStatus)

	mux.Handle("/api/", middleware.Auth(cfg.JWTSecret)(authMux))
	mux.HandleFunc("/ws", a.HandleWebSocket)

	handler := middleware.CORS(mux)

	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
