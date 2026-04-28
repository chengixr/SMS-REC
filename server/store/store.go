package store

import (
	"database/sql"
	"fmt"
	"time"

	"strconv"

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
		if err := rows.Scan(&id, &dtype, &dname, &lastSeen); err != nil {
			return nil, err
		}
		devices = append(devices, map[string]interface{}{
			"id":          strconv.Itoa(id),
			"device_type": dtype,
			"device_name": dname,
			"last_seen":   lastSeen.Format(time.RFC3339),
		})
	}
	return devices, nil
}

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
		if err := rows.Scan(&id, &sender, &content, &receivedAt, &delivered, &deliveredAt); err != nil {
			return nil, err
		}
		item := map[string]interface{}{
			"id":          id,
			"sender":      sender,
			"content":     content,
			"received_at": receivedAt.Format(time.RFC3339),
			"delivered":   delivered,
		}
		if deliveredAt.Valid {
			item["delivered_at"] = deliveredAt.Time.Format(time.RFC3339)
		}
		logs = append(logs, item)
	}
	return logs, nil
}

func (s *Store) InsertConnectionLog(userID int, deviceType, event, detail string) error {
	_, err := s.DB.Exec(
		"INSERT INTO connection_logs (user_id, device_type, event, detail) VALUES (?, ?, ?, ?)",
		userID, deviceType, event, detail)
	return err
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
