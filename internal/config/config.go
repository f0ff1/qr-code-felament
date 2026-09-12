package config

import (
	"bufio"
	"fmt"
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
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		Port:          getenv("PORT", "8080"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://localhost:8080"),
	}
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
