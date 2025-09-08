#!/bin/bash
# validate-env.sh - Validate that all required environment variables are set

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Environment file path
ENV_FILE="${1:-/opt/ideal-tribble/.env}"

echo "Validating environment variables from: $ENV_FILE"
echo "=================================================="

# Check if env file exists
if [[ ! -f "$ENV_FILE" ]]; then
    echo -e "${RED}ERROR: Environment file not found at $ENV_FILE${NC}"
    exit 1
fi

# Load environment file
set -a
source "$ENV_FILE"
set +a

# Define required variables
REQUIRED_VARS=(
    # Database
    "DB_NAME"
    "TURSO_PRIMARY_URL"
    "TURSO_AUTH_TOKEN"
    
    # Slack
    "SLACK_BOT_TOKEN"
    "SLACK_CHANNEL_ID"
    "SLACK_SIGNING_SECRET"
    
    # Playtomic
    "TENANT_ID"
    
    # Server
    "PORT"
    
    # OpenTelemetry
    "OTEL_EXPORTER_OTLP_ENDPOINT"
    "APP_ENV"
    "APP_VERSION"
    
    # Observability
    "GRAFANA_ADMIN_PASSWORD"
)

# Track validation results
missing_vars=()
placeholder_vars=()
valid_vars=()

echo "Checking required environment variables:"
echo ""

# Check each required variable
for var in "${REQUIRED_VARS[@]}"; do
    if [[ -z "${!var}" ]]; then
        missing_vars+=("$var")
        echo -e "  ${RED}✗${NC} $var: NOT SET"
    elif [[ "${!var}" == *"your-"* || "${!var}" == *"_here"* || "${!var}" == *"changeme"* ]]; then
        placeholder_vars+=("$var")
        echo -e "  ${YELLOW}⚠${NC} $var: PLACEHOLDER VALUE (${!var})"
    else
        valid_vars+=("$var")
        echo -e "  ${GREEN}✓${NC} $var: SET"
    fi
done

echo ""
echo "=================================================="
echo "Validation Summary:"
echo "  Valid variables: ${#valid_vars[@]}"
echo "  Placeholder values: ${#placeholder_vars[@]}"
echo "  Missing variables: ${#missing_vars[@]}"

# Display detailed results
if [[ ${#missing_vars[@]} -gt 0 ]]; then
    echo ""
    echo -e "${RED}Missing variables:${NC}"
    for var in "${missing_vars[@]}"; do
        echo "  - $var"
    done
fi

if [[ ${#placeholder_vars[@]} -gt 0 ]]; then
    echo ""
    echo -e "${YELLOW}Variables with placeholder values:${NC}"
    for var in "${placeholder_vars[@]}"; do
        echo "  - $var (current: ${!var})"
    done
fi

# Additional validation checks
echo ""
echo "Additional validation checks:"

# Check Slack token format
if [[ -n "$SLACK_BOT_TOKEN" && ! "$SLACK_BOT_TOKEN" =~ ^xoxb- ]]; then
    echo -e "  ${YELLOW}⚠${NC} SLACK_BOT_TOKEN should start with 'xoxb-'"
fi

# Check Turso URL format
if [[ -n "$TURSO_PRIMARY_URL" && ! "$TURSO_PRIMARY_URL" =~ ^libsql:// ]]; then
    echo -e "  ${YELLOW}⚠${NC} TURSO_PRIMARY_URL should start with 'libsql://'"
fi

# Check APP_ENV values
if [[ -n "$APP_ENV" && ! "$APP_ENV" =~ ^(development|staging|production)$ ]]; then
    echo -e "  ${YELLOW}⚠${NC} APP_ENV should be one of: development, staging, production"
fi

# Check OTEL endpoint format
if [[ -n "$OTEL_EXPORTER_OTLP_ENDPOINT" && ! "$OTEL_EXPORTER_OTLP_ENDPOINT" =~ :[0-9]+$ ]]; then
    echo -e "  ${YELLOW}⚠${NC} OTEL_EXPORTER_OTLP_ENDPOINT should include port (e.g., localhost:4317)"
fi

echo ""

# Final result
if [[ ${#missing_vars[@]} -gt 0 ]]; then
    echo -e "${RED}VALIDATION FAILED: Missing required environment variables${NC}"
    exit 1
elif [[ ${#placeholder_vars[@]} -gt 0 ]]; then
    echo -e "${YELLOW}VALIDATION WARNING: Some variables have placeholder values${NC}"
    echo "Please update these before deploying to production."
    exit 2
else
    echo -e "${GREEN}VALIDATION PASSED: All required variables are set${NC}"
    exit 0
fi