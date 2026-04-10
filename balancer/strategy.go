package balancer

import (
	"net/http"
)

// Strategy defines the interface for load balancing algorithms.
type Strategy interface {
	// Select returns the next backend to handle the request.
	Select(backends []*Backend, r *http.Request) (*Backend, error)
	// Name returns the strategy name for logging and metrics.
	Name() string
}

