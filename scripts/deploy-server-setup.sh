#!/bin/bash
# deploy-server-setup.sh - Server-side deployment setup
# This script runs ON THE SERVER and handles all deployment logic

set -e

APP_USER="tribble"
APP_DIR="/opt/ideal-tribble"

echo "🚀 Starting server-side deployment setup..."

cd $APP_DIR

# Set ownership and permissions
chown -R $APP_USER:$APP_USER $APP_DIR
chmod +x ideal-tribble scripts/*.sh

# Run setup scripts if not already done
if [ ! -f "$APP_DIR/.env" ]; then
    echo "Running initial setup..."
    ./scripts/setup-secrets.sh
    echo "Please edit $APP_DIR/.env with your actual credentials"
fi

if [ ! -f "/etc/systemd/system/ideal-tribble.service" ]; then
    ./scripts/install-systemd-service.sh
fi

# Setup cron jobs
./scripts/setup-cron.sh

# Setup SSL certificate auto-renewal
./scripts/setup-ssl-renewal.sh

# Update nginx security configuration
./scripts/update-nginx-security.sh

# Run database migrations if needed
sudo -u tribble ./ideal-tribble -migrate 2>/dev/null || true

# Restart the service
systemctl daemon-reload
systemctl restart ideal-tribble

# Verify service is running
sleep 5
if systemctl is-active --quiet ideal-tribble; then
  echo "✅ Service restarted successfully"
else
  echo "❌ Service failed to start"
  journalctl -u ideal-tribble --no-pager -n 20
  exit 1
fi

echo "🎉 Server deployment complete!"

# Check what still needs to be done and only show relevant next steps
NEXT_STEPS=()

# Check if .env needs configuration
if [ -f "$APP_DIR/.env" ]; then
    if grep -q "your-.*-here\|CHANGE_ME\|example\|placeholder" "$APP_DIR/.env" 2>/dev/null; then
        NEXT_STEPS+=("Edit $APP_DIR/.env with your actual credentials")
    fi
fi

# Check if SSL certificate is configured
if ! certbot certificates 2>/dev/null | grep -q "Certificate Name: wally-api.utiger.dk"; then
    NEXT_STEPS+=("Setup SSL: sudo certbot --nginx -d wally-api.utiger.dk")
fi

# Show next steps only if there are any
if [ ${#NEXT_STEPS[@]} -gt 0 ]; then
    echo ""
    echo "📋 Next steps needed:"
    for i in "${!NEXT_STEPS[@]}"; do
        echo "  $((i+1)). ${NEXT_STEPS[$i]}"
    done
fi

echo ""
echo "📊 Monitor application:"
echo "  sudo journalctl -u ideal-tribble -f     # View logs"
echo "  systemctl status ideal-tribble          # Check service status"
echo "  curl http://localhost:8080/health       # Test health endpoint"