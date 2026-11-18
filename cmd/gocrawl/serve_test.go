package main

import (
	"net/http"
	"testing"
)

func TestServeDefaultsToLoopback(t *testing.T) {
	cmd := newServeCmd()
	addr, _ := cmd.Flags().GetString("addr")
	if addr != "127.0.0.1:8080" {
		t.Fatalf("default --addr = %q, want 127.0.0.1:8080", addr)
	}
}

func TestNewHTTPServerSetsTimeouts(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if srv.ReadHeaderTimeout <= 0 {
		t.Fatalf("ReadHeaderTimeout = %v, want > 0", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout <= 0 {
		t.Fatalf("IdleTimeout = %v, want > 0", srv.IdleTimeout)
	}
}

func TestIsLoopbackBind(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080": true,
		"localhost:8080": true,
		"[::1]:8080":     true,
		":8080":          false,
		"0.0.0.0:8080":   false,
		"192.168.1.5:80": false,
	}
	for addr, want := range cases {
		if got := isLoopbackBind(addr); got != want {
			t.Errorf("isLoopbackBind(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestServeAuthToken(t *testing.T) {
	cases := []struct {
		name, addr, flagToken, envToken string
		noAuth                          bool
		want                            string // "generated" for a random token
		wantErr                         bool
	}{
		{name: "loopback needs none", addr: "127.0.0.1:8080", want: ""},
		{name: "loopback honours an explicit token", addr: "127.0.0.1:8080", flagToken: "t1", want: "t1"},
		{name: "non-loopback generates one", addr: "0.0.0.0:8080", want: "generated"},
		{name: "flag wins over env", addr: "0.0.0.0:8080", flagToken: "t1", envToken: "t2", want: "t1"},
		{name: "env token", addr: ":8080", envToken: "t2", want: "t2"},
		{name: "no-auth opts out", addr: "0.0.0.0:8080", noAuth: true, want: ""},
		{name: "no-auth with a token is an error", addr: "0.0.0.0:8080", flagToken: "t1", noAuth: true, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("GOCRAWL_AUTH_TOKEN", c.envToken)
			cmd := newServeCmd()
			if c.flagToken != "" {
				_ = cmd.Flags().Set("auth-token", c.flagToken)
			}
			if c.noAuth {
				_ = cmd.Flags().Set("no-auth", "true")
			}
			got, err := serveAuthToken(cmd, c.addr)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			switch {
			case c.want == "generated":
				if len(got) != 48 {
					t.Errorf("token = %q, want a generated 48-char hex token", got)
				}
			case got != c.want:
				t.Errorf("token = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDisplayAddr(t *testing.T) {
	for addr, want := range map[string]string{
		":8080":          "localhost:8080",
		"0.0.0.0:8080":   "localhost:8080",
		"[::]:8080":      "localhost:8080",
		"127.0.0.1:9000": "127.0.0.1:9000",
		"10.0.0.5:8080":  "10.0.0.5:8080",
	} {
		if got := displayAddr(addr); got != want {
			t.Errorf("displayAddr(%q) = %q, want %q", addr, got, want)
		}
	}
}
