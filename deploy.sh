#!/bin/bash

set -euo pipefail

# ----------------------
# CONFIGURATION
# ----------------------
PROJECT_ID="friendly-cubist-465916-e6"
REGION="europe-west3"
SERVICE_NAME="ideal-tribble"
TERRAFORM_DIR="./terraform"

# ----------------------
# FETCH STABLE REVISION
# ----------------------
echo "Fetching current stable Cloud Run revision..."
STABLE_REVISION=$(gcloud run services describe "$SERVICE_NAME" \
  --project="$PROJECT_ID" \
  --region="$REGION" \
  --format="value(status.traffic[0].revisionName)")

if [[ -z "$STABLE_REVISION" ]]; then
  echo "❌ Failed to fetch stable revision. Is the service deployed yet?"
  exit 1
fi

echo "✅ Current stable revision: $STABLE_REVISION"

IMAGE=$(gcloud run revisions describe "$STABLE_REVISION" \
  --project="$PROJECT_ID" \
  --region="$REGION" \
  --format="value(spec.containers[0].image)")

if [[ -z "$IMAGE" ]]; then
  echo "❌ Failed to fetch stable image. Is the service deployed yet?"
  exit 1
fi

echo "✅ Current stable image: $IMAGE"

# ----------------------
# RUN TERRAFORM APPLY
# ----------------------
echo "🔧 Running Terraform apply..."

if [ ! -d "./terraform/.terraform" ]; then
  echo "🔧 Running terraform init..."
  terraform -chdir=./terraform init \
    -backend-config="bucket=$TERRAFORM_STATE_BUCKET"
else
  echo "✅ Terraform already initialized. Skipping init."
fi

terraform -chdir="$TERRAFORM_DIR" apply -auto-approve \
  -var="gcp_project_id=$PROJECT_ID" \
  -var="gcp_region=$REGION" \
  -var="stable_revision=$STABLE_REVISION" \
  -var="image_name=$IMAGE"


echo "🚀 Deployment complete."
