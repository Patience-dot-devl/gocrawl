package config

import (
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

func TestToOptionsParsesProxiesAndPrependsSingle(t *testing.T) {
	c := Default()
	c.Crawl.Proxy = "proxy0:3128"
	c.Crawl.Proxies = []string{"http://proxy1:8080", "socks5://proxy2:1080"}
	c.Crawl.ProxyRotation = "sticky-host"

	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if len(o.Proxies) != 3 {
		t.Fatalf("got %d proxies, want 3", len(o.Proxies))
	}
	// The single Proxy is prepended and the bare host:port gains the http scheme.
	if o.Proxies[0].Scheme != "http" || o.Proxies[0].Host != "proxy0:3128" {
		t.Errorf("proxies[0] = %v, want http://proxy0:3128", o.Proxies[0])
	}
	if o.Proxies[2].Scheme != "socks5" {
		t.Errorf("proxies[2] scheme = %q, want socks5", o.Proxies[2].Scheme)
	}
	if o.ProxyRotation != crawler.RotateStickyHost {
		t.Errorf("ProxyRotation = %v, want sticky-host", o.ProxyRotation)
	}
}

func TestToOptionsRejectsBadProxyScheme(t *testing.T) {
	c := Default()
	c.Crawl.Proxies = []string{"ftp://nope:21"}
	if _, err := c.ToOptions(); err == nil {
		t.Fatal("expected an error for unsupported proxy scheme, got nil")
	}
}

func TestToOptionsRejectsBadRotation(t *testing.T) {
	c := Default()
	c.Crawl.ProxyRotation = "warp-speed"
	if _, err := c.ToOptions(); err == nil {
		t.Fatal("expected an error for unknown proxy rotation, got nil")
	}
}

func TestToOptionsUserAgentsSupersede(t *testing.T) {
	c := Default()
	c.Crawl.UserAgents = []string{"alpha", "beta"}
	c.Crawl.UserAgentRotation = "random"
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if len(o.UserAgents) != 2 {
		t.Fatalf("got %d user agents, want 2", len(o.UserAgents))
	}
	if o.UserAgentRotation != crawler.RotateRandom {
		t.Errorf("UserAgentRotation = %v, want random", o.UserAgentRotation)
	}
}

func TestToOptionsParsesBasicAuth(t *testing.T) {
	c := Default()
	c.Crawl.BasicAuth = "alice:s3cret"
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if o.BasicAuthUser != "alice" || o.BasicAuthPass != "s3cret" {
		t.Errorf("BasicAuthUser/Pass = %q/%q, want alice/s3cret", o.BasicAuthUser, o.BasicAuthPass)
	}
}

func TestToOptionsRejectsBasicAuthWithoutColon(t *testing.T) {
	c := Default()
	c.Crawl.BasicAuth = "alice-no-colon"
	if _, err := c.ToOptions(); err == nil {
		t.Fatal("expected an error for basic_auth missing a colon, got nil")
	}
}
