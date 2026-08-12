package ws

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func originRequest(origin, host string) *app.RequestContext {
	c := app.NewContext(0)
	var req protocol.Request
	req.SetRequestURI("http://" + host + "/ws")
	req.SetHost(host)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	req.CopyTo(&c.Request)
	return c
}

func TestCheckOrigin(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		origin  string
		host    string
		want    bool
	}{
		{
			name:   "no allow-list falls back to same origin",
			origin: "http://localhost:8080",
			host:   "localhost:8080",
			want:   true,
		},
		{
			name:   "no allow-list rejects a foreign origin",
			origin: "http://evil.example",
			host:   "localhost:8080",
			want:   false,
		},
		{
			name:   "no Origin header is a non-browser client",
			origin: "",
			host:   "localhost:8080",
			want:   true,
		},
		{
			name:    "explicit allow-list accepts a listed origin",
			allowed: []string{"https://app.example"},
			origin:  "https://app.example",
			host:    "api.example",
			want:    true,
		},
		{
			name:    "explicit allow-list rejects an unlisted origin",
			allowed: []string{"https://app.example"},
			origin:  "https://other.example",
			host:    "api.example",
			want:    false,
		},
		{
			name:    "allow-list ignores case and a trailing slash",
			allowed: []string{"https://App.Example/"},
			origin:  "https://app.example",
			host:    "api.example",
			want:    true,
		},
		{
			name:    "allow-list overrides same origin",
			allowed: []string{"https://app.example"},
			origin:  "http://localhost:8080",
			host:    "localhost:8080",
			want:    false,
		},
		{
			name:    "wildcard accepts anything",
			allowed: []string{"*"},
			origin:  "https://anywhere.example",
			host:    "localhost:8080",
			want:    true,
		},
		{
			name:    "blank entries do not create an allow-list",
			allowed: []string{"", "  "},
			origin:  "http://localhost:8080",
			host:    "localhost:8080",
			want:    true,
		},
		{
			name:   "malformed origin is rejected",
			origin: "://not a url",
			host:   "localhost:8080",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewUpgrader(tt.allowed)
			got := upgrader.CheckOrigin(originRequest(tt.origin, tt.host))
			if got != tt.want {
				t.Errorf("CheckOrigin(origin=%q, host=%q) = %v, want %v",
					tt.origin, tt.host, got, tt.want)
			}
		})
	}
}

func TestMaxMessageSizeCoversMaxText(t *testing.T) {
	if maxMessageSize < 4*maxTextLength {
		t.Errorf("maxMessageSize %d is too small for %d runes", maxMessageSize, maxTextLength)
	}
}
