# Terraform Infrastructure

This directory contains the infrastructure-as-code configurations for ideal-tribble.

## Automated Deployment with Terraform Cloud & GitHub Actions

Infrastructure is **automatically managed** through the unified deployment pipeline. Manual Terraform operations are not required for normal deployment.

### How It Works

1. **Terraform Cloud Integration**: The configuration uses Terraform Cloud for state management and remote execution
2. **GitHub Actions Trigger**: Infrastructure is deployed automatically when pushing to the `main` branch
3. **Unified Pipeline**: Infrastructure deployment runs in parallel with application building and testing

### Configuration

The `hetzner/` directory contains:
- **main.tf**: Hetzner Cloud server, firewall, and SSH key configuration
- **cloud-init.yml**: Server initialization script for nginx, directories, and base setup
- **variables.tf**: Input variables for configuration
- **outputs.tf**: Server IP output for application deployment

### Infrastructure Components

- **Server**: Hetzner CX22 (2 vCPU, 4GB RAM) running Ubuntu 22.04
- **Firewall**: Configured for HTTP/HTTPS/SSH/App access
- **SSH Access**: Automated key deployment for GitHub Actions
- **Cloud-Init**: Automated server setup with nginx reverse proxy

### Required Secrets (GitHub Repository Settings)

The deployment requires these GitHub secrets:

| Secret | Purpose |
|--------|---------|
| `TF_API_TOKEN` | Terraform Cloud API access |
| `HCLOUD_TOKEN` | Hetzner Cloud API access |
| `SSH_PRIVATE_KEY` | Server deployment access |
| `SSH_PUBLIC_KEY` | Server configuration |

### Manual Operations (If Needed)

For manual infrastructure management:

```bash
cd hetzner/

# Initialize (requires TF_API_TOKEN)
terraform init

# View current state
terraform show

# Apply changes (requires HCLOUD_TOKEN and SSH keys as environment variables)
export TF_VAR_hcloud_token="your-token"
export TF_VAR_ssh_public_key="your-public-key"
terraform apply
```

## Removed Infrastructure

The following GCP resources have been removed and replaced:

- ~~`cloud_run.tf`~~ → Hetzner VPS
- ~~`iam.tf`~~ → Direct server access
- ~~`pubsub.tf`~~ → SQLite job queue
- ~~`scheduler.tf`~~ → Cron jobs
- ~~`gateway.tf`~~ → Nginx reverse proxy

**Cost savings: 92% reduction** (€41/month → €3.29/month)