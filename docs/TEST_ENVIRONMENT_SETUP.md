# Test Environment Setup for API Gateway

This document outlines the complete setup needed for a separate testing environment to safely test the new API Gateway architecture without affecting production.

## Overview

**Goal**: Create an isolated cloud testing environment that mirrors production but uses separate resources to avoid any impact on the live Slack bot and user experience.

**Architecture**: Same GCP project, different resource names and endpoints.

## Infrastructure Components

### 🏗️ **Cloud Resources (New)**
- **Cloud Run Service**: `ideal-tribble-test`
- **API Gateway**: `wally-api-test`  
- **Load Balancer**: `wally-lb-test`
- **Cloud Armor Policy**: `wally-security-policy-test`
- **Domain**: `test-wally-api.your-domain.com`
- **Static IP**: `wally-external-ip-test`

### 🗄️ **Database (New Turso Database)**
```bash
# Commands to create test database
turso db create ideal-tribble-test
turso db tokens create ideal-tribble-test
```

**Reasoning**: 
- Clean test data isolation
- Safe migration testing  
- No risk to production data
- Can reset/seed test data as needed

## Slack Setup

### 🤖 **New Slack Test Bot (Required)**

**Create a separate Slack app** with:
- Different bot token
- Different signing secret
- Separate permissions/scopes

### 📢 **New Test Channel (Required)**
- Create new Slack channel: `#padel-test` or similar
- Install test bot in this channel only
- Keep production bot in original channel

**Reasoning**: Prevents test messages from polluting production channel where real users see notifications.

## Required Secrets

### 🔐 **New Secrets to Create**
```bash
# Database secrets
gcloud secrets create TURSO_PRIMARY_URL_TEST --replication-policy="automatic"
gcloud secrets create TURSO_AUTH_TOKEN_TEST --replication-policy="automatic"

# Slack test bot secrets  
gcloud secrets create SLACK_BOT_TOKEN_TEST --replication-policy="automatic"
gcloud secrets create SLACK_CHANNEL_ID_TEST --replication-policy="automatic"  
gcloud secrets create SLACK_SIGNING_SECRET_TEST --replication-policy="automatic"

# Database name (can reuse same value or create test-specific)
gcloud secrets create DB_NAME_TEST --replication-policy="automatic"
```

### ♻️ **Reusable Secrets**
- `TENANT_ID` - Can reuse existing (if same Playtomic account)

## Terraform Structure

### 📁 **Recommended File Structure**
```
terraform/
├── environments/
│   ├── prod/
│   │   └── terraform.tfvars
│   └── test/  
│       └── terraform.tfvars
├── main.tf
├── variables.tf
├── gateway.tf
├── cloud_run.tf
└── ... (other existing files)
```

### ⚙️ **Test Environment Variables**
Key differences in `terraform/environments/test/terraform.tfvars`:
```hcl
# Service naming
service_name = "ideal-tribble-test"
domain = "test-wally-api.your-domain.com"

# Test-specific secrets
secret_names = [
  "DB_NAME_TEST",
  "SLACK_BOT_TOKEN_TEST", 
  "SLACK_CHANNEL_ID_TEST",
  "SLACK_SIGNING_SECRET_TEST",
  "TENANT_ID",  # Can reuse
  "TURSO_PRIMARY_URL_TEST",
  "TURSO_AUTH_TOKEN_TEST"
]

# Conservative scheduling (avoid conflicts)
fetch_cron_schedule = "0 */3 * * *"    # Every 3 hours
process_cron_schedule = "10 */3 * * *"  # Every 3 hours, offset

# Traffic settings
latest_traffic_percent = 100  # All traffic to latest for testing
```

## Testing Workflow

### 🧪 **Step-by-Step Testing Process**

1. **Setup Phase**:
   - Create new Turso test database
   - Create new Slack test bot and channel
   - Create all required secrets in GCP
   - Set up Terraform environment configuration

2. **Deploy Phase**:
   - Deploy test environment: `terraform apply -var-file="environments/test/terraform.tfvars"`
   - Configure DNS for test domain
   - Update Slack test bot webhook URLs

3. **Validation Phase**:
   - Test all Slack commands in test channel
   - Verify API Gateway rate limiting works
   - Verify Cloud Armor security policies 
   - Test SSL certificates and HTTPS
   - Monitor logs and metrics

4. **Production Deployment**:
   - Once test environment validates successfully
   - Deploy same architecture to production
   - Monitor rollout carefully

## Benefits of This Approach

✅ **Complete isolation**: No risk to production users or data  
✅ **Realistic testing**: Same GCP project, similar resource constraints  
✅ **Easy cleanup**: Can destroy test environment when done  
✅ **Reusable**: Keep test environment for future changes  
✅ **Safe migrations**: Test database schema changes safely  
✅ **Performance testing**: Can load test without affecting users  

## DNS Configuration

### 🌐 **Required DNS Records**
```
test-wally-api.your-domain.com  A  <test-static-ip>
```

## Monitoring & Observability

### 📊 **Separate Dashboards**
- Cloud Run logs: Filter by service name `ideal-tribble-test`
- API Gateway logs: Filter by gateway `wally-api-test` 
- Cloud Armor logs: Filter by policy `wally-security-policy-test`

## Cleanup

### 🧹 **When Testing Complete**
```bash
# Destroy test environment
terraform destroy -var-file="environments/test/terraform.tfvars"

# Clean up secrets (optional)
gcloud secrets delete TURSO_PRIMARY_URL_TEST
gcloud secrets delete TURSO_AUTH_TOKEN_TEST
gcloud secrets delete SLACK_BOT_TOKEN_TEST
gcloud secrets delete SLACK_CHANNEL_ID_TEST
gcloud secrets delete SLACK_SIGNING_SECRET_TEST

# Clean up Turso test database
turso db destroy ideal-tribble-test
```

## Notes

- Keep this document updated as requirements change
- Consider keeping test environment running for ongoing development
- Test environment can also be used for staging deployments
- Monitor costs - test environment should be minimal overhead