#!/usr/bin/env bash
set -euo pipefail

root="${CODEXPPP_ROOT:-/opt/codexppp}"
backup_root="${CODEXPPP_BACKUP_ROOT:-${root}/backups/automatic}"
retention_days="${CODEXPPP_BACKUP_RETENTION_DAYS:-14}"
lock_file="${backup_root}/.backup.lock"

mkdir -p "${backup_root}"
exec 9>"${lock_file}"
flock -n 9 || exit 0

stamp="$(date -u +%Y%m%d-%H%M%S)"
database_tmp="${backup_root}/.${stamp}.dump.tmp"
database_out="${backup_root}/${stamp}-codexppp.dump"
source_tmp="${backup_root}/.${stamp}-runtime.tar.gz.tmp"
source_out="${backup_root}/${stamp}-runtime.tar.gz"

cleanup() {
  rm -f -- "${database_tmp}" "${source_tmp}"
}
trap cleanup EXIT

docker exec codexppp-postgres pg_dump -U codexppp -d codexppp -Fc >"${database_tmp}"
test -s "${database_tmp}"

runtime_paths=(
  backend
  deploy/docker-compose.yml
  scripts/backup-server.sh
)
for optional_path in deploy/nginx deploy/systemd; do
  if [[ -e "${root}/${optional_path}" ]]; then
    runtime_paths+=("${optional_path}")
  fi
done
tar -C "${root}" -czf "${source_tmp}" "${runtime_paths[@]}"
test -s "${source_tmp}"

mv -- "${database_tmp}" "${database_out}"
mv -- "${source_tmp}" "${source_out}"

find "${backup_root}" -maxdepth 1 -type f \( -name '*-codexppp.dump' -o -name '*-runtime.tar.gz' \) -mtime "+${retention_days}" -delete
