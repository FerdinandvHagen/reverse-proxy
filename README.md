# Reverse Proxy with Prometheus Export

This project implements a simple Reverse Proxy that exports HTTP metrics to Prometheus
leveraging Grafana Agent to push metrics to Grafana Cloud.

## How to run
The project sports a simple docker-compose setup which can simply be run directly.
Some configuration might be necessary in the `agent.yaml` file to configure the correct
endpoint to send metrics to.

## About the proxy
The proxy is based on the Reverse Proxy implementation that is built into the core Golang library.
Because we are using the core library without any major modification this should be a fairly stable
and performant setup.

Because Go is by default HTTP/2 enabled, the proxy will also support HTTP/2 if configured with
SSL certificates. HTTP/2 proxying should also work just fine (trailers, etc. seem to be supported just fine).

## Certificates
The proxy will obtain SSL certificates from Let's Encrypt using the ACME protocol. You will only need to configure
the domains you want to allow in the docker-compose.yaml file.


## Configuration
The proxy requires minimal configuration. As a prerequisite, you will need to docker and docker-compose installed.

To configure the proxy, copy the template files and adjust them to your needs.
- `docker-compose.tmpl.yaml` -> `docker-compose.yaml`
  - Make sure to adjust the configurations under `command`
    - `--domains` should be a comma-separated list of domains you want to allow
    - `--email` should be the email address you want to use for Let's Encrypt
    - `--upstream` should be the (full) URL of the upstream server you want to proxy to
- `agent.tmpl.yaml` -> `agent.yaml`
  - Make sure to adjust the configurations under `remote_write` to reflect the correct endpoint for Grafana Cloud
