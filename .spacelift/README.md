# Spacelift Configuration

This directory contains configuration for Spacelift infrastructure management.

## Setup Instructions

### 1. Create Stack in Spacelift

1. Go to your Spacelift dashboard
2. Click **"Create Stack"**
3. Choose **"GitHub"** as source
4. Select your repository: `mauv0809/ideal-tribble`
5. Set **Project root**: `terraform/hetzner`
6. Set **Branch**: `main`
7. Enable **"Autodeploy"**

### 2. Configure Environment Variables

In the Spacelift stack settings, add these environment variables:

| Variable | Value | Secret? | Description |
|----------|-------|---------|-------------|
| `HCLOUD_TOKEN` | `your-hetzner-api-token` | ✅ Yes | Hetzner Cloud API token |
| `SSH_PUBLIC_KEY` | `ssh-rsa AAAAB3...` | ❌ No | SSH public key for server access |
| `TF_VAR_server_name` | `ideal-tribble` | ❌ No | Server name (optional) |
| `TF_VAR_domain` | `utiger.dk` | ❌ No | Your domain name |

### 3. GitHub Secrets

For the GitHub Actions workflow, configure these secrets:

| Secret | Value | Description |
|--------|-------|-------------|
| `SERVER_IP` | `1.2.3.4` | Server IP (set after first deployment) |
| `SSH_PRIVATE_KEY` | `-----BEGIN OPENSSH PRIVATE KEY-----` | SSH private key |

### 4. First Deployment

1. **Trigger via Spacelift**: Click "Deploy" in the Spacelift UI
2. **Trigger via GitHub**: Push to `main` branch
3. **Manual**: Use `spacelift stack deploy` CLI command

### 5. After First Deployment

1. **Get server IP** from Spacelift outputs
2. **Update GitHub secret** `SERVER_IP` with the actual IP
3. **Configure DNS**: Point `wally-api.utiger.dk` to the server IP

## Stack Management

### Via Spacelift UI
- **Plan**: Review changes before applying
- **Deploy**: Apply infrastructure changes
- **Destroy**: Tear down infrastructure (be careful!)

### Via CLI
```bash
# Install Spacelift CLI
spacelift version

# Deploy stack
spacelift stack deploy --id ideal-tribble-hetzner

# Check status
spacelift stack show --id ideal-tribble-hetzner
```

### Via GitHub Actions
- **Push to main**: Automatic deployment
- **Pull Request**: Plan preview in comments

## Outputs

After deployment, these outputs will be available:

- `server_ip`: Public IP of the Hetzner server
- `server_name`: Name of the server  
- `ssh_command`: SSH command to connect
- `dns_instructions`: DNS setup instructions