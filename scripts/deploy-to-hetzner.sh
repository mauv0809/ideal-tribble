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

# Copy files to server
echo "Copying files to server..."
scp "$BINARY_NAME" "root@$SERVER_IP:$APP_DIR/"
scp -r scripts/ "root@$SERVER_IP:$APP_DIR/"
scp -r migrations/ "root@$SERVER_IP:$APP_DIR/"

# Set permissions and run setup scripts
echo "Running setup on server..."
ssh "root@$SERVER_IP" << EOF
    set -e
    cd $APP_DIR
    
    # Set ownership and permissions
    chown -R $APP_USER:$APP_USER $APP_DIR
    chmod +x $BINARY_NAME
    chmod +x scripts/*.sh
    
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
    
    # Restart the service
    systemctl daemon-reload
    systemctl restart ideal-tribble
    systemctl status ideal-tribble --no-pager
    
    echo "Deployment complete!"
    echo ""
    echo "Next steps:"
    echo "1. Edit $APP_DIR/.env with your credentials"
    echo "2. sudo systemctl restart ideal-tribble"
    echo "3. Setup SSL: sudo certbot --nginx -d wally-api.utiger.dk"
    echo ""
    echo "Monitor logs with:"
    echo "  sudo journalctl -u ideal-tribble -f"
EOF

echo "Deployment script completed!"
echo "Don't forget to:"
echo "1. Configure DNS: wally-api.utiger.dk -> $SERVER_IP"  
echo "2. SSH to server and edit .env file"
echo "3. Setup SSL certificate"