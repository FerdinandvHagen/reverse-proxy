package main

import (
	"bytes"
	"fmt"
	"github.com/dgraph-io/ristretto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"io"
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
	cache     *ristretto.Cache
}

// NewCache wraps the provided http.RoundTripper with a caching layer and returns a cached round tripper.
func NewCache(transport http.RoundTripper) (http.RoundTripper, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &Cache{
		transport: transport,
		cache:     cache,
	}, nil
}

type cachedResponse struct {
	*http.Response
	content []byte
}

func (rc *cachedResponse) ToResponse() *http.Response {
	header := rc.Response.Header.Clone()
	header.Set("X-Cache", "HIT")

	return &http.Response{
		Status:           rc.Response.Status,
		StatusCode:       rc.Response.StatusCode,
		Proto:            rc.Response.Proto,
		ProtoMajor:       rc.Response.ProtoMajor,
		ProtoMinor:       rc.Response.ProtoMinor,
		Header:           header,
		Body:             io.NopCloser(bytes.NewBuffer(rc.content)),
		ContentLength:    rc.Response.ContentLength,
		TransferEncoding: rc.Response.TransferEncoding,
		Close:            rc.Response.Close,
		Uncompressed:     rc.Response.Uncompressed,
		Trailer:          rc.Response.Trailer,
		Request:          rc.Response.Request,
		TLS:              rc.Response.TLS,
	}
}

func (c *Cache) RoundTrip(request *http.Request) (*http.Response, error) {
	cacheRequests.WithLabelValues().Inc()

	cacheKey := request.Method + ":" + request.URL.String()

	d, ok := c.cache.Get(cacheKey)
	if ok {
		cacheHits.WithLabelValues().Inc()
		return d.(*cachedResponse).ToResponse(), nil
	}

	response, err := c.transport.RoundTrip(request)
	if err != nil || response.StatusCode != http.StatusOK {
		log.Info().Err(err).Int("status", response.StatusCode).Msg("not caching")
		return response, err
	}

	contentType := response.Header.Get("Content-Type")

	allowed := false
	for _, ct := range []string{"application/javascript", "text/css", "image/png", "application/font-woff", "image/x-icon"} {
		if ct == contentType {
			allowed = true
		}
	}

	if !allowed {
		log.Info().Str("content-type", contentType).Msg("not caching")
		return c.transport.RoundTrip(request)
	}

	content, err := io.ReadAll(response.Body)
	if err != nil {
		log.Info().Err(err).Msg("failed to read response body")
		return response, err
	}

	response.Body = io.NopCloser(bytes.NewBuffer(content))

	finalResp := &cachedResponse{
		Response: response,
		content:  content,
	}

	ok = c.cache.Set(cacheKey, finalResp, int64(len(content)))
	if !ok {
		log.Info().Msg("cache set failed")
	}

	return response, nil
}
