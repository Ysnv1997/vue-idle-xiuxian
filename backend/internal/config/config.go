package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 定义后端服务运行时配置（来自环境变量）。
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
	HuntingSweepTTL  time.Duration
	HuntingSweepMax  int
	ChatCleanupTTL   time.Duration
	ChatRetentionTTL time.Duration
	ChatRetentionMax int
	EnableDevLogin   bool
	ChatAdminUserIDs []string

	EnableRechargeMock     bool
	RechargeCallbackSecret string
	RechargeEPayPID        string
	RechargeEPayKey        string
	RechargeEPayBaseURL    string
	RechargeNotifyURL      string
	RechargeReturnURL      string

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
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	// 从环境变量读取配置，未设置时使用开发默认值。
	cfg := Config{
		Env:                 getEnv("APP_ENV", "development"),
		HTTPPort:            getEnv("HTTP_PORT", "8081"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://malindeng:123456789@localhost:5432/xiuxian?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379/0"),
		MigrationsDir:       getEnv("MIGRATIONS_DIR", "./migrations"),
		JWTSecret:           getEnv("JWT_SECRET", "please-change-me"),
		LinuxDoClientID:     getEnv("LINUX_DO_CLIENT_ID", "a43wuLaaqr4Olw0bfPKeGelOeio4Qbmo"),
		LinuxDoClientSecret: getEnv("LINUX_DO_CLIENT_SECRET", "FrGhMLowtqX1sDBcwNtMK3OSBzTXVwvz"),
		LinuxDoRedirectURL:  getEnv("LINUX_DO_REDIRECT_URL", "http://localhost:8081/api/v1/auth/linux-do/callback"),
		LinuxDoAuthorizeURL: getEnv("LINUX_DO_AUTHORIZE_URL", "https://linux.do/oauth2/authorize"),
		LinuxDoTokenURL:     getEnv("LINUX_DO_TOKEN_URL", "https://linux.do/oauth2/token"),
		LinuxDoUserInfoURL:  getEnv("LINUX_DO_USERINFO_URL", "https://linux.do/api/user"),
		LinuxDoScope:        getEnv("LINUX_DO_SCOPE", "openid profile"),

		RechargeCallbackSecret: getEnv("RECHARGE_CALLBACK_SECRET", ""),
		RechargeEPayPID:        getEnv("RECHARGE_EPAY_PID", ""),
		RechargeEPayKey:        getEnv("RECHARGE_EPAY_KEY", ""),
		RechargeEPayBaseURL:    getEnv("RECHARGE_EPAY_BASE_URL", "https://credit.linux.do/epay"),
		RechargeNotifyURL:      getEnv("RECHARGE_NOTIFY_URL", ""),
		RechargeReturnURL:      getEnv("RECHARGE_RETURN_URL", ""),

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
	huntingSweepTTL, err := parseDurationSeconds("HUNTING_SWEEP_INTERVAL_SECONDS", 1)
	if err != nil {
		return Config{}, err
	}
	huntingSweepMax, err := parsePositiveInt("HUNTING_SWEEP_BATCH_SIZE", 200)
	if err != nil {
		return Config{}, err
	}
	chatCleanupTTL, err := parseDurationSeconds("CHAT_CLEANUP_INTERVAL_SECONDS", 30)
	if err != nil {
		return Config{}, err
	}
	chatRetentionTTL, err := parseDurationSeconds("CHAT_RETENTION_SECONDS", 600)
	if err != nil {
		return Config{}, err
	}
	chatRetentionMax, err := parsePositiveInt("CHAT_RETENTION_MAX_MESSAGES", 500)
	if err != nil {
		return Config{}, err
	}

	cfg.AccessTokenTTL = accessTokenTTL
	cfg.RefreshTokenTTL = refreshTokenTTL
	cfg.OAuthStateTTL = oauthStateTTL
	cfg.AuctionSweepTTL = auctionSweepTTL
	cfg.AuctionSweepMax = auctionSweepMax
	cfg.HuntingSweepTTL = huntingSweepTTL
	cfg.HuntingSweepMax = huntingSweepMax
	cfg.ChatCleanupTTL = chatCleanupTTL
	cfg.ChatRetentionTTL = chatRetentionTTL
	cfg.ChatRetentionMax = chatRetentionMax
	cfg.EnableDevLogin = getEnv("ENABLE_DEV_LOGIN", "true") == "true"
	cfg.EnableRechargeMock = parseBoolEnv("ENABLE_RECHARGE_MOCK", cfg.Env != "production")
	cfg.ChatAdminUserIDs = parseCSV(getEnv("CHAT_ADMIN_USER_IDS", "76bae928-45c2-409a-b066-44be9ed2952c"))
	if strings.TrimSpace(cfg.RechargeEPayKey) == "" {
		cfg.RechargeEPayKey = strings.TrimSpace(cfg.RechargeCallbackSecret)
	}

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET cannot be empty")
	}

	return cfg, nil
}

func loadDotEnv() error {
	customPath := strings.TrimSpace(os.Getenv("APP_CONFIG_ENV_FILE"))
	if customPath != "" {
		if err := loadDotEnvFile(customPath); err != nil {
			return fmt.Errorf("load APP_CONFIG_ENV_FILE %q: %w", customPath, err)
		}
		return nil
	}

	// 兼容两种常见启动目录：
	// 1) 在 backend 目录启动（.env）
	// 2) 在仓库根目录启动（backend/.env）
	for _, candidate := range []string{"backend/.env", ".env"} {
		if err := loadDotEnvFile(candidate); err != nil {
			return fmt.Errorf("load env file %q: %w", candidate, err)
		}
	}

	return nil
}

func loadDotEnvFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if _, err := os.Stat(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	return godotenv.Load(absPath)
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

func parseBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(getEnv(key, "")))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
