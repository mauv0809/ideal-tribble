#!/bin/bash
# deploy-server-setup.sh - Server-side deployment setup
# This script runs ON THE SERVER and handles all deployment logic

set -e

APP_USER="tribble"
APP_DIR="/opt/ideal-tribble"

echo "🚀 Starting server-side deployment setup..."

cd $APP_DIR

# Set ownership and permissions
echo "Setting file ownership and permissions..."
chown -R $APP_USER:$APP_USER $APP_DIR
chmod +x ideal-tribble scripts/*.sh
echo "✅ Permissions set"

# Environment file should be provided by deployment process
if [ -f "$APP_DIR/.env" ]; then
    echo "✅ .env file found"
else
    echo "❌ .env file missing - should be provided by deployment"
    exit 1
fi

if [ ! -f "/etc/systemd/system/ideal-tribble.service" ]; then
    echo "⚙️ Installing systemd service..."
    ./scripts/install-systemd-service.sh
    echo "✅ Systemd service installed"
else
    echo "✅ Systemd service already installed"
fi

# Setup observability stack
if [ ! -f "/etc/systemd/system/observability.service" ]; then
    echo "📊 Setting up observability stack (this may take a few minutes)..."
    ./scripts/setup-observability.sh
    echo "✅ Observability stack configured"
else
    echo "✅ Observability stack already configured"
fi

# Setup cron jobs (only if not already configured)
if ! crontab -l 2>/dev/null | grep -q "ideal-tribble"; then
    echo "⏰ Setting up cron jobs..."
    ./scripts/setup-cron.sh
else
    echo "✅ Cron jobs already configured"
fi

# Setup SSL certificate auto-renewal (only if not configured)
if ! systemctl list-timers | grep -q "certbot.timer"; then
    echo "🔒 Setting up SSL auto-renewal..."
    ./scripts/setup-ssl-renewal.sh
else
    echo "✅ SSL auto-renewal already configured"
fi

# Setup nginx configuration and SSL
if [ ! -f "/etc/nginx/sites-enabled/ideal-tribble" ]; then
    echo "🌐 Setting up nginx configuration..."
    ./scripts/setup-nginx.sh
    echo "✅ Nginx configuration completed"
else
    echo "✅ Nginx configuration already enabled, checking for updates..."
    ./scripts/setup-nginx.sh
fi

# Note: Database migrations run automatically when the application starts

# Validate environment configuration before starting service
echo "Validating environment configuration..."
if [ -f "$APP_DIR/.env" ]; then
    ./scripts/validate-env.sh "$APP_DIR/.env"
    VALIDATION_EXIT_CODE=$?
    if [ $VALIDATION_EXIT_CODE -eq 1 ]; then
        echo "❌ Environment validation failed - missing required variables"
        echo "Please edit $APP_DIR/.env and run this script again"
        exit 1
    elif [ $VALIDATION_EXIT_CODE -eq 2 ]; then
        echo "⚠️ Environment validation warning - some variables have placeholder values"
        echo "Service will start but may not function correctly until these are updated"
    else
        echo "✅ Environment validation passed"
    fi
else
    echo "⚠️ No .env file found - service may fail to start"
fi

# Restart the service
echo "🔄 Restarting application service..."
systemctl daemon-reload
systemctl restart ideal-tribble
echo "✅ Service restart initiated"

# Verify service is running
echo "⏳ Waiting for service to start (5 seconds)..."
sleep 5
echo "🔍 Checking service status..."
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

# Check if SSL certificates are configured
if ! certbot certificates 2>/dev/null | grep -q "Certificate Name: wally-api.utiger.dk"; then
    NEXT_STEPS+=("Setup SSL for API: sudo certbot --nginx -d wally-api.utiger.dk")
fi
if ! certbot certificates 2>/dev/null | grep -q "Certificate Name: wally.utiger.dk"; then
    NEXT_STEPS+=("Setup SSL for Web: sudo certbot --nginx -d wally.utiger.dk")
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