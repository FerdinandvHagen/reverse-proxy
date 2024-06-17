package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	repl = regexp.MustCompile(`\d+`)
	uid  = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
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
	Transport  http.RoundTripper
	prefix     string
	hasVersion bool
	excluded   []string
}

func NewInstrumentedRoundTripper(transport http.RoundTripper, prefix string, hasVersion bool, excluded []string) *InstrumentedRoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &InstrumentedRoundTripper{
		Transport:  transport,
		prefix:     prefix,
		hasVersion: hasVersion,
		excluded:   excluded,
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

	s, ok := cleanupPath(req.URL, p.prefix, p.hasVersion, p.excluded)
	if !ok {
		return res, err
	}

	status := strconv.Itoa(res.StatusCode)
	status = status[:1] + "xx"

	method := req.Method

	totalRequests.WithLabelValues(method, s, status).Inc()
	httpDuration.WithLabelValues(method, s).Observe(duration.Seconds())

	return res, err
}

func cleanupPath(url *url.URL, prefix string, hasVersion bool, excluded []string) (string, bool) {
	prefix = strings.TrimSuffix(prefix, "/")
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	if url == nil {
		return "", false
	}

	// only allow paths that start with the prefix
	uri := url.Path
	if !strings.HasPrefix(uri, prefix) {
		return "", false
	}

	// exclude paths that contain any of the excluded strings
	for _, e := range excluded {
		if ok, _ := path.Match(e, uri); ok {
			return "", false
		}
	}

	// extract version
	version := ""
	remainder := strings.TrimPrefix(strings.TrimPrefix(uri, prefix), "/")
	if hasVersion {
		version = strings.Split(remainder, "/")[0]
		remainder = strings.TrimPrefix(remainder, version)
	}

	remainder = strings.TrimPrefix(remainder, "/")
	uuidReplaced := uid.ReplaceAllString(remainder, "_")
	numbersReplaced := repl.ReplaceAllString(uuidReplaced, "_")

	res := strings.Join([]string{prefix, numbersReplaced}, "/")
	if hasVersion {
		res = strings.Join([]string{prefix, version, numbersReplaced}, "/")
	}

	return strings.TrimSuffix(res, "/"), true
}
