#!/bin/bash
# setup-secrets.sh - Configure environment variables for Hetzner VPS deployment

set -e

APP_USER="tribble"
APP_DIR="/opt/ideal-tribble"
ENV_FILE="$APP_DIR/.env"

echo "Setting up secrets management for ideal-tribble..."

# Create app user if it doesn't exist
if ! id "$APP_USER" &>/dev/null; then
    echo "Creating $APP_USER user..."
    sudo useradd --system --create-home --shell /bin/bash "$APP_USER"
fi

# Create app directory
sudo mkdir -p "$APP_DIR"
sudo chown "$APP_USER:$APP_USER" "$APP_DIR"

# Create .env file with secure permissions
echo "Creating environment file at $ENV_FILE"
sudo tee "$ENV_FILE" > /dev/null << 'EOF'
# Database Configuration
DB_NAME=local_tribble.db
TURSO_PRIMARY_URL=your_turso_url_here
TURSO_AUTH_TOKEN=your_turso_token_here

# Slack Configuration
SLACK_BOT_TOKEN=xoxb-your-slack-bot-token-here
SLACK_CHANNEL_ID=your-slack-channel-id-here
SLACK_SIGNING_SECRET=your-slack-signing-secret-here

# Playtomic Configuration
TENANT_ID=your-tenant-id-here

# Server Configuration
PORT=8080

# Ngrok (optional for development)
NGROK_AUTH_TOKEN=

# GCP Configuration (will be removed)
PROJECT_ID=
EOF

# Set secure permissions (only app user can read)
sudo chown "$APP_USER:$APP_USER" "$ENV_FILE"
sudo chmod 600 "$ENV_FILE"

echo "Environment file created at $ENV_FILE"
echo "Please edit this file with your actual credentials:"
echo "  sudo nano $ENV_FILE"
echo ""
echo "Security notes:"
echo "  - File is owned by $APP_USER user"
echo "  - Permissions are 600 (only owner can read/write)"
echo "  - Add to .gitignore to prevent accidental commits"