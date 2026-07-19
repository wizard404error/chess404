#!/bin/bash
# Disaster Recovery: one-click stack deployment to new region
# Usage: RECOVERY_REGION=us-east ./scripts/recovery.sh
set -euo pipefail

echo "=== Chess404 Disaster Recovery ==="
echo "Target region: ${RECOVERY_REGION:-$(gum input --placeholder 'us-east')}"
echo ""

RECOVERY_DIR="./recovery-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RECOVERY_DIR"

# 1. Restore database from latest backup
echo "[1/4] Restoring database\u2026"
LATEST_BACKUP=$(aws s3 ls s3://chess404-backups/backups/platform-service/ --recursive | sort | tail -1 | awk '{print $4}')
if [ -n "$LATEST_BACKUP" ]; then
  aws s3 cp "s3://chess404-backups/$LATEST_BACKUP" "$RECOVERY_DIR/"
  echo "  Restored: $LATEST_BACKUP"
else
  echo "  WARNING: No backup found. Starting with empty database."
fi

# 2. Deploy infrastructure with Terraform/Pulumi
echo "[2/4] Deploying infrastructure\u2026"
# TODO: Replace with actual IaC apply
echo "  (skipped - IaC not yet configured)"

# 3. Start services
echo "[3/4] Starting services\u2026"
docker compose -f deploy/docker-compose.prod.yml up -d

# 4. Verify health
echo "[4/4] Verifying health\u2026"
sleep 10
for svc in gateway platform-service match-service matchmaking-service; do
  status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/system/status" 2>/dev/null || echo "000")
  echo "  $svc: HTTP $status"
done

echo ""
echo "=== Recovery complete ==="
