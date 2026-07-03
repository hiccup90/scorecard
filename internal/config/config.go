package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Addr      string
	DBPath    string
	AdminPIN  string
	ChildPIN  string
	StaticDir string
}

func Load() Config {
	port := env("PORT", "3003")
	adminPIN := env("ADMIN_PIN", "1234")
	return Config{
		Addr:      ":" + port,
		DBPath:    env("DB_PATH", filepath.Join("data", "scorecard.db")),
		AdminPIN:  adminPIN,
		ChildPIN:  env("CHILD_PIN", adminPIN),
		StaticDir: env("STATIC_DIR", "web/dist"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
