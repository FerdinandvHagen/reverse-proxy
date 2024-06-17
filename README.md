# Reverse Proxy with Prometheus Export

This project implements a simple Reverse Proxy that exports HTTP metrics to Prometheus
leveraging Grafana Agent to push metrics to Grafana Cloud.

## About the proxy

The proxy is based on the Reverse Proxy implementation that is built into the core Golang library.
Because we are using the core library without any major modification this should be a fairly stable
and performant setup.

Because Go is by default HTTP/2 enabled, the proxy will also support HTTP/2 if configured with
SSL certificates. HTTP/2 proxying should also work just fine (trailers, etc. seem to be supported).

## Certificates

The proxy will obtain SSL certificates from Let's Encrypt using the ACME protocol. You will only need to configure
the domains you want to allow in the docker-compose.yaml file.

## Exported Metrics

The proxy will export the following metrics:

- default metrics from the Go runtime
- `http_requests_total (method, path, status)` - a counter for the number of HTTP requests
    - `method` is the HTTP method (GET, POST, etc.)
    - `path` is the URL path
    - `status` is the HTTP status code (i.e. 2xx, 3xx, 4xx, 5xx)
- `http_response_time_seconds (method, path)` - a histogram of the request duration
    - `method` is the HTTP method (GET, POST, etc.)
    - `path` is the URL path

## Configuration

The proxy requires minimal configuration. As a prerequisite, you will need to docker and docker-compose installed.

To configure the proxy, copy the template files and adjust them to your needs.

- `docker-compose.tmpl.yaml` -> `docker-compose.yaml`
    - Make sure to adjust the configurations under `command`
        - `--domains` should be a comma-separated list of domains you want to allow
        - `--email` should be the email address you want to use for Let's Encrypt
        - `--upstream` should be the (full) URL of the upstream server you want to proxy to
        - `--insecure` should be set to `true` if you want to accept invalid SSL certificates from the upstream server
        - `--prefix` limits metric tracking to pathes with given prefix only
        - `--hasVersion` should be set to `true` if the path has a version following the prefix
        - `--excluded` should be a comma-separated list of paths to exclude from metrics. Supports wildcards through
          the https://pkg.go.dev/path#Match package
- `agent.tmpl.yaml` -> `agent.yaml`
    - Make sure to adjust the configurations under `remote_write` to reflect the correct endpoint for Grafana Cloud
