#!/bin/bash
# setup-nginx.sh - Configure nginx with SSL support for ideal-tribble
# Uses Cloudflare DNS-01 challenge for SSL certificates (works with proxied domains)

set -e

API_DOMAIN="wally-api.utiger.dk"
WEB_DOMAIN="wally.utiger.dk"
APP_DIR="/opt/ideal-tribble"
CLOUDFLARE_CREDENTIALS="/etc/letsencrypt/cloudflare.ini"

echo "🌐 Setting up nginx configuration..."

# Copy nginx configuration from repo to sites-available
if [ -f "$APP_DIR/nginx/ideal-tribble.conf" ]; then
    echo "📝 Copying nginx configuration..."
    cp "$APP_DIR/nginx/ideal-tribble.conf" /etc/nginx/sites-available/ideal-tribble
    echo "✅ Nginx configuration copied"
else
    echo "❌ nginx/ideal-tribble.conf not found in app directory"
    exit 1
fi

# Enable the site if not already enabled
if [ ! -f /etc/nginx/sites-enabled/ideal-tribble ]; then
    echo "🔗 Enabling nginx site..."
    ln -s /etc/nginx/sites-available/ideal-tribble /etc/nginx/sites-enabled/
    echo "✅ Nginx site enabled"
else
    echo "✅ Nginx site already enabled"
fi

# Disable default nginx site if it exists
if [ -f /etc/nginx/sites-enabled/default ]; then
    echo "🚫 Disabling default nginx site..."
    rm -f /etc/nginx/sites-enabled/default
    echo "✅ Default site disabled"
fi

# Test nginx configuration
echo "🧪 Testing nginx configuration..."
if nginx -t; then
    echo "✅ Nginx configuration test passed"
    systemctl reload nginx
    echo "✅ Nginx reloaded"
else
    echo "❌ Nginx configuration test failed"
    exit 1
fi

# Check if SSL certificates already exist for both domains
API_SSL_EXISTS=$(certbot certificates 2>/dev/null | grep -q "Certificate Name: $API_DOMAIN" && echo "yes" || echo "no")
WEB_SSL_EXISTS=$(certbot certificates 2>/dev/null | grep -q "Certificate Name: $WEB_DOMAIN" && echo "yes" || echo "no")

if [ "$API_SSL_EXISTS" = "yes" ] && [ "$WEB_SSL_EXISTS" = "yes" ]; then
    echo "🔒 SSL certificates for both domains already exist"

    # Update nginx config to use HTTPS
    if ! grep -q "listen 443 ssl" /etc/nginx/sites-available/ideal-tribble; then
        echo "🔧 Adding HTTPS configuration to nginx..."

        # Create HTTPS version of the config
        cat > /etc/nginx/sites-available/ideal-tribble << 'EOF'
# HTTP servers - redirect to HTTPS
server {
    listen 80;
    server_name wally-api.utiger.dk;
    return 301 https://$server_name$request_uri;
}

server {
    listen 80;
    server_name wally.utiger.dk;
    return 301 https://$server_name$request_uri;
}

# HTTPS API server (wally-api.utiger.dk)
server {
    listen 443 ssl http2;
    server_name wally-api.utiger.dk;

    ssl_certificate /etc/letsencrypt/live/wally-api.utiger.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/wally-api.utiger.dk/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

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
        proxy_pass http://localhost:3000;
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
}

