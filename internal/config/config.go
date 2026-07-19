package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Addr            string
	DBPath          string
	AdminPIN        string
	ChildPIN        string
	StaticDir       string
	Timezone        string
	Location        *time.Location
	MakeupDays      int
	AllowDefaultPIN bool
	TokenTTL        time.Duration
	LogLevel        string
	Version         string
}

func Load() (Config, error) {
	port := env("PORT", "3003")
	adminPIN := env("ADMIN_PIN", "")
	allowDefault := env("ALLOW_DEFAULT_PIN", "") == "1" || env("SCORECARD_DEV", "") == "1"
	if adminPIN == "" {
		if allowDefault {
			adminPIN = "1234"
		} else {
			return Config{}, fmt.Errorf("ADMIN_PIN 未设置；开发环境可设 ALLOW_DEFAULT_PIN=1 或 SCORECARD_DEV=1 使用默认 1234")
		}
	}
	if adminPIN == "1234" && !allowDefault {
		return Config{}, fmt.Errorf("拒绝使用默认 ADMIN_PIN=1234；请修改 PIN，或开发时设置 ALLOW_DEFAULT_PIN=1")
	}

	tz := env("TZ", "Asia/Shanghai")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	makeupDays := envInt("MAKEUP_DAYS", 30)
	if makeupDays < 0 {
		makeupDays = 0
	}

	childPIN := env("CHILD_PIN", adminPIN)
	return Config{
		Addr:            ":" + port,
		DBPath:          env("DB_PATH", filepath.Join("data", "scorecard.db")),
		AdminPIN:        adminPIN,
		ChildPIN:        childPIN,
		StaticDir:       env("STATIC_DIR", "web/dist"),
		Timezone:        tz,
		Location:        loc,
		MakeupDays:      makeupDays,
		AllowDefaultPIN: allowDefault,
		TokenTTL:        time.Duration(envInt("TOKEN_TTL_HOURS", 24)) * time.Hour,
		LogLevel:        env("LOG_LEVEL", "info"),
		Version:         env("APP_VERSION", "dev"),
	}, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
