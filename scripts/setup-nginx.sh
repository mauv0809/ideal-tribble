#!/bin/bash
# setup-nginx.sh - Configure nginx with SSL support for ideal-tribble

set -e

DOMAIN="wally-api.utiger.dk"
APP_DIR="/opt/ideal-tribble"

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

# Check if SSL certificate already exists
if certbot certificates 2>/dev/null | grep -q "Certificate Name: $DOMAIN"; then
    echo "🔒 SSL certificate for $DOMAIN already exists"
    
    # Update nginx config to use HTTPS
    if ! grep -q "listen 443 ssl" /etc/nginx/sites-available/ideal-tribble; then
        echo "🔧 Adding HTTPS configuration to nginx..."
        
        # Create HTTPS version of the config
        cat > /etc/nginx/sites-available/ideal-tribble << 'EOF'
# HTTP server - redirect to HTTPS
server {
    listen 80;
    server_name wally-api.utiger.dk;
    return 301 https://$server_name$request_uri;
}

# HTTPS server
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
    echo "⚠️  No SSL certificate found for $DOMAIN"
    echo "🚀 Attempting to obtain SSL certificate..."
    
    # Try to obtain SSL certificate
    if command -v certbot >/dev/null 2>&1; then
        # Check if domain resolves to this server
        DOMAIN_IP=$(dig +short $DOMAIN 2>/dev/null || echo "")
        SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || echo "")
        
        if [ "$DOMAIN_IP" = "$SERVER_IP" ]; then
            echo "✅ Domain $DOMAIN resolves to this server ($SERVER_IP)"
            echo "🔒 Obtaining SSL certificate..."
            
            if certbot --nginx -d $DOMAIN --non-interactive --agree-tos --email admin@utiger.dk; then
                echo "✅ SSL certificate obtained successfully"
                echo "🔧 Certificate will be auto-renewed via cron"
            else
                echo "⚠️  SSL certificate setup failed"
                echo "📋 You can manually run: sudo certbot --nginx -d $DOMAIN"
            fi
        else
            echo "⚠️  Domain $DOMAIN does not resolve to this server"
            echo "   Domain IP: $DOMAIN_IP"
            echo "   Server IP: $SERVER_IP" 
            echo "📋 Please update DNS and run: sudo certbot --nginx -d $DOMAIN"
        fi
    else
        echo "⚠️  certbot not available"
        echo "📋 Install certbot and run: sudo certbot --nginx -d $DOMAIN"
    fi
fi

echo ""
echo "🎉 Nginx setup completed!"
echo ""
echo "📊 Configuration summary:"
echo "  Domain: $DOMAIN"
echo "  HTTP: Port 80 (redirects to HTTPS if SSL enabled)"
echo "  HTTPS: Port 443 (if SSL certificate exists)"
echo "  Public endpoints: /health, /slack/*, /grafana/*"
echo "  Internal endpoints: /fetch, /process, /clear, /members, /matches, /leaderboard, /test, /metrics"
echo ""
echo "🧪 Test endpoints:"
echo "  curl http://localhost/health                    # Should work"
echo "  curl https://$DOMAIN/health                     # Should work (if SSL enabled)"
echo "  curl https://$DOMAIN/members                    # Should return 403 Forbidden"
echo ""
echo "🔍 Check status:"
echo "  nginx -t                    # Test configuration"
echo "  systemctl status nginx      # Check service status"
echo "  certbot certificates        # Check SSL certificates"