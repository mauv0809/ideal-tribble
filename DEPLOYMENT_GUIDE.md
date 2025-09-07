# Deployment Guide - Spacelift + Hetzner

This guide walks you through deploying ideal-tribble using Spacelift for infrastructure management and Hetzner Cloud for hosting.

## Prerequisites

✅ **Spacelift account** connected to GitHub repo  
✅ **Hetzner Cloud account** and API token  
✅ **SSH key pair** for server access  
✅ **Domain name** (e.g., utiger.dk)  

## Step 1: Configure Spacelift Stack

### In Spacelift Dashboard:

1. **Create New Stack**:
   - Source: GitHub repository
   - Repository: `mauv0809/ideal-tribble`
   - Project root: `terraform/hetzner`
   - Branch: `main`
   - Enable: **Autodeploy**

2. **Environment Variables**:
   ```
   HCLOUD_TOKEN = your-hetzner-api-token (secret ✅)
   SSH_PUBLIC_KEY = ssh-rsa AAAAB3NzaC1yc2E... (not secret)
   TF_VAR_domain = utiger.dk (not secret)
   ```

3. **Save and Deploy** the stack

## Step 2: Configure GitHub Secrets

In your GitHub repository settings → Secrets:

```
SERVER_IP = (get from Spacelift outputs after first deployment)
SSH_PRIVATE_KEY = -----BEGIN OPENSSH PRIVATE KEY-----
                  your-private-key-here
                  -----END OPENSSH PRIVATE KEY-----
```

## Step 3: First Deployment

### Option A: Via Spacelift UI
1. Go to your stack in Spacelift
2. Click **"Deploy"**
3. Review plan and confirm
4. Note the `server_ip` output

### Option B: Via GitHub
1. Push code to `main` branch
2. Spacelift will auto-deploy infrastructure
3. GitHub Actions will deploy application

## Step 4: Server Setup

After infrastructure deployment:

1. **Get server IP** from Spacelift outputs
2. **Update GitHub secret** `SERVER_IP`
3. **SSH to server**:
   ```bash
   ssh root@<server-ip>
   ```

4. **Configure environment** (if not done automatically):
   ```bash
   cd /opt/ideal-tribble
   ./scripts/setup-secrets.sh
   ./scripts/install-systemd-service.sh
   ./scripts/setup-cron.sh
   ```

5. **Edit environment file**:
   ```bash
   sudo nano /opt/ideal-tribble/.env
   ```
   Add your actual credentials:
   - Slack bot token
   - Slack channel ID  
   - Slack signing secret
   - Turso database credentials

6. **Start services**:
   ```bash
   sudo systemctl start ideal-tribble
   sudo systemctl status ideal-tribble
   ```

## Step 5: DNS Configuration

Point your domain to the server:

```
A Record: wally-api.utiger.dk → <server-ip>
```

Wait for DNS propagation (5-30 minutes).

## Step 6: SSL Certificate

On the server, run:
```bash
sudo certbot --nginx -d wally-api.utiger.dk
```

Follow prompts to get free SSL certificate.

## Step 7: Test Deployment

1. **Health check**: `curl https://wally-api.utiger.dk/health`
2. **Metrics**: `curl https://wally-api.utiger.dk/metrics`
3. **Configure Slack** with your new webhook URL

## Ongoing Deployment

### Automatic (Recommended)
- **Push to main** → Infrastructure + application deploy automatically
- **Pull requests** → Get plan previews in Spacelift

### Manual Options
- **Spacelift UI**: Deploy button for infrastructure
- **SSH deployment**: `./scripts/deploy-to-hetzner.sh <server-ip>`

## Monitoring & Logs

```bash
# Service status
sudo systemctl status ideal-tribble

# Application logs
sudo journalctl -u ideal-tribble -f

# Cron job logs  
tail -f /var/log/ideal-tribble-*.log

# Nginx logs
sudo tail -f /var/log/nginx/access.log
```

## Cost Summary

- **Hetzner VPS**: €3.29/month
- **Spacelift**: Free tier (500 resources)
- **Domain + SSL**: €0 (already owned + Let's Encrypt)
- **Total**: **€3.29/month** (vs €41/month on GCP!)

## Troubleshooting

### Service Won't Start
```bash
sudo journalctl -u ideal-tribble -n 50
```

### DNS Issues
```bash
nslookup wally-api.utiger.dk
```

### SSL Issues
```bash
sudo certbot certificates
sudo nginx -t
```

### Database Issues
Check `.env` file permissions and Turso credentials.

---

🎉 **Deployment Complete!** Your Slack bot is now running on affordable, predictable infrastructure!