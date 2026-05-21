package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPLimiter is a per-IP token bucket. Entries are GC'd after a quiet
// period so the map doesn't grow unbounded over time.
type IPLimiter struct {
	mu       sync.Mutex
	clients  map[string]*ipEntry
	rps      rate.Limit
	burst    int
	idleKick time.Duration
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPLimiter caps a single IP to `rps` requests/second with a `burst`
// allowance. After `idleKick` of inactivity an entry is removed.
func NewIPLimiter(rps float64, burst int, idleKick time.Duration) *IPLimiter {
	il := &IPLimiter{
		clients:  make(map[string]*ipEntry),
		rps:      rate.Limit(rps),
		burst:    burst,
		idleKick: idleKick,
	}
	go il.gcLoop()
	return il
}

func (il *IPLimiter) Allow(ip string) bool {
	il.mu.Lock()
	defer il.mu.Unlock()

	e, ok := il.clients[ip]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(il.rps, il.burst)}
		il.clients[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter.Allow()
}

func (il *IPLimiter) gcLoop() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-il.idleKick)
		il.mu.Lock()
		for ip, e := range il.clients {
			if e.lastSeen.Before(cutoff) {
				delete(il.clients, ip)
			}
		}
		il.mu.Unlock()
	}
}

// Middleware returns an HTTP middleware that 429s anyone over the limit.
func (il *IPLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !il.Allow(host) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "trop de requêtes", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
