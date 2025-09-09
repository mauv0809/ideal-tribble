#!/bin/bash
# deploy-to-hetzner.sh - Deploy ideal-tribble to Hetzner VPS

set -e

SERVER_IP="${1:-}"
APP_USER="tribble"
APP_DIR="/opt/ideal-tribble"
BINARY_NAME="ideal-tribble"

if [ -z "$SERVER_IP" ]; then
    echo "Usage: $0 <server-ip>"
    echo "Example: $0 1.2.3.4"
    exit 1
fi

echo "Deploying ideal-tribble to Hetzner server: $SERVER_IP"

# Build the application
echo "Building application..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$BINARY_NAME" .

# Copy files to server (parallel upload for speed)
echo "Copying files to server..."
scp -o ControlMaster=auto -o ControlPath=/tmp/ssh-%r@%h:%p -o ControlPersist=10m \
    "$BINARY_NAME" "root@$SERVER_IP:$APP_DIR/$BINARY_NAME.new" &
scp -o ControlMaster=auto -o ControlPath=/tmp/ssh-%r@%h:%p -o ControlPersist=10m \
    -r scripts/ migrations/ observability/ "root@$SERVER_IP:$APP_DIR/" &
scp -o ControlMaster=auto -o ControlPath=/tmp/ssh-%r@%h:%p -o ControlPersist=10m \
    nginx/ideal-tribble.conf "root@$SERVER_IP:/etc/nginx/sites-available/ideal-tribble" &
wait  # Wait for all uploads to complete
echo "✅ All files copied"

# Atomically replace the binary and fix permissions
echo "Performing atomic binary replacement..."
ssh -o ControlMaster=auto -o ControlPath=/tmp/ssh-%r@%h:%p -o ControlPersist=10m \
    -T "root@$SERVER_IP" << EOF
    set -e
    cd $APP_DIR
    # Ensure directory exists
    mkdir -p $APP_DIR
    # Atomically replace the binary and set permissions
    mv "$BINARY_NAME.new" "$BINARY_NAME"
    chmod +x "$BINARY_NAME" scripts/*.sh
EOF

# Stop existing service to prevent port conflicts
echo "Stopping existing service..."
ssh -o ControlMaster=auto -o ControlPath=/tmp/ssh-%r@%h:%p -o ControlPersist=10m \
    -T "root@$SERVER_IP" << EOF
    systemctl stop ideal-tribble || true
EOF

# Update nginx configuration
echo "Updating nginx configuration..."
ssh -o ControlMaster=auto -o ControlPath=/tmp/ssh-%r@%h:%p -o ControlPersist=10m \
    -T "root@$SERVER_IP" << EOF
    set -e
    # Enable the site
    ln -sf /etc/nginx/sites-available/ideal-tribble /etc/nginx/sites-enabled/
    rm -f /etc/nginx/sites-enabled/default
    # Test and reload nginx
    nginx -t && systemctl reload nginx
    # Auto-configure SSL (non-interactive)
    if ! certbot --nginx -d wally-api.utiger.dk --non-interactive --agree-tos --email admin@utiger.dk; then
        echo "⚠️ SSL configuration failed - check certbot logs"
        echo "Manual command: certbot --nginx -d wally-api.utiger.dk"
    else
        echo "✅ SSL successfully configured"
    fi
EOF

# Run deployment setup on server
echo "Running deployment setup on server..."
ssh -o ControlMaster=auto -o ControlPath=/tmp/ssh-%r@%h:%p -o ControlPersist=10m \
    -T "root@$SERVER_IP" << EOF
    set -e
    cd $APP_DIR
    ./scripts/deploy-server-setup.sh
EOF

echo "🎉 Manual deployment completed!"

# Check if DNS is likely configured (basic check)
if ! nslookup wally-api.utiger.dk 2>/dev/null | grep -q "$SERVER_IP" 2>/dev/null; then
    echo ""
    echo "🌐 Don't forget to configure DNS:"
    echo "   Point wally-api.utiger.dk A record -> $SERVER_IP"
fi

echo ""
echo "💡 To check deployment status:"
echo "   ssh root@$SERVER_IP 'systemctl status ideal-tribble'"
echo "   curl http://$SERVER_IP:8080/health"