# HTTPS Web dashboard (wally.utiger.dk)
server {
    listen 443 ssl http2;
    server_name wally.utiger.dk;

    ssl_certificate /etc/letsencrypt/live/wally.utiger.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/wally.utiger.dk/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    # All web UI routes
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF

        # Test and reload
        if nginx -t; then
            systemctl reload nginx
            echo "✅ HTTPS configuration added and nginx reloaded"
        else
            echo "❌ HTTPS configuration failed"
            exit 1
        fi
    else
        echo "✅ HTTPS already configured"
    fi

else
    echo "🔍 Checking SSL certificate status..."

    # Check if Cloudflare credentials exist
    if [ ! -f "$CLOUDFLARE_CREDENTIALS" ]; then
        echo "⚠️  Cloudflare credentials not found at $CLOUDFLARE_CREDENTIALS"
        echo "📋 Create the file with your Cloudflare API token:"
        echo "   echo 'dns_cloudflare_api_token = YOUR_TOKEN' | sudo tee $CLOUDFLARE_CREDENTIALS"
        echo "   sudo chmod 600 $CLOUDFLARE_CREDENTIALS"
        exit 1
    fi

    # Try to obtain SSL certificates for domains that don't have them
    for DOMAIN in $API_DOMAIN $WEB_DOMAIN; do
        if ! certbot certificates 2>/dev/null | grep -q "Certificate Name: $DOMAIN"; then
            echo "⚠️  No SSL certificate found for $DOMAIN"

            if command -v certbot >/dev/null 2>&1; then
                echo "🔒 Obtaining SSL certificate for $DOMAIN via Cloudflare DNS-01..."

                if certbot certonly \
                    --dns-cloudflare \
                    --dns-cloudflare-credentials "$CLOUDFLARE_CREDENTIALS" \
                    --dns-cloudflare-propagation-seconds 30 \
                    -d "$DOMAIN" \
                    --non-interactive \
                    --agree-tos \
                    --email admin@utiger.dk; then
                    echo "✅ SSL certificate obtained for $DOMAIN"
                else
                    echo "⚠️  SSL certificate setup failed for $DOMAIN"
                    echo "📋 Check Cloudflare API token permissions (Zone:DNS:Edit)"
                fi
            else
                echo "⚠️  certbot not available"
                echo "📋 Install certbot and python3-certbot-dns-cloudflare"
            fi
        else
            echo "✅ SSL certificate for $DOMAIN already exists"
        fi
    done

    # After obtaining certs, update nginx to use HTTPS
    # Re-check if both certs now exist
    API_SSL_EXISTS=$(certbot certificates 2>/dev/null | grep -q "Certificate Name: $API_DOMAIN" && echo "yes" || echo "no")
    WEB_SSL_EXISTS=$(certbot certificates 2>/dev/null | grep -q "Certificate Name: $WEB_DOMAIN" && echo "yes" || echo "no")

    if [ "$API_SSL_EXISTS" = "yes" ] && [ "$WEB_SSL_EXISTS" = "yes" ]; then
        if ! grep -q "listen 443 ssl" /etc/nginx/sites-available/ideal-tribble; then
            echo "🔧 Adding HTTPS configuration to nginx..."
            # Re-run this script to apply the HTTPS config
            exec "$0"
        fi
    fi
fi

echo ""
echo "🎉 Nginx setup completed!"
echo ""
echo "📊 Configuration summary:"
echo "  API Domain: $API_DOMAIN"
echo "  Web Domain: $WEB_DOMAIN"
echo "  HTTP: Port 80 (redirects to HTTPS if SSL enabled)"
echo "  HTTPS: Port 443 (if SSL certificates exist)"
echo ""
echo "  API endpoints ($API_DOMAIN):"
echo "    Public: /health, /slack/*, /grafana/*"
echo "    Internal: /fetch, /process, /clear, /members, /matches, /leaderboard, /test, /metrics"
echo ""
echo "  Web dashboard ($WEB_DOMAIN):"
echo "    All routes proxied to application"
echo ""
echo "🧪 Test endpoints:"
echo "  curl https://$API_DOMAIN/health     # API health check"
echo "  curl https://$WEB_DOMAIN/login      # Web login page"
echo ""
echo "🔍 Check status:"
echo "  nginx -t                    # Test configuration"
echo "  systemctl status nginx      # Check service status"
echo "  certbot certificates        # Check SSL certificates"