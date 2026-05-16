package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppPort   string
	JWTSecret string
	DB        DBConfig
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Jakarta",
		d.Host, d.Port, d.User, d.Password, d.Name,
	)
}

func Load() *Config {
	return &Config{
		AppPort:   getEnv("APP_PORT", "8085"),
		JWTSecret: mustEnv("JWT_SECRET"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnv("DB_PORT", "5432"),
			Name:     getEnv("DB_DATABASE", "db_sekolah"),
			User:     getEnv("DB_USERNAME", "root"),
			Password: getEnv("DB_PASSWORD", ""),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required env var not set: " + key)
	}
	return v
}
