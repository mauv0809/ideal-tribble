#!/bin/bash

# Script to update nginx configuration with endpoint access restrictions
# Run this on the existing Hetzner server to secure endpoints

set -e

echo "🔒 Checking nginx configuration..."

# Create expected configuration in memory first
EXPECTED_CONFIG='server {
    listen 80;
    server_name wally-api.utiger.dk;
    
    # Public endpoints (external access allowed)
    location /health {
        proxy_pass http://localhost:8080/health;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        access_log off;
    }
    
    location /slack/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    location /grafana/ {
        proxy_pass http://localhost:3000/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # Internal endpoints (localhost and Docker networks only)
    location ~ ^/(fetch|process|clear|members|matches|leaderboard|test|metrics) {
        allow 127.0.0.1;
        allow 172.16.0.0/12;  # Docker networks
        deny all;
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # Deny all other requests
    location / {
        return 404;
    }
}'

# Compare with existing configuration
if [ -f /etc/nginx/sites-available/ideal-tribble ]; then
    CURRENT_HASH=$(cat /etc/nginx/sites-available/ideal-tribble | md5sum | cut -d' ' -f1)
    EXPECTED_HASH=$(echo "$EXPECTED_CONFIG" | md5sum | cut -d' ' -f1)
    
    if [ "$CURRENT_HASH" = "$EXPECTED_HASH" ]; then
        echo "✅ Nginx configuration already up to date"
        exit 0
    fi
fi

echo "🔒 Updating nginx configuration with endpoint access restrictions..."

# Backup existing config
cp /etc/nginx/sites-available/ideal-tribble /etc/nginx/sites-available/ideal-tribble.backup.$(date +%Y%m%d_%H%M%S) 2>/dev/null || true
echo "✅ Backed up existing nginx config"

# Create new secure configuration
echo "$EXPECTED_CONFIG" > /etc/nginx/sites-available/ideal-tribble

echo "✅ Updated nginx configuration"

# Test configuration
echo "🧪 Testing nginx configuration..."
nginx -t

if [ $? -eq 0 ]; then
    echo "✅ Nginx configuration test passed"
    
    # Reload nginx
    systemctl reload nginx
    echo "✅ Nginx reloaded successfully"
    
    echo ""
    echo "🎉 Nginx security update completed!"
    echo ""
    echo "📋 Endpoint access summary:"
    echo "  Public:    /health, /slack/*, /grafana/*"
    echo "  Internal:  /fetch, /process, /clear, /members, /matches, /leaderboard, /test, /metrics"
    echo "  Blocked:   All other endpoints (404)"
    echo ""
    echo "🧪 Test internal endpoint restriction:"
    echo "  curl https://wally-api.utiger.dk/members  # Should return 403 Forbidden"
    echo "  curl http://localhost:8080/members        # Should work from server"
    
else
    echo "❌ Nginx configuration test failed!"
    echo "   Restoring backup..."
    cp /etc/nginx/sites-available/ideal-tribble.backup.* /etc/nginx/sites-available/ideal-tribble
    systemctl reload nginx
    echo "   Backup restored"
    exit 1
fi