package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"strconv"
	"time"
)

var totalRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Number of requests by path and status code.",
	},
	[]string{"method", "path", "status"},
)

var httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "http_response_time_seconds",
	Help: "Duration of HTTP requests.",
}, []string{"method", "path"})

func init() {
	prometheus.MustRegister(totalRequests)
	prometheus.MustRegister(httpDuration)
}

type InstrumentedRoundTripper struct {
	Transport http.RoundTripper
}

func NewInstrumentedRoundTripper(transport http.RoundTripper) *InstrumentedRoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &InstrumentedRoundTripper{
		Transport: transport,
	}
}

func (p *InstrumentedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := p.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	start := time.Now()
	res, err := transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil || res == nil {
		return res, err
	}

	// Overly secure path extraction
	path := "/"
	if req.URL != nil {
		path = req.URL.Path
	}

	status := strconv.Itoa(res.StatusCode)
	status = status[:1] + "xx"

	method := req.Method

	totalRequests.WithLabelValues(method, path, status).Inc()
	httpDuration.WithLabelValues(method, path).Observe(duration.Seconds())

	return res, err
}
