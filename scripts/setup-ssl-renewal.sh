#!/bin/bash
# setup-ssl-renewal.sh - Configure automatic SSL certificate renewal

set -e

if ! command -v certbot >/dev/null 2>&1; then
    echo "⚠️  certbot not installed, skipping SSL auto-renewal setup"
    exit 0
fi

echo "Setting up SSL certificate auto-renewal..."

# Add certbot renewal to cron if not already present
if ! crontab -l 2>/dev/null | grep -q "certbot renew"; then
    (crontab -l 2>/dev/null; echo "# Auto-renew SSL certificates twice daily") | crontab -
    (crontab -l 2>/dev/null; echo "0 */12 * * * certbot renew --quiet --nginx && systemctl reload nginx") | crontab -
    echo "✅ SSL auto-renewal configured"
else
    echo "✅ SSL auto-renewal already configured"
fi

echo "📋 SSL certificates will be checked for renewal twice daily"
echo "🔍 Check current certificates: certbot certificates"
echo "🧪 Test renewal: certbot renew --dry-run"