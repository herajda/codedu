#!/bin/sh
set -euo pipefail

: "${POSTGRES_HOST:?POSTGRES_HOST is required}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"

# Ensure pg_dump is available even under the limited cron PATH.
export PATH="/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin:${PATH:-}"

# Allow overriding backup target directories via env vars, but default to mounted volumes.
LOCAL_BACKUP_DIR="${LOCAL_BACKUP_DIR:-/backups/local}"
SECONDARY_BACKUP_DIR="${SECONDARY_BACKUP_DIR:-/backups/gcs}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"

log() {
  printf "%s [db-backup] %s\n" "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$*"
}

TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"
ARCHIVE_NAME="${POSTGRES_DB}_${TIMESTAMP}.sql.gz"
WORK_FILE="/tmp/${ARCHIVE_NAME}"

export PGPASSWORD="${POSTGRES_PASSWORD}"

log "Starting backup for database '${POSTGRES_DB}'."

if ! command -v pg_dump >/dev/null 2>&1; then
  log "pg_dump not found on PATH; aborting backup run."
  exit 1
fi

pg_dump \
  -h "${POSTGRES_HOST}" \
  -p "${POSTGRES_PORT}" \
  -U "${POSTGRES_USER}" \
  "${POSTGRES_DB}" \
  | gzip -9 > "${WORK_FILE}"

for DIR in "${LOCAL_BACKUP_DIR}" "${SECONDARY_BACKUP_DIR}"; do
  [ -n "${DIR}" ] || continue
  mkdir -p "${DIR}"
  cp "${WORK_FILE}" "${DIR}/${ARCHIVE_NAME}"
  log "Stored ${ARCHIVE_NAME} in ${DIR}."
done

rm -f "${WORK_FILE}"

prune_old_backups() {
  TARGET_DIR="$1"
  [ -n "${TARGET_DIR}" ] && [ -d "${TARGET_DIR}" ] || return 0

  OLD_FILES="$(find "${TARGET_DIR}" -type f -mtime +"${RETENTION_DAYS}" -name "${POSTGRES_DB}_*.sql.gz" -print || true)"
  [ -n "${OLD_FILES}" ] || return 0

  printf "%s\n" "${OLD_FILES}" | while IFS= read -r OLD_FILE; do
    rm -f "${OLD_FILE}" && log "Pruned ${OLD_FILE}."
  done || true
}

prune_old_backups "${LOCAL_BACKUP_DIR}"
prune_old_backups "${SECONDARY_BACKUP_DIR}"

log "Backup completed."
