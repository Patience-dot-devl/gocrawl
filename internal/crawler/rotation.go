package crawler

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
)

// RotationStrategy selects how a pool (of proxies or User-Agent strings) picks an entry for
// a given request. It is shared by the proxy pool and the User-Agent pool so both rotate
// with identical semantics.
type RotationStrategy int

const (
	// RotateRoundRobin cycles through the pool in order, one entry per request. It is the
	// default once a pool has more than one entry.
	RotateRoundRobin RotationStrategy = iota
	// RotateRandom picks a uniformly random entry per request.
	RotateRandom
	// RotateStickyHost maps each target host to a fixed entry (by hash), so every request to a
	// given host uses the same proxy/User-Agent. For proxies this avoids a server seeing a
	// single session arrive from several IPs.
	RotateStickyHost
	// RotateOff disables rotation: the first entry is always used. A pool with a single entry
	// behaves this way regardless of strategy.
	RotateOff
)

// ParseRotation converts a config string into a RotationStrategy. An empty string defaults to
// round-robin, so simply supplying a multi-entry pool rotates without extra configuration.
func ParseRotation(s string) (RotationStrategy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "round-robin", "roundrobin", "rr":
		return RotateRoundRobin, nil
	case "random", "rand":
		return RotateRandom, nil
	case "sticky-host", "sticky", "stickyhost":
		return RotateStickyHost, nil
	case "off", "none":
		return RotateOff, nil
	default:
		return RotateRoundRobin, fmt.Errorf("unknown rotation strategy %q (want off, round-robin, random, or sticky-host)", s)
	}
}

// rotator turns a strategy plus a pool size into an index per request. It is safe for
// concurrent use: round-robin advances a single atomic counter, and random/sticky are
// stateless (math/rand/v2's top-level source is goroutine-safe).
type rotator struct {
	strategy RotationStrategy
	counter  atomic.Uint64
}

// index returns the pool position to use for a request to host. n is the pool size; host is
// only consulted for the sticky-host strategy.
func (r *rotator) index(n int, host string) int {
	if n <= 1 {
		return 0
	}
	switch r.strategy {
	case RotateOff:
		return 0
	case RotateRandom:
		return rand.IntN(n)
	case RotateStickyHost:
		return int(hashHost(host) % uint64(n))
	default: // RotateRoundRobin
		return int((r.counter.Add(1) - 1) % uint64(n))
	}
}

func hashHost(h string) uint64 {
	s := fnv.New64a()
	_, _ = s.Write([]byte(strings.ToLower(h)))
	return s.Sum64()
}

// UAPool selects a User-Agent string per request. It is built from Options and shared by the
// raw HTTP fetcher and the headless renderer so both rotate identically. A nil pool, or one
// with no agents, yields the empty string (callers then leave the header unset).
type UAPool struct {
	agents []string
	rot    rotator
}

// NewUAPool builds a User-Agent pool from opts. When UserAgents is set it is the pool;
// otherwise the single UserAgent (if any) is the only entry.
func NewUAPool(opts Options) *UAPool {
	agents := opts.UserAgents
	if len(agents) == 0 && opts.UserAgent != "" {
		agents = []string{opts.UserAgent}
	}
	return &UAPool{agents: agents, rot: rotator{strategy: opts.UserAgentRotation}}
}

// Next returns the User-Agent to use for a request to host, advancing rotation state as the
// strategy requires.
func (p *UAPool) Next(host string) string {
	if p == nil || len(p.agents) == 0 {
		return ""
	}
	return p.agents[p.rot.index(len(p.agents), host)]
}

// Default returns the pool's first agent without advancing rotation state. The headless
// renderer uses it as the browser-level User-Agent; per-navigation rotation is layered on top.
func (p *UAPool) Default() string {
	if p == nil || len(p.agents) == 0 {
		return ""
	}
	return p.agents[0]
}

// Rotates reports whether the pool has more than one agent (i.e. rotation can occur).
func (p *UAPool) Rotates() bool {
	return p != nil && len(p.agents) > 1
}

// proxyPool selects a proxy URL per request. It is nil when no proxies are configured, in
// which case the fetcher leaves the transport's default proxy behavior (environment) in place.
type proxyPool struct {
	proxies []*url.URL
	rot     rotator
}

func newProxyPool(opts Options) *proxyPool {
	if len(opts.Proxies) == 0 {
		return nil
	}
	return &proxyPool{proxies: opts.Proxies, rot: rotator{strategy: opts.ProxyRotation}}
}

// proxyFunc returns a function suitable for http.Transport.Proxy: it selects a proxy from the
// pool per request according to the rotation strategy.
func (p *proxyPool) proxyFunc() func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		host := ""
		if req != nil && req.URL != nil {
			host = req.URL.Hostname()
		}
		return p.proxies[p.rot.index(len(p.proxies), host)], nil
	}
}
