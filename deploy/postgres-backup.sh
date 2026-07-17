#!/usr/bin/env bash
# Postgres backup script for chess404.
# Designed for Railway cron job or manual invocation.
#
# Usage:
#   ./deploy/postgres-backup.sh                  # uses DATABASE_URL env var
#   DATABASE_URL="postgres://..." ./deploy/postgres-backup.sh
#
# Environment variables:
#   DATABASE_URL       — Postgres connection string (required)
#   BACKUP_DIR         — output directory (default: ./backups)
#   BACKUP_RETENTION   — days to keep backups (default: 30)
#   AWS_S3_BUCKET      — if set, uploads to S3-compatible storage
#   RAILWAY_SERVICE_ID — if set, used for naming the backup file
#
# Install in Railway as a cron service:
#   railway cron create --schedule "0 6 * * *" --cmd "./deploy/postgres-backup.sh"

set -euo pipefail

DATABASE_URL="${DATABASE_URL:-}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
BACKUP_RETENTION="${BACKUP_RETENTION:-30}"
TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
SERVICE_TAG="${RAILWAY_SERVICE_ID:-chess404}"

if [ -z "$DATABASE_URL" ]; then
  echo "ERROR: DATABASE_URL is not set"
  exit 1
fi

mkdir -p "$BACKUP_DIR"

DB_NAME=$(echo "$DATABASE_URL" | sed -E 's|.*/([^?]+).*|\1|')
FILENAME="${SERVICE_TAG}_${DB_NAME}_${TIMESTAMP}.sql.gz"
FILEPATH="${BACKUP_DIR}/${FILENAME}"

echo "Backing up $DB_NAME → $FILEPATH ..."

pg_dump "$DATABASE_URL" --no-owner --no-acl | gzip > "$FILEPATH"

FILESIZE=$(stat -c%s "$FILEPATH" 2>/dev/null || stat -f%z "$FILEPATH" 2>/dev/null || echo "?")
echo "Backup complete: $FILESIZE bytes"

# Upload to S3-compatible storage if bucket is configured
if [ -n "${AWS_S3_BUCKET:-}" ]; then
  echo "Uploading to s3://${AWS_S3_BUCKET}/postgres/ ..."
  aws s3 cp "$FILEPATH" "s3://${AWS_S3_BUCKET}/postgres/${FILENAME}" --only-show-errors
  echo "Upload complete"
fi

# Prune old backups
echo "Pruning backups older than ${BACKUP_RETENTION} days ..."
find "$BACKUP_DIR" -name "${SERVICE_TAG}_*.sql.gz" -mtime +"$BACKUP_RETENTION" -delete
echo "Prune complete"

echo "Done: $FILEPATH"
