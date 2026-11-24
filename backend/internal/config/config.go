package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL, JWTSecret, ListenAddr, AllowedOrigin, AvatarDir, UploadDir string
	AccessTTL, RefreshTTL                                                   time.Duration
	CookieSecure                                                            bool
}

// Load reads configuration from the environment and returns an error for
// missing or invalid required values and invalid optional values.
func Load() (Config, error) {
	c := Config{UploadDir: value("UPLOAD_DIR", "uploads/files"), AvatarDir: value("AVATAR_DIR", "uploads/avatars"), DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: os.Getenv("JWT_SECRET"), ListenAddr: value("LISTEN_ADDR", ":8080"), AllowedOrigin: value("ALLOWED_ORIGIN", "http://localhost:3000"), AccessTTL: 15 * time.Minute, RefreshTTL: 30 * 24 * time.Hour}
	if c.DatabaseURL == "" || len(c.JWTSecret) < 32 {
		return c, errors.New("DATABASE_URL and JWT_SECRET (at least 32 bytes) are required")
	}
	u, err := url.Parse(c.AllowedOrigin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return c, errors.New("ALLOWED_ORIGIN must be an HTTP(S) origin")
	}
	c.AllowedOrigin = strings.TrimSuffix(c.AllowedOrigin, "/")
	for _, v := range []struct {
		name string
		dst  *time.Duration
	}{{"ACCESS_TTL", &c.AccessTTL}, {"REFRESH_TTL", &c.RefreshTTL}} {
		if s := os.Getenv(v.name); s != "" {
			d, err := time.ParseDuration(s)
			if err != nil || d <= 0 {
				return c, fmt.Errorf("%s must be a positive duration", v.name)
			}
			*v.dst = d
		}
	}
	switch value("COOKIE_SECURE", "false") {
	case "true":
		c.CookieSecure = true
	case "false":
	default:
		return c, errors.New("COOKIE_SECURE must be true or false")
	}
	return c, nil
}
func value(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
