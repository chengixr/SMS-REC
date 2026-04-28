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
