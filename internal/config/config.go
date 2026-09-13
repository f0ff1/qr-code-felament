package config

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
	EnvTest        Environment = "test"
)

type Settings struct {
	AppEnv        Environment
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Port          string
	PublicBaseURL string

	BambuSecretsKey string
	RequirePostgres bool
	AllowInMemory   bool
	SQLDebug        bool

	AdminUsername     string
	AdminPassword     string // development bootstrap only
	AdminPasswordHash string // bcrypt hash preferred
	SessionSecret     string
	SessionTTL        time.Duration
	CookieSecure      bool

	BambuMonitorInterval time.Duration
	RuntimeSyncInterval  time.Duration
	RuntimeSyncTimeout   time.Duration
	CloudSyncInterval    time.Duration
	CloudSyncTimeout     time.Duration
	SSEHeartbeat         time.Duration
	SchedulerTick        time.Duration
	NotificationTTL      time.Duration

	HTTPReadHeaderTimeout time.Duration
	HTTPIdleTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPShutdownTimeout   time.Duration
	HTTPMaxBodyBytes      int64

	SpoolLowWeightG int
	PrinterPowerKW  float64
	LaborPerHour    float64
	ElectricityPerson float64
	ElectricityLegal  float64

	CORSAllowedOrigins []string
	MigrationsDir      string
}

func Load() Settings {
	_ = loadDotEnv(".env", ".env.local")

	env := Environment(strings.ToLower(strings.TrimSpace(getenv("APP_ENV", "development"))))
	if env == "" {
		env = EnvDevelopment
	}

	s := Settings{
		AppEnv:        env,
		DatabaseURL:   getenv("DATABASE_URL", ""),
		RedisAddr:     getenv("REDIS_ADDR", ""),
		RedisPassword: getenv("REDIS_PASSWORD", ""),
		RedisDB:       getenvInt("REDIS_DB", 0),
		Port:          getenv("PORT", "8080"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", ""),

		BambuSecretsKey: firstNonEmpty(getenv("BAMBU_SECRETS_KEY", ""), getenv("APP_SECRET", "")),
		RequirePostgres: getenvBool("REQUIRE_POSTGRES", env == EnvProduction || onRailway()),
		AllowInMemory:   getenvBool("ALLOW_INMEMORY", false),
		SQLDebug:        getenvBool("SQL_DEBUG", false),

		AdminUsername:     getenv("ADMIN_USERNAME", "admin"),
		AdminPassword:     getenv("ADMIN_PASSWORD", ""),
		AdminPasswordHash: getenv("ADMIN_PASSWORD_HASH", ""),
		SessionSecret:     firstNonEmpty(getenv("SESSION_SECRET", ""), getenv("APP_SECRET", ""), getenv("BAMBU_SECRETS_KEY", "")),
		SessionTTL:        getenvDuration("SESSION_TTL", 24*time.Hour),
		CookieSecure:      getenvBool("COOKIE_SECURE", env == EnvProduction),

		BambuMonitorInterval:  getenvDuration("BAMBU_MONITOR_INTERVAL", 7*time.Second),
		RuntimeSyncInterval:   getenvDuration("RUNTIME_SYNC_INTERVAL", 2*time.Second),
		RuntimeSyncTimeout:    getenvDuration("RUNTIME_SYNC_TIMEOUT", 1500*time.Millisecond),
		CloudSyncInterval:     getenvDuration("CLOUD_SYNC_INTERVAL", 2*time.Minute),
		CloudSyncTimeout:      getenvDuration("CLOUD_SYNC_TIMEOUT", 45*time.Second),
		SSEHeartbeat:          getenvDuration("SSE_HEARTBEAT", 15*time.Second),
		SchedulerTick:         getenvDuration("SCHEDULER_TICK", 10*time.Second),
		NotificationTTL:       getenvDuration("NOTIFICATION_TTL", 24*time.Hour),

		HTTPReadHeaderTimeout: getenvDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		HTTPIdleTimeout:       getenvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		HTTPWriteTimeout:      getenvDuration("HTTP_WRITE_TIMEOUT", 60*time.Second),
		HTTPShutdownTimeout:   getenvDuration("HTTP_SHUTDOWN_TIMEOUT", 5*time.Second),
		HTTPMaxBodyBytes:      int64(getenvInt("HTTP_MAX_BODY_BYTES", 1<<20)),

		SpoolLowWeightG:   getenvInt("SPOOL_LOW_WEIGHT_G", 200),
		PrinterPowerKW:    getenvFloat("PRINTER_POWER_KW", 0.35),
		LaborPerHour:      getenvFloat("LABOR_PER_HOUR", 12.0),
		ElectricityPerson: getenvFloat("ELECTRICITY_RATE_PERSON", 0.1176),
		ElectricityLegal:  getenvFloat("ELECTRICITY_RATE_LEGAL", 0.18381),

		CORSAllowedOrigins: splitCSV(getenv("CORS_ALLOWED_ORIGINS", "")),
		MigrationsDir:      getenv("MIGRATIONS_DIR", "db/migrations"),
	}

	if s.AllowInMemory {
		s.RequirePostgres = false
	}
	return s
}

func (s Settings) IsProduction() bool {
	return s.AppEnv == EnvProduction
}

func (s Settings) IsDevelopment() bool {
	return s.AppEnv == EnvDevelopment || s.AppEnv == EnvTest
}

// AuthConfigured reports whether admin credentials are present.
func (s Settings) AuthConfigured() bool {
	return strings.TrimSpace(s.AdminPasswordHash) != "" || strings.TrimSpace(s.AdminPassword) != ""
}

func (s Settings) Validate() error {
	if s.Port == "" {
		return fmt.Errorf("PORT is required")
	}
	if s.SpoolLowWeightG < 0 {
		return fmt.Errorf("SPOOL_LOW_WEIGHT_G must be >= 0")
	}
	if s.IsProduction() {
		if strings.TrimSpace(s.BambuSecretsKey) == "" {
			return fmt.Errorf("BAMBU_SECRETS_KEY (or APP_SECRET) is required in production")
		}
		if isWeakSecret(s.BambuSecretsKey) {
			return fmt.Errorf("BAMBU_SECRETS_KEY is too weak for production")
		}
		if strings.TrimSpace(s.SessionSecret) == "" || len(s.SessionSecret) < 16 {
			return fmt.Errorf("SESSION_SECRET must be set (>=16 chars) in production")
		}
		if !s.AuthConfigured() {
			return fmt.Errorf("ADMIN_PASSWORD_HASH or ADMIN_PASSWORD is required in production")
		}
		if s.DatabaseURL == "" && s.RequirePostgres {
			return fmt.Errorf("DATABASE_URL is required when REQUIRE_POSTGRES=true")
		}
	}
	return nil
}

func (s Settings) PublicOrigin(host, forwardedProto string, tls bool) string {
	if origin := usableOrigin(s.PublicBaseURL); origin != "" && !isLoopbackOrigin(origin) {
		return origin
	}
	if domain := strings.TrimSpace(os.Getenv("RAILWAY_PUBLIC_DOMAIN")); domain != "" {
		domain = strings.TrimRight(domain, "/")
		if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
			if origin := usableOrigin(domain); origin != "" {
				return origin
			}
		} else {
			return "https://" + strings.TrimPrefix(domain, "/")
		}
	}
	if origin := usableOrigin(s.PublicBaseURL); origin != "" {
		return origin
	}

	scheme := "http"
	if tls || strings.EqualFold(forwardedProto, "https") {
		scheme = "https"
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "localhost:" + s.Port
	}
	return scheme + "://" + host
}

