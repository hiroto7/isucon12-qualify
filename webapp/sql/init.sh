#!/bin/sh

set -ex
cd `dirname $0`

ISUCON_DB_HOST=${ISUCON_DB_HOST:-127.0.0.1}
ISUCON_DB_PORT=${ISUCON_DB_PORT:-3306}
ISUCON_DB_USER=${ISUCON_DB_USER:-isucon}
ISUCON_DB_PASSWORD=${ISUCON_DB_PASSWORD:-isucon}
ISUCON_DB_NAME=${ISUCON_DB_NAME:-isuports}

# MySQLを初期化
mysql -u"$ISUCON_DB_USER" \
		-p"$ISUCON_DB_PASSWORD" \
		--host "$ISUCON_DB_HOST" \
		--port "$ISUCON_DB_PORT" \
		"$ISUCON_DB_NAME" < init.sql

# Existing practice databases also receive the covering visit-history index.
# It persists across initialization; avoid rebuilding millions of rows each run.
mysql_admin() {
  mysql -u"$ISUCON_DB_USER" -p"$ISUCON_DB_PASSWORD" \
    --host "$ISUCON_DB_HOST" --port "$ISUCON_DB_PORT" "$ISUCON_DB_NAME" "$@"
}
if [ "$(mysql_admin -N -s -e "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='visit_history' AND index_name='visit_competition_player'")" = 0 ]; then
  mysql_admin -e 'ALTER TABLE visit_history ADD INDEX visit_competition_player (tenant_id, competition_id, player_id, created_at)'
fi

# SQLiteのデータベースを初期化
# Index the restoration templates once without changing their rows. This avoids
# rebuilding the large tenant's index inside every 30-second initialization.
for db in ../../initial_data/*.db; do
  sqlite3 "$db" 'CREATE INDEX IF NOT EXISTS score_competition_player ON player_score (competition_id, player_id, row_num DESC);'
done
rm -f ../tenant_db/*.db
cp -r ../../initial_data/*.db ../tenant_db/
