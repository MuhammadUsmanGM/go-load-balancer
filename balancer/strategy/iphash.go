package strategy

import (
	"hash/fnv"
	"net"
	"net/http"

	"github.com/go-load-balancer/balancer"
)

// IPHash provides session persistence based on client IP address.
// The same client IP will always be routed to the same backend (if healthy).
type IPHash struct{}

// NewIPHash creates a new IP hash strategy.
func NewIPHash() *IPHash {
	return &IPHash{}
}

// Select returns the backend based on client IP hash for session persistence.
func (ih *IPHash) Select(backends []*balancer.Backend, r *http.Request) (*balancer.Backend, error) {
	clientIP := ih.getClientIP(r)
	hash := ih.hashIP(clientIP)

	// Try backends starting from hash position
	n := uint64(len(backends))
	for i := uint64(0); i < n; i++ {
		idx := (hash + i) % n
		be := backends[idx]
		if be.IsHealthy() {
			return be, nil
		}
	}

	return nil, balancer.ErrNoHealthyBackends
}

// getClientIP extracts client IP from request.
// Checks X-Forwarded-For and X-Real-IP headers first.
func (ih *IPHash) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := indexOf(xff, ','); idx != -1 {
			return xff[:idx]
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// hashIP creates a hash from the client IP.
func (ih *IPHash) hashIP(ip string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(ip))
	return h.Sum64()
}

// indexOf returns the index of the first occurrence of c in s.
func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// Name returns the strategy name.
func (ih *IPHash) Name() string {
	return "ip-hash"
}