func (s Settings) CORSOriginAllowed(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	if len(s.CORSAllowedOrigins) == 0 {
		if pub := usableOrigin(s.PublicBaseURL); pub != "" {
			return strings.EqualFold(origin, pub)
		}
		return s.IsDevelopment()
	}
	for _, allowed := range s.CORSAllowedOrigins {
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(strings.TrimRight(allowed, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}

func usableOrigin(raw string) string {
	raw = strings.TrimSpace(strings.TrimRight(raw, "/"))
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func isLoopbackOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	hostname := parsed.Hostname()
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func onRailway() bool {
	return os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("RAILWAY_ENVIRONMENT_NAME") != ""
}

func isWeakSecret(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if len(v) < 16 {
		return true
	}
	weak := []string{
		"change-me",
		"changeme",
		"secret",
		"filament-tracker-dev-secret-change-me",
		"password",
	}
	for _, w := range weak {
		if strings.Contains(v, w) {
			return true
		}
	}
	return false
}

// loadDotEnv loads each file in order; later files overlay earlier ones for unset keys only
// relative to process env — keys already in the OS environment always win.
func loadDotEnv(paths ...string) error {
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if key == "" {
				continue
			}
			if os.Getenv(key) == "" {
				if err := os.Setenv(key, value); err != nil {
					_ = file.Close()
					return fmt.Errorf("set env %s: %w", key, err)
				}
			}
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return fmt.Errorf("read %s: %w", path, scanErr)
		}
	}
	return nil
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getenvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func getenvFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return n
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
