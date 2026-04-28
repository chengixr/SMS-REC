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
