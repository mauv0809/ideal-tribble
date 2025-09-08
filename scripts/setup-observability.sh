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

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed. Please install Docker Compose first."
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
if ! docker-compose config > /dev/null; then
    echo "❌ Docker Compose configuration is invalid"
    exit 1
fi

# Start the observability stack
echo "Starting observability stack..."
systemctl start observability

# Wait for services to be ready
echo "Waiting for services to be ready..."
sleep 15

# Health checks
echo "Performing health checks..."
HEALTH_CHECKS=0

# Check OTel Collector
if curl -s http://localhost:4318/v1/health > /dev/null; then
    echo "✅ OpenTelemetry Collector is healthy"
    ((HEALTH_CHECKS++))
else
    echo "❌ OpenTelemetry Collector health check failed"
fi

# Check Tempo
if curl -s http://localhost:3200/ready > /dev/null; then
    echo "✅ Tempo is healthy"
    ((HEALTH_CHECKS++))
else
    echo "❌ Tempo health check failed"
fi

# Check Loki
if curl -s http://localhost:3100/ready > /dev/null; then
    echo "✅ Loki is healthy"
    ((HEALTH_CHECKS++))
else
    echo "❌ Loki health check failed"
fi

# Check Grafana
if curl -s http://localhost:3000/api/health > /dev/null; then
    echo "✅ Grafana is healthy"
    ((HEALTH_CHECKS++))
else
    echo "❌ Grafana health check failed"
fi

# Check Promtail
if curl -s http://localhost:9080/ready > /dev/null; then
    echo "✅ Promtail is healthy"
    ((HEALTH_CHECKS++))
else
    echo "❌ Promtail health check failed"
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
else
    echo "⚠️ Some services failed health checks. Check logs:"
    echo "  docker-compose -f $OBSERVABILITY_DIR/docker-compose.yml logs"
fi