package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func mustIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("bad test IP %q", s)
	}
	return ip
}

const testSecret = "0123456789abcdef0123456789abcdef"

func clearEnv() {
	os.Unsetenv("DSN")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("JWT_TTL")
	os.Unsetenv("ADDR")
	os.Unsetenv("ALLOWED_ORIGINS")
	os.Unsetenv("TRUSTED_PROXIES")
	os.Unsetenv("COOKIE_SECURE")
	os.Unsetenv("COOKIE_DOMAIN")
}

func TestLoadConfig_FromFile(t *testing.T) {
	clearEnv()
	defer clearEnv()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "DSN=postgres://localhost/test\nJWT_SECRET=" + testSecret + "\nJWT_TTL=1h\nADDR=:9090\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(envPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.DSN != "postgres://localhost/test" {
		t.Errorf("expected DSN postgres://localhost/test, got %s", cfg.DSN)
	}
	if cfg.JWTSecret != testSecret {
		t.Errorf("expected JWTSecret %s, got %s", testSecret, cfg.JWTSecret)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("expected Addr :9090, got %s", cfg.Addr)
	}
}

func TestLoadConfig_MissingFile_FallsBackToEnv(t *testing.T) {
	clearEnv()
	defer clearEnv()

	t.Setenv("DSN", "postgres://envhost/db")
	t.Setenv("JWT_SECRET", testSecret)

	cfg, err := LoadConfig("nonexistent.env")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.DSN != "postgres://envhost/db" {
		t.Errorf("expected DSN from env, got %s", cfg.DSN)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	clearEnv()
	defer clearEnv()

	_, err := LoadConfig("nonexistent.env")
	if err == nil {
		t.Error("expected error for missing required fields")
	}
}

func TestLoadConfig_ShortSecretRejected(t *testing.T) {
	clearEnv()
	defer clearEnv()

	t.Setenv("DSN", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", "short")

	if _, err := LoadConfig("nonexistent.env"); err == nil {
		t.Error("expected a short JWT_SECRET to be rejected")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	clearEnv()
	defer clearEnv()

	t.Setenv("DSN", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", testSecret)

	cfg, err := LoadConfig("nonexistent.env")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Addr != ":8080" {
		t.Errorf("expected default Addr :8080, got %s", cfg.Addr)
	}
	if cfg.JWTTTL.Hours() != 24 {
		t.Errorf("expected default JWT_TTL 24h, got %v", cfg.JWTTTL)
	}
	if cfg.CookieSecure {
		t.Error("expected COOKIE_SECURE to default to false")
	}
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    int
		wantErr bool
	}{
		{name: "empty trusts nobody", entries: nil, want: 0},
		{name: "blank entries ignored", entries: []string{"", "  "}, want: 0},
		{name: "cidr", entries: []string{"10.0.0.0/8"}, want: 1},
		{name: "bare ipv4 becomes /32", entries: []string{"192.168.1.10"}, want: 1},
		{name: "bare ipv6 becomes /128", entries: []string{"::1"}, want: 1},
		{name: "mixed", entries: []string{"10.0.0.0/8", " 172.16.0.1 "}, want: 2},
		{name: "invalid address", entries: []string{"not-an-ip"}, wantErr: true},
		{name: "invalid cidr", entries: []string{"10.0.0.0/99"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{TrustedProxies: tt.entries}
			got, err := cfg.ParseTrustedProxies()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("expected %d networks, got %d", tt.want, len(got))
			}
		})
	}
}

func TestParseTrustedProxies_BareIPMatchesOnlyItself(t *testing.T) {
	cfg := &Config{TrustedProxies: []string{"192.168.1.10"}}
	nets, err := cfg.ParseTrustedProxies()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nets[0].Contains(mustIP(t, "192.168.1.10")) {
		t.Error("expected the address itself to match")
	}
	if nets[0].Contains(mustIP(t, "192.168.1.11")) {
		t.Error("expected a neighbouring address not to match")
	}
}
