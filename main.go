package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

var (
	upstream = flag.String("upstream", "http://localhost:8081", "remote server")
	email    = flag.String("email", "", "email address")
	domains  = flag.String("domains", "", "comma separated list of domains")
	insecure = flag.Bool("insecure", false, "accept invalid SSL certificates from the upstream server")
)

func main() {
	flag.Parse()

	if *email == "" {
		log.Fatal().Msg("email is required")
	}

	if *domains == "" {
		log.Fatal().Msg("domains are required")
	}

	remote, err := url.Parse(*upstream)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse remote URL")
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	transport := NewPotentiallyInsecureTransport(*insecure)

	cache, err := NewCache(transport)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create cache")
	}

	proxy.Transport = NewInstrumentedRoundTripper(cache) // magic sauce that enables the telemetry

	go func() {
		err = http.ListenAndServe(":9001", promhttp.Handler())
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start prometheus server")
		}
	}()

	provider := NewHttpProvider()
	go func() {
		err = http.ListenAndServe(":80", provider)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start HTTP server")
		}
	}()

	// Initialize lego for the certificate
	config, err := NewCertConfig("/var/lib/cert", *email, strings.Split(*domains, ","), provider)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize cert config")
	}

	// Start the HTTPS server
	srv := &http.Server{
		Addr:      ":443",
		Handler:   proxy,
		TLSConfig: config,
	}

	err = srv.ListenAndServeTLS("", "")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start HTTPS server")
	}
}
