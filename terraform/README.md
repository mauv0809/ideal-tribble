# Terraform Infrastructure

This directory contains the infrastructure-as-code configurations for ideal-tribble.

## Hetzner Cloud Deployment

The `hetzner/` directory contains Terraform configuration for deploying to Hetzner Cloud VPS.

### Prerequisites

1. **Hetzner Cloud account** and API token
2. **SSH key pair** for server access
3. **Domain** configured (e.g., utiger.dk)

### Deployment

```bash
cd hetzner/

# Initialize Terraform
terraform init

# Review the plan
terraform plan

# Apply the configuration
terraform apply
```

### Variables

Create a `terraform.tfvars` file:

```hcl
hcloud_token    = "your-hetzner-api-token"
ssh_public_key  = "ssh-rsa AAAAB3... your-public-key"
server_name     = "ideal-tribble"
domain          = "utiger.dk"
```

### After Deployment

1. Note the server IP from `terraform output`
2. Configure DNS: `wally-api.utiger.dk -> <server-ip>`
3. Deploy application: `../scripts/deploy-to-hetzner.sh <server-ip>`

## Removed Infrastructure

The following GCP resources have been removed and replaced:

- ~~`cloud_run.tf`~~ → Hetzner VPS
- ~~`iam.tf`~~ → Direct server access
- ~~`pubsub.tf`~~ → SQLite job queue
- ~~`scheduler.tf`~~ → Cron jobs
- ~~`gateway.tf`~~ → Nginx reverse proxy

**Cost savings: 92% reduction** (€41/month → €3.29/month)