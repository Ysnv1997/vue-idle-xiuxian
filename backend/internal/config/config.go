package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the backend service.
type Config struct {
	Env              string
	HTTPPort         string
	DatabaseURL      string
	RedisURL         string
	MigrationsDir    string
	JWTSecret        string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	OAuthStateTTL    time.Duration
	AuctionSweepTTL  time.Duration
	AuctionSweepMax  int
	EnableDevLogin   bool
	ChatAdminUserIDs []string

	LinuxDoClientID     string
	LinuxDoClientSecret string
	LinuxDoRedirectURL  string
	LinuxDoAuthorizeURL string
	LinuxDoTokenURL     string
	LinuxDoUserInfoURL  string
	LinuxDoScope        string

	FrontendLoginSuccessURL string
	FrontendLoginFailureURL string
}

func Load() (Config, error) {
	cfg := Config{
		Env:                 getEnv("APP_ENV", "development"),
		HTTPPort:            getEnv("HTTP_PORT", "8081"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://malindeng:123456789@localhost:5432/xiuxian?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379/0"),
		MigrationsDir:       getEnv("MIGRATIONS_DIR", "./migrations"),
		JWTSecret:           getEnv("JWT_SECRET", "please-change-me"),
		LinuxDoClientID:     getEnv("LINUX_DO_CLIENT_ID", "a43wuLaaqr4Olw0bfPKeGelOeio4Qbmo"),
		LinuxDoClientSecret: getEnv("LINUX_DO_CLIENT_SECRET", "FrGhMLowtqX1sDBcwNtMK3OSBzTXVwvz"),
		LinuxDoRedirectURL:  getEnv("LINUX_DO_REDIRECT_URL", "http://localhost:8081/auth/linux-do/callback"),
		LinuxDoAuthorizeURL: getEnv("LINUX_DO_AUTHORIZE_URL", "https://linux.do/oauth2/authorize"),
		LinuxDoTokenURL:     getEnv("LINUX_DO_TOKEN_URL", "https://linux.do/oauth2/token"),
		LinuxDoUserInfoURL:  getEnv("LINUX_DO_USERINFO_URL", "https://linux.do/api/user"),
		LinuxDoScope:        getEnv("LINUX_DO_SCOPE", "openid profile"),

		FrontendLoginSuccessURL: getEnv("FRONTEND_LOGIN_SUCCESS_URL", "http://localhost:2025/#/auth/callback"),
		FrontendLoginFailureURL: getEnv("FRONTEND_LOGIN_FAILURE_URL", "http://localhost:2025/#/auth/callback"),
	}

	accessTokenTTL, err := parseDurationSeconds("ACCESS_TOKEN_TTL_SECONDS", 3600)
	if err != nil {
		return Config{}, err
	}
	refreshTokenTTL, err := parseDurationSeconds("REFRESH_TOKEN_TTL_SECONDS", 604800)
	if err != nil {
		return Config{}, err
	}
	oauthStateTTL, err := parseDurationSeconds("OAUTH_STATE_TTL_SECONDS", 600)
	if err != nil {
		return Config{}, err
	}
	auctionSweepTTL, err := parseDurationSeconds("AUCTION_SWEEP_INTERVAL_SECONDS", 60)
	if err != nil {
		return Config{}, err
	}
	auctionSweepMax, err := parsePositiveInt("AUCTION_SWEEP_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}

	cfg.AccessTokenTTL = accessTokenTTL
	cfg.RefreshTokenTTL = refreshTokenTTL
	cfg.OAuthStateTTL = oauthStateTTL
	cfg.AuctionSweepTTL = auctionSweepTTL
	cfg.AuctionSweepMax = auctionSweepMax
	cfg.EnableDevLogin = getEnv("ENABLE_DEV_LOGIN", "true") == "true"
	cfg.ChatAdminUserIDs = parseCSV(getEnv("CHAT_ADMIN_USER_IDS", "76bae928-45c2-409a-b066-44be9ed2952c"))

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET cannot be empty")
	}

	return cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.HTTPPort)
}

func parseDurationSeconds(key string, fallback int64) (time.Duration, error) {
	value := getEnv(key, strconv.FormatInt(fallback, 10))
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("invalid %s: %q", key, value)
	}
	return time.Duration(seconds) * time.Second, nil
}

func parsePositiveInt(key string, fallback int) (int, error) {
	value := getEnv(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s: %q", key, value)
	}
	return parsed, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseCSV(input string) []string {
	if strings.TrimSpace(input) == "" {
		return []string{}
	}
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
