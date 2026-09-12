package config

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

type Settings struct {
	DatabaseURL   string
	RedisAddr     string
	Port          string
	PublicBaseURL string
}

func Load() Settings {
	_ = loadDotEnv(".env", ".env.local")
	return Settings{
		DatabaseURL:   getenv("DATABASE_URL", ""),
		RedisAddr:     getenv("REDIS_ADDR", ""),
		Port:          getenv("PORT", "8080"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", ""),
	}
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
		host = "localhost:8080"
	}
	return scheme + "://" + host
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

func loadDotEnv(paths ...string) error {
	for _, path := range paths {
		if file, err := os.Open(path); err == nil {
			defer file.Close()
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
				value := strings.TrimSpace(parts[1])
				if value != "" && os.Getenv(key) == "" {
					if err := os.Setenv(key, strings.Trim(value, `"'`)); err != nil {
						return fmt.Errorf("set env %s: %w", key, err)
					}
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			return nil
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
