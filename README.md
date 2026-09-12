# SignalDock

SignalDock is a self-hosted endpoint monitoring application built with Go.

It integrates:

- PostgreSQL
- Prometheus
- Blackbox Exporter
- Grafana
- Alertmanager

Grafana dashboards are embedded directly in the SignalDock web interface.

## Requirements

- Docker
- Docker Compose

## Local deployment

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` and replace the default passwords with secure values.

Start the full stack:

```bash
docker compose up -d --build
```

SignalDock will be available at:

```text
http://localhost:8080
```

## Health check

Check that SignalDock is running:

```bash
curl -f -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/healthz
```

A healthy instance returns:

```text
HTTP 200
```

## Stop the stack

```bash
docker compose down
```

Persistent data is stored in Docker volumes for PostgreSQL, Prometheus, and Grafana.

To remove the stack and its local data:

```bash
docker compose down -v
```

## Services

Only SignalDock is exposed to the host on port `8080`.

The remaining services communicate through the internal Docker Compose network:

- PostgreSQL
- Prometheus
- Blackbox Exporter
- Grafana
- Alertmanager

Grafana is accessed through the authenticated SignalDock reverse proxy under:

```text
/grafana/
```

## Development

Run the application locally:

```bash
make run
```

Run the test suite and checks:

```bash
make check
```

Build the Docker image:

```bash
make docker-build
```

Start or stop the development stack:

```bash
make up
make down
```