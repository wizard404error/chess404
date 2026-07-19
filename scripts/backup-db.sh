#!/bin/bash
# Daily database backup script
# Usage: ./scripts/backup-db.sh <service-name> <db-type> <db-path> <s3-bucket>
set -euo pipefail

SERVICE=$1
DB_TYPE=$2
DB_PATH=$3
S3_BUCKET=${4:-}
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/${SERVICE}_${DB_TYPE}_${TIMESTAMP}"

mkdir -p "$BACKUP_DIR"

case "$DB_TYPE" in
  sqlite)
    echo "Backing up SQLite: $DB_PATH"
    sqlite3 "$DB_PATH" ".backup '${BACKUP_FILE}.sqlite'"
    gzip "${BACKUP_FILE}.sqlite"
    BACKUP_FILE="${BACKUP_FILE}.sqlite.gz"
    ;;
  postgres)
    echo "Backing up Postgres: $DB_PATH"
    pg_dump "$DB_PATH" | gzip > "${BACKUP_FILE}.sql.gz"
    BACKUP_FILE="${BACKUP_FILE}.sql.gz"
    ;;
  *)
    echo "Unknown db type: $DB_TYPE"
    exit 1
    ;;
esac

echo "Backup created: $BACKUP_FILE ($(du -h "$BACKUP_FILE" | cut -f1))"

# Retention: remove backups older than RETENTION_DAYS
find "$BACKUP_DIR" -name "${SERVICE}_${DB_TYPE}_*" -mtime "+${RETENTION_DAYS}" -delete

# Upload to S3 if bucket specified
if [ -n "$S3_BUCKET" ]; then
  aws s3 cp "$BACKUP_FILE" "s3://${S3_BUCKET}/backups/${SERVICE}/$(basename "$BACKUP_FILE")"
  echo "Uploaded to s3://${S3_BUCKET}/backups/${SERVICE}/"
fi

echo "Backup complete"
