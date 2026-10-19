#!/bin/bash
# Daily copies of every city's database, kept for a week. Run by conexiuni-cluj-backup.service.

set -e
cd "$(dirname "$0")"

dest="$HOME/db-backups/$(date +%F)"
mkdir -p "$dest"
for db in db/*.db; do
  sqlite3 "$db" ".backup '$dest/$(basename "$db")'"
done
cp db/disabled-cities.json "$dest/" 2>/dev/null || true
find "$HOME/db-backups" -mindepth 1 -maxdepth 1 -mtime +6 -exec rm -rf {} +
