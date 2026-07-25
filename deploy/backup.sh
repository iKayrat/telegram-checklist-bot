#!/bin/sh
# Daily SQLite backup — real money (штрафы) lives in this file, see
# bot-architecture.md §7. Add to root's crontab, e.g.:
#   0 3 * * * /home/pi/checklist-bot/backup.sh
set -eu

DB_PATH="/home/pi/checklist-bot/data/bot.db"
BACKUP_DIR="/home/pi/checklist-bot/backups"
KEEP_DAYS=30

mkdir -p "$BACKUP_DIR"
cp "$DB_PATH" "$BACKUP_DIR/bot-$(date +%Y-%m-%d).db"
find "$BACKUP_DIR" -name 'bot-*.db' -mtime "+$KEEP_DAYS" -delete
