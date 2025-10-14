#!/bin/sh
set -euo pipefail

: "${CRON_SCHEDULE:=0 */3 * * *}"
: "${CRON_LOG_TARGET:=/proc/1/fd/1}"

log() {
  printf "%s [db-backup] %s\n" "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$*"
}

cat <<EOF >/etc/crontabs/root
SHELL=/bin/sh
PATH=/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
${CRON_SCHEDULE} /bin/sh /opt/run_backup.sh >> ${CRON_LOG_TARGET} 2>&1
EOF

log "Cron schedule set to '${CRON_SCHEDULE}'. Starting initial backup run."
/bin/sh /opt/run_backup.sh || log "Initial backup failed (subsequent runs will retry)."

log "Starting crond."
exec crond -f -l 2
