#!/bin/bash
set -e

# Создать директорию для архивов
mkdir -p /var/lib/postgresql/archives
chown postgres:postgres /var/lib/postgresql/archives
chmod 755 /var/lib/postgresql/archives

# Создать пользователя для репликации (если нужно)
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Пользователь app_user уже должен существовать
    ALTER USER app_user REPLICATION;
EOSQL

echo "Репликация настроена для пользователя $POSTGRES_USER"
echo "Директория архивов создана: /var/lib/postgresql/archives"