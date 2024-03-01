package main

import (
	"crypto/tls"
	"github.com/prometheus/client_golang/prometheus"
	"net"
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

func NewInstrumentedRoundTripper(insecure bool) *InstrumentedRoundTripper {
	// Same Config as DefaultTransport but allows us to set the TLS config
	transport := &http.Transport{
		DialContext:           net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if insecure {
		// Allows insecure connection to private IP addresses if enabled
		transport.TLSClientConfig = &tls.Config{
			GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
				addr, ok := info.Conn.RemoteAddr().(*net.TCPAddr)

				if ok && addr.IP.IsPrivate() {
					return &tls.Config{
						InsecureSkipVerify: true,
					}, nil
				}

				return &tls.Config{}, nil
			},
		}
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
