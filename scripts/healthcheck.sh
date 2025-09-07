#!/bin/bash
# healthcheck.sh - Simple health monitoring for cron jobs

set -e

APP_URL="http://localhost:8080"
LOG_FILE="/var/log/ideal-tribble-health.log"

timestamp() {
  date '+%Y-%m-%d %H:%M:%S'
}

log() {
  echo "$(timestamp) - $1" >> "$LOG_FILE"
}

# Check if service is running
if ! curl -sf "$APP_URL/health" > /dev/null; then
  log "ERROR: Service health check failed at $APP_URL/health"
  exit 1
fi

# Check if service is responding to metrics
if ! curl -sf "$APP_URL/metrics" > /dev/null; then
  log "WARNING: Metrics endpoint not responding at $APP_URL/metrics"
fi

log "INFO: Service is healthy"