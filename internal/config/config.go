package config

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

const minJWTSecretLen = 32

type Config struct {
	DSN            string        `env:"DSN" env-required:"true"`
	ScyllaHosts    []string      `env:"SCYLLA_HOSTS" env-default:"localhost" env-separator:","`
	ScyllaKeyspace string        `env:"SCYLLA_KEYSPACE" env-default:"ws"`
	RedisAddr      string        `env:"REDIS_ADDR" env-default:"localhost:6379"`
	RedisPassword  string        `env:"REDIS_PASSWORD"`
	JWTSecret      string        `env:"JWT_SECRET" env-required:"true"`
	JWTTTL         time.Duration `env:"JWT_TTL" env-default:"24h"`
	Addr           string        `env:"ADDR" env-default:":8080"`
	AllowedOrigins []string      `env:"ALLOWED_ORIGINS" env-separator:","`

	TrustedProxies []string `env:"TRUSTED_PROXIES" env-separator:","`

	CookieSecure bool   `env:"COOKIE_SECURE" env-default:"false"`
	CookieDomain string `env:"COOKIE_DOMAIN"`

	StorageRequired bool `env:"STORAGE_REQUIRED" env-default:"true"`

	RunMigrations bool `env:"RUN_MIGRATIONS" env-default:"true"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}
	if err := cleanenv.ReadConfig(path, cfg); err != nil {
		if envErr := cleanenv.ReadEnv(cfg); envErr != nil {
			return nil, fmt.Errorf("config: %w", envErr)
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if len(c.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("config: JWT_SECRET must be at least %d characters, got %d",
			minJWTSecretLen, len(c.JWTSecret))
	}
	if _, err := c.ParseTrustedProxies(); err != nil {
		return err
	}
	return nil
}

func (c *Config) ParseTrustedProxies() ([]*net.IPNet, error) {
	cidrs := make([]*net.IPNet, 0, len(c.TrustedProxies))
	for _, raw := range c.TrustedProxies {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("config: TRUSTED_PROXIES: invalid address %q", entry)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			cidrs = append(cidrs, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("config: TRUSTED_PROXIES: invalid CIDR %q: %w", entry, err)
		}
		cidrs = append(cidrs, network)
	}
	return cidrs, nil
}
