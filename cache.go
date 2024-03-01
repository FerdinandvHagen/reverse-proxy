package main

import (
	"fmt"
	"github.com/dgraph-io/ristretto"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
)

var cacheRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "cache_requests_total",
		Help: "Number of requests to the cache layer.",
	},
	[]string{},
)

var cacheHits = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Number of cache hits.",
	},
	[]string{},
)

func init() {
	prometheus.MustRegister(cacheRequests)
	prometheus.MustRegister(cacheHits)
}

type Cache struct {
	transport http.RoundTripper
	cache     ristretto.Cache
	whitelist []string
}

// NewCache wraps the provided http.RoundTripper with a caching layer and returns a cached round tripper.
func NewCache(transport http.RoundTripper, whitelist []string) http.RoundTripper {
	return &Cache{
		whitelist: whitelist,
	}
}

func (c *Cache) RoundTrip(request *http.Request) (*http.Response, error) {
	cacheRequests.WithLabelValues().Inc()

	cacheKey := request.Method + ":" + request.URL.String()
	sessionId := request.Header.Get("PHPSESSID")

	fmt.Println("Cache key:", cacheKey)
	fmt.Println("Session ID:", sessionId)

	whitelistKey := request.Method + ":" + request.URL.Path

	// Check if the request is whitelisted
	whitelisted := false
	for _, w := range c.whitelist {
		if w == whitelistKey {
			whitelisted = true
		}
	}

	if !whitelisted {
		return c.transport.RoundTrip(request)
	}

	return c.transport.RoundTrip(request)
}
