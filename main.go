package main

import (
	"flag"
	"github.com/CAFxX/httpcompression"
	"github.com/ferdinandvhagen/reverse-proxy/commands"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

var (
	upstream   = flag.String("upstream", "http://localhost:8081", "remote server")
	email      = flag.String("email", "", "email address")
	domains    = flag.String("domains", "", "comma separated list of domains")
	insecure   = flag.Bool("insecure", false, "accept invalid SSL certificates from the upstream server")
	prefix     = flag.String("prefix", "", "prefix that needs to be present in the path")
	hasVersion = flag.Bool("hasVersion", false, "whether the path has a version following the prefix")
	excluded   = flag.String("excluded", "", "comma separated list of paths to exclude from metrics")
	version    = flag.Bool("version", false, "print version and exit")
)

func main() {
	flag.Parse()

	if *version {
		commands.PrintVersion()
		return
	}

	if *email == "" {
		log.Fatal().Msg("email is required")
	}

	if *domains == "" {
		log.Fatal().Msg("domains are required")
	}

	excludedPaths := strings.Split(*excluded, ",")

	remote, err := url.Parse(*upstream)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse remote URL")
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	transport := NewPotentiallyInsecureTransport(*insecure)
	proxy.Transport = NewInstrumentedRoundTripper(transport, *prefix, *hasVersion, excludedPaths) // magic sauce that enables the telemetry

	compress, err := httpcompression.DefaultAdapter()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create compression adapter")
	}

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
		Handler:   compress(proxy),
		TLSConfig: config,
	}

	err = srv.ListenAndServeTLS("", "")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start HTTPS server")
	}
}
