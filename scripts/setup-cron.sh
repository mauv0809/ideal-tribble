#!/bin/bash
# setup-cron.sh - Configure cron jobs for Hetzner VPS deployment

set -e

echo "Setting up cron jobs for ideal-tribble..."

# Create cron jobs file
cat << 'EOF' > /tmp/ideal-tribble-cron
# Ideal Tribble Scheduled Jobs
# Fetch matches from Playtomic every hour
0 * * * * curl -X POST http://localhost:8080/fetch >> /var/log/ideal-tribble-fetch.log 2>&1

# Process matches 5 minutes after fetch
5 * * * * curl -X POST http://localhost:8080/process >> /var/log/ideal-tribble-process.log 2>&1

# Rotate logs weekly (optional)
0 0 * * 0 find /var/log -name "ideal-tribble-*.log" -type f -mtime +7 -delete

EOF

# Install the cron jobs
crontab /tmp/ideal-tribble-cron

echo "Cron jobs installed successfully:"
crontab -l

# Create log directory
sudo mkdir -p /var/log
sudo chmod 755 /var/log

echo "Cron setup complete!"
echo "Logs will be written to:"
echo "  - /var/log/ideal-tribble-fetch.log"
echo "  - /var/log/ideal-tribble-process.log"