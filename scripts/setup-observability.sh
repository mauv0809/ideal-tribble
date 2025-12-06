#!/bin/bash
# setup-observability.sh - Setup observability stack on Hetzner VPS

set -e

APP_DIR="/opt/ideal-tribble"
OBSERVABILITY_DIR="$APP_DIR/observability"
SERVICE_FILE="/etc/systemd/system/observability.service"

echo "🔧 Setting up observability stack..."

# Ensure Docker and Docker Compose are installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

if ! docker compose version &> /dev/null; then
    echo "❌ Docker Compose is not available. Please ensure Docker with Compose plugin is installed."
    exit 1
fi

# Create observability data directories
echo "Creating data directories..."
mkdir -p /var/lib/observability/{grafana,tempo,loki}

# Set proper ownership for containers
# Grafana runs as user 472
chown -R 472:472 /var/lib/observability/grafana
# Tempo and Loki run as user 10001
chown -R 10001:10001 /var/lib/observability/tempo
chown -R 10001:10001 /var/lib/observability/loki

# Install systemd service
echo "Installing systemd service..."
cp "$OBSERVABILITY_DIR/observability.service" "$SERVICE_FILE"
systemctl daemon-reload
systemctl enable observability

# Test Docker Compose configuration
echo "Testing Docker Compose configuration..."
cd "$OBSERVABILITY_DIR"
if ! docker compose -f docker-compose.yml config > /dev/null; then
    echo "❌ Docker Compose configuration is invalid"
    exit 1
fi

# Start the observability stack
echo "Starting observability stack..."
systemctl start observability

# Health check function with retries
check_service_health() {
    local name="$1"
    local check_cmd="$2"
    local max_attempts=6
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if eval "$check_cmd"; then
            echo "✅ $name is healthy"
            return 0
        fi
        echo "  Attempt $attempt/$max_attempts: $name not ready, waiting 10s..."
        sleep 10
        ((attempt++))
    done

    echo "❌ $name health check failed after $max_attempts attempts"
    return 1
}

# Show container status for debugging
echo "Current container status:"
docker compose ps

# Health checks with retries (total max wait ~60s per service)
echo "Performing health checks..."
HEALTH_CHECKS=0

# Check OTel Collector (check if container is running and port is open)
echo "Checking OpenTelemetry Collector..."
if check_service_health "OpenTelemetry Collector" "docker compose ps otel-collector | grep -q 'Up' && nc -z localhost 4317 2>/dev/null"; then
    ((HEALTH_CHECKS++))
else
    docker compose logs --tail=10 otel-collector
fi

# Check Tempo
echo "Checking Tempo..."
if check_service_health "Tempo" "curl -sf http://localhost:3200/ready > /dev/null 2>&1"; then
    ((HEALTH_CHECKS++))
else
    docker compose logs --tail=10 tempo
fi

# Check Loki
echo "Checking Loki..."
if check_service_health "Loki" "curl -sf http://localhost:3100/ready > /dev/null 2>&1"; then
    ((HEALTH_CHECKS++))
else
    docker compose logs --tail=10 loki
fi

# Check Grafana
echo "Checking Grafana..."
if check_service_health "Grafana" "curl -sf http://localhost:3000/api/health > /dev/null 2>&1"; then
    ((HEALTH_CHECKS++))
else
    docker compose logs --tail=10 grafana
fi

# Check Promtail
echo "Checking Promtail..."
if check_service_health "Promtail" "curl -sf http://localhost:9080/ready > /dev/null 2>&1"; then
    ((HEALTH_CHECKS++))
else
    docker compose logs --tail=10 promtail
fi

echo ""
echo "Health check results: $HEALTH_CHECKS/5 services healthy"

if [ $HEALTH_CHECKS -eq 5 ]; then
    echo "🎉 Observability stack setup completed successfully!"
    echo ""
    echo "📊 Access points:"
    echo "  - Grafana: http://localhost:3000/grafana (admin/\$GRAFANA_ADMIN_PASSWORD)"
    echo "  - Tempo: http://localhost:3200"
    echo "  - Loki: http://localhost:3100"
    echo "  - OTel Collector: grpc://localhost:4317, http://localhost:4318"
    echo "  - Promtail: http://localhost:9080"
    echo ""
    echo "🔧 Management commands:"
    echo "  systemctl status observability    # Check status"
    echo "  systemctl start observability     # Start services"
    echo "  systemctl stop observability      # Stop services"
    echo "  systemctl restart observability   # Restart services"
    exit 0
else
    echo "❌ Observability stack deployment failed ($HEALTH_CHECKS/5 services healthy)"
    echo ""
    echo "🔧 System information for debugging:"
    echo "Docker status: $(systemctl is-active docker)"
    echo "Available disk space:"
    df -h /var/lib/observability 2>/dev/null || df -h /
    echo "Available memory:"
    free -h
    echo ""
    echo "🔍 Debug commands to run manually:"
    echo "  cd $OBSERVABILITY_DIR && docker compose logs"
    echo "  systemctl status observability"
    echo "  docker compose ps"
    echo ""
    echo "All 5 services must be healthy for deployment to proceed."
    exit 1
fi