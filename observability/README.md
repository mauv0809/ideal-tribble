# Observability Stack

This directory contains the configuration for the self-hosted observability stack using the Grafana ecosystem.

## Components

- **OpenTelemetry Collector**: Receives and forwards telemetry data
- **Grafana Tempo**: Distributed tracing backend
- **Grafana Loki**: Log aggregation system
- **Grafana Promtail**: Log collection agent
- **Grafana**: Visualization and dashboards

## Architecture

```
ideal-tribble app → OTel Collector → Tempo (traces)
                 → systemd journal → Promtail → Loki (logs)
                 
Grafana ← Tempo + Loki (correlated traces & logs)
```

## Resource Usage

- Total memory: ~1.3GB (of 4GB available on VPS)
- Disk space: <1GB for 7-day retention
- Network: Internal Docker network + nginx proxy

## Management

The observability stack is managed via systemd:

```bash
# Status
systemctl status observability

# Start/stop/restart
systemctl start observability
systemctl stop observability  
systemctl restart observability

# Logs
journalctl -u observability -f
```

## Access

- **Grafana Web UI**: https://wally-api.utiger.dk/grafana
  - Username: `admin`
  - Password: Set via `GRAFANA_ADMIN_PASSWORD` env var

## Data Sources

Grafana is automatically provisioned with:
- **Tempo**: Trace data source with logs correlation
- **Loki**: Log data source with trace linking

## Retention

Both Tempo and Loki are configured with 7-day retention to manage disk usage.

## Configuration Files

- `docker-compose.yml`: Service orchestration
- `otel-collector-config.yaml`: OTel Collector pipeline
- `tempo.yaml`: Tempo configuration
- `loki.yaml`: Loki configuration  
- `grafana-datasources.yaml`: Grafana data source provisioning
- `grafana.ini`: Grafana settings
- `observability.service`: Systemd unit file

## Troubleshooting

### Check service health
```bash
curl http://localhost:4318/v1/health  # OTel Collector
curl http://localhost:3200/ready      # Tempo  
curl http://localhost:3100/ready      # Loki
curl http://localhost:3000/api/health # Grafana
curl http://localhost:9080/ready      # Promtail
```

### View container logs
```bash
cd /opt/ideal-tribble/observability
docker-compose logs -f [service-name]
```

### Storage locations
- Grafana: `/var/lib/observability/grafana`
- Tempo: `/var/lib/observability/tempo`
- Loki: `/var/lib/observability/loki`