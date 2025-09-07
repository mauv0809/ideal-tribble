#!/bin/bash
# install-systemd-service.sh - Install systemd service for ideal-tribble

set -e

APP_USER="tribble"
APP_DIR="/opt/ideal-tribble"
SERVICE_NAME="ideal-tribble"

echo "Installing systemd service for $SERVICE_NAME..."

# Create systemd service file
sudo tee "/etc/systemd/system/$SERVICE_NAME.service" > /dev/null << EOF
[Unit]
Description=Ideal Tribble - Padel Club Management Bot
After=network.target
Wants=network.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_USER
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/ideal-tribble
EnvironmentFile=$APP_DIR/.env
Restart=always
RestartSec=5

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$APP_DIR
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

[Install]
WantedBy=multi-user.target
EOF

echo "Systemd service installed at /etc/systemd/system/$SERVICE_NAME.service"

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"

echo "Service enabled. Use these commands to manage it:"
echo "  sudo systemctl start $SERVICE_NAME"
echo "  sudo systemctl stop $SERVICE_NAME" 
echo "  sudo systemctl restart $SERVICE_NAME"
echo "  sudo systemctl status $SERVICE_NAME"
echo "  sudo journalctl -u $SERVICE_NAME -f"